// Package link implements smart-link creation, permission checks and public access.
package link

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/compliance"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/notification"
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

// emailCode holds an email address and its access code for async delivery.
type emailCode struct {
	email string
	code  string
}

// Notifier is the subset of notification.Service needed by link.Service.
type Notifier interface {
	Enqueue(ctx context.Context, workspaceID, userID, channel, subject, body string) (notification.Notification, error)
	Evaluate(ctx context.Context, ev notification.Event) error
}

// Service handles smart links.
type Service struct {
	queries         *db.Queries
	pool            Beginner
	redisClient     *redis.Client
	mailer          mailer.Mailer
	notifier        Notifier
	viewerBaseURL   string
	cfg             *config.Config
	llm             LLMClient
	emailSem        chan struct{} // limits concurrent email sends (bounded goroutines)
}

// LLMClient is the subset of llm.Client used by the link service.
type LLMClient interface {
	ChatCompletion(ctx context.Context, systemPrompt string, history []llmMessage) (string, error)
}

// llmMessage mirrors llm.Message without importing the llm package.
type llmMessage struct {
	Role    string
	Content string
}

// NewService creates a link service.
func NewService(q *db.Queries, pool Beginner, r *redis.Client, m mailer.Mailer, viewerBaseURL string, cfg *config.Config, n Notifier, llm LLMClient) *Service {
	return &Service{
		queries: q, pool: pool, redisClient: r, mailer: m, notifier: n, viewerBaseURL: viewerBaseURL,
		cfg:      cfg,
		llm:      llm,
		emailSem: make(chan struct{}, 8), // cap concurrent email goroutines
	}
}

