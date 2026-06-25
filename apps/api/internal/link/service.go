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
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

	perm := normalizePermission(req.PermissionType)
	if err := validatePermissionConfig(perm, req.Password, req.AllowedEmails, req.AllowedDomains); err != nil {
		return db.Link{}, err
	}

	var passwordHash pgtype.Text
	if perm == "password" {
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
		AllowedEmails:    mustMarshalJSON(req.AllowedEmails),
		AllowedDomains:   mustMarshalJSON(req.AllowedDomains),
		PasswordHash:     passwordHash,
		ExpiresAt:        expiresAt,
		MaxAccessCount:   maxAccess,
		DownloadEnabled:  req.DownloadEnabled,
		WatermarkEnabled: req.WatermarkEnabled,
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

	if link.Status == "revoked" {
		return AccessResult{}, ErrLinkRevoked
	}
	if link.ExpiresAt.Valid && link.ExpiresAt.Time.Before(time.Now()) {
		return AccessResult{}, ErrLinkExpired
	}
	if link.MaxAccessCount.Valid && link.AccessCount >= link.MaxAccessCount.Int32 {
		return AccessResult{}, ErrLinkMaxAccessReached
	}

	switch link.PermissionType {
	case "public":
		// no extra check
	case "email_required":
		if strings.TrimSpace(req.Email) == "" {
			return AccessResult{}, ErrRequiresEmail
		}
	case "whitelist":
		if strings.TrimSpace(req.Email) == "" {
			return AccessResult{}, ErrRequiresEmail
		}
		if !isAllowed(req.Email, link.AllowedEmails, link.AllowedDomains) {
			return AccessResult{}, ErrWhitelistDenied
		}
	case "password":
		if req.Password == "" {
			return AccessResult{}, ErrRequiresPassword
		}
		if err := bcrypt.CompareHashAndPassword([]byte(link.PasswordHash.String), []byte(req.Password)); err != nil {
			return AccessResult{}, ErrInvalidPassword
		}
	case "nda":
		if strings.TrimSpace(req.Email) == "" {
			return AccessResult{}, ErrRequiresEmail
		}
		if !req.NDAAgreed {
			return AccessResult{}, ErrRequiresNDA
		}
	}

	visitorID := makeVisitorID(req.Email, req.UA)
	return AccessResult{Link: link, VisitorID: visitorID}, nil
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

func validatePermissionConfig(perm, password string, emails, domains []string) error {
	switch perm {
	case "public", "email_required", "nda":
		return nil
	case "whitelist":
		if len(emails) == 0 && len(domains) == 0 {
			return fmt.Errorf("%w: whitelist requires allowed_emails or allowed_domains", ErrInvalidPermission)
		}
		return nil
	case "password":
		if password == "" {
			return fmt.Errorf("%w: password required", ErrInvalidPermission)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown permission type %q", ErrInvalidPermission, perm)
	}
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
		if strings.EqualFold(e, email) {
			return true
		}
	}
	for _, d := range domains {
		if strings.EqualFold(d, domain) {
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
