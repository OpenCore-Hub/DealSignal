// Package link implements smart-link creation, permission checks and public access.
package link

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/netip"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

// Beginner starts a database transaction.
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Service handles smart links.
type Service struct {
	queries         *db.Queries
	pool            Beginner
	redisClient     *redis.Client
	mailer          mailer.Mailer
	viewerBaseURL   string
	emailSem        chan struct{} // limits concurrent email sends (bounded goroutines)
}

// NewService creates a link service.
func NewService(q *db.Queries, pool Beginner, r *redis.Client, m mailer.Mailer, viewerBaseURL string) *Service {
	return &Service{
		queries: q, pool: pool, redisClient: r, mailer: m, viewerBaseURL: viewerBaseURL,
		emailSem: make(chan struct{}, 8), // cap concurrent email goroutines
	}
}

var (
	ErrDocumentNotReady     = errors.New("document is not ready")
	ErrInvalidPermission    = errors.New("invalid permission configuration")
	ErrLinkNotFound         = errors.New("link not found")
	ErrLinkExpired          = errors.New("link expired")
	ErrLinkRevoked          = errors.New("link revoked")
	ErrLinkDisabled         = errors.New("link disabled")
	ErrLinkMaxAccessReached = errors.New("link max access reached")
	ErrRequiresEmail        = errors.New("email required")
	ErrRequiresPassword     = errors.New("password required")
	ErrInvalidPassword      = errors.New("invalid password")
	ErrWhitelistDenied      = errors.New("email not in whitelist")
	ErrRequiresNDA          = errors.New("nda agreement required")
	ErrRequiresEmailCode    = errors.New("email verification code required")
	ErrInvalidEmailCode     = errors.New("invalid email verification code")
	ErrNotFoundInWorkspace  = errors.New("link not found in workspace")
)

// CreateLinkRequest is the input for creating a link.
type CreateLinkRequest struct {
	DocumentID               string
	Name                     string
	PermissionType           string
	RequireEmailVerification bool
	RequirePassword          bool
	RequireNDA               bool
	AllowedEmails            []string
	AllowedDomains           []string
	Password                 string
	ExpiresAt                *time.Time
	MaxAccessCount           *int32
	DownloadEnabled          bool
	WatermarkEnabled         bool
	ContactIDs               []string
}

