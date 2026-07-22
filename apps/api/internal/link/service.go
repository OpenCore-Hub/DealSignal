// Package link implements smart-link creation, permission checks and public access.
package link

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/compliance"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/nda"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/notification"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/singleflight"
)

// Beginner starts a database transaction.
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// emailCode holds an email address and its access code for async delivery.
// epoch ties the outbound message to the latest code rotation for that
// recipient so a delayed create-time send cannot overwrite a newer manual resend.
type emailCode struct {
	email string
	code  string
	epoch uint64
}

// Notifier is the subset of notification.Service needed by link.Service.
type Notifier interface {
	Enqueue(ctx context.Context, workspaceID, userID, channel, subject, body string, opts ...notification.EnqueueOption) (notification.Notification, error)
	Evaluate(ctx context.Context, ev notification.Event) error
}

// Service handles smart links.
type Service struct {
	queries        *db.Queries
	pool           Beginner
	redisClient    *redis.Client
	mailer         mailer.Mailer
	notifier       Notifier
	viewerBaseURL  string
	cfg            *config.Config
	llm            LLMClient
	emailSem       chan struct{} // limits concurrent email sends (bounded goroutines)
	indexGenGroup  singleflight.Group
	actionSyncer   ActionSyncer
	ndaSvc         *nda.Service
	// accessCodeEpoch tracks the latest code-rotation generation per
	// publicToken+email so async sends can detect superseded codes without
	// touching the DB (safe under the integration-test shared-tx fixture).
	accessCodeEpoch sync.Map // map[string]*uint64
}

// ActionSyncer resolves operational action items when link events are handled.
type ActionSyncer interface {
	ResolveBySource(ctx context.Context, workspaceID, sourceType, sourceID string)
}

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// WithActionSyncer wires an action syncer so link events can resolve action items.
func WithActionSyncer(a ActionSyncer) ServiceOption {
	return func(s *Service) { s.actionSyncer = a }
}

