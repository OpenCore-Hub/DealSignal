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
	"net/netip"
	"net/url"
	"sort"
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
	ExpiresAt                *time.Time
	MaxAccessCount           *int32
	DownloadEnabled          bool
	WatermarkEnabled         bool
	AICopilotEnabled         bool
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
	ExpiresAt                *time.Time
	MaxAccessCount           *int32
	DownloadEnabled          bool
	WatermarkEnabled         bool
	AICopilotEnabled         bool
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
		RequirePassword:          req.RequirePassword,
		PasswordHash:             passwordHash,
		CustomDomain:             customDomain,
		Tags:                     tags,
		NotifyOnAccess:           req.NotifyOnAccess,
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
			inv, err := qtx.ResetLinkInvitation(ctx, db.ResetLinkInvitationParams{
				Token:     token,
				ExpiresAt: expiresAt,
				ID:        existing.ID,
			})
			if err != nil {
				return nil, fmt.Errorf("reset invitation: %w", err)
			}
			invitations = append(invitations, dbInvitationToDomain(inv))
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
			Token:       token,
			Status:      "pending",
			ExpiresAt:   expiresAt,
			CreatedBy:   userUUID,
		})
		if err != nil {
			return nil, fmt.Errorf("create invitation: %w", err)
		}
		invitations = append(invitations, dbInvitationToDomain(inv))
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
	inv, err := s.queries.GetLinkInvitationByToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LinkInvitation{}, ErrLinkNotFound
		}
		return LinkInvitation{}, fmt.Errorf("get invitation: %w", err)
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

func dbInvitationToDomain(inv db.LinkInvitation) LinkInvitation {
	var expiresAt, usedAt *time.Time
	if inv.ExpiresAt.Valid {
		expiresAt = &inv.ExpiresAt.Time
	}
	if inv.UsedAt.Valid {
		usedAt = &inv.UsedAt.Time
	}
	return LinkInvitation{
		ID:        uuid.UUID(inv.ID.Bytes).String(),
		LinkID:    uuid.UUID(inv.LinkID.Bytes).String(),
		Email:     inv.Email,
		Token:     inv.Token,
		Status:    inv.Status,
		ExpiresAt: expiresAt,
		UsedAt:    usedAt,
	}
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

	// Mark invitation as used/verified if present.
	if req.InviteToken != "" {
		if inv, err := s.queries.GetLinkInvitationByToken(ctx, req.InviteToken); err == nil {
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