// CreateLink creates a smart link for a document.
func (s *Service) CreateLink(ctx context.Context, userID, workspaceID string, req CreateLinkRequest) (db.Link, error) {
	workspaceUUID := pgUUID(workspaceID)
	userUUID := pgUUID(userID)

	docID, err := uuid.Parse(req.DocumentID)
	if err != nil {
		return db.Link{}, errors.New("invalid document id")
	}

	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Link{}, errors.New("document not found")
		}
		return db.Link{}, fmt.Errorf("get document: %w", err)
	}
	if doc.Status != "ready" {
		return db.Link{}, ErrDocumentNotReady
	}

	requireEmailVerification, requirePassword, requireNDA, emails, domains, perm, legacy, err := normalizeSecurityConfig(req)
	if err != nil {
		return db.Link{}, err
	}

	if requireEmailVerification && len(req.ContactIDs) == 0 {
		return db.Link{}, fmt.Errorf("%w: at least one contact is required for email verification", ErrInvalidPermission)
	}

	var passwordHash pgtype.Text
	if requirePassword {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return db.Link{}, fmt.Errorf("hash password: %w", err)
		}
		passwordHash = pgtype.Text{String: string(hash), Valid: true}
	}

	token, err := generateToken()
	if err != nil {
		return db.Link{}, fmt.Errorf("generate token: %w", err)
	}

	name := pgtype.Text{String: req.Name, Valid: req.Name != ""}
	expiresAt := pgtype.Timestamptz{Valid: req.ExpiresAt != nil}
	if req.ExpiresAt != nil {
		expiresAt.Time = *req.ExpiresAt
	}
	maxAccess := pgtype.Int4{Valid: req.MaxAccessCount != nil}
	if req.MaxAccessCount != nil {
		maxAccess.Int32 = *req.MaxAccessCount
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Link{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	link, err := qtx.CreateLink(ctx, db.CreateLinkParams{
		TenantID:         doc.TenantID,
		WorkspaceID:      workspaceUUID,
		DocumentID:       doc.ID,
		PublicToken:      token,
		Name:             name,
		PermissionType:   perm,
		AllowedEmails:    mustMarshalJSON(emails),
		AllowedDomains:   mustMarshalJSON(domains),
		PasswordHash:     passwordHash,
		ExpiresAt:        expiresAt,
		MaxAccessCount:   maxAccess,
		DownloadEnabled:  req.DownloadEnabled,
		WatermarkEnabled: req.WatermarkEnabled,
		RequireEmail:             requireEmailVerification && legacy,
		RequireEmailVerification: requireEmailVerification,
		RequirePassword:          requirePassword,
		RequireNda:               requireNDA,
		Status:                   "active",
		CreatedBy:                userUUID,
	})
	if err != nil {
		return db.Link{}, fmt.Errorf("create link: %w", err)
	}

	var emailCodes []struct {
		email string
		code  string
	}

	if requireEmailVerification {
		contacts, err := qtx.ListContactsByWorkspace(ctx, workspaceUUID)
		if err != nil {
			return db.Link{}, fmt.Errorf("list contacts: %w", err)
		}
		contactMap := make(map[string]db.Contact, len(contacts))
		for _, c := range contacts {
			contactMap[uuid.UUID(c.ID.Bytes).String()] = c
		}
		for _, cid := range req.ContactIDs {
			contact, ok := contactMap[cid]
			if !ok {
				return db.Link{}, fmt.Errorf("%w: contact %s not found in workspace", ErrInvalidPermission, cid)
			}
			code, err := generateNumericCode(6)
			if err != nil {
				return db.Link{}, fmt.Errorf("generate access code: %w", err)
			}
			contactUUID := pgUUID(cid)
			if !contactUUID.Valid {
				return db.Link{}, fmt.Errorf("invalid contact id: %s", cid)
			}
			if err := qtx.CreateLinkContact(ctx, db.CreateLinkContactParams{
				LinkID:     link.ID,
				ContactID:  contactUUID,
				AccessCode: code,
			}); err != nil {
				return db.Link{}, fmt.Errorf("create link contact: %w", err)
			}
			emailCodes = append(emailCodes, struct {
				email string
				code  string
			}{email: contact.Email.String, code: code})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Link{}, fmt.Errorf("commit transaction: %w", err)
	}

	// Send verification emails after the transaction commits so the
	// link_contact records are durable. Run asynchronously so SMTP latency does
	// not block the create-link response. Bounded by a semaphore to avoid
	// unbounded goroutine creation when links are created with many contacts.
	linkURL := publicLinkURL(s.viewerBaseURL, token)
	for _, ec := range emailCodes {
		email, code := ec.email, ec.code
		// Try the semaphore; if full, still send without bound (better than
		// silently dropping the email).
		select {
		case s.emailSem <- struct{}{}:
			go func() {
				defer func() { <-s.emailSem }()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := s.mailer.SendLinkAccessCodeEmail(ctx, email, code, req.Name, linkURL); err != nil {
					logger.ErrorCtx(ctx, "failed to send link access code email", err,
						logger.Attr("email_local", localPart(email)),
					)
				}
			}()
		default:
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := s.mailer.SendLinkAccessCodeEmail(ctx, email, code, req.Name, linkURL); err != nil {
					logger.ErrorCtx(ctx, "failed to send link access code email", err,
						logger.Attr("email_local", localPart(email)),
					)
				}
			}()
		}
	}
	return link, nil
}

// AccessRequest is the input for public access.
type AccessRequest struct {
	Email     string
	EmailCode string
	Password  string
	NDAAgreed bool
	IP        string
	UA        string
}

// AccessResult is returned after a successful access check.
type AccessResult struct {
	Link      db.Link
	VisitorID string
	Email     string
}

// Access validates a public token and returns the link if access is granted.
func (s *Service) Access(ctx context.Context, token string, req AccessRequest) (AccessResult, error) {
	link, err := s.queries.GetLinkByPublicToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AccessResult{}, ErrLinkNotFound
		}
		return AccessResult{}, fmt.Errorf("get link: %w", err)
	}

	switch link.Status {
	case "deleted":
		return AccessResult{}, ErrLinkNotFound
	case "disabled", "revoked":
		return AccessResult{}, ErrLinkDisabled
	}
	if link.ExpiresAt.Valid && link.ExpiresAt.Time.Before(time.Now()) {
		return AccessResult{}, ErrLinkExpired
	}
	if link.MaxAccessCount.Valid && link.AccessCount >= link.MaxAccessCount.Int32 {
		return AccessResult{}, ErrLinkMaxAccessReached
	}

	// Legacy permission_type values may not have the boolean flag set.
	requiresEmailVerification := link.RequireEmailVerification || link.PermissionType == "email_required" || link.PermissionType == "whitelist" || link.PermissionType == "nda"
	requiresPassword := link.RequirePassword || link.PermissionType == "password"
	requiresNDA := link.RequireNda || link.PermissionType == "nda"
	hasWhitelist := jsonArrayNotEmpty(link.AllowedEmails) || jsonArrayNotEmpty(link.AllowedDomains)

	// Modern email-verification links (RequireEmailVerification=true, RequireEmail=false)
	// identify the recipient by the access code alone. Legacy gates and whitelist
	// checks still require an explicit email. NDA combined with modern email
	// verification uses the verified contact email for agreement records, so the
	// visitor does not need to re-enter it.
	modernEmailVerification := link.RequireEmailVerification && !link.RequireEmail
	requiresEmail := link.RequireEmail || hasWhitelist || (requiresNDA && !modernEmailVerification)
	if requiresEmail {
		if strings.TrimSpace(req.Email) == "" {
			return AccessResult{}, ErrRequiresEmail
		}
	}
	if hasWhitelist {
		if !isAllowed(req.Email, link.AllowedEmails, link.AllowedDomains) {
			return AccessResult{}, ErrWhitelistDenied
		}
	}
	if requiresEmailVerification {
		if strings.TrimSpace(req.EmailCode) == "" {
			return AccessResult{}, ErrRequiresEmailCode
		}
	}
	if requiresPassword {
		if req.Password == "" {
			return AccessResult{}, ErrRequiresPassword
		}
	}
	if requiresNDA {
		if !req.NDAAgreed {
			return AccessResult{}, ErrRequiresNDA
		}
	}

	// Verify credentials only after all required gates are present. The email
	// verification code is marked as used only when every gate passes so that
	// a missing password or NDA agreement does not consume the code.
	var verifiedContact *db.GetLinkContactByEmailRow
	var verifiedEmail string
	if requiresEmailVerification {
		lc, err := s.verifyLinkContactCode(ctx, token, req.Email, req.EmailCode, modernEmailVerification)
		if err != nil {
			if errors.Is(err, ErrInvalidEmailCode) || errors.Is(err, ErrRequiresEmailCode) {
				return AccessResult{}, err
			}
			return AccessResult{}, fmt.Errorf("verify email code: %w", err)
		}
		verifiedContact = lc
		verifiedEmail = strings.TrimSpace(lc.ContactEmail.String)
	}

	// For modern code-only verification, use the contact email from the database
	// for visitor identity and NDA records.
	emailForRecords := req.Email
	if emailForRecords == "" && verifiedEmail != "" {
		emailForRecords = verifiedEmail
	}
	if requiresPassword {
		if err := bcrypt.CompareHashAndPassword([]byte(link.PasswordHash.String), []byte(req.Password)); err != nil {
			return AccessResult{}, ErrInvalidPassword
		}
	}

	visitorID := makeVisitorID(emailForRecords, req.UA)

	if requiresNDA {
		ipAddr, _ := netip.ParseAddr(req.IP)
		var ip *netip.Addr
		if ipAddr.IsValid() {
			ip = &ipAddr
		}
		_, ndaErr := s.queries.CreateLinkNDAAgreement(ctx, db.CreateLinkNDAAgreementParams{
		TenantID:    link.TenantID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
		VisitorID:   pgtype.Text{String: visitorID, Valid: visitorID != ""},
		Email:       pgtype.Text{String: emailForRecords, Valid: emailForRecords != ""},
		Ip:          ip,
		UserAgent:   pgtype.Text{String: req.UA, Valid: req.UA != ""},
	})
	if ndaErr != nil {
		logger.ErrorCtx(ctx, "create link NDA agreement failed", ndaErr,
			logger.Attr("link_id", uuid.UUID(link.ID.Bytes).String()),
		)
	}
	}

	if verifiedContact != nil {
		if err := s.queries.MarkLinkContactCodeUsed(ctx, verifiedContact.ID); err != nil {
			return AccessResult{}, fmt.Errorf("mark code used: %w", err)
		}
	}

	return AccessResult{Link: link, VisitorID: visitorID, Email: emailForRecords}, nil
}

