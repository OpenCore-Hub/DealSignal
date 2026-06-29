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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

// Service handles smart links.
type Service struct {
	queries *db.Queries
}

// NewService creates a link service.
func NewService(q *db.Queries) *Service {
	return &Service{queries: q}
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
	ErrNotFoundInWorkspace  = errors.New("link not found in workspace")
)

// CreateLinkRequest is the input for creating a link.
type CreateLinkRequest struct {
	DocumentID       string
	Name             string
	PermissionType   string
	RequireEmail     bool
	RequirePassword  bool
	RequireNDA       bool
	AllowedEmails    []string
	AllowedDomains   []string
	Password         string
	ExpiresAt        *time.Time
	MaxAccessCount   *int32
	DownloadEnabled  bool
	WatermarkEnabled bool
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

	requireEmail, requirePassword, requireNDA, emails, domains, perm, err := normalizeSecurityConfig(req)
	if err != nil {
		return db.Link{}, err
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

	return s.queries.CreateLink(ctx, db.CreateLinkParams{
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
		RequireEmail:     requireEmail,
		RequirePassword:  requirePassword,
		RequireNda:       requireNDA,
		Status:           "active",
		CreatedBy:        userUUID,
	})
}

// AccessRequest is the input for public access.
type AccessRequest struct {
	Email     string
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

	requiresEmail := link.RequireEmail || link.PermissionType == "email_required" || link.PermissionType == "whitelist" || link.PermissionType == "nda"
	requiresPassword := link.RequirePassword || link.PermissionType == "password"
	requiresNDA := link.RequireNda || link.PermissionType == "nda"
	hasWhitelist := jsonArrayNotEmpty(link.AllowedEmails) || jsonArrayNotEmpty(link.AllowedDomains)

	if requiresEmail || hasWhitelist {
		if strings.TrimSpace(req.Email) == "" {
			return AccessResult{}, ErrRequiresEmail
		}
	}
	if hasWhitelist {
		if !isAllowed(req.Email, link.AllowedEmails, link.AllowedDomains) {
			return AccessResult{}, ErrWhitelistDenied
		}
	}
	if requiresPassword {
		if req.Password == "" {
			return AccessResult{}, ErrRequiresPassword
		}
		if err := bcrypt.CompareHashAndPassword([]byte(link.PasswordHash.String), []byte(req.Password)); err != nil {
			return AccessResult{}, ErrInvalidPassword
		}
	}
	if requiresNDA {
		if !req.NDAAgreed {
			return AccessResult{}, ErrRequiresNDA
		}
	}

	visitorID := makeVisitorID(req.Email, req.UA)

	if requiresNDA {
		ipAddr, _ := netip.ParseAddr(req.IP)
		var ip *netip.Addr
		if ipAddr.IsValid() {
			ip = &ipAddr
		}
		_, _ = s.queries.CreateLinkNDAAgreement(ctx, db.CreateLinkNDAAgreementParams{
			TenantID:    link.TenantID,
			WorkspaceID: link.WorkspaceID,
			LinkID:      link.ID,
			VisitorID:   pgtype.Text{String: visitorID, Valid: visitorID != ""},
			Email:       pgtype.Text{String: req.Email, Valid: req.Email != ""},
			Ip:          ip,
			UserAgent:   pgtype.Text{String: req.UA, Valid: req.UA != ""},
		})
	}

	return AccessResult{Link: link, VisitorID: visitorID, Email: req.Email}, nil
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
		return db.Link{}, err
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
		return db.Link{}, err
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
			return err
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
	return s.queries.ListAccessLogsByLink(ctx, pgtype.UUID{Bytes: id, Valid: true})
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
// normalized email/domain lists, and a display permission_type.
func normalizeSecurityConfig(req CreateLinkRequest) (requireEmail, requirePassword, requireNDA bool, emails, domains []string, perm string, err error) {
	requireEmail = req.RequireEmail
	requirePassword = req.RequirePassword
	requireNDA = req.RequireNDA
	emails = req.AllowedEmails
	domains = req.AllowedDomains
	perm = normalizePermission(req.PermissionType)

	// If the caller only sent the legacy permission_type, derive the flags.
	if !requireEmail && !requirePassword && !requireNDA && len(emails) == 0 && len(domains) == 0 {
		switch perm {
		case "email_required":
			requireEmail = true
		case "whitelist":
			requireEmail = true
		case "password":
			requirePassword = true
		case "nda":
			requireEmail = true
			requireNDA = true
		}
	}

	// Whitelist always requires an email to check against.
	if len(emails) > 0 || len(domains) > 0 {
		requireEmail = true
	}
	// NDA always requires an email for audit.
	if requireNDA {
		requireEmail = true
	}

	if requirePassword && req.Password == "" {
		return false, false, false, nil, nil, "", fmt.Errorf("%w: password required", ErrInvalidPermission)
	}

	// Derive a canonical permission_type for display/backward compatibility.
	if requirePassword {
		perm = "password"
	} else if requireNDA {
		perm = "nda"
	} else if len(emails) > 0 || len(domains) > 0 {
		perm = "whitelist"
	} else if requireEmail {
		perm = "email_required"
	} else {
		perm = "public"
	}

	return requireEmail, requirePassword, requireNDA, emails, domains, perm, nil
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
