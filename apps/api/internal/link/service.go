// Package link implements smart-link creation, permission checks and public access.
package link

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
)

// Beginner starts a database transaction.
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// emailCode holds an email address and its access code for async delivery.
type emailCode struct {
	email string
	code  string
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
	ErrRequiresNDA          = errors.New("nda agreement required")
	ErrRequiresEmailCode    = errors.New("email verification code required")
	ErrInvalidEmailCode     = errors.New("invalid email verification code")
	ErrNotFoundInWorkspace  = errors.New("link not found in workspace")
)

// ResolvePublicLink validates a public token and returns the active link.
// It checks status, expiry, and max access limits. It is intended for
// public features (e.g. AI Copilot) that already hold a valid link session.
func (s *Service) ResolvePublicLink(ctx context.Context, publicToken string) (db.Link, error) {
	link, err := s.queries.GetLinkByPublicToken(ctx, publicToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Link{}, ErrLinkNotFound
		}
		return db.Link{}, err
	}
	if link.Status == "deleted" {
		return db.Link{}, ErrLinkNotFound
	}
	if link.Status == "disabled" || link.Status == "revoked" {
		return db.Link{}, ErrLinkDisabled
	}
	if link.ExpiresAt.Valid && link.ExpiresAt.Time.Before(time.Now()) {
		return db.Link{}, ErrLinkExpired
	}
	if link.MaxAccessCount.Valid && int32(link.AccessCount) >= link.MaxAccessCount.Int32 {
		return db.Link{}, ErrLinkMaxAccessReached
	}
	return link, nil
}

// CreateLinkRequest is the input for creating a link.
type CreateLinkRequest struct {
	DocumentID               string
	DocumentIDs              []string // Multi-document bundle (takes precedence when non-empty)
	Name                     string
	PermissionType           string
	RequireEmail             bool
	RequireEmailVerification bool
	RequireNDA               bool
	ExpiresAt                *time.Time
	MaxAccessCount           *int32
	DownloadEnabled          bool
	WatermarkEnabled         bool
	AICopilotEnabled         bool
	ContactIDs               []string
}

// UpdateLinkRequest is the input for updating an existing link (full replacement).
type UpdateLinkRequest struct {
	DocumentIDs              []string
	Name                     string
	PermissionType           string
	RequireEmail             bool
	RequireEmailVerification bool
	RequireNDA               bool
	ExpiresAt                *time.Time
	MaxAccessCount           *int32
	DownloadEnabled          bool
	WatermarkEnabled         bool
	AICopilotEnabled         bool
	ContactIDs               []string
}

// CreateLink creates a smart link for one or more documents.
func (s *Service) CreateLink(ctx context.Context, userID, workspaceID string, req CreateLinkRequest) (db.Link, error) {
	workspaceUUID := pgUUID(workspaceID)
	userUUID := pgUUID(userID)

	// Resolve document IDs: use DocumentIDs if provided, else fall back to single DocumentID.
	documentIDs := req.DocumentIDs
	if len(documentIDs) == 0 && req.DocumentID != "" {
		documentIDs = []string{req.DocumentID}
	}
	if len(documentIDs) == 0 {
		return db.Link{}, errors.New("at least one document_id is required")
	}

	// Validate all documents exist and are ready.
	var primaryDocID pgtype.UUID
	for _, did := range documentIDs {
		docUUID, err := uuid.Parse(did)
		if err != nil {
			return db.Link{}, fmt.Errorf("invalid document id: %s", did)
		}
		doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
			ID:          pgtype.UUID{Bytes: docUUID, Valid: true},
			WorkspaceID: workspaceUUID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.Link{}, fmt.Errorf("document not found: %s", did)
			}
			return db.Link{}, fmt.Errorf("get document: %w", err)
		}
		if doc.Status != "ready" {
			return db.Link{}, ErrDocumentNotReady
		}
		if primaryDocID.Bytes == uuidParseNil() {
			primaryDocID = doc.ID
		}
	}

	requireEmail, requireEmailVerification, requireNDA, perm, err := normalizeSecurityConfig(req)
	if err != nil {
		return db.Link{}, err
	}

	if requireEmailVerification && len(req.ContactIDs) == 0 {
		return db.Link{}, fmt.Errorf("%w: at least one contact is required for email verification", ErrInvalidPermission)
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

	// Fetch the primary document to obtain a valid tenant_id for CreateLink.
	primaryDoc, err := qtx.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          primaryDocID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		return db.Link{}, fmt.Errorf("get primary document: %w", err)
	}

	link, err := qtx.CreateLink(ctx, db.CreateLinkParams{
		TenantID:                 primaryDoc.TenantID,
		WorkspaceID:              workspaceUUID,
		DocumentID:               primaryDocID,
		PublicToken:              token,
		Name:                     name,
		PermissionType:           perm,
		ExpiresAt:                expiresAt,
		MaxAccessCount:           maxAccess,
		DownloadEnabled:          req.DownloadEnabled,
		WatermarkEnabled:         req.WatermarkEnabled,
		AiCopilotEnabled:         req.AICopilotEnabled,
		RequireEmail:             requireEmail,
		RequireEmailVerification: requireEmailVerification,
		RequireNda:               requireNDA,
		Status:                   "active",
		CreatedBy:                userUUID,
	})
	if err != nil {
		return db.Link{}, fmt.Errorf("create link: %w", err)
	}

	// Insert link_documents for all document IDs.
	for i, did := range documentIDs {
		docUUID, _ := uuid.Parse(did)
		if err := qtx.CreateLinkDocument(ctx, db.CreateLinkDocumentParams{
			LinkID:     link.ID,
			DocumentID: pgtype.UUID{Bytes: docUUID, Valid: true},
			SortOrder:  int32(i),
		}); err != nil {
			return db.Link{}, fmt.Errorf("create link document %s: %w", did, err)
		}
	}

	var emailCodes []emailCode

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
			emailCodes = append(emailCodes, emailCode{email: contact.Email.String, code: code})
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
	s.sendAccessCodeEmails(ctx, emailCodes, req.Name, linkURL)
	return link, nil
}