// SendEmailVerificationCode resends the access code for a contact.
// It returns no error if the email is not associated with the link to avoid leaking addresses.
func (s *Service) SendEmailVerificationCode(ctx context.Context, token, email, viewerBaseURL string) error {
	link, err := s.queries.GetLinkByPublicToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLinkNotFound
		}
		return fmt.Errorf("get link: %w", err)
	}

	if !link.RequireEmailVerification {
		return nil
	}
	if link.Status != "active" {
		return nil
	}
	if link.ExpiresAt.Valid && link.ExpiresAt.Time.Before(time.Now()) {
		return nil
	}

	email = strings.TrimSpace(email)
	if email == "" {
		return ErrRequiresEmail
	}

	lc, err := s.queries.GetLinkContactByEmail(ctx, db.GetLinkContactByEmailParams{
		PublicToken: token,
		Email:       pgtype.Text{String: email, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Silently succeed to avoid leaking valid addresses.
			return nil
		}
		return fmt.Errorf("get link contact: %w", err)
	}
	if lc.UsedAt.Valid {
		// Code already consumed; do not resend.
		return nil
	}

	allowed, err := s.allowEmailCodeSend(ctx, token, email)
	if err != nil {
		return fmt.Errorf("rate limit check: %w", err)
	}
	if !allowed {
		return nil
	}

	linkURL := publicLinkURL(viewerBaseURL, link.PublicToken)
	if err := s.mailer.SendLinkAccessCodeEmail(ctx, email, lc.AccessCode, link.Name.String, linkURL); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

func publicLinkURL(baseURL, token string) string {
	if baseURL == "" {
		return "/l/" + token
	}
	return strings.TrimRight(baseURL, "/") + "/l/" + token
}

func resendRateLimitKey(token, email string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return fmt.Sprintf("link:resend:ratelimit:%s:%s", token, hex.EncodeToString(h[:]))
}

func (s *Service) allowEmailCodeSend(ctx context.Context, token, email string) (bool, error) {
	if s.redisClient == nil {
		return false, errors.New("redis is required for email code resend rate limiting")
	}
	return s.redisClient.AllowEmailCodeSend(ctx, resendRateLimitKey(token, email), 3, time.Minute)
}

func (s *Service) verifyLinkContactCode(ctx context.Context, token, email, code string, modern bool) (*db.GetLinkContactByEmailRow, error) {
	code = strings.TrimSpace(code)
	if modern && strings.TrimSpace(email) == "" {
		lc, err := s.queries.GetLinkContactByCode(ctx, db.GetLinkContactByCodeParams{
			PublicToken: token,
			AccessCode:  code,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrInvalidEmailCode
			}
			return nil, fmt.Errorf("get link contact by code: %w", err)
		}
		if lc.UsedAt.Valid {
			return nil, ErrInvalidEmailCode
		}
		return &db.GetLinkContactByEmailRow{
			ID:           lc.ID,
			LinkID:       lc.LinkID,
			ContactID:    lc.ContactID,
			AccessCode:   lc.AccessCode,
			CodeSentAt:   lc.CodeSentAt,
			UsedAt:       lc.UsedAt,
			CreatedAt:    lc.CreatedAt,
			ContactEmail: lc.ContactEmail,
			ContactName:  lc.ContactName,
		}, nil
	}

	lc, err := s.queries.GetLinkContactByEmail(ctx, db.GetLinkContactByEmailParams{
		PublicToken: token,
		Email:       pgtype.Text{String: strings.TrimSpace(email), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidEmailCode
		}
		return nil, fmt.Errorf("get link contact: %w", err)
	}
	if lc.UsedAt.Valid {
		return nil, ErrInvalidEmailCode
	}
	if !strings.EqualFold(lc.AccessCode, code) {
		return nil, ErrInvalidEmailCode
	}
	return &lc, nil
}

func generateNumericCode(length int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, length)
	for i := range b {
		raw := make([]byte, 1)
		if _, err := rand.Read(raw); err != nil {
			return "", err
		}
		b[i] = digits[int(raw[0])%len(digits)]
	}
	return string(b), nil
}

// GetByID returns a link scoped to a workspace.
func (s *Service) GetByID(ctx context.Context, linkID, workspaceID string) (db.Link, error) {
	id, err := uuid.Parse(linkID)
	if err != nil {
		return db.Link{}, errors.New("invalid link id")
	}
	link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Link{}, ErrNotFoundInWorkspace
		}
		return db.Link{}, fmt.Errorf("get link by id: %w", err)
	}
	return link, nil
}