// WithNDAService wires One-Click NDA sealing and notifications.
func WithNDAService(n *nda.Service) ServiceOption {
	return func(s *Service) { s.ndaSvc = n }
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
func NewService(q *db.Queries, pool Beginner, r *redis.Client, m mailer.Mailer, viewerBaseURL string, cfg *config.Config, n Notifier, llm LLMClient, opts ...ServiceOption) *Service {
	s := &Service{
		queries: q, pool: pool, redisClient: r, mailer: m, notifier: n, viewerBaseURL: viewerBaseURL,
		cfg:      cfg,
		llm:      llm,
		emailSem: make(chan struct{}, 8), // cap concurrent email goroutines
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

var (
	ErrDocumentNotReady     = errors.New("document is not ready")
	ErrInvalidInput         = errors.New("invalid input")
	ErrInvalidPermission    = errors.New("invalid permission configuration")
	ErrLinkNotFound         = errors.New("link not found")
	ErrLinkExpired          = errors.New("link expired")
	ErrLinkArchived         = errors.New("link archived")
	ErrLinkRevoked          = errors.New("link revoked")
	ErrLinkDisabled         = errors.New("link disabled")
	ErrLinkMaxAccessReached = errors.New("link max access reached")
	ErrRequiresEmail        = errors.New("email required")
	ErrRequiresNDA          = errors.New("nda agreement required")
	ErrInvalidSignerName    = errors.New("signer name is required")
	ErrRequiresEmailCode    = errors.New("email verification code required")
	ErrInvalidEmailCode     = errors.New("invalid email verification code")
	ErrNotFoundInWorkspace  = errors.New("link not found in workspace")

	// Deal-room sharing / access-rule errors.
	ErrDealRoomNotFound      = errors.New("deal room not found")
	ErrBlockedEmail          = errors.New("email is blocked")
	ErrNotAllowedEmail       = errors.New("email is not allowed")
	// ErrDeliveryEmailMismatch means the visitor submitted an email that does not
	// match the address bound to a valid verification code. Handlers MUST NOT
	// expose AuthorizedEmail to clients (privacy); it is audit-only.
	ErrDeliveryEmailMismatch = errors.New("delivery email does not match verified email")
	ErrRequiresPassword      = errors.New("password required")
	ErrInvalidPassword       = errors.New("invalid password")
	ErrInviteExpired         = errors.New("invitation expired")
	ErrInviteRevoked         = errors.New("invitation revoked")
	ErrInviteAlreadyUsed     = errors.New("invitation already used")
	ErrInvalidAccessRule     = errors.New("invalid access rule")
	ErrConflictingAccessRule = errors.New("conflicting access rule")
	ErrDuplicateName             = errors.New("a link with this name already exists")
	ErrEmailCodeRateLimited      = errors.New("too many verification code requests, please try again later")
	ErrAccessCodeContactNotFound = errors.New("access code contact not found")
	ErrAccessCodeResendNotNeeded = errors.New("access code already delivered; pass force=true to resend")
	ErrEmailVerificationDisabled = errors.New("email verification is not enabled for this link")
	// ErrAccessCodeSendFailed means the access request was approved (allow rule
	// committed) but the verification-code email could not be delivered. Callers
	// should surface this so the owner can retry via resend.
	ErrAccessCodeSendFailed = errors.New("access approved but verification code could not be sent")
	ErrAccessAlreadyAllowed = errors.New("email already has access")
)

// ownerResendPendingStale is how long a contact may stay "pending" before the
// owner remediates UI treats it as stuck (async send likely lost). Shorter
// windows would encourage duplicate sends while the background job is in flight.
const ownerResendPendingStale = 2 * time.Minute

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
	if link.Status == "archived" {
		return db.Link{}, ErrLinkArchived
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
		WorkspaceID:     uuid.UUID(link.WorkspaceID.Bytes).String(),
		LinkID:          uuid.UUID(link.ID.Bytes).String(),
		EventType:       eventType,
		VisitorID:       visitorID,
		VisitorEmail:    visitorEmail,
		RecipientUserID: uuid.UUID(link.CreatedBy.Bytes).String(),
		Metadata:        metadata,
	})
}

// CreateLinkRequest is the input for creating a link.
type CreateLinkRequest struct {
	DocumentID                  string
	DocumentIDs                 []string // Multi-document bundle (takes precedence when non-empty)
	DealRoomID                  string
	Name                        string
	PermissionType              string
	RequireEmail                bool
	RequireEmailVerification    bool
	RequireNDA                  bool
	NDADocumentID               string
	NDATemplateID               string
	RequirePassword             bool
	Password                    string // plaintext; stored as bcrypt hash
	AllowedEmails               []string
	BlockedEmails               []string
	ExpiresAt                   *time.Time
	MaxAccessCount              *int32
	DownloadEnabled             bool
	WatermarkEnabled            bool
	AICopilotEnabled            bool
	QaEnabled                   bool
	FileRequestsEnabled         bool
	IndexFileEnabled            bool
	ScreenshotProtectionEnabled bool
	LinkType                    string // "share" or "file_request"
	TargetFolderPath            string // required when LinkType == "file_request"
	ContactIDs                  []string
	CustomDomain                string
	Tags                        []string
	NotifyOnAccess              bool
	// FolderPaths scopes a deal-room link to a set of folder paths when the
	// link uses allowlist mode. Empty allowlist denies all documents.
	FolderPaths []string
	// FolderScopeMode is "full" (legacy whole-room) or "allowlist".
	// Deal-room creates always persist allowlist. Omit on update to leave unchanged
	// unless FolderPaths is provided (which forces allowlist).
	FolderScopeMode string
}

// UpdateLinkRequest is the input for updating an existing link (full replacement).
type UpdateLinkRequest struct {
	DocumentIDs                 []string
	DealRoomID                  string
	Name                        string
	PermissionType              string
	RequireEmail                bool
	RequireEmailVerification    bool
	RequireNDA                  bool
	NDADocumentID               string
	NDATemplateID               string
	RequirePassword             bool
	Password                    string // plaintext; if empty and require_password unchanged, keep existing hash
	AllowedEmails               []string
	ExpiresAt                   *time.Time
	MaxAccessCount              *int32
	DownloadEnabled             bool
	WatermarkEnabled            bool
	AICopilotEnabled            bool
	QaEnabled                   bool
	FileRequestsEnabled         bool
	IndexFileEnabled            bool
	ScreenshotProtectionEnabled bool
	LinkType                    string
	TargetFolderPath            string
	ContactIDs                  []string
	CustomDomain                string
	Tags                        []string
	NotifyOnAccess              bool
	// FolderPaths scopes a deal-room link to a set of folder paths when the
	// link uses allowlist mode. Empty allowlist denies all documents.
	FolderPaths []string
	// FolderScopeMode is "full" (legacy whole-room) or "allowlist".
	// Deal-room creates always persist allowlist. Omit on update to leave unchanged
	// unless FolderPaths is provided (which forces allowlist).
	FolderScopeMode string
}

// AccessRule represents a single allow/block rule for a link.
type AccessRule struct {
	RuleType string `json:"ruleType"` // "email"
	Value    string `json:"value"`
	Action   string `json:"action"` // "allow" or "block"
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
	NDADocumentID            string
	NDATemplateID            string
	RequirePassword          bool
	Password                 string
	AllowedEmails            []string
	BlockedEmails            []string
	ExpiresAt                *time.Time
	DownloadEnabled          bool
	WatermarkEnabled         bool
	AICopilotEnabled         bool
	QaEnabled                bool
	FileRequestsEnabled      bool
	IndexFileEnabled         bool
	ScreenshotProtectionEnabled bool
	CustomDomain             string
	Tags                     []string
	NotifyOnAccess           bool
	// FolderPaths is the allowlist of deal-room folders. Empty deny-all.
	// New deal-room links always persist folder_scope_mode=allowlist.
	FolderPaths []string
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
	// Deal-room links use folder paths for scoping, never document IDs.
	if hasDealRoom && (req.DocumentID != "" || len(req.DocumentIDs) > 0) {
		return db.Link{}, errors.New("deal-room links cannot use document_id or document_ids")
	}

	linkType := req.LinkType
	if linkType == "" {
		linkType = "share"
	}
	if linkType != "share" && linkType != "file_request" {
		return db.Link{}, fmt.Errorf("%w: link_type must be 'share' or 'file_request'", ErrInvalidInput)
	}
	if linkType == "file_request" && !hasDealRoom {
		return db.Link{}, fmt.Errorf("%w: file_request links must be associated with a deal_room_id", ErrInvalidInput)
	}
	if linkType == "file_request" && hasDocuments {
		return db.Link{}, fmt.Errorf("%w: file_request links cannot be associated with documents", ErrInvalidInput)
	}
	targetFolderPath := req.TargetFolderPath
	if targetFolderPath == "" {
		targetFolderPath = "/Uploads"
	}

	requireEmail, requireEmailVerification, requireNDA, perm, err := normalizeSecurityConfig(req)
	if err != nil {
		return db.Link{}, err
	}

	// Email verification for document links needs at least one pre-defined contact
	// so the access code has a destination. Deal-room links skip this requirement;
	// their access flow uses the code itself to identify the visitor.
	if requireEmailVerification && !hasDealRoom && len(req.ContactIDs) == 0 {
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
	folderScopePaths := []string{}
	linkDocumentIDs := []string{}

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

		if err := ensureAskDocsKnowledgeBase(ctx, qtx, dealRoomID, req.AICopilotEnabled); err != nil {
			return db.Link{}, err
		}

		// Deal-room links always use allowlist mode (empty = deny-all).
		if err := s.validateDealRoomFolderPaths(ctx, qtx, workspaceUUID, dealRoomID, req.FolderPaths); err != nil {
			return db.Link{}, err
		}
		folderScopePaths = make([]string, len(req.FolderPaths))
		copy(folderScopePaths, req.FolderPaths)
	} else {
		// Resolve document IDs: use DocumentIDs if provided, else fall back to single DocumentID.
		linkDocumentIDs = req.DocumentIDs
		if len(linkDocumentIDs) == 0 && req.DocumentID != "" {
			linkDocumentIDs = []string{req.DocumentID}
		}
		if len(linkDocumentIDs) == 0 {
			return db.Link{}, errors.New("at least one document_id is required")
		}

		// Validate all documents exist and are ready.
		for _, did := range linkDocumentIDs {
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

	ndaDocumentID, ndaTemplateID, err := s.resolveNdaBinding(ctx, qtx, tenantID, workspaceUUID, userUUID, req.NDATemplateID, req.NDADocumentID, requireNDA)
	if err != nil {
		return db.Link{}, err
	}

	if err := s.ensureUniqueLinkName(ctx, qtx, workspaceUUID, dealRoomID, req.Name, pgtype.UUID{}); err != nil {
		return db.Link{}, err
	}

	folderScopeMode := FolderScopeModeFull
	hasDocumentScope := false
	if hasDealRoom {
		// Secure default: deal-room links are allowlists (empty = deny-all).
		folderScopeMode = FolderScopeModeAllowlist
		hasDocumentScope = true
	}

	link, err := qtx.CreateLink(ctx, db.CreateLinkParams{
		TenantID:                    tenantID,
		WorkspaceID:                 workspaceUUID,
		DocumentID:                  primaryDocID,
		DealRoomID:                  dealRoomID,
		PublicToken:                 token,
		Name:                        name,
		PermissionType:              perm,
		ExpiresAt:                   expiresAt,
		MaxAccessCount:              maxAccess,
		DownloadEnabled:             req.DownloadEnabled,
		WatermarkEnabled:            req.WatermarkEnabled,
		AiCopilotEnabled:            req.AICopilotEnabled,
		QaEnabled:                   req.QaEnabled,
		FileRequestsEnabled:         req.FileRequestsEnabled,
		IndexFileEnabled:            req.IndexFileEnabled,
		ScreenshotProtectionEnabled: req.ScreenshotProtectionEnabled,
		LinkType:                    linkType,
		TargetFolderPath:            targetFolderPath,
		NdaDocumentID:               ndaDocumentID,
		NdaTemplateID:               ndaTemplateID,
		RequireEmail:                requireEmail,
		RequireEmailVerification:    requireEmailVerification,
		RequireNda:                  requireNDA,
		RequirePassword:             req.RequirePassword,
		PasswordHash:                passwordHash,
		CustomDomain:                pgtype.Text{String: req.CustomDomain, Valid: req.CustomDomain != ""},
		Tags:                        req.Tags,
		NotifyOnAccess:              req.NotifyOnAccess,
		HasDocumentScope:            hasDocumentScope,
		FolderScopePaths:            folderScopePaths,
		FolderScopeMode:             folderScopeMode,
		Status:                      "active",
		CreatedBy:                   userUUID,
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
			emailCodes = append(emailCodes, emailCode{
				email: contact.Email.String,
				code:  code,
				epoch: s.bumpAccessCodeEpoch(token, contact.Email.String),
			})
		}

		// Deal-room share links identify visitors via allowed_emails rather than
		// pre-selected contact IDs. Provision contacts + access codes at create
		// time so recipients receive verification emails immediately.
		if hasDealRoom {
			seenEmails := make(map[string]struct{}, len(emailCodes)+len(req.AllowedEmails))
			for _, ec := range emailCodes {
				seenEmails[strings.ToLower(strings.TrimSpace(ec.email))] = struct{}{}
			}
			provisioned, err := s.upsertDealRoomAccessCodes(ctx, qtx, link, req.AllowedEmails, seenEmails)
			if err != nil {
				return db.Link{}, err
			}
			emailCodes = append(emailCodes, provisioned...)
		}
	}

	// Create allow-list and block-list rules from *_emails.
	rules := make([]AccessRule, 0, len(req.AllowedEmails)+len(req.BlockedEmails))
	seenRules := make(map[string]struct{})
	for _, email := range req.AllowedEmails {
		v := strings.TrimSpace(strings.ToLower(email))
		if v == "" {
			continue
		}
		key := "allow:email:" + v
		if _, ok := seenRules[key]; ok {
			continue
		}
		seenRules[key] = struct{}{}
		rules = append(rules, AccessRule{RuleType: "email", Value: v, Action: "allow"})
	}
	for _, email := range req.BlockedEmails {
		v := strings.TrimSpace(strings.ToLower(email))
		if v == "" {
			continue
		}
		key := "block:email:" + v
		if _, ok := seenRules[key]; ok {
			continue
		}
		seenRules[key] = struct{}{}
		rules = append(rules, AccessRule{RuleType: "email", Value: v, Action: "block"})
	}
	if len(rules) > 0 {
		if err := validateAccessRules(rules); err != nil {
			return db.Link{}, err
		}
		// Disallow the same email being both allowed and blocked.
		conflict := make(map[string]string)
		for _, r := range rules {
			k := r.Value
			if existing, ok := conflict[k]; ok && existing != r.Action {
				return db.Link{}, fmt.Errorf("%w: %s cannot be both allowed and blocked", ErrConflictingAccessRule, k)
			}
			conflict[k] = r.Action
		}
		for i, r := range rules {
			if err := qtx.CreateLinkAccessRule(ctx, db.CreateLinkAccessRuleParams{
				TenantID:    tenantID,
				WorkspaceID: workspaceUUID,
				LinkID:      link.ID,
				RuleType:    r.RuleType,
				Value:       r.Value,
				Action:      r.Action,
				SortOrder:   int32(i),
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
	s.sendAccessCodeEmails(ctx, token, emailCodes, req.Name, linkURL)
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
	wasEmailVerification := existing.RequireEmailVerification

	createReq := CreateLinkRequest{
		Name:                     req.Name,
		PermissionType:           req.PermissionType,
		RequireEmail:             req.RequireEmail,
		RequireEmailVerification: req.RequireEmailVerification,
		RequireNDA:               req.RequireNDA,
		NDADocumentID:            req.NDADocumentID,
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
		LinkType:                 existing.LinkType,
		TargetFolderPath:         existing.TargetFolderPath,
		ContactIDs:               req.ContactIDs,
	}

	requireEmail, requireEmailVerification, requireNDA, perm, err := normalizeSecurityConfig(createReq)
	if err != nil {
		return db.Link{}, err
	}

	if err := ensureAskDocsKnowledgeBase(ctx, s.queries, existing.DealRoomID, req.AICopilotEnabled); err != nil {
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

	targetFolderPath := existing.TargetFolderPath
	if req.TargetFolderPath != "" {
		targetFolderPath = req.TargetFolderPath
	}

	folderScopePaths := existing.FolderScopePaths
	folderScopeMode := existing.FolderScopeMode
	if normalizeFolderScopeMode(folderScopeMode) == "" {
		if len(folderScopePaths) > 0 {
			folderScopeMode = FolderScopeModeAllowlist
		} else {
			folderScopeMode = FolderScopeModeFull
		}
	}
	hasDocumentScope := existing.HasDocumentScope
	if isDealRoomLink && req.FolderPaths != nil {
		// Any explicit path update (including empty) locks into allowlist.
		folderScopePaths = req.FolderPaths
		folderScopeMode = FolderScopeModeAllowlist
		hasDocumentScope = true
	} else if isDealRoomLink && normalizeFolderScopeMode(req.FolderScopeMode) == FolderScopeModeFull {
		// Preserve legacy whole-room links when FE re-saves without touching scope.
		// Do not allow widening an allowlist link back to full.
		if !dealRoomUsesFolderAllowlist(existing) {
			folderScopeMode = FolderScopeModeFull
			hasDocumentScope = false
		}
	} else if isDealRoomLink && normalizeFolderScopeMode(req.FolderScopeMode) == FolderScopeModeAllowlist {
		folderScopeMode = FolderScopeModeAllowlist
		hasDocumentScope = true
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

	// For deal-room links, validate folder paths before writing anything.
	if isDealRoomLink && req.FolderPaths != nil {
		if err := s.validateDealRoomFolderPaths(ctx, qtx, workspaceUUID, existing.DealRoomID, folderScopePaths); err != nil {
			return db.Link{}, err
		}
	}

	// Validate NDA template/document when required.
	ndaDocumentID, ndaTemplateID, err := s.resolveNdaBinding(ctx, qtx, existing.TenantID, workspaceUUID, existing.CreatedBy, req.NDATemplateID, req.NDADocumentID, requireNDA)
	if err != nil {
		return db.Link{}, err
	}

	if name.Valid {
		if err := s.ensureUniqueLinkName(ctx, qtx, workspaceUUID, existing.DealRoomID, name.String, existing.ID); err != nil {
			return db.Link{}, err
		}
	}

	// Folder-scope (and other live-enforced) edits must not invalidate visitor
	// sessions: Access re-reads documents from DB on every refresh. Gate/password
	// /expiry changes still bump security_version so sessions re-authenticate.
	securityVersion := existing.SecurityVersion
	if linkSessionInvalidatingChange(
		existing,
		requireEmail,
		requireEmailVerification,
		requireNDA,
		req.RequirePassword,
		passwordHash,
		perm,
		expiresAt,
		maxAccess,
		ndaDocumentID,
		ndaTemplateID,
	) {
		securityVersion = existing.SecurityVersion + 1
	}

	// Update the link record using the sqlc-generated UpdateLinkFull.
	_, err = qtx.UpdateLinkFull(ctx, db.UpdateLinkFullParams{
		Name:                        name,
		DocumentID:                  existing.DocumentID,
		DealRoomID:                  existing.DealRoomID,
		PermissionType:              perm,
		ExpiresAt:                   expiresAt,
		MaxAccessCount:              maxAccess,
		DownloadEnabled:             req.DownloadEnabled,
		WatermarkEnabled:            req.WatermarkEnabled,
		NdaDocumentID:               ndaDocumentID,
		RequireEmail:                requireEmail,
		RequireEmailVerification:    requireEmailVerification,
		RequireNda:                  requireNDA,
		AiCopilotEnabled:            req.AICopilotEnabled,
		QaEnabled:                   req.QaEnabled,
		FileRequestsEnabled:         req.FileRequestsEnabled,
		IndexFileEnabled:            req.IndexFileEnabled,
		ScreenshotProtectionEnabled: req.ScreenshotProtectionEnabled,
		LinkType:                    existing.LinkType,
		TargetFolderPath:            targetFolderPath,
		RequirePassword:             req.RequirePassword,
		PasswordHash:                passwordHash,
		CustomDomain:                customDomain,
		Tags:                        tags,
		NotifyOnAccess:              req.NotifyOnAccess,
		SecurityVersion:             securityVersion,
		HasDocumentScope:            hasDocumentScope,
		FolderScopePaths:            folderScopePaths,
		FolderScopeMode:             folderScopeMode,
		ID:                          existing.ID,
		WorkspaceID:                 workspaceUUID,
	})
	if err != nil {
		return db.Link{}, fmt.Errorf("update link: %w", err)
	}
	if err := qtx.SetLinkNDABinding(ctx, db.SetLinkNDABindingParams{
		NdaTemplateID: ndaTemplateID,
		NdaDocumentID: ndaDocumentID,
		ID:            existing.ID,
		WorkspaceID:   workspaceUUID,
	}); err != nil {
		return db.Link{}, fmt.Errorf("set link NDA binding: %w", err)
	}

	// Replace all link_documents for document links only.
	if isDocumentLink {
		if len(req.DocumentIDs) == 0 {
			return db.Link{}, errors.New("at least one document_id is required")
		}

		var documentUUIDs []uuid.UUID
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
		}
		documentUUIDs = make([]uuid.UUID, len(req.DocumentIDs))
		for i, did := range req.DocumentIDs {
			documentUUIDs[i], _ = uuid.Parse(did)
		}

		if err := qtx.DeleteLinkDocumentsByLink(ctx, existing.ID); err != nil {
			return db.Link{}, fmt.Errorf("delete link documents: %w", err)
		}
		for i, docUUID := range documentUUIDs {
			if err := qtx.CreateLinkDocument(ctx, db.CreateLinkDocumentParams{
				LinkID:     existing.ID,
				DocumentID: pgtype.UUID{Bytes: docUUID, Valid: true},
				SortOrder:  int32(i),
			}); err != nil {
				return db.Link{}, fmt.Errorf("create link document %s: %w", docUUID, err)
			}
		}
	}

	// Document links replace link_contacts from ContactIDs. Deal-room links gate
	// visitors via access rules (allowed_emails); their link_contacts are owned by
	// CreateLink / UpdateAccessRules / SendEmailVerificationCode. Wiping them on
	// every UpdateLink would destroy create-time auto-sent codes on the next save.
	var emailCodes []emailCode
	if isDealRoomLink {
		if !requireEmailVerification {
			if err := qtx.DeleteLinkContactsByLink(ctx, existing.ID); err != nil {
				return db.Link{}, fmt.Errorf("delete link contacts: %w", err)
			}
		}
	} else {
		// Fetch existing link contacts before deletion so we can:
		//  - reuse codes for contacts that remain (不骚扰 — no re-mail)
		//  - preserve delivery status metadata (不漏信息 — status must stay truthful)
		//  - email only newly added contacts
		type contactDeliverySnapshot struct {
			AccessCode     string
			CodeSendStatus string
			CodeSendError  string
			CodeSentAt     pgtype.Timestamptz
			UsedAt         pgtype.Timestamptz
		}
		var existingByContactID map[string]contactDeliverySnapshot
		if requireEmailVerification {
			existingContacts, _ := qtx.GetLinkContactsByPublicToken(ctx, existing.PublicToken)
			existingByContactID = make(map[string]contactDeliverySnapshot, len(existingContacts))
			for _, lc := range existingContacts {
				errMsg := ""
				if lc.CodeSendError.Valid {
					errMsg = lc.CodeSendError.String
				}
				status := lc.CodeSendStatus
				if status == "" {
					status = "pending"
				}
				existingByContactID[uuid.UUID(lc.ContactID.Bytes).String()] = contactDeliverySnapshot{
					AccessCode:     lc.AccessCode,
					CodeSendStatus: status,
					CodeSendError:  errMsg,
					CodeSentAt:     lc.CodeSentAt,
					UsedAt:         lc.UsedAt,
				}
			}
		}

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

				if snap, existed := existingByContactID[cid]; existed {
					if err := qtx.CreateLinkContactWithDelivery(ctx, db.CreateLinkContactWithDeliveryParams{
						LinkID:         existing.ID,
						ContactID:      contactUUID,
						AccessCode:     snap.AccessCode,
						CodeSendStatus: snap.CodeSendStatus,
						CodeSendError:  snap.CodeSendError,
						CodeSentAt:     snap.CodeSentAt,
						UsedAt:         snap.UsedAt,
					}); err != nil {
						return db.Link{}, fmt.Errorf("create link contact: %w", err)
					}
					continue
				}

				code, err := generateNumericCode(6)
				if err != nil {
					return db.Link{}, fmt.Errorf("generate access code: %w", err)
				}
				emailCodes = append(emailCodes, emailCode{
					email: contact.Email.String,
					code:  code,
					epoch: s.bumpAccessCodeEpoch(existing.PublicToken, contact.Email.String),
				})
				if err := qtx.CreateLinkContact(ctx, db.CreateLinkContactParams{
					LinkID:     existing.ID,
					ContactID:  contactUUID,
					AccessCode: code,
				}); err != nil {
					return db.Link{}, fmt.Errorf("create link contact: %w", err)
				}
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Link{}, fmt.Errorf("commit transaction: %w", err)
	}

	// Send verification emails after commit for updated contacts.
	linkURL := publicLinkURL(s.viewerBaseURL, existing.PublicToken, req.CustomDomain)
	s.sendAccessCodeEmails(ctx, existing.PublicToken, emailCodes, req.Name, linkURL)

	// Re-fetch to get the updated record.
	updated, err := s.GetByID(ctx, linkID, workspaceID)
	if err != nil {
		return db.Link{}, err
	}

	// Deal-room: turning email verification on must provision codes for the
	// current allow list (不漏发). Subsequent UpdateAccessRules with the same
	// allows will see existing link_contacts and skip (不重复发).
	if isDealRoomLink && requireEmailVerification && !wasEmailVerification {
		if syncErr := s.syncDealRoomAccessCodeEmails(ctx, updated, nil); syncErr != nil {
			logger.ErrorCtx(ctx, "failed to auto-send access codes after enabling email verification", syncErr,
				logger.Attr("link_id", linkID),
			)
		}
	}

	return updated, nil
}

// CreateDealRoomLink creates a share link scoped to a deal room.
func (s *Service) CreateDealRoomLink(ctx context.Context, userID, workspaceID, dealRoomID string, req DealRoomLinkRequest) (db.Link, error) {
	return s.CreateLink(ctx, userID, workspaceID, CreateLinkRequest{
		DealRoomID:               dealRoomID,
		Name:                     req.Name,
		RequireEmail:             req.RequireEmail,
		RequireEmailVerification: req.RequireEmailVerification,
		RequireNDA:               req.RequireNDA,
		NDADocumentID:            req.NDADocumentID,
		NDATemplateID:            req.NDATemplateID,
		RequirePassword:          req.RequirePassword,
		Password:                 req.Password,
		AllowedEmails:            req.AllowedEmails,
		BlockedEmails:            req.BlockedEmails,
		ExpiresAt:                req.ExpiresAt,
		DownloadEnabled:             req.DownloadEnabled,
		WatermarkEnabled:            req.WatermarkEnabled,
		AICopilotEnabled:            req.AICopilotEnabled,
		QaEnabled:                   req.QaEnabled,
		FileRequestsEnabled:         req.FileRequestsEnabled,
		IndexFileEnabled:            req.IndexFileEnabled,
		ScreenshotProtectionEnabled: req.ScreenshotProtectionEnabled,
		CustomDomain:                req.CustomDomain,
		Tags:                        req.Tags,
		NotifyOnAccess:              req.NotifyOnAccess,
		FolderPaths:                 req.FolderPaths,
	})
}

// validateDealRoomFolderPaths checks that every provided folder path exists in
// the deal room's folder structure.
func (s *Service) validateDealRoomFolderPaths(ctx context.Context, qtx *db.Queries, workspaceID pgtype.UUID, dealRoomID pgtype.UUID, folderPaths []string) error {
	if len(folderPaths) == 0 {
		return nil
	}
	foldersJSON, err := qtx.GetDealRoomFolderPaths(ctx, db.GetDealRoomFolderPathsParams{
		ID:          dealRoomID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return fmt.Errorf("get deal room folders: %w", err)
	}
	var folders []struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(foldersJSON), &folders); err != nil {
		return fmt.Errorf("parse deal room folders: %w", err)
	}
	allowed := make(map[string]bool, len(folders))
	for _, f := range folders {
		allowed[f.Path] = true
	}
	for _, p := range folderPaths {
		if !allowed[p] {
			return fmt.Errorf("folder path not found in deal room: %s", p)
		}
	}
	return nil
}

// validateDealRoomDocumentIDs checks that every provided document ID belongs to
// the deal room and is ready. It returns the parsed UUIDs in request order.
func (s *Service) validateDealRoomDocumentIDs(ctx context.Context, qtx *db.Queries, dealRoomID pgtype.UUID, documentIDs []string) ([]uuid.UUID, error) {
	if len(documentIDs) == 0 {
		return nil, nil
	}
	roomDocs, err := qtx.ListDealRoomDocumentsWithMeta(ctx, dealRoomID)
	if err != nil {
		return nil, fmt.Errorf("list deal room documents: %w", err)
	}
	allowed := make(map[string]db.ListDealRoomDocumentsWithMetaRow, len(roomDocs))
	for _, d := range roomDocs {
		allowed[uuid.UUID(d.DocumentID.Bytes).String()] = d
	}

	out := make([]uuid.UUID, 0, len(documentIDs))
	for _, did := range documentIDs {
		docUUID, err := uuid.Parse(did)
		if err != nil {
			return nil, fmt.Errorf("invalid document id: %s", did)
		}
		roomDoc, ok := allowed[did]
		if !ok {
			return nil, fmt.Errorf("document not found in deal room: %s", did)
		}
		if roomDoc.Status != "ready" {
			return nil, ErrDocumentNotReady
		}
		out = append(out, docUUID)
	}
	return out, nil
}

func (s *Service) resolveNdaBinding(
	ctx context.Context,
	qtx *db.Queries,
	tenantID, workspaceID, createdBy pgtype.UUID,
	ndaTemplateID, ndaDocumentID string,
	requireNDA bool,
) (docID pgtype.UUID, templateID pgtype.UUID, err error) {
	if !requireNDA {
		return pgtype.UUID{}, pgtype.UUID{}, nil
	}

	templateIDStr := strings.TrimSpace(ndaTemplateID)
	documentIDStr := strings.TrimSpace(ndaDocumentID)

	if templateIDStr == "" && documentIDStr == "" {
		return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("%w: NDA template is required when NDA is enabled", ErrInvalidPermission)
	}

	var tpl db.NdaTemplate
	if templateIDStr != "" {
		tid, perr := uuid.Parse(templateIDStr)
		if perr != nil {
			return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("%w: invalid NDA template id: %s", ErrInvalidInput, templateIDStr)
		}
		tpl, err = qtx.GetNDATemplateByID(ctx, db.GetNDATemplateByIDParams{
			ID:          pgtype.UUID{Bytes: tid, Valid: true},
			WorkspaceID: workspaceID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("%w: NDA template not found", ErrInvalidPermission)
			}
			return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("get NDA template: %w", err)
		}
		if tpl.Status != "active" {
			return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("%w: NDA template is archived", ErrInvalidPermission)
		}
	} else {
		// Compat / ensure: create-or-get template from a workspace document.
		if s.ndaSvc == nil {
			// Fallback without sealer: validate document exists and create template via queries.
			docUUID, perr := uuid.Parse(documentIDStr)
			if perr != nil {
				return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("%w: invalid NDA document id: %s", ErrInvalidInput, documentIDStr)
			}
			docPG := pgtype.UUID{Bytes: docUUID, Valid: true}
			doc, derr := qtx.GetDocumentByID(ctx, db.GetDocumentByIDParams{ID: docPG, WorkspaceID: workspaceID})
			if derr != nil {
				if errors.Is(derr, pgx.ErrNoRows) {
					return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("%w: NDA document not found: %s", ErrInvalidPermission, documentIDStr)
				}
				return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("get NDA document: %w", derr)
			}
			existing, gerr := qtx.GetNDATemplateBySourceDocument(ctx, db.GetNDATemplateBySourceDocumentParams{
				WorkspaceID:      workspaceID,
				SourceDocumentID: docPG,
			})
			if gerr == nil {
				tpl = existing
			} else if errors.Is(gerr, pgx.ErrNoRows) {
				name := strings.TrimSpace(doc.Title)
				if name == "" {
					name = "NDA Agreement"
				}
				tpl, err = qtx.CreateNDATemplate(ctx, db.CreateNDATemplateParams{
					TenantID:          tenantID,
					WorkspaceID:       workspaceID,
					Name:              name,
					SourceDocumentID:  docPG,
					ContentSha256:     "",
					RequireSignerName: true,
					Status:            "active",
					CreatedBy:         createdBy,
				})
				if err != nil {
					return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("create NDA template: %w", err)
				}
			} else {
				return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("get NDA template by document: %w", gerr)
			}
		} else {
			tpl, err = s.ndaSvc.EnsureTemplateFromDocument(ctx, tenantID, workspaceID, documentIDStr, createdBy, "")
			if err != nil {
				return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("%w: %v", ErrInvalidPermission, err)
			}
		}
	}

	return tpl.SourceDocumentID, tpl.ID, nil
}

// resolveNdaDocumentID is retained for tests; prefer resolveNdaBinding.
func (s *Service) resolveNdaDocumentID(
	ctx context.Context,
	qtx *db.Queries,
	workspaceID pgtype.UUID,
	dealRoomID pgtype.UUID,
	documentIDs []string,
	ndaDocumentID string,
	requireNDA bool,
) (pgtype.UUID, error) {
	docID, _, err := s.resolveNdaBinding(ctx, qtx, pgtype.UUID{}, workspaceID, pgtype.UUID{}, "", ndaDocumentID, requireNDA)
	_ = dealRoomID
	_ = documentIDs
	return docID, err
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
//
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
//  2. If any allow rule exists and none match, access is denied.
//  3. If no rules exist, access is allowed.
func evaluateAccessRules(rules []AccessRule, email string) AccessEvaluation {
	if len(rules) == 0 {
		return AccessEvaluation{Allowed: true, Reason: "no_rules"}
	}

	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		// If there are any allow rules, empty email cannot satisfy them.
		for _, r := range rules {
			if r.Action == "allow" {
				return AccessEvaluation{Allowed: false, Reason: "no_allow_email_match"}
			}
		}
		return AccessEvaluation{Allowed: true, Reason: "no_rules"}
	}

	var allowExists bool
	for _, r := range rules {
		if r.Action == "allow" {
			allowExists = true
		}
	}

	// First pass: block rules.
	for _, r := range rules {
		if r.Action != "block" {
			continue
		}
		if constantTimeEmailCompare(r.Value, email) {
			return AccessEvaluation{
				Allowed:     false,
				Reason:      "blocked_email",
				MatchedRule: &AccessRule{RuleType: r.RuleType, Value: r.Value, Action: r.Action},
			}
		}
	}

	// Second pass: allow rules.
	for _, r := range rules {
		if r.Action != "allow" {
			continue
		}
		if constantTimeEmailCompare(r.Value, email) {
			return AccessEvaluation{
				Allowed:     true,
				Reason:      "allowed_email",
				MatchedRule: &AccessRule{RuleType: r.RuleType, Value: r.Value, Action: r.Action},
			}
		}
	}

	if allowExists {
		return AccessEvaluation{Allowed: false, Reason: "no_allow_email_match"}
	}
	return AccessEvaluation{Allowed: true, Reason: "no_rules"}
}

// validateAccessRules checks that a set of rules is internally consistent.
func validateAccessRules(rules []AccessRule) error {
	seen := make(map[string]struct{}, len(rules))
	for _, r := range rules {
		if r.RuleType != "email" {
			return fmt.Errorf("%w: rule_type must be email", ErrInvalidAccessRule)
		}
		if r.Action != "allow" && r.Action != "block" {
			return fmt.Errorf("%w: action must be allow or block", ErrInvalidAccessRule)
		}
		value := strings.TrimSpace(strings.ToLower(r.Value))
		if value == "" {
			return fmt.Errorf("%w: rule value cannot be empty", ErrInvalidAccessRule)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("%w: duplicate rule for %s", ErrConflictingAccessRule, value)
		}
		seen[value] = struct{}{}
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

	// If any allow rule exists, an email identity is required so the rule can be
	// evaluated. Either explicit email collection or email verification satisfies this,
	// because verification links still identify the visitor by their email address.
	for _, r := range rules {
		if r.Action == "allow" && !link.RequireEmail && !link.RequireEmailVerification {
			return fmt.Errorf("%w: require_email or require_email_verification must be enabled when allow rules exist", ErrInvalidAccessRule)
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	// Snapshot current rules before replacing them.
	oldRules, _ := qtx.ListLinkAccessRulesByLink(ctx, pgtype.UUID{Bytes: linkUUID, Valid: true})
	snapshot := make([]AccessRule, 0, len(oldRules))
	oldAllow := make(map[string]struct{})
	for _, r := range oldRules {
		snapshot = append(snapshot, AccessRule{RuleType: r.RuleType, Value: r.Value, Action: r.Action})
		if r.Action == "allow" && r.RuleType == "email" {
			oldAllow[strings.ToLower(strings.TrimSpace(r.Value))] = struct{}{}
		}
	}
	snapshotBytes, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal rule snapshot: %w", err)
	}
	if err := qtx.InsertLinkAccessRuleRevision(ctx, db.InsertLinkAccessRuleRevisionParams{
		TenantID:      link.TenantID,
		WorkspaceID:   workspaceUUID,
		LinkID:        link.ID,
		ChangedBy:     userUUID,
		RulesSnapshot: snapshotBytes,
	}); err != nil {
		return fmt.Errorf("insert rule revision: %w", err)
	}

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

	allowedEmails := make(map[string]struct{}, len(rules))
	for _, r := range rules {
		if r.Action == "allow" && r.RuleType == "email" {
			allowedEmails[strings.ToLower(strings.TrimSpace(r.Value))] = struct{}{}
		}
	}

	// Owner removed someone from the allow list: drop their approved access
	// request so CheckPublicEmail heal cannot restore the allow rule.
	for email := range oldAllow {
		if _, keep := allowedEmails[email]; keep {
			continue
		}
		if _, rejectErr := qtx.RejectApprovedLinkAccessRequestByEmail(ctx, db.RejectApprovedLinkAccessRequestByEmailParams{
			LinkID:     link.ID,
			Email:      email,
			ReviewedBy: userUUID,
		}); rejectErr != nil && !errors.Is(rejectErr, pgx.ErrNoRows) {
			return fmt.Errorf("reject approved access request for removed allow email: %w", rejectErr)
		}
	}

	// Revoke invitations only for emails that are no longer allowed.
	// Used invitations are left untouched for audit purposes.
	invitations, err := qtx.ListLinkInvitationsByLink(ctx, link.ID)
	if err != nil {
		return fmt.Errorf("list invitations: %w", err)
	}
	for _, inv := range invitations {
		if inv.Status != "pending" && inv.Status != "opened" && inv.Status != "verified" {
			continue
		}
		if _, ok := allowedEmails[strings.ToLower(strings.TrimSpace(inv.Email))]; ok {
			continue
		}
		if _, err := qtx.UpdateLinkInvitationStatus(ctx, db.UpdateLinkInvitationStatusParams{
			Status: "revoked",
			UsedAt: pgtype.Timestamptz{},
			ID:     inv.ID,
		}); err != nil {
			return fmt.Errorf("revoke invitation: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	// Auto-send access codes only for recipients that need them:
	//  - newly allow-listed emails, or
	//  - allow-listed emails still missing a link_contact.
	// Already-allow-listed contacts with an existing code are skipped (不重复发).
	// Refresh link flags so a preceding UpdateLink that just enabled verification
	// is visible here.
	fresh, err := s.GetByID(ctx, linkID, workspaceID)
	if err != nil {
		return err
	}
	if fresh.RequireEmailVerification && fresh.DealRoomID.Valid {
		oldAllow := make(map[string]struct{}, len(snapshot))
		for _, r := range snapshot {
			if r.Action == "allow" {
				oldAllow[strings.ToLower(strings.TrimSpace(r.Value))] = struct{}{}
			}
		}
		if syncErr := s.syncDealRoomAccessCodeEmails(ctx, fresh, oldAllow); syncErr != nil {
			logger.ErrorCtx(ctx, "failed to sync access codes after access rules update", syncErr,
				logger.Attr("link_id", linkID),
			)
		}
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
			// Existing invitations store only the token hash; the raw token cannot
			// be recovered. Regenerate a fresh token (and bump expiry) so the email
			// always contains a usable inviteToken, regardless of prior status.
			token, err := generateToken()
			if err != nil {
				return nil, fmt.Errorf("generate invite token: %w", err)
			}
			expiresAt := pgtype.Timestamptz{Valid: true, Time: time.Now().Add(7 * 24 * time.Hour)}
			status := existing.Status
			if status == "revoked" {
				status = "pending"
			}
			if _, err := qtx.ResetLinkInvitation(ctx, db.ResetLinkInvitationParams{
				Token:     pgtype.Text{String: "", Valid: false},
				TokenHash: pgtype.Text{String: s.hashToken(token), Valid: true},
				ExpiresAt: expiresAt,
				ID:        existing.ID,
			}); err != nil {
				return nil, fmt.Errorf("reset invitation: %w", err)
			}
			invitations = append(invitations, invitationFromRaw(token, existing.ID, link.ID, email, status, expiresAt, pgtype.Timestamptz{}))
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
			TokenHash:   pgtype.Text{String: s.hashToken(token), Valid: true},
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
	wsID := ""
	if link.WorkspaceID.Valid {
		wsID = uuid.UUID(link.WorkspaceID.Bytes).String()
	}
	creatorID := ""
	if link.CreatedBy.Valid {
		creatorID = uuid.UUID(link.CreatedBy.Bytes).String()
	}
	for _, inv := range invitations {
		s.sendInvitationEmail(ctx, inv, wsID, creatorID, link.Name.String, linkURL)
	}

	return invitations, nil
}

// ResolveInviteToken validates an invitation token and returns the invitation.
func (s *Service) ResolveInviteToken(ctx context.Context, token string) (LinkInvitation, error) {
	// Look up by HMAC-SHA256 first.
	inv, err := s.queries.GetLinkInvitationByToken(ctx, pgtype.Text{String: s.hashToken(token), Valid: true})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return LinkInvitation{}, fmt.Errorf("get invitation: %w", err)
		}

		// Fallback: legacy SHA-256 hashes. On match, backfill to HMAC.
		inv, err = s.queries.GetLinkInvitationByToken(ctx, pgtype.Text{String: s.legacyHashToken(token), Valid: true})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return LinkInvitation{}, ErrLinkNotFound
			}
			return LinkInvitation{}, fmt.Errorf("get invitation: %w", err)
		}

		if err := s.queries.UpdateLinkInvitationTokenHash(ctx, db.UpdateLinkInvitationTokenHashParams{
			TokenHash: pgtype.Text{String: s.hashToken(token), Valid: true},
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
	case "used":
		return LinkInvitation{}, ErrInviteAlreadyUsed
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
		SignerName: r.SignerName.String,
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

// CheckPublicEmail evaluates whether the given email is allowed by the link's
// access rules (allow/block). Used by the NDA sign step before entering review,
// so visitors who are not on the allowlist never spend time on the 30s preview.
// It does not consume max_access_count or create sessions.
func (s *Service) CheckPublicEmail(ctx context.Context, publicToken, email, clientIP string) (db.Link, error) {
	link, err := s.ResolvePublicLink(ctx, publicToken)
	if err != nil {
		return db.Link{}, err
	}
	if err := s.checkAccessAttemptRateLimit(ctx, publicToken, clientIP); err != nil {
		return link, err
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if !isValidEmail(email) {
		return link, ErrRequiresEmail
	}
	eval, err := s.EvaluateAccessRules(ctx, uuid.UUID(link.ID.Bytes).String(), email)
	if err != nil {
		return link, fmt.Errorf("evaluate access rules: %w", err)
	}
	if !eval.Allowed {
		// Approved access requests must remain usable even if a later share-dialog
		// save wiped the allow rule. Heal the rule and re-check.
		if eval.Reason != "blocked_email" {
			if healed, healErr := s.healAllowRuleForApprovedRequest(ctx, link, email); healErr != nil {
				return link, healErr
			} else if healed {
				return link, nil
			}
		}
		return link, mapRuleError(eval.Reason)
	}
	return link, nil
}

// mergeApprovedAllowRules appends allow rules for approved applicants that are
// not already allowed and not explicitly blocked in the incoming rule set.
func mergeApprovedAllowRules(rules []AccessRule, approvedEmails []string) []AccessRule {
	if len(approvedEmails) == 0 {
		return rules
	}
	blocked := make(map[string]struct{})
	allowed := make(map[string]struct{})
	for _, r := range rules {
		v := strings.ToLower(strings.TrimSpace(r.Value))
		if r.RuleType != "email" || v == "" {
			continue
		}
		switch r.Action {
		case "block":
			blocked[v] = struct{}{}
		case "allow":
			allowed[v] = struct{}{}
		}
	}
	merged := append([]AccessRule(nil), rules...)
	for _, email := range approvedEmails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}
		if _, ok := blocked[email]; ok {
			continue
		}
		if _, ok := allowed[email]; ok {
			continue
		}
		merged = append(merged, AccessRule{RuleType: "email", Value: email, Action: "allow"})
		allowed[email] = struct{}{}
	}
	return merged
}

// healAllowRuleForApprovedRequest restores an allow rule when the visitor has an
// approved access request but is currently denied by missing allowlist entry.
func (s *Service) healAllowRuleForApprovedRequest(ctx context.Context, link db.Link, email string) (bool, error) {
	existing, err := s.queries.GetLinkAccessRequestByLinkAndEmail(ctx, db.GetLinkAccessRequestByLinkAndEmailParams{
		LinkID: link.ID,
		Email:  email,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("lookup access request: %w", err)
	}
	if existing.Status != "approved" {
		return false, nil
	}
	if err := s.ensureEmailAllowRule(ctx, s.queries, link, email); err != nil {
		return false, err
	}
	return true, nil
}

// RequestAccess lets a blocked or not-allowed visitor request access to a link.
// signerName is optional; when provided it is stored on the request and used to
// name the workspace contact on approval.
func (s *Service) RequestAccess(ctx context.Context, link db.Link, email, reason, signerName string) (LinkAccessRequest, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if !isValidEmail(email) {
		return LinkAccessRequest{}, errors.New("invalid email address")
	}
	reason = strings.TrimSpace(reason)
	if len(reason) > 500 {
		return LinkAccessRequest{}, errors.New("reason must be 500 characters or less")
	}
	signerName = strings.TrimSpace(signerName)
	if len(signerName) > 200 {
		return LinkAccessRequest{}, errors.New("signer name must be 200 characters or less")
	}

	linkID := uuid.UUID(link.ID.Bytes).String()
	ev, err := s.EvaluateAccessRules(ctx, linkID, email)
	if err != nil {
		return LinkAccessRequest{}, fmt.Errorf("evaluate access rules: %w", err)
	}
	if ev.Allowed {
		return LinkAccessRequest{}, ErrAccessAlreadyAllowed
	}
	if ev.Reason == "blocked_email" {
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
		// Previously approved/rejected but still denied by rules (e.g. allow rule
		// missing or later removed) — reopen so owners see a fresh pending item.
		reasonText := existing.Reason
		if reason != "" {
			reasonText = pgtype.Text{String: reason, Valid: true}
		}
		signerText := existing.SignerName
		if signerName != "" {
			signerText = pgtype.Text{String: signerName, Valid: true}
		}
		reopened, reopenErr := s.queries.ReopenLinkAccessRequest(ctx, db.ReopenLinkAccessRequestParams{
			ID:         existing.ID,
			Reason:     reasonText,
			SignerName: signerText,
		})
		if reopenErr != nil {
			return LinkAccessRequest{}, fmt.Errorf("reopen access request: %w", reopenErr)
		}
		s.notifyLinkAccessRequest(ctx, link, linkID, email, reason, signerName)
		return dbAccessRequestToDomain(reopened), nil
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
		SignerName:  pgtype.Text{String: signerName, Valid: signerName != ""},
	})
	if err != nil {
		return LinkAccessRequest{}, fmt.Errorf("create access request: %w", err)
	}

	s.notifyLinkAccessRequest(ctx, link, linkID, email, reason, signerName)
	return dbAccessRequestToDomain(row), nil
}

func (s *Service) notifyLinkAccessRequest(ctx context.Context, link db.Link, linkID, email, reason, signerName string) {
	if s.notifier == nil {
		return
	}
	creatorID := uuid.UUID(link.CreatedBy.Bytes).String()
	wsID := uuid.UUID(link.WorkspaceID.Bytes).String()
	subject := "New access request on your link"
	body := fmt.Sprintf("A visitor (%s) requested access to \"%s\". Reason: %s. Review the request in the share dialog.", email, link.Name.String, reason)
	if reason == "" {
		body = fmt.Sprintf("A visitor (%s) requested access to \"%s\". Review the request in the share dialog.", email, link.Name.String)
	}
	if signerName != "" {
		body = fmt.Sprintf("%s Signer name: %s.", body, signerName)
	}
	if _, notifyErr := s.notifier.Enqueue(ctx, wsID, creatorID, "email", subject, body); notifyErr != nil {
		logger.ErrorCtx(ctx, "failed to enqueue access request notification", notifyErr,
			logger.Attr("link_id", linkID),
			logger.Attr("email", email),
		)
	}
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
		if errors.Is(err, pgx.ErrNoRows) {
			return LinkAccessRequest{}, errors.New("access request is not pending")
		}
		return LinkAccessRequest{}, fmt.Errorf("approve access request: %w", err)
	}

	inv, err := s.createInvitationForRequest(ctx, qtx, link, reqRow.Email, pgtype.UUID{Bytes: reviewerUUID, Valid: true})
	if err != nil {
		return LinkAccessRequest{}, fmt.Errorf("create invitation: %w", err)
	}

	// Persist applicant on the workspace contacts list (reject does not).
	contactName := strings.TrimSpace(reqRow.SignerName.String)
	if _, err := qtx.UpsertContactByEmail(ctx, db.UpsertContactByEmailParams{
		WorkspaceID: link.WorkspaceID,
		Email:       pgtype.Text{String: strings.TrimSpace(strings.ToLower(reqRow.Email)), Valid: true},
		Name:        contactName,
	}); err != nil {
		return LinkAccessRequest{}, fmt.Errorf("upsert contact from access request: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return LinkAccessRequest{}, fmt.Errorf("commit transaction: %w", err)
	}

	linkURL := publicLinkURL(s.viewerBaseURL, link.PublicToken, link.CustomDomain.String)
	wsID := ""
	if link.WorkspaceID.Valid {
		wsID = uuid.UUID(link.WorkspaceID.Bytes).String()
	}
	creatorID := ""
	if link.CreatedBy.Valid {
		creatorID = uuid.UUID(link.CreatedBy.Bytes).String()
	}
	s.sendInvitationEmail(ctx, inv, wsID, creatorID, link.Name.String, linkURL)
	s.resolveLinkAccessRequest(workspaceID, linkID)

	// When email verification is enabled, provision + send an access code so the
	// approved visitor can complete Access after refresh (NDA intent is short-stored
	// client-side; seal still happens only on successful Access). Approval is already
	// committed; surface send failures so the owner can retry via resend.
	if link.RequireEmailVerification {
		if codeErr := s.sendDealRoomEmailVerificationCode(ctx, link, reqRow.Email, s.viewerBaseURL); codeErr != nil {
			logger.ErrorCtx(ctx, "send access code after approval failed", codeErr,
				logger.Attr("link_id", linkID),
				logger.Attr("email", reqRow.Email),
			)
			return LinkAccessRequest{}, fmt.Errorf("%w: %v", ErrAccessCodeSendFailed, codeErr)
		}
	}

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
		if errors.Is(err, pgx.ErrNoRows) {
			return LinkAccessRequest{}, errors.New("access request is not pending")
		}
		return LinkAccessRequest{}, fmt.Errorf("reject access request: %w", err)
	}
	s.resolveLinkAccessRequest(workspaceID, linkID)
	return dbAccessRequestToDomain(updated), nil
}

func (s *Service) createInvitationForRequest(ctx context.Context, qtx *db.Queries, link db.Link, email string, createdBy pgtype.UUID) (LinkInvitation, error) {
	workspaceUUID := link.WorkspaceID
	email = strings.TrimSpace(strings.ToLower(email))
	token, err := generateToken()
	if err != nil {
		return LinkInvitation{}, fmt.Errorf("generate invite token: %w", err)
	}
	expiresAt := pgtype.Timestamptz{Valid: true, Time: time.Now().Add(7 * 24 * time.Hour)}

	existing, err := qtx.GetLinkInvitationByLinkAndEmail(ctx, db.GetLinkInvitationByLinkAndEmailParams{
		LinkID: link.ID,
		Email:  email,
	})
	var invID pgtype.UUID
	if err == nil {
		// Existing rows store only the token hash; plaintext cannot be recovered.
		// Always reissue so the approval email contains a usable inviteToken,
		// including when the prior invite was used/expired/pending.
		if _, resetErr := qtx.ResetLinkInvitation(ctx, db.ResetLinkInvitationParams{
			Token:     pgtype.Text{String: "", Valid: false},
			TokenHash: pgtype.Text{String: s.hashToken(token), Valid: true},
			ExpiresAt: expiresAt,
			ID:        existing.ID,
		}); resetErr != nil {
			return LinkInvitation{}, fmt.Errorf("reset invitation: %w", resetErr)
		}
		invID = existing.ID
	} else if errors.Is(err, pgx.ErrNoRows) {
		inv, createErr := qtx.CreateLinkInvitation(ctx, db.CreateLinkInvitationParams{
			TenantID:    link.TenantID,
			WorkspaceID: workspaceUUID,
			LinkID:      link.ID,
			Email:       email,
			Token:       pgtype.Text{String: "", Valid: false},
			TokenHash:   pgtype.Text{String: s.hashToken(token), Valid: true},
			Status:      "pending",
			ExpiresAt:   expiresAt,
			CreatedBy:   createdBy,
		})
		if createErr != nil {
			return LinkInvitation{}, fmt.Errorf("create invitation: %w", createErr)
		}
		invID = inv.ID
	} else {
		return LinkInvitation{}, fmt.Errorf("get invitation by email: %w", err)
	}

	if err := s.ensureEmailAllowRule(ctx, qtx, link, email); err != nil {
		return LinkInvitation{}, err
	}

	return invitationFromRaw(token, invID, link.ID, email, "pending", expiresAt, pgtype.Timestamptz{}), nil
}

// ensureEmailAllowRule inserts an allow rule for email when missing (idempotent).
func (s *Service) ensureEmailAllowRule(ctx context.Context, qtx *db.Queries, link db.Link, email string) error {
	rules, err := qtx.ListLinkAccessRulesByLink(ctx, link.ID)
	if err != nil {
		return fmt.Errorf("list access rules: %w", err)
	}
	for _, r := range rules {
		if r.Action == "allow" && r.RuleType == "email" && strings.EqualFold(r.Value, email) {
			return nil
		}
	}
	if err := qtx.CreateLinkAccessRule(ctx, db.CreateLinkAccessRuleParams{
		TenantID:    link.TenantID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
		RuleType:    "email",
		Value:       email,
		Action:      "allow",
		SortOrder:   0,
	}); err != nil {
		return fmt.Errorf("create allow rule: %w", err)
	}
	return nil
}

// hashToken returns the HMAC-SHA256 hash (hex) of an invite token.
func (s *Service) hashToken(token string) string {
	if s.cfg == nil {
		// Defensive fallback for tests or misconfigured environments.
		return s.legacyHashToken(token)
	}
	key := s.cfg.InviteTokenHashKey
	if key == "" {
		// Defensive fallback: hashing with an empty key still produces a stable
		// value, but production should always configure a secret.
		key = s.cfg.JWTSecret
	}
	h := hmac.New(sha256.New, []byte(key))
	_, _ = h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// legacyHashToken computes the original SHA-256 hash used before HMAC was
// introduced. It is kept only for backward-compatible token lookup.
func (s *Service) legacyHashToken(token string) string {
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

func (s *Service) sendInvitationEmail(ctx context.Context, inv LinkInvitation, workspaceID, userID, linkName, linkURL string) {
	if s.notifier == nil {
		logger.InfoCtx(ctx, "notifier not configured, skipping invitation email")
		return
	}
	inviteURL := fmt.Sprintf("%s?inviteToken=%s", linkURL, inv.Token)
	subject := fmt.Sprintf("You've been invited to view \"%s\"", linkName)
	body := fmt.Sprintf("You have been invited to view \"%s\". Open the invitation: %s", linkName, inviteURL)
	if _, err := s.notifier.Enqueue(ctx, workspaceID, userID, "email", subject, body, notification.WithRecipient(inv.Email)); err != nil {
		logger.ErrorCtx(ctx, "failed to enqueue invitation email", err,
			logger.Attr("email_local", localPart(inv.Email)),
		)
	}
}

func (s *Service) sendAccessNotificationEmail(ctx context.Context, workspaceID, userID, linkName, visitorEmail, linkURL string) {
	if s.notifier == nil {
		logger.InfoCtx(ctx, "notifier not configured, skipping access notification email")
		return
	}
	subject := fmt.Sprintf("Someone viewed your DealSignal link \"%s\"", linkName)
	body := fmt.Sprintf("A visitor (%s) viewed \"%s\". Open the link: %s", visitorEmail, linkName, linkURL)
	if _, err := s.notifier.Enqueue(ctx, workspaceID, userID, "email", subject, body); err != nil {
		logger.ErrorCtx(ctx, "failed to enqueue access notification email", err,
			logger.Attr("workspace_id", workspaceID),
			logger.Attr("user_id", userID),
		)
	}
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
	SignerName  string
	InviteToken string
	IP          string
	UA          string
}

// AccessResult is returned after a successful access check.
type AccessResult struct {
	Link             db.Link
	VisitorID        string
	Email            string
	EmailVerified    bool
	SessionToken     string // refreshed session token for sliding expiry; empty if no session was used
	NDAResponseID    string
	NDACertificateID string
}

// LinkAccessRequest is the domain representation of a visitor access request.
type LinkAccessRequest struct {
	ID         string     `json:"id"`
	LinkID     string     `json:"link_id"`
	Email      string     `json:"email"`
	Reason     string     `json:"reason,omitempty"`
	SignerName string     `json:"signer_name,omitempty"`
	Status     string     `json:"status"`
	ReviewedBy *string    `json:"reviewed_by,omitempty"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

var (
	ErrAccessRequestBlocked = errors.New("this email is blocked from requesting access")
	ErrAccessRequestExists  = errors.New("an access request from this email is already pending")
)

// DeliveryEmailMismatchError is returned when a submitted email differs from the
// email bound to a valid verification code. AuthorizedEmail is for internal audit
// only and must never be serialized to public API clients.
type DeliveryEmailMismatchError struct {
	AuthorizedEmail string
}

func (e *DeliveryEmailMismatchError) Error() string {
	return ErrDeliveryEmailMismatch.Error()
}

func (e *DeliveryEmailMismatchError) Is(target error) bool {
	return target == ErrDeliveryEmailMismatch
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

	// Submitted email is treated as NDA delivery / explicit claim only. For
	// email-verification links, allowlist identity always comes from the code
	// (or invite), never from this field — otherwise a reserved delivery address
	// that differs from the authorized mailbox fails allowlist before mismatch UX.
	submittedEmail := strings.TrimSpace(req.Email)

	// Resolve effective email: invite token takes priority and is immutable.
	// Consume the invitation atomically here so concurrent Access calls cannot
	// both succeed with the same single-use token. A later gate failure burns
	// the invite (owner can re-issue); that is preferred over double-grant.
	var effectiveEmail string
	if req.InviteToken != "" {
		inv, err := s.ResolveInviteToken(ctx, req.InviteToken)
		if err != nil {
			s.recordSecurityEvent(ctx, link, "", submittedEmail, "invite_token_failed", err.Error())
			return AccessResult{}, err
		}
		if inv.LinkID != uuid.UUID(link.ID.Bytes).String() {
			s.recordSecurityEvent(ctx, link, "", submittedEmail, "invite_token_failed", "invitation does not belong to link")
			return AccessResult{}, ErrLinkNotFound
		}
		invUUID, parseErr := uuid.Parse(inv.ID)
		if parseErr != nil {
			return AccessResult{}, fmt.Errorf("parse invitation id: %w", parseErr)
		}
		if _, err := s.queries.ConsumeLinkInvitation(ctx, pgtype.UUID{Bytes: invUUID, Valid: true}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				s.recordSecurityEvent(ctx, link, "", submittedEmail, "invite_token_failed", "invitation already used or expired")
				return AccessResult{}, ErrInviteAlreadyUsed
			}
			return AccessResult{}, fmt.Errorf("consume invitation: %w", err)
		}
		effectiveEmail = inv.Email
	} else if requiresEmailVerification {
		// Code owns identity. Resolve before allowlist so NDA delivery ≠ authorized
		// does not surface as a generic not_allowed / invalid_code against D.
		code := strings.TrimSpace(req.EmailCode)
		if code == "" {
			return AccessResult{}, ErrRequiresEmailCode
		}
		lc, err := s.queries.GetLinkContactByCode(ctx, db.GetLinkContactByCodeParams{
			PublicToken: token,
			AccessCode:  code,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return AccessResult{}, ErrInvalidEmailCode
			}
			return AccessResult{}, fmt.Errorf("get link contact by code: %w", err)
		}
		effectiveEmail = strings.TrimSpace(lc.ContactEmail.String)
		if effectiveEmail == "" {
			return AccessResult{}, ErrInvalidEmailCode
		}
	} else {
		effectiveEmail = submittedEmail
	}

	// If email is required but not provided yet, ask for it before evaluating
	// access rules so the first visit doesn't show an "email not allowed" error
	// on an empty input.
	if requiresEmail && effectiveEmail == "" {
		return AccessResult{}, ErrRequiresEmail
	}

	// Evaluate access rules before any gate checks.
	eval, err := s.EvaluateAccessRules(ctx, uuid.UUID(link.ID.Bytes).String(), effectiveEmail)
	if err != nil {
		return AccessResult{}, fmt.Errorf("evaluate access rules: %w", err)
	}
	if !eval.Allowed {
		if eval.Reason != "blocked_email" && effectiveEmail != "" {
			if healed, healErr := s.healAllowRuleForApprovedRequest(ctx, link, effectiveEmail); healErr != nil {
				return AccessResult{}, healErr
			} else if healed {
				eval = AccessEvaluation{Allowed: true, Reason: "approved_access_request"}
			}
		}
		if !eval.Allowed {
			s.recordSecurityEvent(ctx, link, "", effectiveEmail, eval.Reason, "")
			return AccessResult{}, mapRuleError(eval.Reason)
		}
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

	if requiresNDA {
		if !req.NDAAgreed {
			return AccessResult{}, ErrRequiresNDA
		}
	}

	var verifiedEmail string
	if requiresEmailVerification {
		// Identity already resolved from the code above; re-verify via code lookup.
		lc, err := s.verifyLinkContactCode(ctx, token, effectiveEmail, req.EmailCode, true)
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

	// P2-strict: NDA + email verification requires an explicit delivery email that
	// matches the code-authorized mailbox (no silent fall-through to A).
	if requiresNDA && requiresEmailVerification {
		if submittedEmail == "" || !isValidEmail(submittedEmail) {
			return AccessResult{}, ErrRequiresEmail
		}
		if emailForRecords == "" || !strings.EqualFold(submittedEmail, emailForRecords) {
			s.recordSecurityEvent(ctx, link, "", emailForRecords, "delivery_email_mismatch", submittedEmail)
			return AccessResult{}, &DeliveryEmailMismatchError{AuthorizedEmail: emailForRecords}
		}
	} else if submittedEmail != "" && emailForRecords != "" && !strings.EqualFold(submittedEmail, emailForRecords) {
		// Non-NDA verification: submitted claim must still match the code mailbox.
		s.recordSecurityEvent(ctx, link, "", emailForRecords, "delivery_email_mismatch", submittedEmail)
		return AccessResult{}, &DeliveryEmailMismatchError{AuthorizedEmail: emailForRecords}
	}

	// NDA delivery email is required so the sealed PDF can be sent to the visitor.
	// Prefer the verified/authorized email; otherwise accept the email submitted with NDA.
	if requiresNDA {
		if emailForRecords == "" {
			emailForRecords = submittedEmail
		}
		if emailForRecords == "" || !isValidEmail(emailForRecords) {
			return AccessResult{}, ErrRequiresEmail
		}
	}

	visitorID := makeVisitorID(emailForRecords, req.UA)

	var ndaResponseID, ndaCertificateID string
	if requiresNDA {
		tpl, tplErr := s.resolveLinkNDATemplate(ctx, link)
		if tplErr != nil {
			return AccessResult{}, tplErr
		}
		signerName, snErr := nda.NormalizeSignerName(req.SignerName, tpl.RequireSignerName)
		if snErr != nil {
			return AccessResult{}, ErrInvalidSignerName
		}

		// Idempotent: reuse an existing signed response for this visitor+template.
		if existing, gerr := s.queries.GetLinkNDAAgreementByLinkVisitorTemplate(ctx, db.GetLinkNDAAgreementByLinkVisitorTemplateParams{
			LinkID:        link.ID,
			VisitorID:     pgtype.Text{String: visitorID, Valid: visitorID != ""},
			NdaTemplateID: tpl.ID,
		}); gerr == nil {
			ndaResponseID = uuid.UUID(existing.ID.Bytes).String()
			ndaCertificateID = existing.CertificateID
		} else if !errors.Is(gerr, pgx.ErrNoRows) {
			return AccessResult{}, fmt.Errorf("lookup NDA agreement: %w", gerr)
		} else {
			certID := uuid.NewString()
			contentHash := tpl.ContentSha256
			if contentHash == "" && s.ndaSvc != nil {
				if doc, derr := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
					ID:          tpl.SourceDocumentID,
					WorkspaceID: link.WorkspaceID,
				}); derr == nil {
					if h, herr := s.ndaSvc.HashDocumentContent(ctx, doc.StorageKey); herr == nil {
						contentHash = h
					}
				}
			}
			agreement, ndaErr := s.queries.CreateLinkNDAAgreement(ctx, db.CreateLinkNDAAgreementParams{
				TenantID:      link.TenantID,
				WorkspaceID:   link.WorkspaceID,
				LinkID:        link.ID,
				VisitorID:     pgtype.Text{String: visitorID, Valid: visitorID != ""},
				Email:         pgtype.Text{String: emailForRecords, Valid: emailForRecords != ""},
				Ip:            hashIPText(s.cfg.IPHashKey, req.IP),
				UserAgent:     pgtype.Text{String: req.UA, Valid: req.UA != ""},
				NdaTemplateID: tpl.ID,
				ContentSha256: contentHash,
				SignerName:    signerName,
				CertificateID: certID,
				SignedFileKey: "",
				Status:        "signed",
			})
			if ndaErr != nil {
				return AccessResult{}, fmt.Errorf("create link NDA agreement: %w", ndaErr)
			}
			ndaResponseID = uuid.UUID(agreement.ID.Bytes).String()
			ndaCertificateID = certID

			// Best-effort seal + notify; agreement row is already fail-closed.
			if s.ndaSvc != nil {
				go s.sealAndNotifyNDA(context.WithoutCancel(ctx), link, tpl, agreement, signerName, emailForRecords, req.UA)
			}
		}
	}

	if link.NotifyOnAccess && emailForRecords != "" && link.CreatedBy.Valid {
		wsID := ""
		if link.WorkspaceID.Valid {
			wsID = uuid.UUID(link.WorkspaceID.Bytes).String()
		}
		creatorID := uuid.UUID(link.CreatedBy.Bytes).String()
		s.sendAccessNotificationEmail(ctx, wsID, creatorID, link.Name.String, emailForRecords, publicLinkURL(s.viewerBaseURL, link.PublicToken, link.CustomDomain.String))
	}

	return AccessResult{
		Link:             link,
		VisitorID:        visitorID,
		Email:            emailForRecords,
		EmailVerified:    requiresEmailVerification,
		NDAResponseID:    ndaResponseID,
		NDACertificateID: ndaCertificateID,
	}, nil
}

func (s *Service) resolveLinkNDATemplate(ctx context.Context, link db.Link) (db.NdaTemplate, error) {
	if link.NdaTemplateID.Valid {
		tpl, err := s.queries.GetNDATemplateByID(ctx, db.GetNDATemplateByIDParams{
			ID:          link.NdaTemplateID,
			WorkspaceID: link.WorkspaceID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.NdaTemplate{}, ErrRequiresNDA
			}
			return db.NdaTemplate{}, fmt.Errorf("get NDA template: %w", err)
		}
		if tpl.Status != "active" {
			return db.NdaTemplate{}, ErrRequiresNDA
		}
		return tpl, nil
	}
	if link.NdaDocumentID.Valid {
		tpl, err := s.queries.GetNDATemplateBySourceDocument(ctx, db.GetNDATemplateBySourceDocumentParams{
			WorkspaceID:      link.WorkspaceID,
			SourceDocumentID: link.NdaDocumentID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.NdaTemplate{}, ErrRequiresNDA
			}
			return db.NdaTemplate{}, fmt.Errorf("get NDA template by document: %w", err)
		}
		if tpl.Status != "active" {
			return db.NdaTemplate{}, ErrRequiresNDA
		}
		return tpl, nil
	}
	return db.NdaTemplate{}, ErrRequiresNDA
}

func (s *Service) sealAndNotifyNDA(ctx context.Context, link db.Link, tpl db.NdaTemplate, agreement db.LinkNdaAgreement, signerName, email, ua string) {
	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          tpl.SourceDocumentID,
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil {
		logger.ErrorCtx(ctx, "nda seal: get document failed", err)
		return
	}
	signedAt := time.Now().UTC()
	if agreement.SignedAt.Valid {
		signedAt = agreement.SignedAt.Time.UTC()
	}
	signedKey, err := s.ndaSvc.SealAgreementPDF(
		ctx,
		uuid.UUID(link.TenantID.Bytes).String(),
		uuid.UUID(link.WorkspaceID.Bytes).String(),
		uuid.UUID(agreement.ID.Bytes).String(),
		doc.StorageKey,
		nda.SealParams{
			TemplateName:  tpl.Name,
			CertificateID: agreement.CertificateID,
			SignerName:    signerName,
			SignerEmail:   email,
			ContentSHA256: agreement.ContentSha256,
			LinkID:        uuid.UUID(link.ID.Bytes).String(),
			IPHash:        agreement.Ip.String,
			UserAgent:     ua,
			SignedAt:      signedAt,
		},
	)
	if err != nil {
		logger.ErrorCtx(ctx, "nda seal failed", err,
			logger.Attr("agreement_id", uuid.UUID(agreement.ID.Bytes).String()),
		)
		return
	}

	ownerEmail := ""
	if link.CreatedBy.Valid {
		if u, uerr := s.queries.GetUserByID(ctx, link.CreatedBy); uerr == nil {
			ownerEmail = u.Email
		}
	}
	s.ndaSvc.NotifySigned(ctx, email, ownerEmail, tpl.Name, agreement.CertificateID, link.Name.String, signedKey)
}

// mapRuleError maps an access rule evaluation reason to a public error.
func mapRuleError(reason string) error {
	switch reason {
	case "blocked_email":
		return ErrBlockedEmail
	case "no_allow_email_match":
		return ErrNotAllowedEmail
	case "no_allow_match":
		// Fallback for any caller still producing the legacy reason.
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

	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return ErrRequiresEmail
	}

	// Deal-room links generate contacts on demand so allow-listed recipients can
	// receive a code (owner resend or legacy public callers).
	if link.DealRoomID.Valid {
		return s.sendDealRoomEmailVerificationCode(ctx, link, email, viewerBaseURL)
	}

	// Document links use pre-defined contacts only; silently fail to avoid
	// leaking which emails are valid contacts.
	lc, err := s.queries.GetLinkContactByEmail(ctx, db.GetLinkContactByEmailParams{
		PublicToken: token,
		Email:       pgtype.Text{String: email, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("get link contact: %w", err)
	}
	allowed, err := s.allowEmailCodeSend(ctx, token, email)
	if err != nil {
		return fmt.Errorf("rate limit check: %w", err)
	}
	if !allowed {
		return ErrEmailCodeRateLimited
	}

	linkURL := publicLinkURL(viewerBaseURL, link.PublicToken, link.CustomDomain.String)
	if _, err := s.mailer.SendLinkAccessCodeEmail(ctx, email, lc.AccessCode, link.Name.String, linkURL); err != nil {
		s.markAccessCodeSendStatus(ctx, link.PublicToken, email, "failed", err.Error())
		return fmt.Errorf("send email: %w", err)
	}
	s.markAccessCodeSendStatus(ctx, link.PublicToken, email, "sent", "")
	return nil
}

// sendDealRoomEmailVerificationCode creates or refreshes a contact for the
// given email, evaluates the link's access rules, rotates the one-time code,
// then attempts delivery. The code is committed before send so a delivery
// failure still leaves a durable (rotated) code that a retry can email.
func (s *Service) sendDealRoomEmailVerificationCode(ctx context.Context, link db.Link, email, viewerBaseURL string) error {
	linkID := uuid.UUID(link.ID.Bytes).String()
	email = strings.TrimSpace(strings.ToLower(email))

	eval, err := s.EvaluateAccessRules(ctx, linkID, email)
	if err != nil {
		return fmt.Errorf("evaluate access rules: %w", err)
	}
	if !eval.Allowed {
		// Mirror the access endpoint behaviour: return the mapped rule error so
		// the caller knows the email is blocked or not allowed.
		return mapRuleError(eval.Reason)
	}

	allowed, err := s.allowEmailCodeSend(ctx, link.PublicToken, email)
	if err != nil {
		return fmt.Errorf("rate limit check: %w", err)
	}
	if !allowed {
		return ErrEmailCodeRateLimited
	}

	codes, err := s.provisionDealRoomAccessCodes(ctx, link, []string{email})
	if err != nil {
		return err
	}
	if len(codes) != 1 {
		return fmt.Errorf("provision access code: unexpected count %d", len(codes))
	}

	linkURL := publicLinkURL(viewerBaseURL, link.PublicToken, link.CustomDomain.String)
	if _, err := s.mailer.SendLinkAccessCodeEmail(ctx, email, codes[0].code, link.Name.String, linkURL); err != nil {
		s.markAccessCodeSendStatus(ctx, link.PublicToken, email, "failed", err.Error())
		return fmt.Errorf("send email: %w", err)
	}
	s.markAccessCodeSendStatus(ctx, link.PublicToken, email, "sent", "")
	return nil
}

// OwnerResendSummary is returned by OwnerResendFailedAccessCodes.
type OwnerResendSummary struct {
	Attempted int      `json:"attempted"`
	Sent      int      `json:"sent"`
	Failed    int      `json:"failed"`
	Skipped   int      `json:"skipped"`
	Errors    []string `json:"errors,omitempty"`
}

// OwnerResendAccessCode rotates and emails a verification code for one contact
// on a workspace-owned link.
//
// Dual constraint:
//   - 不漏发: failed / stuck-pending contacts can always be remediates by the owner
//   - 不骚扰: status "sent" is refused unless force=true (explicit owner intent)
func (s *Service) OwnerResendAccessCode(ctx context.Context, linkID, workspaceID, email string, force bool) error {
	link, err := s.GetByID(ctx, linkID, workspaceID)
	if err != nil {
		return err
	}
	if !link.RequireEmailVerification {
		return ErrEmailVerificationDisabled
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return ErrRequiresEmail
	}

	lc, err := s.queries.GetLinkContactByEmail(ctx, db.GetLinkContactByEmailParams{
		PublicToken: link.PublicToken,
		Email:       pgtype.Text{String: email, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAccessCodeContactNotFound
		}
		return fmt.Errorf("get link contact: %w", err)
	}

	if !force && !accessCodeNeedsRemediation(lc.CodeSendStatus, lc.CreatedAt.Time, time.Now()) {
		return ErrAccessCodeResendNotNeeded
	}

	if link.DealRoomID.Valid {
		return s.sendDealRoomEmailVerificationCode(ctx, link, email, s.viewerBaseURL)
	}

	allowed, err := s.allowEmailCodeSend(ctx, link.PublicToken, email)
	if err != nil {
		return fmt.Errorf("rate limit check: %w", err)
	}
	if !allowed {
		return ErrEmailCodeRateLimited
	}

	code, err := generateNumericCode(6)
	if err != nil {
		return fmt.Errorf("generate access code: %w", err)
	}
	if err := s.queries.UpdateLinkContactAccessCode(ctx, db.UpdateLinkContactAccessCodeParams{
		ID:         lc.ID,
		AccessCode: code,
	}); err != nil {
		return fmt.Errorf("update access code: %w", err)
	}
	_ = s.bumpAccessCodeEpoch(link.PublicToken, email)

	linkURL := publicLinkURL(s.viewerBaseURL, link.PublicToken, link.CustomDomain.String)
	if _, err := s.mailer.SendLinkAccessCodeEmail(ctx, email, code, link.Name.String, linkURL); err != nil {
		s.markAccessCodeSendStatus(ctx, link.PublicToken, email, "failed", err.Error())
		return fmt.Errorf("send email: %w", err)
	}
	s.markAccessCodeSendStatus(ctx, link.PublicToken, email, "sent", "")
	return nil
}

// OwnerResendFailedAccessCodes remediates only failed and stuck-pending contacts.
// Delivered ("sent") contacts are never included — 不骚扰.
func (s *Service) OwnerResendFailedAccessCodes(ctx context.Context, linkID, workspaceID string) (OwnerResendSummary, error) {
	link, err := s.GetByID(ctx, linkID, workspaceID)
	if err != nil {
		return OwnerResendSummary{}, err
	}
	if !link.RequireEmailVerification {
		return OwnerResendSummary{}, ErrEmailVerificationDisabled
	}

	rows, err := s.queries.ListLinkAccessCodeContactsByLink(ctx, db.ListLinkAccessCodeContactsByLinkParams{
		LinkID: link.ID,
		Limit:  int32(accessCodeContactsResendLimit),
		Offset: 0,
	})
	if err != nil {
		return OwnerResendSummary{}, fmt.Errorf("list access code contacts: %w", err)
	}

	now := time.Now()
	summary := OwnerResendSummary{}
	for _, row := range rows {
		if !accessCodeNeedsRemediation(row.CodeSendStatus, row.CreatedAt.Time, now) {
			summary.Skipped++
			continue
		}
		summary.Attempted++
		if err := s.OwnerResendAccessCode(ctx, linkID, workspaceID, row.ContactEmail, false); err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, row.ContactEmail+": "+err.Error())
			continue
		}
		summary.Sent++
	}
	return summary, nil
}

func accessCodeNeedsRemediation(status string, createdAt, now time.Time) bool {
	switch status {
	case "failed":
		return true
	case "pending":
		if createdAt.IsZero() {
			return true
		}
		return now.Sub(createdAt) >= ownerResendPendingStale
	default:
		return false
	}
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

type contactWriter interface {
	GetContactByEmailAndWorkspace(ctx context.Context, arg db.GetContactByEmailAndWorkspaceParams) (db.Contact, error)
	CreateContact(ctx context.Context, arg db.CreateContactParams) (db.Contact, error)
}

func (s *Service) getOrCreateContactByEmail(ctx context.Context, q contactWriter, workspaceID pgtype.UUID, email string) (db.Contact, error) {
	normalized := strings.TrimSpace(strings.ToLower(email))
	contact, err := q.GetContactByEmailAndWorkspace(ctx, db.GetContactByEmailAndWorkspaceParams{
		Email:       pgtype.Text{String: normalized, Valid: true},
		WorkspaceID: workspaceID,
	})
	if err == nil {
		return contact, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.Contact{}, fmt.Errorf("get contact: %w", err)
	}
	contact, err = q.CreateContact(ctx, db.CreateContactParams{
		WorkspaceID: workspaceID,
		Email:       pgtype.Text{String: normalized, Valid: true},
	})
	if err != nil {
		return db.Contact{}, fmt.Errorf("create contact: %w", err)
	}
	return contact, nil
}

func resendRateLimitKey(token, email string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return fmt.Sprintf("link:resend:ratelimit:%s:%s", token, hex.EncodeToString(h[:]))
}

func (s *Service) allowEmailCodeSend(ctx context.Context, token, email string) (bool, error) {
	if s.redisClient == nil {
		logger.InfoCtx(ctx, "redis unavailable for email code resend rate limiting; allowing send")
		return true, nil
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

// PublicLinkMetadata is the subset of link data exposed to unauthenticated visitors.
type PublicLinkMetadata struct {
	ID                          pgtype.UUID
	PublicToken                 string
	Name                        string
	Status                      string
	ExpiresAt                   pgtype.Timestamptz
	PermissionType              string
	RequireEmail                bool
	RequireEmailVerification    bool
	RequirePassword             bool
	RequireNda                  bool
	NdaDocumentID               pgtype.UUID
	DownloadEnabled             bool
	WatermarkEnabled            bool
	ScreenshotProtectionEnabled bool
	CustomDomain                string
	AiCopilotEnabled            bool
	QaEnabled                   bool
	FileRequestsEnabled         bool
	IndexFileEnabled            bool
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
	return link, nil
}

// GetPublicLinkMetadata returns safe metadata for a public link.
func (s *Service) GetPublicLinkMetadata(ctx context.Context, publicToken string) (PublicLinkMetadata, error) {
	link, err := s.GetByPublicToken(ctx, publicToken)
	if err != nil {
		return PublicLinkMetadata{}, err
	}
	return PublicLinkMetadata{
		ID:                          link.ID,
		PublicToken:                 link.PublicToken,
		Name:                        link.Name.String,
		Status:                      link.Status,
		ExpiresAt:                   link.ExpiresAt,
		PermissionType:              link.PermissionType,
		RequireEmail:                link.RequireEmail,
		RequireEmailVerification:    link.RequireEmailVerification,
		RequirePassword:             link.RequirePassword,
		RequireNda:                  link.RequireNda,
		NdaDocumentID:               link.NdaDocumentID,
		DownloadEnabled:             link.DownloadEnabled,
		WatermarkEnabled:            link.WatermarkEnabled,
		ScreenshotProtectionEnabled: link.ScreenshotProtectionEnabled,
		CustomDomain:                link.CustomDomain.String,
		AiCopilotEnabled:            link.AiCopilotEnabled,
		QaEnabled:                   link.QaEnabled,
		FileRequestsEnabled:         link.FileRequestsEnabled,
		IndexFileEnabled:            link.IndexFileEnabled,
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
	s.resolveExpiringLink(workspaceID, linkID)
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
		HasDocumentScope:         link.HasDocumentScope,
		FolderScopePaths:         link.FolderScopePaths,
		FolderScopeMode:          link.FolderScopeMode,
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
	s.resolveExpiringLink(workspaceID, linkID)
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
			s.resolveExpiringLink(workspaceID, linkID)
			return nil
		}
		return fmt.Errorf("delete link: %w", err)
	}
	if rows == 0 {
		return ErrNotFoundInWorkspace
	}
	s.resolveExpiringLink(workspaceID, linkID)
	return nil
}

// ListAccessLogs returns a page of access events for a link, including both raw
// access logs and per-page views with their durations.
func (s *Service) ListAccessLogs(ctx context.Context, linkID, workspaceID string, limit, offset int) (AccessLogsPage, error) {
	id, err := uuid.Parse(linkID)
	if err != nil {
		return AccessLogsPage{}, errors.New("invalid link id")
	}
	// Verify link exists in workspace.
	if _, err := s.GetByID(ctx, linkID, workspaceID); err != nil {
		return AccessLogsPage{}, err
	}

	limit = clampAccessLogsLimit(limit)
	offset = clampAccessLogsOffset(offset)

	rows, err := s.queries.ListAccessLogsByLink(ctx, db.ListAccessLogsByLinkParams{
		LinkID: pgtype.UUID{Bytes: id, Valid: true},
		Limit:  int32(limit + 1),
		Offset: int32(offset),
	})
	if err != nil {
		return AccessLogsPage{}, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return AccessLogsPage{Items: rows, HasMore: hasMore}, nil
}

// AccessLogsPage is one page of link access events.
type AccessLogsPage struct {
	Items   []db.ListAccessLogsByLinkRow
	HasMore bool
}

const (
	accessLogsDefaultLimit = 200
	accessLogsMaxLimit     = 200
)

func clampAccessLogsLimit(limit int) int {
	if limit <= 0 {
		return accessLogsDefaultLimit
	}
	if limit > accessLogsMaxLimit {
		return accessLogsMaxLimit
	}
	return limit
}

func clampAccessLogsOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

// LinkAnalytics aggregates access metrics for a single link.
type LinkAnalytics struct {
	TotalViews             int64                 `json:"total_views"`
	UniqueVisitors         int64                 `json:"unique_visitors"`
	DownloadAttempts       int64                 `json:"download_attempts"`
	FirstAccessAt          *time.Time            `json:"first_access_at,omitempty"`
	LastAccessAt           *time.Time            `json:"last_access_at,omitempty"`
	ViewsOverTime          []DailyView           `json:"views_over_time"`
	AverageDurationSeconds float64               `json:"average_duration_seconds"`
	RecentVisitors         []RecentVisitor       `json:"recent_visitors"`
	// RecentVisitorsHasMore is true when more visitors exist beyond the first page.
	RecentVisitorsHasMore bool                 `json:"recent_visitors_has_more"`
	KeyPages              []KeyPage            `json:"key_pages"`
	QARecords             []QARecord           `json:"qa_records"`
	AccessCodeContacts    []AccessCodeContact  `json:"access_code_contacts"`
	// AccessCodeContactsHasMore is true when more contacts exist beyond the first page.
	AccessCodeContactsHasMore bool `json:"access_code_contacts_has_more"`
	// AccessCodeFailedCount is the total failed deliveries (for nav badges).
	AccessCodeFailedCount int64 `json:"access_code_failed_count"`
	// AccessCodeRemediableCount is failed + stale-pending contacts (for resend UI).
	AccessCodeRemediableCount int64 `json:"access_code_remediable_count"`
}

// AccessCodeContact is a link-scoped contact with verification-code delivery status.
type AccessCodeContact struct {
	Email      string     `json:"email"`
	Name       string     `json:"name,omitempty"`
	SendStatus string     `json:"send_status"` // pending | sent | failed
	SendError  string     `json:"send_error,omitempty"`
	CodeSentAt *time.Time `json:"code_sent_at,omitempty"`
	UsedAt     *time.Time `json:"used_at,omitempty"`
	// CanResend is true for failed or stuck-pending contacts. Delivered contacts
	// stay false so the UI does not invite accidental re-mails (不骚扰).
	CanResend bool `json:"can_resend"`
}

// DailyView is a single day in the views-over-time series.
type DailyView struct {
	Day   string `json:"day"`
	Views int64  `json:"views"`
}

// RecentVisitor is a single aggregated visitor summary.
type RecentVisitor struct {
	VisitorID     string    `json:"visitor_id"`
	VisitorEmail  string    `json:"visitor_email,omitempty"`
	FirstAccessAt time.Time `json:"first_access_at"`
	LastAccessAt  time.Time `json:"last_access_at"`
	TotalViews    int64     `json:"total_views"`
}

// RecentVisitorsPage is one page of aggregated visitors for a link.
type RecentVisitorsPage struct {
	Items   []RecentVisitor `json:"items"`
	HasMore bool            `json:"has_more"`
}

const (
	recentVisitorsPageSize    = 10
	recentVisitorsMaxPageSize = 50
	accessCodeContactsPageSize    = 10
	accessCodeContactsMaxPageSize = 100
	accessCodeContactsResendLimit = 1000
)

func clampAccessCodeContactsLimit(limit int) int {
	if limit <= 0 {
		return accessCodeContactsPageSize
	}
	if limit > accessCodeContactsMaxPageSize {
		return accessCodeContactsMaxPageSize
	}
	return limit
}

func clampAccessCodeContactsOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func mapAccessCodeContactRows(rows []db.ListLinkAccessCodeContactsByLinkRow, now time.Time) []AccessCodeContact {
	out := make([]AccessCodeContact, 0, len(rows))
	for _, c := range rows {
		item := AccessCodeContact{
			Email:      c.ContactEmail,
			Name:       c.ContactName,
			SendStatus: c.CodeSendStatus,
			SendError:  c.CodeSendError,
			CanResend:  accessCodeNeedsRemediation(c.CodeSendStatus, c.CreatedAt.Time, now),
		}
		if c.CodeSentAt.Valid {
			t := c.CodeSentAt.Time
			item.CodeSentAt = &t
		}
		if c.UsedAt.Valid {
			t := c.UsedAt.Time
			item.UsedAt = &t
		}
		out = append(out, item)
	}
	return out
}

func trimAccessCodeContactsPage(rows []db.ListLinkAccessCodeContactsByLinkRow, limit int, now time.Time) ([]AccessCodeContact, bool) {
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return mapAccessCodeContactRows(rows, now), hasMore
}

func clampRecentVisitorsLimit(limit int) int {
	if limit <= 0 {
		return recentVisitorsPageSize
	}
	if limit > recentVisitorsMaxPageSize {
		return recentVisitorsMaxPageSize
	}
	return limit
}

func clampRecentVisitorsOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func mapRecentVisitorRows(rows []db.ListRecentVisitorsByLinkRow) []RecentVisitor {
	out := make([]RecentVisitor, 0, len(rows))
	for _, v := range rows {
		out = append(out, RecentVisitor{
			VisitorID:     v.VisitorID.String,
			TotalViews:    v.TotalViews,
			FirstAccessAt: v.FirstAccessAt.Time,
			LastAccessAt:  v.LastAccessAt.Time,
			VisitorEmail:  v.VisitorEmail,
		})
	}
	return out
}

// trimRecentVisitorsPage applies the limit+1 pattern: request limit+1 rows,
// return at most limit, and report whether another page exists.
func trimRecentVisitorsPage(rows []db.ListRecentVisitorsByLinkRow, limit int) ([]RecentVisitor, bool) {
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return mapRecentVisitorRows(rows), hasMore
}

// KeyPage is a page that received meaningful attention.
type KeyPage struct {
	PageNumber             int     `json:"page_number"`
	Views                  int64   `json:"views"`
	AverageDurationSeconds float64 `json:"average_duration_seconds"`
}

// QARecord is a visitor question and its owner answer.
type QARecord struct {
	VisitorEmail string    `json:"visitor_email,omitempty"`
	Question     string    `json:"question"`
	Answer       string    `json:"answer,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// GetLinkAnalytics returns aggregated access metrics for a link.
func (s *Service) GetLinkAnalytics(ctx context.Context, linkID, workspaceID string) (LinkAnalytics, error) {
	id, err := uuid.Parse(linkID)
	if err != nil {
		return LinkAnalytics{}, errors.New("invalid link id")
	}
	// Verify link exists in workspace.
	if _, err := s.GetByID(ctx, linkID, workspaceID); err != nil {
		return LinkAnalytics{}, err
	}

	row, err := s.queries.GetLinkAnalytics(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		return LinkAnalytics{}, fmt.Errorf("get link analytics: %w", err)
	}

	analytics := LinkAnalytics{
		TotalViews:         row.TotalViews,
		UniqueVisitors:     row.UniqueVisitors,
		DownloadAttempts:   row.DownloadAttempts,
		ViewsOverTime:      []DailyView{},
		RecentVisitors:     []RecentVisitor{},
		KeyPages:           []KeyPage{},
		QARecords:          []QARecord{},
		AccessCodeContacts: []AccessCodeContact{},
	}
	if row.FirstAccessAt.Valid {
		t := row.FirstAccessAt.Time
		analytics.FirstAccessAt = &t
	}
	if row.LastAccessAt.Valid {
		t := row.LastAccessAt.Time
		analytics.LastAccessAt = &t
	}
	if len(row.ViewsOverTime) > 0 {
		if err := json.Unmarshal(row.ViewsOverTime, &analytics.ViewsOverTime); err != nil {
			logger.ErrorCtx(ctx, "failed to unmarshal views_over_time", err)
		}
	}

	avgDur, err := s.queries.GetAverageDurationByLink(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		logger.ErrorCtx(ctx, "failed to get average duration", err)
	} else {
		analytics.AverageDurationSeconds = avgDur
	}

	visitors, err := s.queries.ListRecentVisitorsByLink(ctx, db.ListRecentVisitorsByLinkParams{
		LinkID: pgtype.UUID{Bytes: id, Valid: true},
		Limit:  int32(recentVisitorsPageSize + 1),
		Offset: 0,
	})
	if err != nil {
		logger.ErrorCtx(ctx, "failed to list recent visitors", err)
	} else {
		page, hasMore := trimRecentVisitorsPage(visitors, recentVisitorsPageSize)
		analytics.RecentVisitors = page
		analytics.RecentVisitorsHasMore = hasMore
	}

	pages, err := s.queries.ListTopPagesByLink(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		logger.ErrorCtx(ctx, "failed to list top pages", err)
	} else {
		analytics.KeyPages = make([]KeyPage, 0, len(pages))
		for _, p := range pages {
			analytics.KeyPages = append(analytics.KeyPages, KeyPage{
				PageNumber:             int(p.PageNumber),
				Views:                  p.Views,
				AverageDurationSeconds: p.AvgDurationSeconds,
			})
		}
	}

	questions, err := s.queries.ListVisitorQuestionsByLink(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		logger.ErrorCtx(ctx, "failed to list visitor questions", err)
	} else {
		analytics.QARecords = make([]QARecord, 0, len(questions))
		for _, q := range questions {
			record := QARecord{
				Question:  q.Question,
				CreatedAt: q.CreatedAt.Time,
			}
			if q.VisitorEmail.Valid {
				record.VisitorEmail = q.VisitorEmail.String
			}
			if q.Answer.Valid {
				record.Answer = q.Answer.String
			}
			analytics.QARecords = append(analytics.QARecords, record)
		}
	}

	codeContacts, err := s.queries.ListLinkAccessCodeContactsByLink(ctx, db.ListLinkAccessCodeContactsByLinkParams{
		LinkID: pgtype.UUID{Bytes: id, Valid: true},
		Limit:  int32(accessCodeContactsPageSize + 1),
		Offset: 0,
	})
	if err != nil {
		logger.ErrorCtx(ctx, "failed to list access code contacts", err)
	} else {
		page, hasMore := trimAccessCodeContactsPage(codeContacts, accessCodeContactsPageSize, time.Now())
		analytics.AccessCodeContacts = page
		analytics.AccessCodeContactsHasMore = hasMore
	}

	if failedCount, err := s.queries.CountLinkAccessCodeFailedByLink(ctx, pgtype.UUID{Bytes: id, Valid: true}); err != nil {
		logger.ErrorCtx(ctx, "failed to count failed access code contacts", err)
	} else {
		analytics.AccessCodeFailedCount = failedCount
	}
	if remediableCount, err := s.queries.CountLinkAccessCodeRemediableByLink(ctx, pgtype.UUID{Bytes: id, Valid: true}); err != nil {
		logger.ErrorCtx(ctx, "failed to count remediable access code contacts", err)
	} else {
		analytics.AccessCodeRemediableCount = remediableCount
	}

	return analytics, nil
}

// ListRecentVisitors returns a paginated page of aggregated visitors for a link.
func (s *Service) ListRecentVisitors(ctx context.Context, linkID, workspaceID string, limit, offset int) (RecentVisitorsPage, error) {
	id, err := uuid.Parse(linkID)
	if err != nil {
		return RecentVisitorsPage{}, errors.New("invalid link id")
	}
	if _, err := s.GetByID(ctx, linkID, workspaceID); err != nil {
		return RecentVisitorsPage{}, err
	}

	limit = clampRecentVisitorsLimit(limit)
	offset = clampRecentVisitorsOffset(offset)

	rows, err := s.queries.ListRecentVisitorsByLink(ctx, db.ListRecentVisitorsByLinkParams{
		LinkID: pgtype.UUID{Bytes: id, Valid: true},
		Limit:  int32(limit + 1),
		Offset: int32(offset),
	})
	if err != nil {
		return RecentVisitorsPage{}, fmt.Errorf("list recent visitors: %w", err)
	}

	items, hasMore := trimRecentVisitorsPage(rows, limit)
	return RecentVisitorsPage{Items: items, HasMore: hasMore}, nil
}

// AccessCodeContactsPage is one page of verification-code delivery contacts.
type AccessCodeContactsPage struct {
	Items   []AccessCodeContact `json:"items"`
	HasMore bool                `json:"has_more"`
}

// ListAccessCodeContacts returns a paginated page of access-code contacts for a link.
func (s *Service) ListAccessCodeContacts(ctx context.Context, linkID, workspaceID string, limit, offset int) (AccessCodeContactsPage, error) {
	id, err := uuid.Parse(linkID)
	if err != nil {
		return AccessCodeContactsPage{}, errors.New("invalid link id")
	}
	if _, err := s.GetByID(ctx, linkID, workspaceID); err != nil {
		return AccessCodeContactsPage{}, err
	}

	limit = clampAccessCodeContactsLimit(limit)
	offset = clampAccessCodeContactsOffset(offset)

	rows, err := s.queries.ListLinkAccessCodeContactsByLink(ctx, db.ListLinkAccessCodeContactsByLinkParams{
		LinkID: pgtype.UUID{Bytes: id, Valid: true},
		Limit:  int32(limit + 1),
		Offset: int32(offset),
	})
	if err != nil {
		return AccessCodeContactsPage{}, fmt.Errorf("list access code contacts: %w", err)
	}

	items, hasMore := trimAccessCodeContactsPage(rows, limit, time.Now())
	return AccessCodeContactsPage{Items: items, HasMore: hasMore}, nil
}

// normalizeSecurityConfig resolves the security configuration from the modern
// boolean flags, with backward compatibility for the legacy permission_type field.
// When explicit boolean flags are absent, permission_type drives the flags.
func normalizeSecurityConfig(req CreateLinkRequest) (requireEmail, requireEmailVerification, requireNDA bool, perm string, err error) {
	requireEmail = req.RequireEmail
	requireEmailVerification = req.RequireEmailVerification
	requireNDA = req.RequireNDA

	// Backward compatibility: legacy permission_type drives flags when explicit
	// boolean flags are not set. Modern email-verification links identify the
	// visitor by access code, so they should not force an email input field.
	switch req.PermissionType {
	case "email", "email_required":
		if !requireEmail && !requireEmailVerification {
			requireEmail = true
		}
	case "nda":
		if !requireNDA {
			requireNDA = true
		}
		if !requireEmail && !requireEmailVerification {
			requireEmail = true
		}
	}

	// NDA without email verification still needs an email for the agreement record.
	// When email verification is enabled, the contact email is derived from the code.
	if !requireEmail && requireNDA && !requireEmailVerification {
		requireEmail = true
	}

	// Allow-list rules need an email to evaluate unless email verification is on,
	// in which case the access code already identifies the allowed contact.
	if !requireEmail && !requireEmailVerification && len(req.AllowedEmails) > 0 {
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
// The entire operation runs asynchronously (detached from the request context)
// so create/update responses are never blocked by SMTP/provider latency.
// When the underlying mailer supports batching and there are multiple
// recipients, it prefers one provider batch, then falls back to per-email sends.
//
// Each emailCode carries an epoch from bumpAccessCodeEpoch. If a manual resend
// rotates the code first, the older epoch is dropped and the stale message is
// never delivered.
func (s *Service) sendAccessCodeEmails(_ context.Context, publicToken string, emailCodes []emailCode, linkName, linkURL string) {
	if len(emailCodes) == 0 {
		return
	}

	name := linkName
	if name == "" {
		name = "A shared document"
	}
	// Copy so the caller can mutate / return without racing the background send.
	codes := append([]emailCode(nil), emailCodes...)
	token := publicToken

	go func() {
		batchCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		codes = filterCurrentAccessCodeEpochs(s, token, codes)
		if len(codes) == 0 {
			return
		}

		retryCodes := codes
		if bm, ok := s.mailer.(mailer.BatchSender); ok && len(codes) > 1 {
			jobs := make([]mailer.EmailJob, 0, len(codes))
			for _, ec := range codes {
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
			if err == nil && batchAcceptedAll(result, len(jobs)) {
				for _, ec := range codes {
					s.markAccessCodeSendStatus(batchCtx, token, ec.email, "sent", "")
				}
				return
			}
			if err != nil {
				// Hard batch failure often returns an empty Failed slice — retry all.
				logger.ErrorCtx(batchCtx, "batch send access code emails failed, falling back to individual sends", err)
				retryCodes = codes
			} else {
				logger.ErrorCtx(batchCtx, "batch send access code emails had partial failures, falling back to individual sends", nil,
					logger.Attr("failed_count", len(result.Failed)),
				)
				failedSet := make(map[int]struct{}, len(result.Failed))
				for _, f := range result.Failed {
					if f.Index >= 0 && f.Index < len(codes) {
						failedSet[f.Index] = struct{}{}
					}
				}
				// Mark successes from the partial batch so the activity page reflects them.
				for i, ec := range codes {
					if _, failed := failedSet[i]; !failed {
						s.markAccessCodeSendStatus(batchCtx, token, ec.email, "sent", "")
					}
				}
				retryCodes = make([]emailCode, 0, len(failedSet))
				for i, ec := range codes {
					if _, failed := failedSet[i]; failed {
						retryCodes = append(retryCodes, ec)
					}
				}
			}
		}

		retryCodes = filterCurrentAccessCodeEpochs(s, token, retryCodes)
		for _, ec := range retryCodes {
			email, code := ec.email, ec.code
			s.emailSem <- struct{}{} // blocks until a slot is available
			go func() {
				defer func() { <-s.emailSem }()
				sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if _, err := s.mailer.SendLinkAccessCodeEmail(sendCtx, email, code, name, linkURL); err != nil {
					logger.ErrorCtx(sendCtx, "failed to send link access code email", err,
						logger.Attr("email_local", localPart(email)),
					)
					s.markAccessCodeSendStatus(sendCtx, token, email, "failed", err.Error())
					return
				}
				s.markAccessCodeSendStatus(sendCtx, token, email, "sent", "")
			}()
		}
	}()
}

func (s *Service) markAccessCodeSendStatus(ctx context.Context, publicToken, email, status, errMsg string) {
	if publicToken == "" || email == "" {
		return
	}
	if err := s.queries.UpdateLinkContactSendStatusByEmail(ctx, db.UpdateLinkContactSendStatusByEmailParams{
		PublicToken:  publicToken,
		Email:        strings.ToLower(strings.TrimSpace(email)),
		Status:       status,
		ErrorMessage: errMsg,
	}); err != nil {
		logger.ErrorCtx(ctx, "failed to update access code send status", err,
			logger.Attr("email_local", localPart(email)),
			logger.Attr("status", status),
		)
	}
}

func accessCodeEpochKey(publicToken, email string) string {
	return publicToken + "\x00" + strings.ToLower(strings.TrimSpace(email))
}

// bumpAccessCodeEpoch records a new code rotation for token+email and returns
// the generation that outbound mail for this rotation must carry.
func (s *Service) bumpAccessCodeEpoch(publicToken, email string) uint64 {
	key := accessCodeEpochKey(publicToken, email)
	for {
		fresh := new(uint64)
		atomic.StoreUint64(fresh, 1)
		val, loaded := s.accessCodeEpoch.LoadOrStore(key, fresh)
		if !loaded {
			return 1
		}
		ptr := val.(*uint64)
		old := atomic.LoadUint64(ptr)
		if atomic.CompareAndSwapUint64(ptr, old, old+1) {
			return old + 1
		}
	}
}

func (s *Service) accessCodeEpochCurrent(publicToken, email string, epoch uint64) bool {
	key := accessCodeEpochKey(publicToken, email)
	val, ok := s.accessCodeEpoch.Load(key)
	if !ok {
		return epoch == 0
	}
	return atomic.LoadUint64(val.(*uint64)) == epoch
}

func filterCurrentAccessCodeEpochs(s *Service, publicToken string, codes []emailCode) []emailCode {
	if len(codes) == 0 {
		return nil
	}
	out := make([]emailCode, 0, len(codes))
	for _, ec := range codes {
		if s.accessCodeEpochCurrent(publicToken, ec.email, ec.epoch) {
			out = append(out, ec)
		}
	}
	return out
}

// batchAcceptedAll reports whether a batch result accounts for every input job.
// AllSucceeded alone is insufficient: an empty Failed slice with no MessageIDs
// would otherwise look like success and drop emails (漏发).
func batchAcceptedAll(result mailer.BatchResult, jobCount int) bool {
	if !result.AllSucceeded() || jobCount == 0 {
		return false
	}
	accepted := len(result.MessageIDs)
	if len(result.SuccessIndexes) > accepted {
		accepted = len(result.SuccessIndexes)
	}
	return accepted >= jobCount
}

// syncDealRoomAccessCodeEmails provisions and async-sends access codes for allow
// recipients that need an outbound message.
//
// Dual constraint (不漏发 / 不骚扰):
//   - Send only for newly allow-listed emails OR allow emails still missing a
//     link_contact (first delivery).
//   - Never auto-resend when a link_contact already exists — including failed.
//     Failed / stuck-pending remediates is owner-driven via OwnerResendAccessCode
//     so a routine rules save cannot spam invitees.
//
// previousAllow:
//   - nil  → verification just enabled; only fill missing link_contacts for the
//     current allow list (不漏发). Existing contacts are left untouched (不重复发).
//   - set → rules update; send for newly allow-listed emails OR missing contacts.
func (s *Service) syncDealRoomAccessCodeEmails(ctx context.Context, link db.Link, previousAllow map[string]struct{}) error {
	if !link.RequireEmailVerification || !link.DealRoomID.Valid {
		return nil
	}

	rules, err := s.queries.ListLinkAccessRulesByLink(ctx, link.ID)
	if err != nil {
		return fmt.Errorf("list access rules: %w", err)
	}

	blocked := make(map[string]struct{})
	var allows []string
	seenAllow := make(map[string]struct{})
	for _, r := range rules {
		email := strings.TrimSpace(strings.ToLower(r.Value))
		if email == "" || r.RuleType != "email" {
			continue
		}
		if r.Action == "block" {
			blocked[email] = struct{}{}
			continue
		}
		if r.Action != "allow" {
			continue
		}
		if _, ok := seenAllow[email]; ok {
			continue
		}
		seenAllow[email] = struct{}{}
		allows = append(allows, email)
	}

	need := make([]string, 0, len(allows))
	for _, email := range allows {
		if _, isBlocked := blocked[email]; isBlocked {
			continue
		}

		if previousAllow != nil {
			if _, existed := previousAllow[email]; !existed {
				need = append(need, email)
				continue
			}
		}

		exists, lookupErr := s.hasDealRoomLinkContact(ctx, link.PublicToken, email)
		if lookupErr != nil {
			// 不漏发: treat lookup failures as missing and provision.
			logger.ErrorCtx(ctx, "link contact lookup failed during access-code sync; provisioning", lookupErr,
				logger.Attr("email_local", localPart(email)),
			)
			need = append(need, email)
			continue
		}
		if !exists {
			need = append(need, email)
		}
	}

	if len(need) == 0 {
		return nil
	}

	emailCodes, err := s.provisionDealRoomAccessCodes(ctx, link, need)
	if err != nil {
		return err
	}
	linkURL := publicLinkURL(s.viewerBaseURL, link.PublicToken, link.CustomDomain.String)
	s.sendAccessCodeEmails(ctx, link.PublicToken, emailCodes, link.Name.String, linkURL)
	return nil
}

func (s *Service) hasDealRoomLinkContact(ctx context.Context, publicToken, email string) (bool, error) {
	_, err := s.queries.GetLinkContactByEmail(ctx, db.GetLinkContactByEmailParams{
		PublicToken: publicToken,
		Email:       pgtype.Text{String: strings.ToLower(strings.TrimSpace(email)), Valid: true},
	})
	if err == nil {
		return true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return false, err
}

// linkContactProvisioner is the query surface needed to upsert deal-room access codes.
type linkContactProvisioner interface {
	contactWriter
	GetLinkContactByEmail(ctx context.Context, arg db.GetLinkContactByEmailParams) (db.GetLinkContactByEmailRow, error)
	CreateLinkContact(ctx context.Context, arg db.CreateLinkContactParams) error
	UpdateLinkContactAccessCode(ctx context.Context, arg db.UpdateLinkContactAccessCodeParams) error
}

// upsertDealRoomAccessCodes creates or rotates link_contacts for the given emails.
// Emails already present in skip are ignored. Returns codes that should be emailed.
func (s *Service) upsertDealRoomAccessCodes(
	ctx context.Context,
	q linkContactProvisioner,
	link db.Link,
	emails []string,
	skip map[string]struct{},
) ([]emailCode, error) {
	if skip == nil {
		skip = make(map[string]struct{})
	}
	out := make([]emailCode, 0, len(emails))
	for _, raw := range emails {
		email := strings.TrimSpace(strings.ToLower(raw))
		if email == "" {
			continue
		}
		if _, ok := skip[email]; ok {
			continue
		}
		skip[email] = struct{}{}

		contact, err := s.getOrCreateContactByEmail(ctx, q, link.WorkspaceID, email)
		if err != nil {
			return nil, err
		}
		code, err := generateNumericCode(6)
		if err != nil {
			return nil, fmt.Errorf("generate access code: %w", err)
		}

		lc, lcErr := q.GetLinkContactByEmail(ctx, db.GetLinkContactByEmailParams{
			PublicToken: link.PublicToken,
			Email:       pgtype.Text{String: email, Valid: true},
		})
		switch {
		case lcErr == nil:
			if err := q.UpdateLinkContactAccessCode(ctx, db.UpdateLinkContactAccessCodeParams{
				ID:         lc.ID,
				AccessCode: code,
			}); err != nil {
				return nil, fmt.Errorf("update link contact code: %w", err)
			}
		case errors.Is(lcErr, pgx.ErrNoRows):
			if err := q.CreateLinkContact(ctx, db.CreateLinkContactParams{
				LinkID:     link.ID,
				ContactID:  contact.ID,
				AccessCode: code,
			}); err != nil {
				return nil, fmt.Errorf("create link contact: %w", err)
			}
		default:
			return nil, fmt.Errorf("get link contact: %w", lcErr)
		}
		out = append(out, emailCode{
			email: email,
			code:  code,
			epoch: s.bumpAccessCodeEpoch(link.PublicToken, email),
		})
	}
	return out, nil
}

// provisionDealRoomAccessCodes creates workspace contacts + link_contacts with
// fresh access codes for the given emails. It returns the codes that should be
// emailed. Callers must only pass emails that need a new outbound message.
func (s *Service) provisionDealRoomAccessCodes(ctx context.Context, link db.Link, emails []string) ([]emailCode, error) {
	if len(emails) == 0 {
		return nil, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	out, err := s.upsertDealRoomAccessCodes(ctx, qtx, link, emails, nil)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}
	return out, nil
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

// ensureUniqueLinkName rejects duplicate link names within the relevant scope.
// Deal-room links are unique per deal room; document links are unique per workspace.
type linkNameQuerier interface {
	ExistsLinkNameInDealRoom(ctx context.Context, arg db.ExistsLinkNameInDealRoomParams) (bool, error)
	ExistsLinkNameInWorkspace(ctx context.Context, arg db.ExistsLinkNameInWorkspaceParams) (bool, error)
}

func (s *Service) ensureUniqueLinkName(
	ctx context.Context,
	q linkNameQuerier,
	workspaceID, dealRoomID pgtype.UUID,
	name string,
	excludeID pgtype.UUID,
) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	var (
		exists bool
		err    error
	)
	if dealRoomID.Valid {
		exists, err = q.ExistsLinkNameInDealRoom(ctx, db.ExistsLinkNameInDealRoomParams{
			DealRoomID: dealRoomID,
			Name:       trimmed,
			ExcludeID:  excludeID,
		})
	} else {
		exists, err = q.ExistsLinkNameInWorkspace(ctx, db.ExistsLinkNameInWorkspaceParams{
			WorkspaceID: workspaceID,
			Name:        trimmed,
			ExcludeID:   excludeID,
		})
	}
	if err != nil {
		return fmt.Errorf("check link name uniqueness: %w", err)
	}
	if exists {
		return ErrDuplicateName
	}
	return nil
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
	const minPasswordLength = 8
	if len(password) < minPasswordLength {
		return pgtype.Text{}, fmt.Errorf("%w: password must be at least %d characters", ErrInvalidPassword, minPasswordLength)
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
func (s *Service) ListLinkVisitorQuestions(ctx context.Context, link db.Link, userID string) ([]VisitorQuestion, error) {
	if err := authorizeAskHostOwnerView(ctx, s.queries, link.WorkspaceID, link.DealRoomID, userID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListVisitorQuestionsByLink(ctx, link.ID)
	if err != nil {
		return nil, err
	}
	return mapVisitorQuestions(rows), nil
}

// ListRoomVisitorQuestions returns Ask Host questions across all links in a deal room.
// Optional linkID filters to a single link within the room.
func (s *Service) ListRoomVisitorQuestions(ctx context.Context, workspaceID, roomID, userID, linkID string) ([]VisitorQuestion, error) {
	roomUUID, err := uuid.Parse(roomID)
	if err != nil {
		return nil, fmt.Errorf("invalid deal room id")
	}
	wsUUID := pgUUID(workspaceID)
	if !wsUUID.Valid {
		return nil, fmt.Errorf("invalid workspace id")
	}

	room, err := s.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
		ID:          pgtype.UUID{Bytes: roomUUID, Valid: true},
		WorkspaceID: wsUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFoundInWorkspace
		}
		return nil, fmt.Errorf("get deal room: %w", err)
	}

	if err := authorizeAskHostOwnerView(ctx, s.queries, room.WorkspaceID, room.ID, userID); err != nil {
		return nil, err
	}

	rows, err := s.queries.ListVisitorQuestionsByRoom(ctx, db.ListVisitorQuestionsByRoomParams{
		DealRoomID:  room.ID,
		WorkspaceID: wsUUID,
		Limit:       visitorQuestionsListLimit,
	})
	if err != nil {
		return nil, err
	}

	var filterLink pgtype.UUID
	if linkID != "" {
		filterLink = pgUUID(linkID)
		if !filterLink.Valid {
			return nil, ErrNotFoundInWorkspace
		}
	}

	out := make([]VisitorQuestion, 0, len(rows))
	for _, q := range rows {
		if filterLink.Valid && q.LinkID != filterLink {
			continue
		}
		out = append(out, mapVisitorQuestion(q))
	}
	return out, nil
}

// AnswerVisitorQuestion records an answer to a visitor question on a specific link.
func (s *Service) AnswerVisitorQuestion(ctx context.Context, link db.Link, questionID, userID pgtype.UUID, answer string) (VisitorQuestion, error) {
	if strings.TrimSpace(answer) == "" {
		return VisitorQuestion{}, fmt.Errorf("answer is required")
	}
	if err := authorizeAskHostOwnerView(ctx, s.queries, link.WorkspaceID, link.DealRoomID, uuid.UUID(userID.Bytes).String()); err != nil {
		return VisitorQuestion{}, err
	}
	q, err := s.queries.AnswerVisitorQuestion(ctx, db.AnswerVisitorQuestionParams{
		Answer:      pgtype.Text{String: strings.TrimSpace(answer), Valid: true},
		AnsweredBy:  userID,
		ID:          questionID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VisitorQuestion{}, ErrNotFoundInWorkspace
		}
		return VisitorQuestion{}, err
	}
	s.resolveLinkQuestion(uuid.UUID(link.WorkspaceID.Bytes).String(), uuid.UUID(questionID.Bytes).String())
	return mapVisitorQuestion(q), nil
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

const (
	indexFileCacheTTL   = 24 * time.Hour
	indexFileLLMTimeout = 30 * time.Second
)

// GenerateIndexFile creates or regenerates an AI-powered summary index for a link.
// Concurrent calls for the same link are deduplicated with singleflight, and
// ready results are cached for 24 hours before regeneration.
// Returns the index file record. The caller must verify the link belongs to the workspace.
func (s *Service) GenerateIndexFile(ctx context.Context, link db.Link) (db.LinkIndexFile, error) {
	key := uuid.UUID(link.ID.Bytes).String()
	v, err, _ := s.indexGenGroup.Do(key, func() (interface{}, error) {
		return s.generateIndexFileOnce(ctx, link)
	})
	if err != nil {
		return db.LinkIndexFile{}, err
	}
	return v.(db.LinkIndexFile), nil
}

func (s *Service) generateIndexFileOnce(ctx context.Context, link db.Link) (db.LinkIndexFile, error) {
	if s.llm == nil {
		_ = s.queries.UpdateLinkIndexFileFailed(ctx, db.UpdateLinkIndexFileFailedParams{
			ErrorMessage: pgtype.Text{String: "AI service is not configured", Valid: true},
			LinkID:       link.ID,
		})
		return db.LinkIndexFile{}, fmt.Errorf("AI service not configured")
	}

	existing, err := s.queries.GetLinkIndexFileByLink(ctx, link.ID)
	if err == nil && existing.Status == "ready" && existing.GeneratedAt.Valid {
		if time.Since(existing.GeneratedAt.Time) < indexFileCacheTTL {
			return existing, nil
		}
	}

	if _, err := s.queries.UpsertLinkIndexFile(ctx, db.UpsertLinkIndexFileParams{
		TenantID:    link.TenantID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	}); err != nil {
		return db.LinkIndexFile{}, fmt.Errorf("upsert index file: %w", err)
	}

	docContext, docErr := s.buildIndexDocumentContext(ctx, link)
	if docErr != nil {
		_ = s.queries.UpdateLinkIndexFileFailed(ctx, db.UpdateLinkIndexFileFailedParams{
			ErrorMessage: pgtype.Text{String: docErr.Error(), Valid: true},
			LinkID:       link.ID,
		})
		return db.LinkIndexFile{}, fmt.Errorf("build document context: %w", docErr)
	}

	systemPrompt := "You are an AI assistant that creates executive summaries of shared documents. Generate a concise index with: 1) a 2-3 sentence executive summary, 2) a bullet-point list of key topics covered, and 3) a recommended reading order. Format the output in HTML (without <html>/<body> tags). Use <h2>, <p>, <ul>/<li>. Keep it under 2000 characters. Do NOT make up content not in the documents."

	llmCtx, cancel := context.WithTimeout(ctx, indexFileLLMTimeout)
	defer cancel()
	content, chatErr := s.llm.ChatCompletion(llmCtx, systemPrompt, []llmMessage{
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

// buildIndexDocumentContext gathers the visible document text for a link and
// returns a structured context string suitable for an LLM index-generation
// prompt. The total length is capped to avoid exceeding model context windows.
func (s *Service) buildIndexDocumentContext(ctx context.Context, link db.Link) (string, error) {
	const maxContextChars = 100000

	docIDs := make([]pgtype.UUID, 0)
	if link.DocumentID.Valid {
		docIDs = append(docIDs, link.DocumentID)
	}
	linkDocs, err := s.queries.ListLinkDocumentsByLink(ctx, link.ID)
	if err == nil {
		for _, ld := range linkDocs {
			if ld.DocumentID.Valid {
				docIDs = append(docIDs, ld.DocumentID)
			}
		}
	}

	if len(docIDs) == 0 && link.DealRoomID.Valid {
		room, err := s.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
			ID:          link.DealRoomID,
			WorkspaceID: link.WorkspaceID,
		})
		if err == nil && room.Name != "" {
			return fmt.Sprintf("Deal room: %s\nNo document text is available yet.", room.Name), nil
		}
		return "", errors.New("no documents available for index generation")
	}

	chunks, err := s.queries.ListChunksByDocumentIDs(ctx, docIDs)
	if err != nil {
		return "", fmt.Errorf("list chunks: %w", err)
	}

	var b strings.Builder
	b.WriteString("Documents in this link:\n")
	currentDoc := uuid.Nil
	for _, c := range chunks {
		docID := uuid.UUID(c.DocumentID.Bytes)
		if docID != currentDoc {
			b.WriteString(fmt.Sprintf("\n--- Document %s (page %d) ---\n", docID, c.PageNumber))
			currentDoc = docID
		}
		b.WriteString(c.Text)
		b.WriteString("\n")
		if b.Len() > maxContextChars {
			b.WriteString("\n[Additional content truncated due to length limit]\n")
			break
		}
	}

	if b.Len() == len("Documents in this link:\n") {
		return "", errors.New("documents found but no extractable text chunks")
	}
	return b.String(), nil
}

// sanitizeHTML sanitizes untrusted LLM output to a safe HTML subset.
// It allows only the structural tags used in index files and strips all
// event handlers, styles, and unknown attributes.
func sanitizeHTML(html string) string {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("class").Globally()
	p.AllowAttrs("id").Globally()
	return p.Sanitize(html)
}

// FileUploader abstracts file storage for uploaded files.
type FileUploader interface {
	PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
}

var allowedUploadMimeTypes = map[string]bool{
	"application/pdf": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
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

// ApproveUploadedFile approves a pending uploaded file and promotes it to a
// workspace document inside the link's deal room. It runs in a transaction and
// queues an ingestion job so the document becomes searchable.
func (s *Service) ApproveUploadedFile(ctx context.Context, fileID pgtype.UUID, reviewerID pgtype.UUID) error {
	file, err := s.queries.GetUploadedFileByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("get uploaded file: %w", err)
	}
	if file.Status != "pending_review" {
		return fmt.Errorf("uploaded file is not pending review")
	}

	link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          file.LinkID,
		WorkspaceID: file.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("get link: %w", err)
	}
	if !link.DealRoomID.Valid {
		return fmt.Errorf("uploaded file approval requires a deal-room link")
	}
	if uuid.UUID(link.CreatedBy.Bytes) != uuid.UUID(reviewerID.Bytes) {
		return fmt.Errorf("only the link creator can approve uploads")
	}

	sourceType := mimeToSourceType(file.MimeType)
	if sourceType == "" {
		return fmt.Errorf("unsupported mime type for document ingestion: %s", file.MimeType)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	docID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	doc, err := qtx.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          docID,
		TenantID:    link.TenantID,
		WorkspaceID: link.WorkspaceID,
		CreatedBy:   reviewerID,
		Title:       file.OriginalFilename,
		SourceType:  sourceType,
		Status:      "uploaded",
		StorageKey:  file.StorageKey,
		FileSize:    pgtype.Int8{Int64: file.FileSize, Valid: true},
		Category:    "uploaded",
	})
	if err != nil {
		return fmt.Errorf("create document: %w", err)
	}

	folderPath := link.TargetFolderPath
	if folderPath == "" {
		folderPath = "/Uploads"
	}
	_, err = qtx.AddDealRoomDocument(ctx, db.AddDealRoomDocumentParams{
		TenantID:    link.TenantID,
		WorkspaceID: link.WorkspaceID,
		RoomID:      link.DealRoomID,
		DocumentID:  doc.ID,
		FolderPath:  folderPath,
		SortOrder:   0,
	})
	if err != nil {
		return fmt.Errorf("add deal room document: %w", err)
	}

	_, err = qtx.CreateIngestionJob(ctx, db.CreateIngestionJobParams{
		TenantID:      link.TenantID,
		WorkspaceID:   link.WorkspaceID,
		DocumentID:    doc.ID,
		Status:        "queued",
		SkipEmbedding: true, // deal-room visitor uploads: preview only until KB embed
	})
	if err != nil {
		return fmt.Errorf("create ingestion job: %w", err)
	}

	if err := qtx.UpdateUploadedFileStatus(ctx, db.UpdateUploadedFileStatusParams{
		Status:     "approved",
		ReviewedBy: reviewerID,
		ID:         fileID,
	}); err != nil {
		return fmt.Errorf("update uploaded file status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit approval: %w", err)
	}

	// Notify the uploader asynchronously; failures are logged but do not fail
	// the approval transaction.
	if file.UploaderEmail.Valid && file.UploaderEmail.String != "" && s.notifier != nil {
		_, _ = s.notifier.Enqueue(ctx,
			uuid.UUID(link.WorkspaceID.Bytes).String(),
			uuid.UUID(link.CreatedBy.Bytes).String(),
			"email",
			fmt.Sprintf("Your uploaded file has been approved: %s", file.OriginalFilename),
			fmt.Sprintf("The file '%s' you uploaded to the deal room has been approved and is now available.", file.OriginalFilename),
			notification.WithRecipient(file.UploaderEmail.String),
		)
	}

	s.resolveUploadedFile(uuid.UUID(file.WorkspaceID.Bytes).String(), uuid.UUID(fileID.Bytes).String())
	return nil
}

// RejectUploadedFile rejects a pending uploaded file.
func (s *Service) RejectUploadedFile(ctx context.Context, fileID pgtype.UUID, reviewerID pgtype.UUID) error {
	file, err := s.queries.GetUploadedFileByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("get uploaded file: %w", err)
	}
	if file.Status != "pending_review" {
		return fmt.Errorf("uploaded file is not pending review")
	}
	link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          file.LinkID,
		WorkspaceID: file.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("get link: %w", err)
	}
	if uuid.UUID(link.CreatedBy.Bytes) != uuid.UUID(reviewerID.Bytes) {
		return fmt.Errorf("only the link creator can reject uploads")
	}
	if err := s.queries.UpdateUploadedFileStatus(ctx, db.UpdateUploadedFileStatusParams{
		Status:     "rejected",
		ReviewedBy: reviewerID,
		ID:         fileID,
	}); err != nil {
		return err
	}
	s.resolveUploadedFile(uuid.UUID(file.WorkspaceID.Bytes).String(), uuid.UUID(fileID.Bytes).String())
	return nil
}

// mimeToSourceType maps uploaded-file MIME types to the document source_type
// values accepted by the documents table.
func mimeToSourceType(mime string) string {
	switch mime {
	case "application/pdf":
		return "pdf"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return "xlsx"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return "pptx"
	default:
		return ""
	}
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

func (s *Service) resolveLinkAccessRequest(workspaceID, linkID string) {
	if s.actionSyncer == nil {
		return
	}
	// Actions are keyed by link ID (see syncLinkAccessRequests). Only clear when
	// no pending requests remain for this link.
	id, err := uuid.Parse(linkID)
	if err != nil {
		return
	}
	rows, err := s.queries.ListLinkAccessRequestsByLink(context.Background(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		return
	}
	for _, r := range rows {
		if r.Status == "pending" {
			return
		}
	}
	s.actionSyncer.ResolveBySource(context.Background(), workspaceID, action.SourceTypeLinkAccessRequest, linkID)
}

func (s *Service) resolveLinkQuestion(workspaceID, questionID string) {
	if s.actionSyncer == nil {
		return
	}
	s.actionSyncer.ResolveBySource(context.Background(), workspaceID, action.SourceTypeLinkQuestion, questionID)
}

func (s *Service) resolveExpiringLink(workspaceID, linkID string) {
	if s.actionSyncer == nil {
		return
	}
	s.actionSyncer.ResolveBySource(context.Background(), workspaceID, action.SourceTypeExpiringLink, linkID)
}

func (s *Service) resolveUploadedFile(workspaceID, fileID string) {
	if s.actionSyncer == nil {
		return
	}
	s.actionSyncer.ResolveBySource(context.Background(), workspaceID, action.SourceTypeUploadedFile, fileID)
}