// UpdateLink fully replaces a link's document set and security configuration.
func (s *Service) UpdateLink(ctx context.Context, linkID, workspaceID string, req UpdateLinkRequest) (db.Link, error) {
	workspaceUUID := pgUUID(workspaceID)

	// Verify link exists in workspace and is not deleted.
	existing, err := s.GetByID(ctx, linkID, workspaceID)
	if err != nil {
		return db.Link{}, err
	}
	if existing.Status == "deleted" {
		return db.Link{}, ErrNotFoundInWorkspace
	}

	if len(req.DocumentIDs) == 0 {
		return db.Link{}, errors.New("at least one document_id is required")
	}

	// Validate all documents exist and are ready.
	var primaryDocID pgtype.UUID
	for _, did := range req.DocumentIDs {
		docUUID, err := uuid.Parse(did)
		if err != nil {
			return db.Link{}, fmt.Errorf("invalid document id: %s", did)
		}
		doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
			ID:          pgtype.UUID{Bytes: docUUID, Valid: true},
			WorkspaceID: workspaceUUID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.Link{}, fmt.Errorf("document not found: %s", did)
			}
			return db.Link{}, fmt.Errorf("get document: %w", err)
		}
		if doc.Status != "ready" {
			return db.Link{}, ErrDocumentNotReady
		}
		if primaryDocID.Bytes == uuidParseNil() {
			primaryDocID = doc.ID
		}
	}

	createReq := CreateLinkRequest{
		DocumentID:               uuid.UUID(primaryDocID.Bytes).String(),
		DocumentIDs:              req.DocumentIDs,
		Name:                     req.Name,
		PermissionType:           req.PermissionType,
		RequireEmail:             req.RequireEmail,
		RequireEmailVerification: req.RequireEmailVerification,
		RequireNDA:               req.RequireNDA,
		ExpiresAt:                req.ExpiresAt,
		MaxAccessCount:           req.MaxAccessCount,
		DownloadEnabled:          req.DownloadEnabled,
		WatermarkEnabled:         req.WatermarkEnabled,
		AICopilotEnabled:         req.AICopilotEnabled,
		ContactIDs:               req.ContactIDs,
	}

	requireEmail, requireEmailVerification, requireNDA, perm, err := normalizeSecurityConfig(createReq)
	if err != nil {
		return db.Link{}, err
	}

	// Validate contacts for email verification.
	if requireEmailVerification && len(req.ContactIDs) == 0 {
		return db.Link{}, fmt.Errorf("%w: at least one contact is required for email verification", ErrInvalidPermission)
	}

	name := pgtype.Text{String: req.Name, Valid: req.Name != ""}
	if !name.Valid {
		name = existing.Name
	}
	expiresAt := pgtype.Timestamptz{Valid: req.ExpiresAt != nil}
	if req.ExpiresAt != nil {
		expiresAt.Time = *req.ExpiresAt
	} else {
		expiresAt = existing.ExpiresAt
	}
	maxAccess := pgtype.Int4{Valid: req.MaxAccessCount != nil}
	if req.MaxAccessCount != nil {
		maxAccess.Int32 = *req.MaxAccessCount
	} else {
		maxAccess = existing.MaxAccessCount
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Link{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	// Update the link record using the sqlc-generated UpdateLinkFull.
	_, err = qtx.UpdateLinkFull(ctx, db.UpdateLinkFullParams{
		Name:                     name,
		DocumentID:               primaryDocID,
		PermissionType:           perm,
		ExpiresAt:                expiresAt,
		MaxAccessCount:           maxAccess,
		DownloadEnabled:          req.DownloadEnabled,
		WatermarkEnabled:         req.WatermarkEnabled,
		RequireEmail:             requireEmail,
		RequireEmailVerification: requireEmailVerification,
		RequireNda:               requireNDA,
		AiCopilotEnabled:         req.AICopilotEnabled,
		ID:                       existing.ID,
		WorkspaceID:              workspaceUUID,
	})
	if err != nil {
		return db.Link{}, fmt.Errorf("update link: %w", err)
	}

	// Replace all link_documents.
	if err := qtx.DeleteLinkDocumentsByLink(ctx, existing.ID); err != nil {
		return db.Link{}, fmt.Errorf("delete link documents: %w", err)
	}
	for i, did := range req.DocumentIDs {
		docUUID, _ := uuid.Parse(did)
		if err := qtx.CreateLinkDocument(ctx, db.CreateLinkDocumentParams{
			LinkID:     existing.ID,
			DocumentID: pgtype.UUID{Bytes: docUUID, Valid: true},
			SortOrder:  int32(i),
		}); err != nil {
			return db.Link{}, fmt.Errorf("create link document %s: %w", did, err)
		}
	}

	// Fetch existing link contacts before deletion for diff-based email sending.
	// We only send new verification codes to contacts that are being added in
	// this update, not to contacts that were already on the link.
	var existingContactIDs map[string]string // contactID -> accessCode
	if requireEmailVerification {
		existingContacts, _ := qtx.GetLinkContactsByPublicToken(ctx, existing.PublicToken)
		existingContactIDs = make(map[string]string, len(existingContacts))
		for _, lc := range existingContacts {
			existingContactIDs[uuid.UUID(lc.ContactID.Bytes).String()] = lc.AccessCode
		}
	}

	// Replace link_contacts. Always clean up existing; only re-create if email
	// verification is still enabled.
	var emailCodes []emailCode
	if err := qtx.DeleteLinkContactsByLink(ctx, existing.ID); err != nil {
		return db.Link{}, fmt.Errorf("delete link contacts: %w", err)
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
			contactUUID := pgUUID(cid)
			if !contactUUID.Valid {
				return db.Link{}, fmt.Errorf("invalid contact id: %s", cid)
			}

			// Reuse existing access code if the contact was already on this link,
			// and only send a new email to newly added contacts.
			var code string
			if existingCode, existed := existingContactIDs[cid]; existed {
				code = existingCode
			} else {
				var err error
				code, err = generateNumericCode(6)
				if err != nil {
					return db.Link{}, fmt.Errorf("generate access code: %w", err)
				}
				emailCodes = append(emailCodes, emailCode{email: contact.Email.String, code: code})
			}

			if err := qtx.CreateLinkContact(ctx, db.CreateLinkContactParams{
				LinkID:     existing.ID,
				ContactID:  contactUUID,
				AccessCode: code,
			}); err != nil {
				return db.Link{}, fmt.Errorf("create link contact: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Link{}, fmt.Errorf("commit transaction: %w", err)
	}

	// Send verification emails after commit for updated contacts.
	linkURL := publicLinkURL(s.viewerBaseURL, existing.PublicToken)
	s.sendAccessCodeEmails(ctx, emailCodes, req.Name, linkURL)

	// Re-fetch to get the updated record.
	return s.GetByID(ctx, linkID, workspaceID)
}

func uuidParseNil() [16]byte {
	return [16]byte{}
}

// AccessRequest is the input for public access.
type AccessRequest struct {
	Email     string
	EmailCode string
	NDAAgreed bool
	IP        string
	UA        string
}

// AccessResult is returned after a successful access check.
type AccessResult struct {
	Link          db.Link
	VisitorID     string
	Email         string
	EmailVerified bool
	SessionToken  string // refreshed session token for sliding expiry; empty if no session was used
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

	// Use the shared linkSecurityFlags helper (defined in handler.go, same package)
	// to compute gate requirements identically with the handler's response formatting.
	requiresEmail, requiresEmailVerification, requiresNDA := linkSecurityFlags(link)

	if requiresEmail {
		if strings.TrimSpace(req.Email) == "" {
			return AccessResult{}, ErrRequiresEmail
		}
	}
	if requiresEmailVerification {
		if strings.TrimSpace(req.EmailCode) == "" {
			return AccessResult{}, ErrRequiresEmailCode
		}
	}
	if requiresNDA {
		if !req.NDAAgreed {
			return AccessResult{}, ErrRequiresNDA
		}
	}

	var verifiedEmail string
	if requiresEmailVerification {
		lc, err := s.verifyLinkContactCode(ctx, token, "", req.EmailCode, true)
		if err != nil {
			if errors.Is(err, ErrInvalidEmailCode) || errors.Is(err, ErrRequiresEmailCode) {
				return AccessResult{}, err
			}
			return AccessResult{}, fmt.Errorf("verify email code: %w", err)
		}
		verifiedEmail = strings.TrimSpace(lc.ContactEmail.String)
	}

	var emailForRecords string
	if requiresEmailVerification {
		emailForRecords = verifiedEmail
	} else {
		emailForRecords = req.Email
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

	return AccessResult{Link: link, VisitorID: visitorID, Email: emailForRecords, EmailVerified: requiresEmailVerification}, nil
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
	allowed, err := s.allowEmailCodeSend(ctx, token, email)
	if err != nil {
		return fmt.Errorf("rate limit check: %w", err)
	}
	if !allowed {
		return nil
	}

	linkURL := publicLinkURL(viewerBaseURL, link.PublicToken)
	if _, err := s.mailer.SendLinkAccessCodeEmail(ctx, email, lc.AccessCode, link.Name.String, linkURL); err != nil {
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

// checkAccessAttemptRateLimit enforces a sliding-window rate limit on access
// attempts (POST /v1/public/links/:token). Returns nil if allowed, or an
// error if the limit has been exceeded. Each IP+token pair is limited to
// 10 attempts per minute to prevent brute-force attacks on access codes
// and passwords. Redis is required; if unavailable, the check is skipped
// with a logged warning (fail-open for availability).
func (s *Service) checkAccessAttemptRateLimit(ctx context.Context, token, ip string) error {
	if s.redisClient == nil {
		return nil
	}
	key := fmt.Sprintf("link:access:ratelimit:%s:%s", token, hashIPForRateLimit(ip))
	allowed, _, err := s.redisClient.RateLimitAllow(ctx, key, 10, time.Minute)
	if err != nil {
		logger.ErrorCtx(ctx, "access rate limit check failed", err)
		return nil // fail-open
	}
	if !allowed {
		return errors.New("rate limit exceeded")
	}
	return nil
}

func hashIPForRateLimit(ip string) string {
	h := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(h[:])[:16]
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
// deleted status, it falls back to marking the link as disabled.
// Hard delete is not used as a fallback because CASCADE can be blocked by
// append-only triggers on access_logs / page_views.
func (s *Service) Delete(ctx context.Context, linkID, workspaceID string) error {
	id, err := uuid.Parse(linkID)
	if err != nil {
		return errors.New("invalid link id")
	}
	params := db.DeleteLinkParams{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		WorkspaceID: pgUUID(workspaceID),
	}

	// 1. Preferred: soft-delete (set status = 'deleted').
	rows, err := s.queries.DeleteLink(ctx, params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			// 2. 'deleted' not in CHECK constraint (migration 025 not applied).
			//    Fall back to 'disabled' — list queries filter it just like 'deleted'.
			_, fallbackErr := s.queries.UpdateLinkStatus(ctx, db.UpdateLinkStatusParams{
				Status:      "disabled",
				ID:          params.ID,
				WorkspaceID: params.WorkspaceID,
			})
			if fallbackErr != nil {
				return fmt.Errorf("delete link: %w", fallbackErr)
			}
			return nil
		}
		return fmt.Errorf("delete link: %w", err)
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

// normalizeSecurityConfig resolves the security configuration from the modern
// boolean flags, with backward compatibility for the legacy permission_type field.
// When explicit boolean flags are absent, permission_type drives the flags.
func normalizeSecurityConfig(req CreateLinkRequest) (requireEmail, requireEmailVerification, requireNDA bool, perm string, err error) {
	requireEmail = req.RequireEmail
	requireEmailVerification = req.RequireEmailVerification
	requireNDA = req.RequireNDA

	// Backward compatibility: legacy permission_type drives flags when explicit
	// boolean flags are not set.
	switch req.PermissionType {
	case "email", "email_required":
		if !requireEmail {
			requireEmail = true
		}
	case "nda":
		if !requireNDA {
			requireNDA = true
		}
		if !requireEmail {
			requireEmail = true
		}
	}

	// Email verification implies email collection.
	if !requireEmail && requireEmailVerification {
		requireEmail = true
	}

	// NDA always requires email verification for identity check.
	if !requireEmailVerification && requireNDA {
		requireEmailVerification = true
	}
	if !requireEmail && requireNDA {
		requireEmail = true
	}

	// Derive display permission_type from boolean flags (priority order).
	if requireNDA {
		perm = "nda"
	} else if requireEmailVerification {
		perm = "email_required"
	} else if requireEmail {
		perm = "email_required"
	} else {
		perm = "public"
	}

	return requireEmail, requireEmailVerification, requireNDA, perm, nil
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

// sendAccessCodeEmails sends verification emails for the given contacts.
// When the underlying mailer supports batching and there are multiple
// recipients, it sends them in one provider batch to reduce round-trips.
// Otherwise it falls back to asynchronous per-email sends bounded by the
// email semaphore.
func (s *Service) sendAccessCodeEmails(ctx context.Context, emailCodes []emailCode, linkName, linkURL string) {
	if len(emailCodes) == 0 {
		return
	}

	name := linkName
	if name == "" {
		name = "A shared document"
	}

	// Try batch path first if the mailer supports it.
	if bm, ok := s.mailer.(mailer.BatchSender); ok && len(emailCodes) > 1 {
		batchCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		jobs := make([]mailer.EmailJob, 0, len(emailCodes))
		for _, ec := range emailCodes {
			jobs = append(jobs, mailer.EmailJob{
				EmailType: mailer.EmailTypeAccessCode,
				Recipient: ec.email,
				Code:      ec.code,
				LinkName:  name,
				LinkURL:   linkURL,
				TemplateVariables: map[string]string{
					"Code":     ec.code,
					"LinkName": name,
					"LinkURL":  linkURL,
				},
			})
		}
		result, err := bm.SendBatch(batchCtx, jobs)
		if err == nil && result.AllSucceeded() {
			return
		}
		if err != nil {
			logger.ErrorCtx(batchCtx, "batch send access code emails failed, falling back to individual sends", err)
		} else {
			logger.ErrorCtx(batchCtx, "batch send access code emails had partial failures, falling back to individual sends", nil,
				logger.Attr("failed_count", len(result.Failed)),
			)
		}
		// Fall through to individual sends for any failed jobs.
		failedSet := make(map[int]struct{}, len(result.Failed))
		for _, f := range result.Failed {
			if f.Index >= 0 && f.Index < len(emailCodes) {
				failedSet[f.Index] = struct{}{}
			}
		}
		retryCodes := make([]emailCode, 0, len(failedSet))
		for i, ec := range emailCodes {
			if _, failed := failedSet[i]; failed {
				retryCodes = append(retryCodes, ec)
			}
		}
		emailCodes = retryCodes
	}

	for _, ec := range emailCodes {
		email, code := ec.email, ec.code
		s.emailSem <- struct{}{} // blocks until a slot is available
		go func() {
			defer func() { <-s.emailSem }()
			// Detach from the request context: this goroutine outlives the HTTP
			// handler, and using the request context causes CreateEmailLog to fail
			// with context.Canceled as soon as the create/update response is sent.
			sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := s.mailer.SendLinkAccessCodeEmail(sendCtx, email, code, name, linkURL); err != nil {
				logger.ErrorCtx(sendCtx, "failed to send link access code email", err,
					logger.Attr("email_local", localPart(email)),
				)
			}
		}()
	}
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