// List returns all non-deleted links in a workspace.
func (s *Service) List(ctx context.Context, workspaceID string) ([]db.Link, error) {
	return s.queries.ListLinksByWorkspace(ctx, pgUUID(workspaceID))
}

// ListByDocument returns links for a specific document.
func (s *Service) ListByDocument(ctx context.Context, workspaceID, documentID string) ([]db.Link, error) {
	docID, err := uuid.Parse(documentID)
	if err != nil {
		return nil, errors.New("invalid document id")
	}
	return s.queries.ListLinksByDocument(ctx, db.ListLinksByDocumentParams{
		WorkspaceID: pgUUID(workspaceID),
		DocumentID:  pgtype.UUID{Bytes: docID, Valid: true},
	})
}

// UpdateStatus updates a link's status (e.g. active / revoked).
func (s *Service) UpdateStatus(ctx context.Context, linkID, workspaceID, status string) (db.Link, error) {
	id, err := uuid.Parse(linkID)
	if err != nil {
		return db.Link{}, errors.New("invalid link id")
	}
	if status != "active" && status != "revoked" {
		return db.Link{}, errors.New("invalid status")
	}
	link, err := s.queries.UpdateLinkStatus(ctx, db.UpdateLinkStatusParams{
		Status:      status,
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Link{}, ErrNotFoundInWorkspace
		}
		return db.Link{}, fmt.Errorf("update link status: %w", err)
	}
	return link, nil
}