var (
	ErrDocumentNotReady     = errors.New("document is not ready")
	ErrInvalidPermission    = errors.New("invalid permission configuration")
	ErrLinkNotFound         = errors.New("link not found")
	ErrLinkExpired          = errors.New("link expired")
	ErrLinkArchived         = errors.New("link archived")
	ErrLinkRevoked          = errors.New("link revoked")
	ErrLinkDisabled         = errors.New("link disabled")
	ErrLinkMaxAccessReached = errors.New("link max access reached")
	ErrRequiresEmail        = errors.New("email required")
	ErrRequiresNDA          = errors.New("nda agreement required")
	ErrRequiresEmailCode    = errors.New("email verification code required")
	ErrInvalidEmailCode     = errors.New("invalid email verification code")
	ErrNotFoundInWorkspace  = errors.New("link not found in workspace")

	// Deal-room sharing / access-rule errors.
	ErrDealRoomNotFound      = errors.New("deal room not found")
	ErrBlockedEmail          = errors.New("email is blocked")
	ErrBlockedDomain         = errors.New("domain is blocked")
	ErrNotAllowedEmail       = errors.New("email is not allowed")
	ErrNotAllowedDomain      = errors.New("domain is not allowed")
	ErrRequiresPassword      = errors.New("password required")
	ErrInvalidPassword       = errors.New("invalid password")
	ErrInviteExpired         = errors.New("invitation expired")
	ErrInviteRevoked         = errors.New("invitation revoked")
	ErrInviteAlreadyUsed     = errors.New("invitation already used")
	ErrInvalidAccessRule     = errors.New("invalid access rule")
	ErrConflictingAccessRule = errors.New("conflicting access rule")
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

// EvaluateNotificationRules runs a link activity event through the workspace's
// notification rule engine. Callers should invoke this after recording events
// such as link_opened or page_viewed.
func (s *Service) EvaluateNotificationRules(ctx context.Context, link db.Link, eventType, visitorID, visitorEmail string, metadata map[string]string) error {
	if s.notifier == nil {
		return nil
	}
	return s.notifier.Evaluate(ctx, notification.Event{
		WorkspaceID:  uuid.UUID(link.WorkspaceID.Bytes).String(),
		LinkID:       uuid.UUID(link.ID.Bytes).String(),
		EventType:    eventType,
		VisitorID:    visitorID,
		VisitorEmail: visitorEmail,
		Metadata:     metadata,
	})
}

// CreateLinkRequest is the input for creating a link.
type CreateLinkRequest struct {
	DocumentID               string
	DocumentIDs              []string // Multi-document bundle (takes precedence when non-empty)
	DealRoomID               string
	Name                     string
	PermissionType           string
	RequireEmail             bool
	RequireEmailVerification bool
	RequireNDA               bool
	RequirePassword          bool
	Password                 string // plaintext; stored as bcrypt hash
	AllowedEmails            []string
	AllowedDomains           []string
	ExpiresAt                *time.Time
	MaxAccessCount           *int32
	DownloadEnabled          bool
	WatermarkEnabled         bool
	AICopilotEnabled         bool
	QaEnabled                bool
	FileRequestsEnabled      bool
	IndexFileEnabled         bool
	ContactIDs               []string
	CustomDomain             string
	Tags                     []string
	NotifyOnAccess           bool
}

// UpdateLinkRequest is the input for updating an existing link (full replacement).
type UpdateLinkRequest struct {
	DocumentIDs              []string
	DealRoomID               string
	Name                     string
	PermissionType           string
	RequireEmail             bool
	RequireEmailVerification bool
	RequireNDA               bool
	RequirePassword          bool
	Password                 string // plaintext; if empty and require_password unchanged, keep existing hash
	AllowedEmails            []string
	AllowedDomains           []string
	ExpiresAt                *time.Time
	MaxAccessCount           *int32
	DownloadEnabled          bool
	WatermarkEnabled         bool
	AICopilotEnabled         bool
	QaEnabled                bool
	FileRequestsEnabled      bool
	IndexFileEnabled         bool
	ContactIDs               []string
	CustomDomain             string
	Tags                     []string
	NotifyOnAccess           bool
}

// AccessRule represents a single allow/block rule for a link.
type AccessRule struct {
	RuleType string // "email" or "domain"
	Value    string
	Action   string // "allow" or "block"
}

// AccessEvaluation is the result of evaluating access rules for an email.
type AccessEvaluation struct {
	Allowed     bool
	Reason      string
	MatchedRule *AccessRule
}

// LinkInvitation represents an invitation to view a link.
type LinkInvitation struct {
	ID        string
	LinkID    string
	Email     string
	Token     string
	Status    string
	ExpiresAt *time.Time
	UsedAt    *time.Time
}

// DealRoomLinkRequest is the input for creating a link tied to a deal room.
type DealRoomLinkRequest struct {
	Name                     string
	RequireEmail             bool
	RequireEmailVerification bool
	RequireNDA               bool
	RequirePassword          bool
	Password                 string
	ExpiresAt                *time.Time
	DownloadEnabled          bool
	WatermarkEnabled         bool
	AICopilotEnabled         bool
	CustomDomain             string
	Tags                     []string
	NotifyOnAccess           bool
}

// CreateLink creates a smart link for one or more documents.
func (s *Service) CreateLink(ctx context.Context, userID, workspaceID string, req CreateLinkRequest) (db.Link, error) {
	workspaceUUID := pgUUID(workspaceID)
	userUUID := pgUUID(userID)

	// A link is either document-based or deal-room-based, never both.
	hasDocuments := req.DocumentID != "" || len(req.DocumentIDs) > 0
	hasDealRoom := req.DealRoomID != ""
	if !hasDocuments && !hasDealRoom {
		return db.Link{}, errors.New("either document_id(s) or deal_room_id is required")
	}
	if hasDocuments && hasDealRoom {
		return db.Link{}, errors.New("a link cannot be associated with both documents and a deal room")
	}

	requireEmail, requireEmailVerification, requireNDA, perm, err := normalizeSecurityConfig(req)
	if err != nil {
		return db.Link{}, err
	}

	if requireEmailVerification && len(req.ContactIDs) == 0 {
		return db.Link{}, fmt.Errorf("%w: at least one contact is required for email verification", ErrInvalidPermission)
	}

	passwordHash, err := s.hashPasswordIfRequired(req.RequirePassword, req.Password)
	if err != nil {
		return db.Link{}, err
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

	var tenantID pgtype.UUID
	var primaryDocID pgtype.UUID
	var dealRoomID pgtype.UUID

	if hasDealRoom {
		drUUID, err := uuid.Parse(req.DealRoomID)
		if err != nil {
			return db.Link{}, fmt.Errorf("invalid deal room id: %s", req.DealRoomID)
		}
		dealRoom, err := qtx.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
			ID:          pgtype.UUID{Bytes: drUUID, Valid: true},
			WorkspaceID: workspaceUUID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.Link{}, ErrDealRoomNotFound
			}
			return db.Link{}, fmt.Errorf("get deal room: %w", err)
		}
		if uuid.UUID(dealRoom.WorkspaceID.Bytes).String() != workspaceID {
			return db.Link{}, ErrDealRoomNotFound
		}
		tenantID = dealRoom.TenantID
		dealRoomID = dealRoom.ID
	} else {
		// Resolve document IDs: use DocumentIDs if provided, else fall back to single DocumentID.
		documentIDs := req.DocumentIDs
		if len(documentIDs) == 0 && req.DocumentID != "" {
			documentIDs = []string{req.DocumentID}
		}
		if len(documentIDs) == 0 {
			return db.Link{}, errors.New("at least one document_id is required")
		}

		// Validate all documents exist and are ready.
		for _, did := range documentIDs {
			docUUID, err := uuid.Parse(did)
			if err != nil {
				return db.Link{}, fmt.Errorf("invalid document id: %s", did)
			}
			doc, err := qtx.GetDocumentByID(ctx, db.GetDocumentByIDParams{
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

		// Fetch the primary document to obtain a valid tenant_id for CreateLink.
		primaryDoc, err := qtx.GetDocumentByID(ctx, db.GetDocumentByIDParams{
			ID:          primaryDocID,
			WorkspaceID: workspaceUUID,
		})
		if err != nil {
			return db.Link{}, fmt.Errorf("get primary document: %w", err)
		}
		tenantID = primaryDoc.TenantID
	}

	link, err := qtx.CreateLink(ctx, db.CreateLinkParams{
		TenantID:                 tenantID,
		WorkspaceID:              workspaceUUID,
		DocumentID:               primaryDocID,
		DealRoomID:               dealRoomID,
		PublicToken:              token,
		Name:                     name,
		PermissionType:           perm,
		ExpiresAt:                expiresAt,
		MaxAccessCount:           maxAccess,
		DownloadEnabled:          req.DownloadEnabled,
		WatermarkEnabled:         req.WatermarkEnabled,
		AiCopilotEnabled:         req.AICopilotEnabled,
		QaEnabled:                req.QaEnabled,
		FileRequestsEnabled:      req.FileRequestsEnabled,
		IndexFileEnabled:         req.IndexFileEnabled,
		RequireEmail:             requireEmail,
		RequireEmailVerification: requireEmailVerification,
		RequireNda:               requireNDA,
		RequirePassword:          req.RequirePassword,
		PasswordHash:             passwordHash,
		CustomDomain:             pgtype.Text{String: req.CustomDomain, Valid: req.CustomDomain != ""},
		Tags:                     req.Tags,
		NotifyOnAccess:           req.NotifyOnAccess,
		Status:                   "active",
		CreatedBy:                userUUID,
	})
	if err != nil {
		return db.Link{}, fmt.Errorf("create link: %w", err)
	}

	if hasDocuments {
		// Insert link_documents for all document IDs.
		documentIDs := req.DocumentIDs
		if len(documentIDs) == 0 && req.DocumentID != "" {
			documentIDs = []string{req.DocumentID}
		}
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

	// Create allow-list rules from allowed_emails / allowed_domains.
	allowRules := make([]AccessRule, 0, len(req.AllowedEmails)+len(req.AllowedDomains))
	seenRules := make(map[string]struct{})
	for _, email := range req.AllowedEmails {
		v := strings.TrimSpace(strings.ToLower(email))
		if v == "" {
			continue
		}
		key := "email:" + v
		if _, ok := seenRules[key]; ok {
			continue
		}
		seenRules[key] = struct{}{}
		allowRules = append(allowRules, AccessRule{RuleType: "email", Value: v, Action: "allow"})
	}
	for _, domain := range req.AllowedDomains {
		v := strings.TrimSpace(strings.ToLower(domain))
		if v == "" {
			continue
		}
		v = strings.TrimPrefix(v, "@")
		if strings.Contains(v, "@") {
			return db.Link{}, fmt.Errorf("%w: invalid allowed domain %q", ErrInvalidAccessRule, domain)
		}
		key := "domain:" + v
		if _, ok := seenRules[key]; ok {
			continue
		}
		seenRules[key] = struct{}{}
		allowRules = append(allowRules, AccessRule{RuleType: "domain", Value: v, Action: "allow"})
	}
	if len(allowRules) > 0 {
		if err := validateAccessRules(allowRules); err != nil {
			return db.Link{}, err
		}
		for _, r := range allowRules {
			if err := qtx.CreateLinkAccessRule(ctx, db.CreateLinkAccessRuleParams{
				TenantID:    tenantID,
				WorkspaceID: workspaceUUID,
				LinkID:      link.ID,
				RuleType:    r.RuleType,
				Value:       r.Value,
				Action:      r.Action,
				SortOrder:   0,
			}); err != nil {
				return db.Link{}, fmt.Errorf("create access rule: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Link{}, fmt.Errorf("commit transaction: %w", err)
	}

	// Send verification emails after the transaction commits so the
	// link_contact records are durable. Run asynchronously so SMTP latency does
	// not block the create-link response. Bounded by a semaphore to avoid
	// unbounded goroutine creation when links are created with many contacts.
	linkURL := publicLinkURL(s.viewerBaseURL, token, req.CustomDomain)
	s.sendAccessCodeEmails(ctx, emailCodes, req.Name, linkURL)
	return link, nil
}

// UpdateLink fully replaces a link's configuration.
// The link type (document-based vs deal-room-based) is immutable; only fields
// and the document set (for document links) can be changed.
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

	isDealRoomLink := existing.DealRoomID.Valid
	isDocumentLink := existing.DocumentID.Valid

	createReq := CreateLinkRequest{
		Name:                     req.Name,
		PermissionType:           req.PermissionType,
		RequireEmail:             req.RequireEmail,
		RequireEmailVerification: req.RequireEmailVerification,
		RequireNDA:               req.RequireNDA,
		RequirePassword:          req.RequirePassword,
		Password:                 req.Password,
		ExpiresAt:                req.ExpiresAt,
		MaxAccessCount:           req.MaxAccessCount,
		DownloadEnabled:          req.DownloadEnabled,
		WatermarkEnabled:         req.WatermarkEnabled,
		AICopilotEnabled:         req.AICopilotEnabled,
		QaEnabled:                req.QaEnabled,
		FileRequestsEnabled:      req.FileRequestsEnabled,
		IndexFileEnabled:         req.IndexFileEnabled,
		ContactIDs:               req.ContactIDs,
	}

	requireEmail, requireEmailVerification, requireNDA, perm, err := normalizeSecurityConfig(createReq)
	if err != nil {
		return db.Link{}, err
	}

	// Validate contacts for email verification (document links only).
	if !isDealRoomLink && requireEmailVerification && len(req.ContactIDs) == 0 {
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

	customDomain := existing.CustomDomain
	if req.CustomDomain != "" || existing.CustomDomain.Valid {
		customDomain = pgtype.Text{String: req.CustomDomain, Valid: req.CustomDomain != ""}
	}

	tags := existing.Tags
	if req.Tags != nil {
		tags = req.Tags
	}

	passwordHash, err := s.resolvePasswordHashForUpdate(existing, req.RequirePassword, req.Password)
	if err != nil {
		return db.Link{}, err
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
		DocumentID:               existing.DocumentID,
		DealRoomID:               existing.DealRoomID,
		PermissionType:           perm,
		ExpiresAt:                expiresAt,
		MaxAccessCount:           maxAccess,
		DownloadEnabled:          req.DownloadEnabled,
		WatermarkEnabled:         req.WatermarkEnabled,
		RequireEmail:             requireEmail,
		RequireEmailVerification: requireEmailVerification,
		RequireNda:               requireNDA,
		AiCopilotEnabled:         req.AICopilotEnabled,
		QaEnabled:                req.QaEnabled,
		FileRequestsEnabled:      req.FileRequestsEnabled,
		IndexFileEnabled:         req.IndexFileEnabled,
		RequirePassword:          req.RequirePassword,
		PasswordHash:             passwordHash,
		CustomDomain:             customDomain,
		Tags:                     tags,
		NotifyOnAccess:           req.NotifyOnAccess,
		SecurityVersion:          existing.SecurityVersion + 1,
		ID:                       existing.ID,
		WorkspaceID:              workspaceUUID,
	})
	if err != nil {
		return db.Link{}, fmt.Errorf("update link: %w", err)
	}

	// Replace all link_documents for document links.
	if isDocumentLink {
		if len(req.DocumentIDs) == 0 {
			return db.Link{}, errors.New("at least one document_id is required")
		}
		var primaryDocID pgtype.UUID
		for _, did := range req.DocumentIDs {
			docUUID, err := uuid.Parse(did)
			if err != nil {
				return db.Link{}, fmt.Errorf("invalid document id: %s", did)
			}
			doc, err := qtx.GetDocumentByID(ctx, db.GetDocumentByIDParams{
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
	linkURL := publicLinkURL(s.viewerBaseURL, existing.PublicToken, req.CustomDomain)
	s.sendAccessCodeEmails(ctx, emailCodes, req.Name, linkURL)

	// Re-fetch to get the updated record.
	return s.GetByID(ctx, linkID, workspaceID)
}

// CreateDealRoomLink creates a share link scoped to a deal room.
func (s *Service) CreateDealRoomLink(ctx context.Context, userID, workspaceID, dealRoomID string, req DealRoomLinkRequest) (db.Link, error) {
	return s.CreateLink(ctx, userID, workspaceID, CreateLinkRequest{
		DealRoomID:               dealRoomID,
		Name:                     req.Name,
		RequireEmail:             req.RequireEmail,
		RequireEmailVerification: req.RequireEmailVerification,
		RequireNDA:               req.RequireNDA,
		RequirePassword:          req.RequirePassword,
		Password:                 req.Password,
		ExpiresAt:                req.ExpiresAt,
		DownloadEnabled:          req.DownloadEnabled,
		WatermarkEnabled:         req.WatermarkEnabled,
		AICopilotEnabled:         req.AICopilotEnabled,
		CustomDomain:             req.CustomDomain,
		Tags:                     req.Tags,
		NotifyOnAccess:           req.NotifyOnAccess,
	})
}

// ListDealRoomLinks returns active share links for a deal room.
func (s *Service) ListDealRoomLinks(ctx context.Context, workspaceID, dealRoomID string) ([]db.Link, error) {
	workspaceUUID := pgUUID(workspaceID)
	drUUID, err := uuid.Parse(dealRoomID)
	if err != nil {
		return nil, errors.New("invalid deal room id")
	}
	return s.queries.ListLinksByDealRoom(ctx, db.ListLinksByDealRoomParams{
		WorkspaceID: workspaceUUID,
		DealRoomID:  pgtype.UUID{Bytes: drUUID, Valid: true},
	})
}

// ResolveDealRoomSlug looks up a deal room by slug and returns the public token
// of its first active share link. Returns empty string if no link exists.
func (s *Service) ResolveDealRoomSlug(ctx context.Context, slug string) (string, error) {
	room, err := s.queries.GetDealRoomBySlug(ctx, slug)
	if err != nil {
		return "", fmt.Errorf("deal room not found: %w", err)
	}
	links, err := s.queries.ListLinksByDealRoom(ctx, db.ListLinksByDealRoomParams{
		WorkspaceID: room.WorkspaceID,
		DealRoomID:  room.ID,
	})
	if err != nil {
		return "", fmt.Errorf("list links: %w", err)
	}
	for _, l := range links {
		if l.Status == "active" {
			return l.PublicToken, nil
		}
	}
	return "", nil
}

// EvaluateAccessRules determines whether the given email is allowed to access
// the link according to its allow/block rules.
//
// Evaluation order:
//  1. Block rules take priority over allow rules.
//  2. email rules take priority over domain rules.
//  3. If any allow rule exists and none match, access is denied.
//  4. If no rules exist, access is allowed.
// EvaluateAccessRules determines whether the given email is allowed to access
// the link according to its allow/block rules.
func (s *Service) EvaluateAccessRules(ctx context.Context, linkID, email string) (AccessEvaluation, error) {
	id, err := uuid.Parse(linkID)
	if err != nil {
		return AccessEvaluation{}, errors.New("invalid link id")
	}
	dbRules, err := s.queries.ListLinkAccessRulesByLink(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		return AccessEvaluation{}, fmt.Errorf("list access rules: %w", err)
	}
	rules := make([]AccessRule, 0, len(dbRules))
	for _, r := range dbRules {
		rules = append(rules, AccessRule{RuleType: r.RuleType, Value: r.Value, Action: r.Action})
	}
	return evaluateAccessRules(rules, email), nil
}

// evaluateAccessRules is the pure rule-evaluation engine. It is exposed
// separately so the access-control logic can be unit-tested without a database.
//
// Evaluation order:
//  1. Block rules take priority over allow rules.
//  2. Email rules take priority over domain rules.
//  3. If any allow rule exists and none match, access is denied.
//  4. If no rules exist, access is allowed.
func evaluateAccessRules(rules []AccessRule, email string) AccessEvaluation {
	// Work on a copy so the evaluation order (block before allow, email before
	// domain) matches the documented semantics regardless of storage order.
	if len(rules) > 0 {
		rules = append([]AccessRule(nil), rules...)
		sort.SliceStable(rules, func(i, j int) bool {
			if rules[i].Action != rules[j].Action {
				// block sorts before allow.
				return rules[i].Action == "block"
			}
			// email sorts before domain.
			return rules[i].RuleType == "email" && rules[j].RuleType == "domain"
		})
	}

	if len(rules) == 0 {
		return AccessEvaluation{Allowed: true, Reason: "no_rules"}
	}

	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		// If there are any allow rules, empty email cannot satisfy them.
		for _, r := range rules {
			if r.Action == "allow" {
				return AccessEvaluation{Allowed: false, Reason: "no_allow_match"}
			}
		}
		return AccessEvaluation{Allowed: true, Reason: "no_rules"}
	}

	domain := ""
	if at := strings.LastIndex(email, "@"); at >= 0 {
		domain = email[at+1:]
	}

	var allowExists bool
	for _, r := range rules {
		if r.Action == "allow" {
			allowExists = true
		}
	}

	// First pass: block rules (email then domain).
	for _, r := range rules {
		if r.Action != "block" {
			continue
		}
		if r.RuleType == "email" && constantTimeEmailCompare(r.Value, email) {
			return AccessEvaluation{
				Allowed:     false,
				Reason:      "blocked_email",
				MatchedRule: &AccessRule{RuleType: r.RuleType, Value: r.Value, Action: r.Action},
			}
		}
		if r.RuleType == "domain" && domain != "" && strings.EqualFold(r.Value, domain) {
			return AccessEvaluation{
				Allowed:     false,
				Reason:      "blocked_domain",
				MatchedRule: &AccessRule{RuleType: r.RuleType, Value: r.Value, Action: r.Action},
			}
		}
	}

	// Second pass: allow rules (email then domain).
	for _, r := range rules {
		if r.Action != "allow" {
			continue
		}
		if r.RuleType == "email" && constantTimeEmailCompare(r.Value, email) {
			return AccessEvaluation{
				Allowed:     true,
				Reason:      "allowed_email",
				MatchedRule: &AccessRule{RuleType: r.RuleType, Value: r.Value, Action: r.Action},
			}
		}
		if r.RuleType == "domain" && domain != "" && strings.EqualFold(r.Value, domain) {
			return AccessEvaluation{
				Allowed:     true,
				Reason:      "allowed_domain",
				MatchedRule: &AccessRule{RuleType: r.RuleType, Value: r.Value, Action: r.Action},
			}
		}
	}

	if allowExists {
		return AccessEvaluation{Allowed: false, Reason: "no_allow_match"}
	}
	return AccessEvaluation{Allowed: true, Reason: "no_allow_rules"}
}

// validateAccessRules checks that a set of rules is internally consistent.
func validateAccessRules(rules []AccessRule) error {
	seen := make(map[string]struct{}, len(rules))
	for _, r := range rules {
		if r.RuleType != "email" && r.RuleType != "domain" {
			return fmt.Errorf("%w: rule_type must be email or domain", ErrInvalidAccessRule)
		}
		if r.Action != "allow" && r.Action != "block" {
			return fmt.Errorf("%w: action must be allow or block", ErrInvalidAccessRule)
		}
		value := strings.TrimSpace(strings.ToLower(r.Value))
		if value == "" {
			return fmt.Errorf("%w: rule value cannot be empty", ErrInvalidAccessRule)
		}
		if r.RuleType == "domain" && strings.Contains(value, "@") {
			return fmt.Errorf("%w: domain rule cannot contain @", ErrInvalidAccessRule)
		}
		key := r.RuleType + ":" + value
		if _, ok := seen[key]; ok {
			return fmt.Errorf("%w: duplicate rule for %s", ErrConflictingAccessRule, value)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// UpdateAccessRules replaces all access rules for a link.
func (s *Service) UpdateAccessRules(ctx context.Context, userID, workspaceID, linkID string, rules []AccessRule) error {
	workspaceUUID := pgUUID(workspaceID)
	userUUID := pgUUID(userID)
	linkUUID, err := uuid.Parse(linkID)
	if err != nil {
		return errors.New("invalid link id")
	}

	if err := validateAccessRules(rules); err != nil {
		return err
	}

	// Verify link exists in workspace.
	link, err := s.GetByID(ctx, linkID, workspaceID)
	if err != nil {
		return err
	}
	if link.Status == "deleted" {
		return ErrNotFoundInWorkspace
	}

	// If any allow rule exists, email must be required.
	for _, r := range rules {
		if r.Action == "allow" && !link.RequireEmail {
			return fmt.Errorf("%w: require_email must be enabled when allow rules exist", ErrInvalidAccessRule)
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	if err := qtx.DeleteLinkAccessRulesByLink(ctx, pgtype.UUID{Bytes: linkUUID, Valid: true}); err != nil {
		return fmt.Errorf("delete access rules: %w", err)
	}

	for i, r := range rules {
		value := strings.TrimSpace(strings.ToLower(r.Value))
		if err := qtx.CreateLinkAccessRule(ctx, db.CreateLinkAccessRuleParams{
			TenantID:    link.TenantID,
			WorkspaceID: workspaceUUID,
			LinkID:      link.ID,
			RuleType:    r.RuleType,
			Value:       value,
			Action:      r.Action,
			SortOrder:   int32(i),
		}); err != nil {
			return fmt.Errorf("create access rule: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	// Audit log.
	s.recordSecurityEvent(ctx, link, uuid.UUID(userUUID.Bytes).String(), "", "access_rules_updated", "")
	return nil
}

// ListAccessRules returns all access rules for a link.
func (s *Service) ListAccessRules(ctx context.Context, workspaceID, linkID string) ([]AccessRule, error) {
	linkUUID, err := uuid.Parse(linkID)
	if err != nil {
		return nil, errors.New("invalid link id")
	}
	// Verify link exists in workspace.
	link, err := s.GetByID(ctx, linkID, workspaceID)
	if err != nil {
		return nil, err
	}
	if link.Status == "deleted" {
		return nil, ErrNotFoundInWorkspace
	}

	rules, err := s.queries.ListLinkAccessRulesByLink(ctx, pgtype.UUID{Bytes: linkUUID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list access rules: %w", err)
	}
	out := make([]AccessRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, AccessRule{
			RuleType: r.RuleType,
			Value:    r.Value,
			Action:   r.Action,
		})
	}
	return out, nil
}

// InviteViewers creates invitations for the given emails and sends invitation emails.
func (s *Service) InviteViewers(ctx context.Context, userID, workspaceID, linkID string, emails []string) ([]LinkInvitation, error) {
	workspaceUUID := pgUUID(workspaceID)
	userUUID := pgUUID(userID)

	// Verify link exists in workspace.
	link, err := s.GetByID(ctx, linkID, workspaceID)
	if err != nil {
		return nil, err
	}
	if link.Status == "deleted" {
		return nil, ErrNotFoundInWorkspace
	}
	if link.Status != "active" {
		return nil, ErrLinkDisabled
	}

	// Normalize and validate emails.
	normalized := make([]string, 0, len(emails))
	seen := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		email := strings.TrimSpace(strings.ToLower(e))
		if email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		normalized = append(normalized, email)
	}
	if len(normalized) == 0 {
		return nil, errors.New("at least one valid email is required")
	}
	if !link.RequireEmail {
		return nil, fmt.Errorf("%w: require_email must be enabled to invite viewers", ErrInvalidPermission)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	invitations := make([]LinkInvitation, 0, len(normalized))
	for _, email := range normalized {
		// Reuse or reset an existing invitation if present.
		existing, err := qtx.GetLinkInvitationByLinkAndEmail(ctx, db.GetLinkInvitationByLinkAndEmailParams{
			LinkID: link.ID,
			Email:  email,
		})
		if err == nil {
			if existing.Status != "revoked" {
				invitations = append(invitations, dbInvitationToDomain(existing))
				continue
			}
			// Revoked invitations still occupy the unique (link_id, email) slot,
			// so reset them instead of trying to insert a duplicate.
			token, err := generateToken()
			if err != nil {
				return nil, fmt.Errorf("generate invite token: %w", err)
			}
			expiresAt := pgtype.Timestamptz{Valid: true, Time: time.Now().Add(7 * 24 * time.Hour)}
			if _, err := qtx.ResetLinkInvitation(ctx, db.ResetLinkInvitationParams{
				Token:     pgtype.Text{String: "", Valid: false},
				TokenHash: pgtype.Text{String: hashToken(token), Valid: true},
				ExpiresAt: expiresAt,
				ID:        existing.ID,
			}); err != nil {
				return nil, fmt.Errorf("reset invitation: %w", err)
			}
			invitations = append(invitations, invitationFromRaw(token, existing.ID, link.ID, email, "pending", expiresAt, pgtype.Timestamptz{}))
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get invitation by email: %w", err)
		}

		token, err := generateToken()
		if err != nil {
			return nil, fmt.Errorf("generate invite token: %w", err)
		}

		expiresAt := pgtype.Timestamptz{Valid: true, Time: time.Now().Add(7 * 24 * time.Hour)}
		inv, err := qtx.CreateLinkInvitation(ctx, db.CreateLinkInvitationParams{
			TenantID:    link.TenantID,
			WorkspaceID: workspaceUUID,
			LinkID:      link.ID,
			Email:       email,
			Token:       pgtype.Text{String: "", Valid: false},
			TokenHash:   pgtype.Text{String: hashToken(token), Valid: true},
			Status:      "pending",
			ExpiresAt:   expiresAt,
			CreatedBy:   userUUID,
		})
		if err != nil {
			return nil, fmt.Errorf("create invitation: %w", err)
		}
		invitations = append(invitations, invitationFromRaw(token, inv.ID, link.ID, email, inv.Status, expiresAt, pgtype.Timestamptz{}))
	}

	// Add invited emails to allow list.
	rules, _ := qtx.ListLinkAccessRulesByLink(ctx, link.ID)
	hasAllow := false
	for _, r := range rules {
		if r.Action == "allow" {
			hasAllow = true
			break
		}
	}
	for _, email := range normalized {
		// Add email allow rule.
		if err := qtx.CreateLinkAccessRule(ctx, db.CreateLinkAccessRuleParams{
			TenantID:    link.TenantID,
			WorkspaceID: workspaceUUID,
			LinkID:      link.ID,
			RuleType:    "email",
			Value:       email,
			Action:      "allow",
			SortOrder:   0,
		}); err != nil {
			return nil, fmt.Errorf("create allow rule: %w", err)
		}
	}
	if hasAllow && !link.RequireEmail {
		// This should not happen if validation is correct, but enforce consistency.
		return nil, fmt.Errorf("%w: require_email must be enabled when allow rules exist", ErrInvalidAccessRule)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// Send invitation emails after commit.
	linkURL := publicLinkURL(s.viewerBaseURL, link.PublicToken, link.CustomDomain.String)
	for _, inv := range invitations {
		s.sendInvitationEmail(ctx, inv, link.Name.String, linkURL)
	}

	return invitations, nil
}

// ResolveInviteToken validates an invitation token and returns the invitation.
func (s *Service) ResolveInviteToken(ctx context.Context, token string) (LinkInvitation, error) {
	inv, err := s.queries.GetLinkInvitationByToken(ctx, pgtype.Text{String: hashToken(token), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LinkInvitation{}, ErrLinkNotFound
		}
		return LinkInvitation{}, fmt.Errorf("get invitation: %w", err)
	}

	// Lazy backfill: legacy invitations stored the plaintext token. Compute and
	// persist the hash on first lookup so future lookups use the hash path.
	if !inv.TokenHash.Valid || inv.TokenHash.String == "" {
		if err := s.queries.UpdateLinkInvitationTokenHash(ctx, db.UpdateLinkInvitationTokenHashParams{
			TokenHash: pgtype.Text{String: hashToken(token), Valid: true},
			ID:        inv.ID,
		}); err != nil {
			logger.ErrorCtx(ctx, "failed to backfill invitation token hash", err)
		}
	}

	switch inv.Status {
	case "revoked":
		return LinkInvitation{}, ErrInviteRevoked
	case "expired":
		return LinkInvitation{}, ErrInviteExpired
	}
	if inv.ExpiresAt.Valid && inv.ExpiresAt.Time.Before(time.Now()) {
		// Auto-expire if past expiration.
		if _, err := s.queries.UpdateLinkInvitationStatus(ctx, db.UpdateLinkInvitationStatusParams{
			Status: "expired",
			ID:     inv.ID,
		}); err != nil {
			logger.ErrorCtx(ctx, "failed to mark invitation as expired", err)
		}
		return LinkInvitation{}, ErrInviteExpired
	}
	return dbInvitationToDomain(inv), nil
}

// RevokeInvitation revokes an invitation and optionally removes the email from
// the link's allow list.
func (s *Service) RevokeInvitation(ctx context.Context, workspaceID, invitationID string, removeFromAllowList bool) error {
	id, err := uuid.Parse(invitationID)
	if err != nil {
		return errors.New("invalid invitation id")
	}

	inv, err := s.queries.GetLinkInvitationByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLinkNotFound
		}
		return fmt.Errorf("get invitation: %w", err)
	}
	// Verify link belongs to workspace.
	if _, err := s.GetByID(ctx, uuid.UUID(inv.LinkID.Bytes).String(), workspaceID); err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	if _, err := qtx.UpdateLinkInvitationStatus(ctx, db.UpdateLinkInvitationStatusParams{
		Status: "revoked",
		ID:     inv.ID,
	}); err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}

	if removeFromAllowList {
		// Remove the email allow rule for this invitation.
		if err := qtx.DeleteLinkAccessRuleByLinkAndValue(ctx, db.DeleteLinkAccessRuleByLinkAndValueParams{
			LinkID:   inv.LinkID,
			RuleType: "email",
			Value:    strings.ToLower(inv.Email),
			Action:   "allow",
		}); err != nil {
			return fmt.Errorf("remove allow rule: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// ListInvitations returns all invitations for a link.
func (s *Service) ListInvitations(ctx context.Context, workspaceID, linkID string) ([]LinkInvitation, error) {
	linkUUID, err := uuid.Parse(linkID)
	if err != nil {
		return nil, errors.New("invalid link id")
	}
	// Verify link exists in workspace.
	if _, err := s.GetByID(ctx, linkID, workspaceID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListLinkInvitationsByLink(ctx, pgtype.UUID{Bytes: linkUUID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	out := make([]LinkInvitation, 0, len(rows))
	for _, r := range rows {
		out = append(out, dbInvitationToDomain(r))
	}
	return out, nil
}

func dbInvitationToDomain(inv any) LinkInvitation {
	switch v := inv.(type) {
	case db.LinkInvitation:
		return linkInvitationFromFields(v.ID, v.LinkID, v.Email, v.Token, v.Status, v.ExpiresAt, v.UsedAt)
	case db.CreateLinkInvitationRow:
		return linkInvitationFromFields(v.ID, v.LinkID, v.Email, v.Token, v.Status, v.ExpiresAt, v.UsedAt)
	case db.GetLinkInvitationByLinkAndEmailRow:
		return linkInvitationFromFields(v.ID, v.LinkID, v.Email, v.Token, v.Status, v.ExpiresAt, v.UsedAt)
	case db.ResetLinkInvitationRow:
		return linkInvitationFromFields(v.ID, v.LinkID, v.Email, v.Token, v.Status, v.ExpiresAt, v.UsedAt)
	case db.GetLinkInvitationByTokenRow:
		return linkInvitationFromFields(v.ID, v.LinkID, v.Email, v.Token, v.Status, v.ExpiresAt, v.UsedAt)
	case db.ListLinkInvitationsByLinkRow:
		return linkInvitationFromFields(v.ID, v.LinkID, v.Email, v.Token, v.Status, v.ExpiresAt, v.UsedAt)
	case db.UpdateLinkInvitationStatusRow:
		return linkInvitationFromFields(v.ID, v.LinkID, v.Email, v.Token, v.Status, v.ExpiresAt, v.UsedAt)
	case db.GetLinkInvitationByIDRow:
		return linkInvitationFromFields(v.ID, v.LinkID, v.Email, v.Token, v.Status, v.ExpiresAt, v.UsedAt)
	}
	return LinkInvitation{}
}

func linkInvitationFromFields(
	id pgtype.UUID,
	linkID pgtype.UUID,
	email string,
	token pgtype.Text,
	status string,
	expiresAt pgtype.Timestamptz,
	usedAt pgtype.Timestamptz,
) LinkInvitation {
	var ea, ua *time.Time
	if expiresAt.Valid {
		ea = &expiresAt.Time
	}
	if usedAt.Valid {
		ua = &usedAt.Time
	}
	return LinkInvitation{
		ID:        uuid.UUID(id.Bytes).String(),
		LinkID:    uuid.UUID(linkID.Bytes).String(),
		Email:     email,
		Token:     token.String,
		Status:    status,
		ExpiresAt: ea,
		UsedAt:    ua,
	}
}

func dbAccessRequestToDomain(r db.LinkAccessRequest) LinkAccessRequest {
	var reviewedBy *string
	if r.ReviewedBy.Valid {
		s := uuid.UUID(r.ReviewedBy.Bytes).String()
		reviewedBy = &s
	}
	var reviewedAt *time.Time
	if r.ReviewedAt.Valid {
		reviewedAt = &r.ReviewedAt.Time
	}
	return LinkAccessRequest{
		ID:         uuid.UUID(r.ID.Bytes).String(),
		LinkID:     uuid.UUID(r.LinkID.Bytes).String(),
		Email:      r.Email,
		Reason:     r.Reason.String,
		Status:     r.Status,
		ReviewedBy: reviewedBy,
		ReviewedAt: reviewedAt,
		CreatedAt:  r.CreatedAt.Time,
		UpdatedAt:  r.UpdatedAt.Time,
	}
}

func isValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	user := email[:at]
	domain := email[at+1:]
	if len(user) == 0 || len(domain) == 0 {
		return false
	}
	if strings.Contains(domain, ".") {
		parts := strings.Split(domain, ".")
		for _, p := range parts {
			if len(p) == 0 {
				return false
			}
		}
		return true
	}
	return false
}

// AllowAccessRequest checks the per-IP per-link rate limit for access requests.
func (s *Service) AllowAccessRequest(ctx context.Context, clientIP, publicToken string) (bool, error) {
	if s.redisClient == nil {
		return true, nil
	}
	key := "access_request:" + clientIP + ":" + publicToken
	allowed, _, err := s.redisClient.RateLimitAllow(ctx, key, 5, time.Hour)
	return allowed, err
}

// RequestAccess lets a blocked or not-allowed visitor request access to a link.
func (s *Service) RequestAccess(ctx context.Context, link db.Link, email, reason string) (LinkAccessRequest, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if !isValidEmail(email) {
		return LinkAccessRequest{}, errors.New("invalid email address")
	}
	reason = strings.TrimSpace(reason)
	if len(reason) > 500 {
		return LinkAccessRequest{}, errors.New("reason must be 500 characters or less")
	}

	linkID := uuid.UUID(link.ID.Bytes).String()
	ev, err := s.EvaluateAccessRules(ctx, linkID, email)
	if err != nil {
		return LinkAccessRequest{}, fmt.Errorf("evaluate access rules: %w", err)
	}
	if !ev.Allowed && (ev.Reason == "blocked_email" || ev.Reason == "blocked_domain") {
		return LinkAccessRequest{}, ErrAccessRequestBlocked
	}

	existing, err := s.queries.GetLinkAccessRequestByLinkAndEmail(ctx, db.GetLinkAccessRequestByLinkAndEmailParams{
		LinkID: link.ID,
		Email:  email,
	})
	if err == nil {
		if existing.Status == "pending" {
			return dbAccessRequestToDomain(existing), nil
		}
		return LinkAccessRequest{}, ErrAccessRequestExists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return LinkAccessRequest{}, fmt.Errorf("lookup existing access request: %w", err)
	}

	row, err := s.queries.CreateLinkAccessRequest(ctx, db.CreateLinkAccessRequestParams{
		TenantID:    link.TenantID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
		Email:       email,
		Reason:      pgtype.Text{String: reason, Valid: reason != ""},
	})
	if err != nil {
		return LinkAccessRequest{}, fmt.Errorf("create access request: %w", err)
	}

	if s.notifier != nil {
		creatorID := uuid.UUID(link.CreatedBy.Bytes).String()
		wsID := uuid.UUID(link.WorkspaceID.Bytes).String()
		subject := "New access request on your link"
		body := fmt.Sprintf("A visitor (%s) requested access to \"%s\". Reason: %s. Review the request in the share dialog.", email, link.Name.String, reason)
		if reason == "" {
			body = fmt.Sprintf("A visitor (%s) requested access to \"%s\". Review the request in the share dialog.", email, link.Name.String)
		}
		if _, notifyErr := s.notifier.Enqueue(ctx, wsID, creatorID, "email", subject, body); notifyErr != nil {
			logger.ErrorCtx(ctx, "failed to enqueue access request notification", notifyErr,
				logger.Attr("link_id", linkID),
				logger.Attr("email", email),
			)
		}
	}

	return dbAccessRequestToDomain(row), nil
}

// ListAccessRequests returns all access requests for a link.
func (s *Service) ListAccessRequests(ctx context.Context, workspaceID, linkID string) ([]LinkAccessRequest, error) {
	if _, err := s.GetByID(ctx, linkID, workspaceID); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(linkID)
	if err != nil {
		return nil, errors.New("invalid link id")
	}
	rows, err := s.queries.ListLinkAccessRequestsByLink(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list access requests: %w", err)
	}
	out := make([]LinkAccessRequest, 0, len(rows))
	for _, r := range rows {
		out = append(out, dbAccessRequestToDomain(r))
	}
	return out, nil
}

// ApproveAccessRequest approves a pending request, adds an allow-rule and sends an invitation email.
func (s *Service) ApproveAccessRequest(ctx context.Context, workspaceID, linkID, requestID, reviewerID string) (LinkAccessRequest, error) {
	link, err := s.GetByID(ctx, linkID, workspaceID)
	if err != nil {
		return LinkAccessRequest{}, err
	}
	if uuid.UUID(link.CreatedBy.Bytes).String() != reviewerID {
		return LinkAccessRequest{}, errors.New("only the link creator can approve access requests")
	}

	reqUUID, err := uuid.Parse(requestID)
	if err != nil {
		return LinkAccessRequest{}, errors.New("invalid request id")
	}
	reqRow, err := s.queries.GetLinkAccessRequestByID(ctx, pgtype.UUID{Bytes: reqUUID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LinkAccessRequest{}, errors.New("access request not found")
		}
		return LinkAccessRequest{}, fmt.Errorf("get access request: %w", err)
	}
	if reqRow.Status != "pending" {
		return LinkAccessRequest{}, errors.New("access request is not pending")
	}
	if uuid.UUID(reqRow.LinkID.Bytes).String() != linkID {
		return LinkAccessRequest{}, errors.New("access request does not belong to this link")
	}

	reviewerUUID, err := uuid.Parse(reviewerID)
	if err != nil {
		return LinkAccessRequest{}, errors.New("invalid reviewer id")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return LinkAccessRequest{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	updated, err := qtx.UpdateLinkAccessRequestStatus(ctx, db.UpdateLinkAccessRequestStatusParams{
		Status:     "approved",
		ReviewedBy: pgtype.UUID{Bytes: reviewerUUID, Valid: true},
		ID:         pgtype.UUID{Bytes: reqUUID, Valid: true},
	})
	if err != nil {
		return LinkAccessRequest{}, fmt.Errorf("approve access request: %w", err)
	}

	inv, err := s.createInvitationForRequest(ctx, qtx, link, reqRow.Email, pgtype.UUID{Bytes: reviewerUUID, Valid: true})
	if err != nil {
		return LinkAccessRequest{}, fmt.Errorf("create invitation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return LinkAccessRequest{}, fmt.Errorf("commit transaction: %w", err)
	}

	linkURL := publicLinkURL(s.viewerBaseURL, link.PublicToken, link.CustomDomain.String)
	s.sendInvitationEmail(ctx, inv, link.Name.String, linkURL)

	return dbAccessRequestToDomain(updated), nil
}

// RejectAccessRequest rejects a pending access request.
func (s *Service) RejectAccessRequest(ctx context.Context, workspaceID, linkID, requestID, reviewerID string) (LinkAccessRequest, error) {
	link, err := s.GetByID(ctx, linkID, workspaceID)
	if err != nil {
		return LinkAccessRequest{}, err
	}
	if uuid.UUID(link.CreatedBy.Bytes).String() != reviewerID {
		return LinkAccessRequest{}, errors.New("only the link creator can reject access requests")
	}

	reqUUID, err := uuid.Parse(requestID)
	if err != nil {
		return LinkAccessRequest{}, errors.New("invalid request id")
	}
	reqRow, err := s.queries.GetLinkAccessRequestByID(ctx, pgtype.UUID{Bytes: reqUUID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LinkAccessRequest{}, errors.New("access request not found")
		}
		return LinkAccessRequest{}, fmt.Errorf("get access request: %w", err)
	}
	if reqRow.Status != "pending" {
		return LinkAccessRequest{}, errors.New("access request is not pending")
	}
	if uuid.UUID(reqRow.LinkID.Bytes).String() != linkID {
		return LinkAccessRequest{}, errors.New("access request does not belong to this link")
	}

	reviewerUUID, err := uuid.Parse(reviewerID)
	if err != nil {
		return LinkAccessRequest{}, errors.New("invalid reviewer id")
	}

	updated, err := s.queries.UpdateLinkAccessRequestStatus(ctx, db.UpdateLinkAccessRequestStatusParams{
		Status:     "rejected",
		ReviewedBy: pgtype.UUID{Bytes: reviewerUUID, Valid: true},
		ID:         pgtype.UUID{Bytes: reqUUID, Valid: true},
	})
	if err != nil {
		return LinkAccessRequest{}, fmt.Errorf("reject access request: %w", err)
	}
	return dbAccessRequestToDomain(updated), nil
}

func (s *Service) createInvitationForRequest(ctx context.Context, qtx *db.Queries, link db.Link, email string, createdBy pgtype.UUID) (LinkInvitation, error) {
	workspaceUUID := link.WorkspaceID
	existing, err := qtx.GetLinkInvitationByLinkAndEmail(ctx, db.GetLinkInvitationByLinkAndEmailParams{
		LinkID: link.ID,
		Email:  email,
	})
	if err == nil {
		if existing.Status != "revoked" {
			return dbInvitationToDomain(existing), nil
		}
		token, err := generateToken()
		if err != nil {
			return LinkInvitation{}, fmt.Errorf("generate invite token: %w", err)
		}
		expiresAt := pgtype.Timestamptz{Valid: true, Time: time.Now().Add(7 * 24 * time.Hour)}
		if _, err := qtx.ResetLinkInvitation(ctx, db.ResetLinkInvitationParams{
			Token:     pgtype.Text{String: "", Valid: false},
			TokenHash: pgtype.Text{String: hashToken(token), Valid: true},
			ExpiresAt: expiresAt,
			ID:        existing.ID,
		}); err != nil {
			return LinkInvitation{}, fmt.Errorf("reset invitation: %w", err)
		}
		return invitationFromRaw(token, existing.ID, link.ID, email, "pending", expiresAt, pgtype.Timestamptz{}), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return LinkInvitation{}, fmt.Errorf("get invitation by email: %w", err)
	}

	token, err := generateToken()
	if err != nil {
		return LinkInvitation{}, fmt.Errorf("generate invite token: %w", err)
	}
	expiresAt := pgtype.Timestamptz{Valid: true, Time: time.Now().Add(7 * 24 * time.Hour)}
	inv, err := qtx.CreateLinkInvitation(ctx, db.CreateLinkInvitationParams{
		TenantID:    link.TenantID,
		WorkspaceID: workspaceUUID,
		LinkID:      link.ID,
		Email:       email,
		Token:       pgtype.Text{String: "", Valid: false},
		TokenHash:   pgtype.Text{String: hashToken(token), Valid: true},
		Status:      "pending",
		ExpiresAt:   expiresAt,
		CreatedBy:   createdBy,
	})
	if err != nil {
		return LinkInvitation{}, fmt.Errorf("create invitation: %w", err)
	}

	if err := qtx.CreateLinkAccessRule(ctx, db.CreateLinkAccessRuleParams{
		TenantID:    link.TenantID,
		WorkspaceID: workspaceUUID,
		LinkID:      link.ID,
		RuleType:    "email",
		Value:       email,
		Action:      "allow",
		SortOrder:   0,
	}); err != nil {
		return LinkInvitation{}, fmt.Errorf("create allow rule: %w", err)
	}

	return invitationFromRaw(token, inv.ID, link.ID, email, inv.Status, expiresAt, pgtype.Timestamptz{}), nil
}

// hashToken returns the HMAC-SHA256 hash (hex) of an invite token.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// invitationFromRaw builds a LinkInvitation domain object from a raw token and
// database fields. The raw token is never persisted; it is only returned to the
// caller once, immediately after creation or reset.
func invitationFromRaw(token string, id, linkID pgtype.UUID, email, status string, expiresAt, usedAt pgtype.Timestamptz) LinkInvitation {
	inv := LinkInvitation{
		ID:     uuid.UUID(id.Bytes).String(),
		LinkID: uuid.UUID(linkID.Bytes).String(),
		Email:  email,
		Token:  token,
		Status: status,
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		inv.ExpiresAt = &t
	}
	if usedAt.Valid {
		t := usedAt.Time
		inv.UsedAt = &t
	}
	return inv
}

func (s *Service) sendInvitationEmail(ctx context.Context, inv LinkInvitation, linkName, linkURL string) {
	inviteURL := fmt.Sprintf("%s?inviteToken=%s", linkURL, inv.Token)
	s.emailSem <- struct{}{}
	go func() {
		defer func() { <-s.emailSem }()
		sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := s.mailer.SendEmail(sendCtx, mailer.EmailJob{
			EmailType: mailer.EmailTypeLinkInvite,
			Recipient: inv.Email,
			LinkName:  linkName,
			LinkURL:   inviteURL,
			TemplateVariables: map[string]string{
				"InvitationLink": inviteURL,
				"LinkName":       linkName,
				"Email":          inv.Email,
			},
		}); err != nil {
			logger.ErrorCtx(sendCtx, "failed to send invitation email", err,
				logger.Attr("email_local", localPart(inv.Email)),
			)
		}
	}()
}

func (s *Service) sendAccessNotificationEmail(ctx context.Context, recipient, linkName, visitorEmail, linkURL string) {
	s.emailSem <- struct{}{}
	go func() {
		defer func() { <-s.emailSem }()
		sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := s.mailer.SendEmail(sendCtx, mailer.EmailJob{
			EmailType: mailer.EmailTypeLinkAccess,
			Recipient: recipient,
			LinkName:  linkName,
			LinkURL:   linkURL,
			TemplateVariables: map[string]string{
				"VisitorEmail": visitorEmail,
				"LinkName":     linkName,
				"LinkURL":      linkURL,
			},
		}); err != nil {
			logger.ErrorCtx(sendCtx, "failed to send access notification email", err,
				logger.Attr("email_local", localPart(recipient)),
			)
		}
	}()
}

// recordSecurityEvent writes a security event for a link.
func (s *Service) recordSecurityEvent(ctx context.Context, link db.Link, visitorID, email, eventType, reason string) {
	if err := s.queries.CreateSecurityEvent(ctx, db.CreateSecurityEventParams{
		LinkID:    link.ID,
		EventType: eventType,
		VisitorID: pgtype.Text{String: visitorID, Valid: visitorID != ""},
		Email:     pgtype.Text{String: email, Valid: email != ""},
		Reason:    pgtype.Text{String: reason, Valid: reason != ""},
	}); err != nil {
		logger.ErrorCtx(ctx, "failed to record security event", err,
			logger.Attr("link_id", uuid.UUID(link.ID.Bytes).String()),
			logger.Attr("event_type", eventType),
		)
	}
}

func uuidParseNil() [16]byte {
	return [16]byte{}
}

// AccessRequest is the input for public access.
type AccessRequest struct {
	Email       string
	EmailCode   string
	Password    string
	NDAAgreed   bool
	InviteToken string
	IP          string
	UA          string
}

// AccessResult is returned after a successful access check.
type AccessResult struct {
	Link          db.Link
	VisitorID     string
	Email         string
	EmailVerified bool
	SessionToken  string // refreshed session token for sliding expiry; empty if no session was used
}

// LinkAccessRequest is the domain representation of a visitor access request.
type LinkAccessRequest struct {
	ID         string
	LinkID     string
	Email      string
	Reason     string
	Status     string
	ReviewedBy *string
	ReviewedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var (
	ErrAccessRequestBlocked = errors.New("this email is blocked from requesting access")
	ErrAccessRequestExists  = errors.New("an access request from this email is already pending")
)

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
	case "disabled":
		return AccessResult{}, ErrLinkDisabled
	case "revoked":
		return AccessResult{}, ErrLinkRevoked
	case "archived":
		return AccessResult{}, ErrLinkArchived
	case "expired":
		return AccessResult{}, ErrLinkExpired
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

	// Resolve effective email: invite token takes priority and is immutable.
	var effectiveEmail string
	if req.InviteToken != "" {
		inv, err := s.ResolveInviteToken(ctx, req.InviteToken)
		if err != nil {
			s.recordSecurityEvent(ctx, link, "", req.Email, "invite_token_failed", err.Error())
			return AccessResult{}, err
		}
		if inv.LinkID != uuid.UUID(link.ID.Bytes).String() {
			s.recordSecurityEvent(ctx, link, "", req.Email, "invite_token_failed", "invitation does not belong to link")
			return AccessResult{}, ErrLinkNotFound
		}
		effectiveEmail = inv.Email
	} else {
		effectiveEmail = strings.TrimSpace(req.Email)
	}

	// Evaluate access rules before any gate checks.
	eval, err := s.EvaluateAccessRules(ctx, uuid.UUID(link.ID.Bytes).String(), effectiveEmail)
	if err != nil {
		return AccessResult{}, fmt.Errorf("evaluate access rules: %w", err)
	}
	if !eval.Allowed {
		s.recordSecurityEvent(ctx, link, "", effectiveEmail, eval.Reason, "")
		return AccessResult{}, mapRuleError(eval.Reason)
	}

	if requiresEmail {
		if effectiveEmail == "" {
			return AccessResult{}, ErrRequiresEmail
		}
	}

	// Password verification.
	if link.RequirePassword {
		if err := s.verifyPassword(link.PasswordHash.String, req.Password); err != nil {
			s.recordSecurityEvent(ctx, link, "", effectiveEmail, "invalid_password", "")
			return AccessResult{}, err
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
		emailForRecords = effectiveEmail
	}

	visitorID := makeVisitorID(emailForRecords, req.UA)

	if requiresNDA {
		_, ndaErr := s.queries.CreateLinkNDAAgreement(ctx, db.CreateLinkNDAAgreementParams{
			TenantID:    link.TenantID,
			WorkspaceID: link.WorkspaceID,
			LinkID:      link.ID,
			VisitorID:   pgtype.Text{String: visitorID, Valid: visitorID != ""},
			Email:       pgtype.Text{String: emailForRecords, Valid: emailForRecords != ""},
			Ip:          hashIPText(s.cfg.IPHashKey, req.IP),
			UserAgent:   pgtype.Text{String: req.UA, Valid: req.UA != ""},
		})
		if ndaErr != nil {
			logger.ErrorCtx(ctx, "create link NDA agreement failed", ndaErr,
				logger.Attr("link_id", uuid.UUID(link.ID.Bytes).String()),
			)
		}
	}

	// Mark invitation as used/verified if present.
	if req.InviteToken != "" {
		if inv, err := s.queries.GetLinkInvitationByToken(ctx, pgtype.Text{String: hashToken(req.InviteToken), Valid: true}); err == nil {
			usedAt := pgtype.Timestamptz{Valid: true, Time: time.Now()}
			status := "verified"
			if inv.Status == "pending" {
				status = "opened"
			}
			if _, err := s.queries.UpdateLinkInvitationStatus(ctx, db.UpdateLinkInvitationStatusParams{
				Status: status,
				UsedAt: usedAt,
				ID:     inv.ID,
			}); err != nil {
				logger.ErrorCtx(ctx, "update invitation status failed", err)
			}
		}
	}

	if link.NotifyOnAccess && emailForRecords != "" && link.CreatedBy.Valid {
		if creator, err := s.queries.GetUserByID(ctx, link.CreatedBy); err == nil && creator.Email != "" {
			s.sendAccessNotificationEmail(ctx, creator.Email, link.Name.String, emailForRecords, publicLinkURL(s.viewerBaseURL, link.PublicToken, link.CustomDomain.String))
		}
	}

	return AccessResult{Link: link, VisitorID: visitorID, Email: emailForRecords, EmailVerified: requiresEmailVerification}, nil
}

// mapRuleError maps an access rule evaluation reason to a public error.
func mapRuleError(reason string) error {
	switch reason {
	case "blocked_email":
		return ErrBlockedEmail
	case "blocked_domain":
		return ErrBlockedDomain
	case "no_allow_match":
		if strings.Contains(reason, "domain") {
			return ErrNotAllowedDomain
		}
		return ErrNotAllowedEmail
	default:
		return ErrNotAllowedEmail
	}
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

	linkURL := publicLinkURL(viewerBaseURL, link.PublicToken, link.CustomDomain.String)
	if _, err := s.mailer.SendLinkAccessCodeEmail(ctx, email, lc.AccessCode, link.Name.String, linkURL); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

func publicLinkURL(baseURL, token, customDomain string) string {
	if customDomain != "" {
		scheme := "https"
		if baseURL != "" {
			if u, err := url.Parse(baseURL); err == nil && u.Scheme != "" {
				scheme = u.Scheme
			}
		}
		host := strings.TrimRight(customDomain, "/")
		return scheme + "://" + host + "/l/" + token
	}
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
	key := fmt.Sprintf("link:access:ratelimit:%s:%s", token, hashIPForRateLimit(s.cfg.IPHashKey, ip))
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

func hashIPForRateLimit(key, ip string) string {
	return compliance.ShortHashIP(key, ip, 16)
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

// GetByPublicToken returns a link by its public token.
func (s *Service) GetByPublicToken(ctx context.Context, publicToken string) (db.Link, error) {
	link, err := s.queries.GetLinkByPublicToken(ctx, publicToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Link{}, ErrLinkNotFound
		}
		return db.Link{}, fmt.Errorf("get link by public token: %w", err)
	}
	return db.Link{
		ID:                       link.ID,
		TenantID:                 link.TenantID,
		WorkspaceID:              link.WorkspaceID,
		DocumentID:               link.DocumentID,
		DealRoomID:               link.DealRoomID,
		PublicToken:              link.PublicToken,
		Name:                     link.Name,
		PermissionType:           link.PermissionType,
		ExpiresAt:                link.ExpiresAt,
		MaxAccessCount:           link.MaxAccessCount,
		AccessCount:              link.AccessCount,
		DownloadEnabled:          link.DownloadEnabled,
		WatermarkEnabled:         link.WatermarkEnabled,
		Status:                   link.Status,
		CreatedBy:                link.CreatedBy,
		CreatedAt:                link.CreatedAt,
		UpdatedAt:                link.UpdatedAt,
		RequireEmail:             link.RequireEmail,
		RequireNda:               link.RequireNda,
		RequireEmailVerification: link.RequireEmailVerification,
		AiCopilotEnabled:         link.AiCopilotEnabled,
		RequirePassword:          link.RequirePassword,
		PasswordHash:             link.PasswordHash,
	}, nil
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

// ArchiveLink soft-archives an active link, denying public access.
// Archived links can be renewed to restore access.
func (s *Service) ArchiveLink(ctx context.Context, workspaceID, linkID string) (db.Link, error) {
	link, err := s.GetByID(ctx, linkID, workspaceID)
	if err != nil {
		return db.Link{}, err
	}
	if link.Status == "deleted" {
		return db.Link{}, ErrNotFoundInWorkspace
	}
	if link.Status == "archived" {
		return link, nil // idempotent
	}
	if _, err := s.queries.UpdateLinkStatus(ctx, db.UpdateLinkStatusParams{
		ID:          link.ID,
		WorkspaceID: link.WorkspaceID,
		Status:      "archived",
	}); err != nil {
		return db.Link{}, fmt.Errorf("archive link: %w", err)
	}
	s.recordSecurityEvent(ctx, link, "", "", "link_archived", "")
	return s.GetByID(ctx, linkID, workspaceID)
}

// RenewLink reactivates an archived or expired link, optionally extending its expiry.
func (s *Service) RenewLink(ctx context.Context, workspaceID, linkID string, newExpiresAt *time.Time) (db.Link, error) {
	link, err := s.GetByID(ctx, linkID, workspaceID)
	if err != nil {
		return db.Link{}, err
	}
	if link.Status == "deleted" {
		return db.Link{}, ErrNotFoundInWorkspace
	}
	if link.Status != "archived" && link.Status != "expired" {
		return db.Link{}, errors.New("only archived or expired links can be renewed")
	}
	// Validate new expiry date is in the future.
	if newExpiresAt != nil && newExpiresAt.Before(time.Now()) {
		return db.Link{}, errors.New("expiry date must be in the future")
	}
	expiresAt := link.ExpiresAt
	if newExpiresAt != nil {
		expiresAt = pgtype.Timestamptz{Time: *newExpiresAt, Valid: true}
	}
	// Bump security_version so any stale sessions are invalidated.
	newVersion := link.SecurityVersion + 1
	if _, err := s.queries.UpdateLinkFull(ctx, db.UpdateLinkFullParams{
		Name:                     link.Name,
		DocumentID:               link.DocumentID,
		DealRoomID:               link.DealRoomID,
		PermissionType:           link.PermissionType,
		ExpiresAt:                expiresAt,
		MaxAccessCount:           link.MaxAccessCount,
		DownloadEnabled:          link.DownloadEnabled,
		WatermarkEnabled:         link.WatermarkEnabled,
		RequireEmail:             link.RequireEmail,
		RequireEmailVerification: link.RequireEmailVerification,
		RequireNda:               link.RequireNda,
		AiCopilotEnabled:         link.AiCopilotEnabled,
		RequirePassword:          link.RequirePassword,
		PasswordHash:             link.PasswordHash,
		CustomDomain:             link.CustomDomain,
		Tags:                     link.Tags,
		NotifyOnAccess:           link.NotifyOnAccess,
		QaEnabled:                link.QaEnabled,
		FileRequestsEnabled:      link.FileRequestsEnabled,
		IndexFileEnabled:         link.IndexFileEnabled,
		SecurityVersion:          newVersion,
		ID:                       link.ID,
		WorkspaceID:              link.WorkspaceID,
	}); err != nil {
		return db.Link{}, fmt.Errorf("renew link: %w", err)
	}
	// Reset status to active.
	if _, err := s.queries.UpdateLinkStatus(ctx, db.UpdateLinkStatusParams{
		ID:          link.ID,
		WorkspaceID: link.WorkspaceID,
		Status:      "active",
	}); err != nil {
		return db.Link{}, fmt.Errorf("reactivate link: %w", err)
	}
	s.recordSecurityEvent(ctx, link, "", "", "link_renewed", "")
	return s.GetByID(ctx, linkID, workspaceID)
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

	// NDA requires email collection but not a verification code.
	if !requireEmail && requireNDA {
		requireEmail = true
	}

	// Allow-list rules require email collection so there is an email to evaluate.
	if !requireEmail && (len(req.AllowedEmails) > 0 || len(req.AllowedDomains) > 0) {
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

// hashPasswordIfRequired hashes a plaintext password when password protection is enabled.
// When disabled it returns an empty (invalid) Text so the column stays NULL.
func (s *Service) hashPasswordIfRequired(requirePassword bool, password string) (pgtype.Text, error) {
	if !requirePassword {
		return pgtype.Text{}, nil
	}
	if strings.TrimSpace(password) == "" {
		return pgtype.Text{}, ErrRequiresPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return pgtype.Text{}, fmt.Errorf("hash password: %w", err)
	}
	return pgtype.Text{String: string(hash), Valid: true}, nil
}

// verifyPassword compares a plaintext password with the stored bcrypt hash using
// constant-time comparison to avoid leaking timing information.
func (s *Service) verifyPassword(passwordHash, password string) error {
	if strings.TrimSpace(passwordHash) == "" {
		return ErrInvalidPassword
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return ErrInvalidPassword
	}
	return nil
}

// resolvePasswordHashForUpdate decides the password hash to store during an update.
// - If password protection is disabled, the hash is cleared.
// - If password protection is enabled and a new password is provided, it is hashed.
// - If password protection is enabled and no password is provided, the existing hash is kept.
func (s *Service) resolvePasswordHashForUpdate(existing db.Link, requirePassword bool, password string) (pgtype.Text, error) {
	if !requirePassword {
		return pgtype.Text{}, nil
	}
	if strings.TrimSpace(password) != "" {
		return s.hashPasswordIfRequired(true, password)
	}
	if existing.RequirePassword && existing.PasswordHash.Valid {
		return existing.PasswordHash, nil
	}
	return pgtype.Text{}, ErrRequiresPassword
}

// constantTimeEmailCompare performs a case-insensitive constant-time comparison
// of two email addresses to avoid timing side channels.
func constantTimeEmailCompare(a, b string) bool {
	return subtle.ConstantTimeCompare(
		[]byte(strings.ToLower(strings.TrimSpace(a))),
		[]byte(strings.ToLower(strings.TrimSpace(b))),
	) == 1
}

// CreateVisitorQuestion creates a new visitor question on a public link.
// The qa_enabled flag must be checked by the caller before invoking this method.
func (s *Service) CreateVisitorQuestion(ctx context.Context, link db.Link, visitorID, visitorEmail, question string) (db.LinkVisitorQuestion, error) {
	if strings.TrimSpace(question) == "" {
		return db.LinkVisitorQuestion{}, fmt.Errorf("question is required")
	}
	if len(question) > 500 {
		return db.LinkVisitorQuestion{}, fmt.Errorf("question must not exceed 500 characters")
	}

	return s.queries.CreateVisitorQuestion(ctx, db.CreateVisitorQuestionParams{
		TenantID:     link.TenantID,
		WorkspaceID:  link.WorkspaceID,
		LinkID:       link.ID,
		VisitorID:    visitorID,
		VisitorEmail: pgtype.Text{String: visitorEmail, Valid: visitorEmail != ""},
		Question:     strings.TrimSpace(question),
	})
}

// ListMyVisitorQuestions returns all questions submitted by a specific visitor on a link.
func (s *Service) ListMyVisitorQuestions(ctx context.Context, linkID pgtype.UUID, visitorID string) ([]db.LinkVisitorQuestion, error) {
	return s.queries.ListVisitorQuestionsByVisitor(ctx, db.ListVisitorQuestionsByVisitorParams{
		LinkID:    linkID,
		VisitorID: visitorID,
	})
}

// ListLinkVisitorQuestions returns all questions for a link (owner view).
func (s *Service) ListLinkVisitorQuestions(ctx context.Context, linkID pgtype.UUID) ([]db.LinkVisitorQuestion, error) {
	return s.queries.ListVisitorQuestionsByLink(ctx, linkID)
}

// AnswerVisitorQuestion records an answer to a visitor question.
// The caller must verify workspace ownership before invoking.
func (s *Service) AnswerVisitorQuestion(ctx context.Context, questionID, workspaceID, userID pgtype.UUID, answer string) (db.LinkVisitorQuestion, error) {
	if strings.TrimSpace(answer) == "" {
		return db.LinkVisitorQuestion{}, fmt.Errorf("answer is required")
	}
	return s.queries.AnswerVisitorQuestion(ctx, db.AnswerVisitorQuestionParams{
		Answer:      pgtype.Text{String: strings.TrimSpace(answer), Valid: true},
		AnsweredBy:  userID,
		ID:          questionID,
		WorkspaceID: workspaceID,
	})
}

const maxPendingFileRequestsPerVisitor = 3

// CreateFileRequest allows a visitor to request a missing file from the link owner.
func (s *Service) CreateFileRequest(ctx context.Context, link db.Link, visitorID, visitorEmail, message string) (db.LinkFileRequest, error) {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return db.LinkFileRequest{}, fmt.Errorf("message is required")
	}
	if len(msg) > 500 {
		return db.LinkFileRequest{}, fmt.Errorf("message must not exceed 500 characters")
	}

	count, err := s.queries.CountPendingFileRequestsByVisitor(ctx, db.CountPendingFileRequestsByVisitorParams{
		LinkID:    link.ID,
		VisitorID: pgtype.Text{String: visitorID, Valid: true},
	})
	if err != nil {
		return db.LinkFileRequest{}, fmt.Errorf("count pending: %w", err)
	}
	if count >= maxPendingFileRequestsPerVisitor {
		return db.LinkFileRequest{}, fmt.Errorf("too many pending requests")
	}

	return s.queries.CreateFileRequest(ctx, db.CreateFileRequestParams{
		TenantID:     link.TenantID,
		WorkspaceID:  link.WorkspaceID,
		LinkID:       link.ID,
		VisitorID:    pgtype.Text{String: visitorID, Valid: visitorID != ""},
		VisitorEmail: pgtype.Text{String: visitorEmail, Valid: visitorEmail != ""},
		Message:      msg,
	})
}

// ListMyFileRequests returns file requests submitted by a visitor on a link.
func (s *Service) ListMyFileRequests(ctx context.Context, linkID pgtype.UUID, visitorID string) ([]db.LinkFileRequest, error) {
	return s.queries.ListFileRequestsByVisitor(ctx, db.ListFileRequestsByVisitorParams{
		LinkID:    linkID,
		VisitorID: pgtype.Text{String: visitorID, Valid: true},
	})
}

// ListLinkFileRequests returns all file requests for a link (owner view).
func (s *Service) ListLinkFileRequests(ctx context.Context, linkID pgtype.UUID) ([]db.LinkFileRequest, error) {
	return s.queries.ListFileRequestsByLink(ctx, linkID)
}

// UpdateFileRequestStatus changes the status of a file request.
func (s *Service) UpdateFileRequestStatus(ctx context.Context, requestID pgtype.UUID, status string) error {
	return s.queries.UpdateFileRequestStatus(ctx, db.UpdateFileRequestStatusParams{
		Status: status,
		ID:     requestID,
	})
}

// GetFileRequestByID returns a single file request.
func (s *Service) GetFileRequestByID(ctx context.Context, id pgtype.UUID) (db.LinkFileRequest, error) {
	return s.queries.GetFileRequestByID(ctx, id)
}

// GetLinkIndexFileByLink returns the index file for a link.
func (s *Service) GetLinkIndexFileByLink(ctx context.Context, linkID pgtype.UUID) (db.LinkIndexFile, error) {
	return s.queries.GetLinkIndexFileByLink(ctx, linkID)
}

// GenerateIndexFile creates or regenerates an AI-powered summary index for a link.
// Returns the index file record. The caller must verify the link belongs to the workspace.
func (s *Service) GenerateIndexFile(ctx context.Context, link db.Link) (db.LinkIndexFile, error) {
	if s.llm == nil {
		_ = s.queries.UpdateLinkIndexFileFailed(ctx, db.UpdateLinkIndexFileFailedParams{
			ErrorMessage: pgtype.Text{String: "AI service is not configured", Valid: true},
			LinkID:       link.ID,
		})
		return db.LinkIndexFile{}, fmt.Errorf("AI service not configured")
	}

	if _, err := s.queries.UpsertLinkIndexFile(ctx, db.UpsertLinkIndexFileParams{
		TenantID:    link.TenantID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	}); err != nil {
		return db.LinkIndexFile{}, fmt.Errorf("upsert index file: %w", err)
	}

	docTitles := s.collectDocTitles(ctx, link)
	if len(docTitles) == 0 {
		docTitles = []string{link.Name.String}
	}

	systemPrompt := "You are an AI assistant that creates executive summaries of shared documents. Generate a concise index with: 1) a 2-3 sentence executive summary, 2) a bullet-point list of key topics covered, and 3) a recommended reading order. Format the output in HTML (without <html>/<body> tags). Use <h2>, <p>, <ul>/<li>. Keep it under 2000 characters. Do NOT make up content not in the documents."

	docContext := "Documents in this link:\n"
	for _, t := range docTitles {
		docContext += "- " + t + "\n"
	}

	content, chatErr := s.llm.ChatCompletion(ctx, systemPrompt, []llmMessage{
		{Role: "user", Content: fmt.Sprintf("Generate an AI-generated index/summary for the following documents. Label the output as AI-generated.\n\n%s", docContext)},
	})

	if chatErr != nil {
		_ = s.queries.UpdateLinkIndexFileFailed(ctx, db.UpdateLinkIndexFileFailedParams{
			ErrorMessage: pgtype.Text{String: chatErr.Error(), Valid: true},
			LinkID:       link.ID,
		})
		return db.LinkIndexFile{}, fmt.Errorf("AI generation failed: %w", chatErr)
	}

	sanitized := sanitizeHTML(content)
	if err := s.queries.UpdateLinkIndexFileReady(ctx, db.UpdateLinkIndexFileReadyParams{
		ContentHtml: pgtype.Text{String: sanitized, Valid: true},
		LinkID:      link.ID,
	}); err != nil {
		return db.LinkIndexFile{}, fmt.Errorf("update index file ready: %w", err)
	}

	row, _ := s.queries.GetLinkIndexFileByLink(ctx, link.ID)
	return row, nil
}

func (s *Service) collectDocTitles(ctx context.Context, link db.Link) []string {
	var titles []string
	if link.DocumentID.Valid {
		doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
			ID:          link.DocumentID,
			WorkspaceID: link.WorkspaceID,
		})
		if err == nil && doc.Title != "" {
			titles = append(titles, doc.Title)
		}
	}
	// For deal room links, generate index from the deal room context.
	if link.DealRoomID.Valid {
		room, err := s.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
			ID:          link.DealRoomID,
			WorkspaceID: link.WorkspaceID,
		})
		if err == nil && room.Name != "" {
			titles = append(titles, room.Name)
		}
	}
	return titles
}

// sanitizeHTML removes script, iframe, and object tags for basic XSS prevention.
func sanitizeHTML(html string) string {
	html = strings.ReplaceAll(html, "<script", "<!--removed-script")
	html = strings.ReplaceAll(html, "</script>", "removed-script-->")
	html = strings.ReplaceAll(html, "<iframe", "<!--removed-iframe")
	html = strings.ReplaceAll(html, "</iframe>", "removed-iframe-->")
	html = strings.ReplaceAll(html, "<object", "<!--removed-object")
	html = strings.ReplaceAll(html, "</object>", "removed-object-->")
	return html
}

// FileUploader abstracts file storage for uploaded files.
type FileUploader interface {
	PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
}

var allowedUploadMimeTypes = map[string]bool{
	"application/pdf": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
	"application/zip": true,
}

// UploadFileForLink stores a file uploaded through a file-request link.
func (s *Service) UploadFileForLink(ctx context.Context, storage FileUploader, link db.Link, filename, mimeType string, size int64, body io.Reader, visitorID, visitorEmail, ip, ua string) (db.LinkUploadedFile, error) {
	if link.LinkType != "file_request" {
		return db.LinkUploadedFile{}, fmt.Errorf("link is not a file request link")
	}
	if !allowedUploadMimeTypes[mimeType] {
		return db.LinkUploadedFile{}, fmt.Errorf("unsupported file type: %s", mimeType)
	}
	const maxSize = 50 * 1024 * 1024
	if size > maxSize {
		return db.LinkUploadedFile{}, fmt.Errorf("file too large (max 50MB)")
	}
	if size <= 0 {
		return db.LinkUploadedFile{}, fmt.Errorf("file is empty")
	}

	storageKey := fmt.Sprintf("uploads/%s/%s/%s", uuid.New().String(), link.PublicToken, filename)
	if err := storage.PutObject(ctx, storageKey, body, size, mimeType); err != nil {
		return db.LinkUploadedFile{}, fmt.Errorf("store upload: %w", err)
	}

	return s.queries.CreateUploadedFile(ctx, db.CreateUploadedFileParams{
		TenantID:          link.TenantID,
		WorkspaceID:       link.WorkspaceID,
		LinkID:            link.ID,
		OriginalFilename:  filename,
		StorageKey:        storageKey,
		FileSize:          size,
		MimeType:          mimeType,
		UploaderEmail:     pgtype.Text{String: visitorEmail, Valid: visitorEmail != ""},
		UploaderVisitorID: pgtype.Text{String: visitorID, Valid: visitorID != ""},
		UploaderIp:        hashIPText(s.cfg.IPHashKey, ip),
		UploaderUserAgent: pgtype.Text{String: ua, Valid: ua != ""},
	})
}

// ListUploadedFiles returns all uploaded files for a link.
func (s *Service) ListUploadedFiles(ctx context.Context, linkID pgtype.UUID) ([]db.LinkUploadedFile, error) {
	return s.queries.ListUploadedFilesByLink(ctx, linkID)
}

// ApproveUploadedFile approves a pending uploaded file.
func (s *Service) ApproveUploadedFile(ctx context.Context, fileID pgtype.UUID, reviewerID pgtype.UUID) error {
	return s.queries.UpdateUploadedFileStatus(ctx, db.UpdateUploadedFileStatusParams{
		Status:     "approved",
		ReviewedBy: reviewerID,
		ID:         fileID,
	})
}

// RejectUploadedFile rejects a pending uploaded file.
func (s *Service) RejectUploadedFile(ctx context.Context, fileID pgtype.UUID, reviewerID pgtype.UUID) error {
	return s.queries.UpdateUploadedFileStatus(ctx, db.UpdateUploadedFileStatusParams{
		Status:     "rejected",
		ReviewedBy: reviewerID,
		ID:         fileID,
	})
}

// GetUploadedFileByID returns a single uploaded file record.
func (s *Service) GetUploadedFileByID(ctx context.Context, id pgtype.UUID) (db.LinkUploadedFile, error) {
	return s.queries.GetUploadedFileByID(ctx, id)
}

// ClassifyQuestionIntent runs an LLM-powered intent classification on a visitor
// question and stores the result. Called asynchronously after question creation.
func (s *Service) ClassifyQuestionIntent(ctx context.Context, questionID pgtype.UUID, questionText string) {
	if s.llm == nil || questionText == "" {
		return
	}

	systemPrompt := "You are an intent classifier for document sharing Q&A. " +
		"Analyze the question and respond with exactly ONE label from: " +
		"pricing, security, timeline, implementation, feature_request, support, objection, general. " +
		"Output only the label, no explanation."

	label, err := s.llm.ChatCompletion(ctx, systemPrompt, []llmMessage{
		{Role: "user", Content: questionText},
	})
	if err != nil {
		return
	}

	label = strings.TrimSpace(label)
	if label == "" {
		return
	}

	_ = s.queries.UpdateQuestionIntentTag(ctx, db.UpdateQuestionIntentTagParams{
		IntentTag: label,
		ID:        questionID,
	})
}

// ListDormantLinks returns links that were active but went cold, ranked by reactivation potential.
func (s *Service) ListDormantLinks(ctx context.Context, workspaceID pgtype.UUID) ([]db.ListDormantLinksRow, error) {
	return s.queries.ListDormantLinks(ctx, workspaceID)
}

func hashIPText(key, ip string) pgtype.Text {
	if ip == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: compliance.HashIP(key, ip), Valid: true}
}