// Delete removes a link from listings. It first attempts a soft delete by marking
// the status as deleted; if the schema has not yet been migrated to allow the
// deleted status, it falls back to a hard delete so the operation succeeds.
func (s *Service) Delete(ctx context.Context, linkID, workspaceID string) error {
	id, err := uuid.Parse(linkID)
	if err != nil {
		return errors.New("invalid link id")
	}
	params := db.DeleteLinkParams{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		WorkspaceID: pgUUID(workspaceID),
	}
	rows, err := s.queries.DeleteLink(ctx, params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			rows, err = s.queries.HardDeleteLink(ctx, db.HardDeleteLinkParams(params))
		}
		if err != nil {
			return fmt.Errorf("delete link: %w", err)
		}
	}
	if rows == 0 {
		return ErrNotFoundInWorkspace
	}
	return nil
}

// ListAccessLogs returns access events for a link, including both raw access logs
// and per-page views with their durations.
func (s *Service) ListAccessLogs(ctx context.Context, linkID, workspaceID string) ([]db.ListAccessLogsByLinkRow, error) {
	id, err := uuid.Parse(linkID)
	if err != nil {
		return nil, errors.New("invalid link id")
	}
	// Verify link exists in workspace.
	if _, err := s.GetByID(ctx, linkID, workspaceID); err != nil {
		return nil, err
	}
	return s.queries.ListAccessLogsByLink(ctx, db.ListAccessLogsByLinkParams{
		LinkID: pgtype.UUID{Bytes: id, Valid: true},
		Limit:  200,
	})
}

func normalizePermission(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return "public"
	}
	return p
}

// normalizeSecurityConfig reconciles the legacy single permission_type with the
// independent boolean flags sent by the new UI. It returns the resolved booleans,
// normalized email/domain lists, a display permission_type, and whether the config
// was derived from a legacy permission_type (which still requires the visitor to
// enter their email).
func normalizeSecurityConfig(req CreateLinkRequest) (requireEmailVerification, requirePassword, requireNDA bool, emails, domains []string, perm string, legacy bool, err error) {
	requireEmailVerification = req.RequireEmailVerification
	requirePassword = req.RequirePassword
	requireNDA = req.RequireNDA
	emails = req.AllowedEmails
	domains = req.AllowedDomains
	perm = normalizePermission(req.PermissionType)
	legacyPerm := perm

	// If the caller only sent the legacy permission_type, derive the flags.
	if !requireEmailVerification && !requirePassword && !requireNDA && len(emails) == 0 && len(domains) == 0 {
		switch perm {
		case "email_required":
			requireEmailVerification = true
			legacy = true
		case "whitelist":
			requireEmailVerification = true
			legacy = true
		case "password":
			requirePassword = true
		case "nda":
			requireEmailVerification = true
			requireNDA = true
			legacy = true
		}
	}

	// Whitelist and NDA links always require email verification so the visitor
	// identity can be checked.
	if !requireEmailVerification && (len(emails) > 0 || len(domains) > 0 || requireNDA) {
		requireEmailVerification = true
	}

	// Validate that allowed email entries have proper email format.
	for _, e := range emails {
		if strings.TrimSpace(e) == "" {
			continue
		}
		if _, err := mail.ParseAddress(e); err != nil {
			return false, false, false, nil, nil, "", false, fmt.Errorf("%w: invalid email in whitelist: %s", ErrInvalidPermission, e)
		}
	}

	if requirePassword && req.Password == "" {
		return false, false, false, nil, nil, "", false, fmt.Errorf("%w: password required", ErrInvalidPermission)
	}

	// Derive a canonical permission_type for display/backward compatibility.
	// Modern email verification is an independent flag; it should not be mapped
	// to the legacy "email_required" permission_type.
	if requirePassword {
		perm = "password"
	} else if requireNDA {
		perm = "nda"
	} else if len(emails) > 0 || len(domains) > 0 {
		perm = "whitelist"
	} else {
		perm = "public"
	}

	// Preserve the legacy display value for clients that explicitly used the
	// old "email_required" permission_type.
	if legacy && legacyPerm == "email_required" {
		perm = "email_required"
	}

	return requireEmailVerification, requirePassword, requireNDA, emails, domains, perm, legacy, nil
}

func jsonArrayNotEmpty(b []byte) bool {
	return len(b) > 0 && string(b) != "[]" && string(b) != "null"
}

func isAllowed(email string, allowedEmails, allowedDomains []byte) bool {
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	email = strings.ToLower(addr.Address)
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	domain := parts[1]

	var emails, domains []string
	_ = json.Unmarshal(allowedEmails, &emails)
	_ = json.Unmarshal(allowedDomains, &domains)

	for _, e := range emails {
		entry := strings.TrimSpace(e)
		if entry == "" {
			continue
		}
		// Treat entries without '@' or with a leading '@' as domains for backward
		// compatibility with UI-created links that may store domains in the emails list.
		if strings.HasPrefix(entry, "@") || !strings.Contains(entry, "@") {
			entry = strings.TrimPrefix(entry, "@")
			if strings.EqualFold(entry, domain) {
				return true
			}
		} else {
			if strings.EqualFold(entry, email) {
				return true
			}
		}
	}
	for _, d := range domains {
		entry := strings.TrimSpace(strings.TrimPrefix(d, "@"))
		if entry != "" && strings.EqualFold(entry, domain) {
			return true
		}
	}
	return false
}

func makeVisitorID(email, ua string) string {
	src := strings.ToLower(strings.TrimSpace(email))
	if src == "" {
		src = ua
	}
	if src == "" {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		return hex.EncodeToString(b)
	}
	sum := sha256.Sum256([]byte(src))
	return hex.EncodeToString(sum[:])[:16]
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// localPart extracts the part before "@" from an email address (for safe logging).
func localPart(email string) string {
	idx := strings.Index(email, "@")
	if idx <= 0 {
		return email
	}
	return email[:idx]
}

func pgUUID(id string) pgtype.UUID {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

func mustMarshalJSON(v []string) []byte {
	if v == nil {
		return []byte("[]")
	}
	b, _ := json.Marshal(v)
	return b
}
