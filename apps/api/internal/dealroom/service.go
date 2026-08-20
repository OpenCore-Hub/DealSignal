// Package dealroom implements data-room CRUD, membership, approvals and permissions.
package dealroom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth/emailid"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/roomacl"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/singleflight"
)

var (
	slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

	ErrRoomNotFound          = errors.New("room not found")
	ErrInvalidSlug           = errors.New("the data room URL can only contain lowercase letters, numbers, and hyphens")
	ErrDuplicateSlug         = errors.New("a data room with this URL already exists. please choose a different name")
	ErrNotRoomAdmin          = errors.New("not a room admin")
	ErrCannotManageMember    = errors.New("cannot manage this member")
	ErrInvalidRole           = errors.New("invalid role")
	ErrMemberNotFound        = errors.New("member not found")
	ErrRequestNotFound       = errors.New("access request not found")
	ErrNDARequired           = errors.New("nda required")
	ErrApprovalRequired      = errors.New("access not approved")
	ErrFolderAccessDenied    = errors.New("folder access denied")
	ErrInvalidEmail          = errors.New("invalid email")
	ErrFolderNotEmpty        = errors.New("folder is not empty")
	ErrFolderNotFound        = errors.New("folder not found")
	ErrFolderExists          = errors.New("folder already exists")
	ErrInvalidFolder         = errors.New("invalid folder")
	ErrResourceLocked        = errors.New("resource is locked")
	ErrNDANotRequired        = errors.New("nda is not required for this room")
	ErrNDAAgreementRequired  = errors.New("nda agreement is required")
	ErrNDAConsentRequired    = errors.New("nda consent is required")
	ErrNDAContentMismatch    = errors.New("nda content does not match")
	ErrNDAPreviewUnavailable = errors.New("nda preview is unavailable")
	ErrAccessRequestExists   = errors.New("access request already exists")
	ErrRateLimited           = errors.New("too many requests")
	// ErrAgreementNotAllowedInDealRoom blocks attaching agreement-library docs to rooms.
	ErrAgreementNotAllowedInDealRoom = errors.New("agreement documents cannot be added to a data room")
	// ErrArchivedDocumentNotAllowed blocks attaching library-archived docs to rooms.
	ErrArchivedDocumentNotAllowed = errors.New("archived documents cannot be added to a data room")
	ErrRoomInviteNotFound         = errors.New("invitation not found")
	ErrRoomInviteEmailMismatch    = errors.New("signed-in email does not match this invitation")
	ErrRoomInviteUsed             = errors.New("invitation already used")
	// ErrDocumentExistsOutsideRoom rejects attaching a document that already
	// belongs to another live data room. Same-id membership would share bytes.
	ErrDocumentExistsOutsideRoom = errors.New("this document is already in another live data room")
	// ErrDocumentTitleExistsInRoom rejects attaching a different id that collides
	// with a live filename already in this room.
	ErrDocumentTitleExistsInRoom   = errors.New("a document with this filename already exists in this data room")
	ErrDocumentUploadNotConfigured = errors.New("data room upload is not configured")
	ErrFolderPathRequired          = errors.New("folder path is required")
)

const (
	workspaceRoleOwner  = "owner"
	workspaceRoleAdmin  = "admin"
	workspaceRoleMember = "member"
	workspaceRoleGuest  = "guest"
)

// Beginner starts a database transaction.
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// RateLimiter is the subset of redis.Client used for public endpoint throttling.
type RateLimiter interface {
	RateLimitAllow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error)
}

// Service handles data rooms.
type Service struct {
	queries           *db.Queries
	pool              Beginner
	cfg               *config.Config
	actionSyncer      ActionSyncer
	rateLimiter       RateLimiter
	knowledgeEnqueuer KnowledgeEnqueuer
	// kvCache backs list cards and room analytics (JSON get/set/del).
	kvCache     ListCache
	listFlight  singleflight.Group
	planChecker plan.Checker
	mailer      mailer.Mailer
	docs        WorkspaceDocuments
}

// ActionSyncer resolves operational action items when room events are handled.
type ActionSyncer interface {
	ResolveBySource(ctx context.Context, workspaceID, sourceType, sourceID string)
}

// KnowledgeEnqueuer schedules external knowledge-base deletes.
// Ingest is Knowledge-tab Sync only; deal-room membership must not mint a corpus.
type KnowledgeEnqueuer interface {
	EnqueueDeleteDocument(ctx context.Context, roomID, workspaceID, documentID string) error
}

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// WithActionSyncer wires an action syncer so room events can resolve action items.
func WithActionSyncer(a ActionSyncer) ServiceOption {
	return func(s *Service) { s.actionSyncer = a }
}

// WithRateLimiter wires a rate limiter for public access-request throttling.
func WithRateLimiter(r RateLimiter) ServiceOption {
	return func(s *Service) { s.rateLimiter = r }
}

// WithKnowledgeEnqueuer wires knowledge-base delete hooks for lock/remove/delete-room.
func WithKnowledgeEnqueuer(k KnowledgeEnqueuer) ServiceOption {
	return func(s *Service) { s.knowledgeEnqueuer = k }
}

// WithListCache wires a Redis (or compatible) KV cache for room lists and analytics.
func WithListCache(c ListCache) ServiceOption {
	return func(s *Service) { s.kvCache = c }
}

// WithPlanChecker enforces workspace plan limits on room create. Nil skips checks.
func WithPlanChecker(c plan.Checker) ServiceOption {
	return func(s *Service) { s.planChecker = c }
}

// WithMailer sends room-invite notification mail. Nil skips send.
func WithMailer(m mailer.Mailer) ServiceOption {
	return func(s *Service) { s.mailer = m }
}

// NewService creates a deal room service.
func NewService(q *db.Queries, pool Beginner, cfg *config.Config, opts ...ServiceOption) *Service {
	s := &Service{queries: q, pool: pool, cfg: cfg}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CreateRoomRequest is the input for creating a room.
type CreateRoomRequest struct {
	Slug             string                 `json:"slug"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description,omitempty"`
	TemplateType     string                 `json:"template_type,omitempty"`
	Settings         map[string]interface{} `json:"settings,omitempty"`
	RequiresNDA      bool                   `json:"requires_nda,omitempty"`
	RequiresApproval bool                   `json:"requires_approval,omitempty"`
	Folders          []Folder               `json:"folders,omitempty"`
}

// Folder describes a folder stored in deal_rooms.settings.
type Folder struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	SortOrder   int    `json:"sort_order"`
	// Locked protects structure edits (rename/delete/upload/create child).
	Locked bool `json:"locked,omitempty"`
}

// DocumentOrder is used to reorder documents within a folder.
type DocumentOrder struct {
	DocumentID string `json:"document_id"`
	SortOrder  int32  `json:"sort_order"`
}

// RoomDocument is a deal room document enriched with document metadata.
type RoomDocument struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Title      string    `json:"title"`
	PageCount  int32     `json:"page_count"`
	FileSize   int64     `json:"file_size"`
	SourceType string    `json:"source_type"`
	Status     string    `json:"status"`
	FolderPath string    `json:"folder_path"`
	SortOrder  int32     `json:"sort_order"`
	Locked     bool      `json:"locked"`
	CreatedAt  time.Time `json:"created_at"`
}

// SetResourceLocksRequest toggles structure locks for folders and/or documents.
type SetResourceLocksRequest struct {
	FolderPaths []string `json:"folder_paths"`
	DocumentIDs []string `json:"document_ids"`
}

// FolderDocs groups documents by folder for the room detail response.
type FolderDocs struct {
	Folder     Folder         `json:"folder"`
	Permission string         `json:"permission"`
	Documents  []RoomDocument `json:"documents"`
}

// RoomMemberDetail is a room member with optional user name.
type RoomMemberDetail struct {
	ID          pgtype.UUID        `json:"id"`
	TenantID    pgtype.UUID        `json:"tenant_id"`
	WorkspaceID pgtype.UUID        `json:"workspace_id"`
	RoomID      pgtype.UUID        `json:"room_id"`
	Email       string             `json:"email"`
	UserID      pgtype.UUID        `json:"user_id"`
	Role        string             `json:"role"`
	NdaStatus   string             `json:"nda_status"`
	NdaSignedAt pgtype.Timestamptz `json:"nda_signed_at"`
	Status      string             `json:"status"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
	UpdatedAt   pgtype.Timestamptz `json:"updated_at"`
	UserName    string             `json:"user_name"`
}

// RoomDetail is the full enriched response for a single room.
type RoomDetail struct {
	Room             db.DealRoom
	DocumentCount    int64
	MemberCount      int64
	PendingApprovals int64
	Folders          []Folder
	Documents        []FolderDocs
	Members          []RoomMemberDetail
	AccessRequests   []db.RoomAccessRequest
	// IsAdmin is true when the caller holds room owner/admin, not workspace oversight.
	IsAdmin bool
	// Oversight is true when a workspace owner/admin can view but not mutate.
	Oversight bool
	// CanContribute is true when the caller can add/move docs or use knowledge write.
	CanContribute bool
	// RoomRole is the caller's active room_members.role, empty for oversight-only.
	RoomRole string
	// NdaRequired is true when the caller is an invited pending member and must
	// sign before documents unlock. It is not set for oversight or active members.
	NdaRequired bool
	// MemberStatus is the caller's room_members.status when they have a row.
	MemberStatus string
}

// CreateRoom creates a data room in a workspace.
func (s *Service) CreateRoom(ctx context.Context, userID, workspaceID string, req CreateRoomRequest) (db.DealRoom, error) {
	name, err := ValidateDealRoomName(req.Name)
	if err != nil {
		return db.DealRoom{}, err
	}
	req.Name = name
	if !slugRegex.MatchString(req.Slug) {
		return db.DealRoom{}, ErrInvalidSlug
	}
	if s.planChecker != nil {
		// Create-path NDA: false→true only (rooms start without NDA when flag is off).
		if req.RequiresNDA {
			if err := s.planChecker.AssertCanUseNDA(ctx, workspaceID); err != nil {
				return db.DealRoom{}, err
			}
		}
	}

	var room db.DealRoom
	insert := func(ctx context.Context) error {
		var err error
		room, err = s.insertDealRoomUniqueSlug(ctx, userID, workspaceID, req)
		return err
	}
	if s.planChecker != nil {
		if err := s.planChecker.WithCreateRoomQuota(ctx, workspaceID, insert); err != nil {
			return db.DealRoom{}, err
		}
	} else if err := insert(ctx); err != nil {
		return db.DealRoom{}, err
	}
	s.invalidateListCache(ctx, workspaceID)
	return room, nil
}

func (s *Service) insertDealRoomUniqueSlug(ctx context.Context, userID, workspaceID string, req CreateRoomRequest) (db.DealRoom, error) {
	base := req.Slug
	var last error
	for attempt := 0; attempt < slugCollisionAttempts; attempt++ {
		next := req
		if attempt > 0 {
			next.Slug = fmt.Sprintf("%s-%d", base, attempt+1)
			if !slugRegex.MatchString(next.Slug) {
				continue
			}
		}
		room, err := s.insertDealRoom(ctx, userID, workspaceID, next)
		if err == nil {
			return room, nil
		}
		last = err
		if !errors.Is(err, ErrDuplicateSlug) {
			return db.DealRoom{}, err
		}
	}
	if last == nil {
		last = ErrDuplicateSlug
	}
	return db.DealRoom{}, last
}

func (s *Service) insertDealRoom(ctx context.Context, userID, workspaceID string, req CreateRoomRequest) (db.DealRoom, error) {
	workspaceUUID := pgUUID(workspaceID)
	userUUID := pgUUID(userID)

	tenant, err := s.getTenantForWorkspace(ctx, workspaceUUID)
	if err != nil {
		return db.DealRoom{}, err
	}

	settings := make(map[string]interface{})
	for k, v := range req.Settings {
		settings[k] = v
	}

	folders := defaultFolders()
	// Client `folders` replace the template. Template structure is only a create-dialog preset.
	if req.Folders != nil {
		normalized, normErr := normalizeCreateFolders(req.Folders)
		if normErr != nil {
			return db.DealRoom{}, normErr
		}
		folders = normalized
	} else if req.TemplateType != "" {
		if tmplFolders := templateFolders(req.TemplateType); len(tmplFolders) > 0 {
			for _, f := range tmplFolders {
				folders = append(folders, Folder{
					Path:        f.Path,
					Name:        f.Name,
					Description: f.Description,
					SortOrder:   f.SortOrder,
				})
			}
		}
	}
	if len(folders) == 0 {
		folders = []Folder{generalFolder()}
	}
	settings["folders"] = folders

	settingsBytes, err := json.Marshal(settings)
	if err != nil {
		return db.DealRoom{}, fmt.Errorf("marshal settings: %w", err)
	}

	var room db.DealRoom
	if err := s.runInTx(ctx, func(q *db.Queries) error {
		created, createErr := q.CreateDealRoom(ctx, db.CreateDealRoomParams{
			TenantID:         tenant,
			WorkspaceID:      workspaceUUID,
			Slug:             req.Slug,
			Name:             req.Name,
			Description:      pgtype.Text{String: req.Description, Valid: req.Description != ""},
			TemplateType:     pgtype.Text{String: req.TemplateType, Valid: req.TemplateType != ""},
			Settings:         settingsBytes,
			RequiresNda:      req.RequiresNDA,
			RequiresApproval: req.RequiresApproval,
			Status:           "active",
			CreatedBy:        userUUID,
		})
		if createErr != nil {
			return createErr
		}
		if _, addErr := q.AddRoomMember(ctx, db.AddRoomMemberParams{
			TenantID:    tenant,
			WorkspaceID: workspaceUUID,
			RoomID:      created.ID,
			Email:       "",
			UserID:      userUUID,
			Role:        "owner",
			// Owners operate the room; they never sign the room NDA themselves.
			NdaStatus: ndaStatusForRole(created.RequiresNda, "owner"),
			Status:    "active",
		}); addErr != nil {
			return fmt.Errorf("add room owner: %w", addErr)
		}
		room = created
		return nil
	}); err != nil {
		if strings.Contains(err.Error(), "unique") {
			return db.DealRoom{}, ErrDuplicateSlug
		}
		return db.DealRoom{}, fmt.Errorf("create room: %w", err)
	}
	return room, nil
}

// RoomSummary enriches a deal room with computed aggregates.
type RoomSummary struct {
	Room             db.DealRoom
	DocumentCount    int64
	MemberCount      int64
	PendingApprovals int64
	VisitorCount     int64
	ViewCount        int64
	ActiveLinkCount  int64
	UnreadQuestions  int64
	LastAccessedAt   pgtype.Timestamptz
	HeatScore        int32
}

// RoomListPage is a paginated deal-room list response.
type RoomListPage struct {
	Items    []RoomSummary
	Page     int
	PageSize int
	Total    int64
	HasMore  bool
}

// ListRooms returns all rooms in a workspace with computed aggregates.
func (s *Service) ListRooms(ctx context.Context, workspaceID string) ([]RoomSummary, error) {
	return s.loadCachedRoomSummaries(ctx, workspaceID)
}

// ListRoomsForUser returns rooms the caller may see. Workspace owner/admin see all.
func (s *Service) ListRoomsForUser(ctx context.Context, workspaceID, userID string) ([]RoomSummary, error) {
	all, err := s.ListRooms(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.scopeRoomSummaries(ctx, workspaceID, userID, all)
}

// ListRoomsPage returns a page of rooms with aggregates. Prefer this for UI lists.
// query filters by name/description (case-insensitive); empty means no text filter.
func (s *Service) ListRoomsPage(ctx context.Context, workspaceID string, page, pageSize int, query string) (RoomListPage, error) {
	wsUUID := pgUUID(workspaceID)
	query = strings.TrimSpace(query)
	likeQuery := escapeILIKEPattern(query)
	total, err := s.queries.CountDealRoomsByWorkspace(ctx, db.CountDealRoomsByWorkspaceParams{
		WorkspaceID: wsUUID,
		Query:       likeQuery,
	})
	if err != nil {
		return RoomListPage{}, err
	}
	page, pageSize, offset := normalizeDealRoomsPaging(page, pageSize, total)
	if total == 0 {
		return RoomListPage{Items: []RoomSummary{}, Page: page, PageSize: pageSize, Total: 0}, nil
	}

	// Unfiltered small/medium workspaces: slice the slim full-list cache.
	if query == "" && total <= int64(dealRoomsMaxPageSize) {
		all, loadErr := s.loadCachedRoomSummaries(ctx, workspaceID)
		if loadErr != nil {
			return RoomListPage{}, loadErr
		}
		end := offset + pageSize
		if end > len(all) {
			end = len(all)
		}
		items := []RoomSummary{}
		if offset < len(all) {
			items = all[offset:end]
		}
		return RoomListPage{
			Items:    items,
			Page:     page,
			PageSize: pageSize,
			Total:    total,
			HasMore:  int64(page*pageSize) < total,
		}, nil
	}

	// Filtered or large workspaces: DB page + aggregates only for those room IDs.
	rooms, err := s.queries.ListDealRoomsByWorkspacePage(ctx, db.ListDealRoomsByWorkspacePageParams{
		WorkspaceID: wsUUID,
		Limit:       int32(pageSize),
		Offset:      int32(offset),
		Query:       likeQuery,
	})
	if err != nil {
		return RoomListPage{}, err
	}
	roomIDs := make([]pgtype.UUID, len(rooms))
	for i, room := range rooms {
		roomIDs[i] = room.ID
	}
	items, err := s.summariesForRooms(ctx, wsUUID, rooms, roomIDs)
	if err != nil {
		return RoomListPage{}, err
	}
	return RoomListPage{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		HasMore:  int64(page*pageSize) < total,
	}, nil
}

// ListRoomsPageForUser pages rooms the caller may see.
func (s *Service) ListRoomsPageForUser(ctx context.Context, workspaceID, userID string, page, pageSize int, query string) (RoomListPage, error) {
	if !s.roomListScoped() || strings.TrimSpace(userID) == "" {
		return s.ListRoomsPage(ctx, workspaceID, page, pageSize, query)
	}
	if ok, err := s.isWorkspaceManager(ctx, pgUUID(workspaceID), userID); err != nil {
		return RoomListPage{}, err
	} else if ok {
		return s.ListRoomsPage(ctx, workspaceID, page, pageSize, query)
	}

	wsUUID := pgUUID(workspaceID)
	query = strings.TrimSpace(query)
	likeQuery := escapeILIKEPattern(query)
	total, err := s.queries.CountDealRoomsVisibleToUser(ctx, db.CountDealRoomsVisibleToUserParams{
		WorkspaceID: wsUUID,
		Query:       likeQuery,
		UserID:      pgUUID(userID),
	})
	if err != nil {
		return RoomListPage{}, err
	}
	page, pageSize, offset := normalizeDealRoomsPaging(page, pageSize, total)
	if total == 0 {
		return RoomListPage{Items: []RoomSummary{}, Page: page, PageSize: pageSize, Total: 0}, nil
	}
	if query == "" && total <= int64(dealRoomsMaxPageSize) {
		all, loadErr := s.ListRoomsForUser(ctx, workspaceID, userID)
		if loadErr != nil {
			return RoomListPage{}, loadErr
		}
		end := offset + pageSize
		if end > len(all) {
			end = len(all)
		}
		items := []RoomSummary{}
		if offset < len(all) {
			items = all[offset:end]
		}
		return RoomListPage{Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: int64(page*pageSize) < total}, nil
	}
	rooms, err := s.queries.ListDealRoomsVisiblePage(ctx, db.ListDealRoomsVisiblePageParams{
		WorkspaceID: wsUUID,
		Query:       likeQuery,
		UserID:      pgUUID(userID),
		LimitCount:  int32(pageSize),
		OffsetCount: int32(offset),
	})
	if err != nil {
		return RoomListPage{}, err
	}
	roomIDs := make([]pgtype.UUID, len(rooms))
	for i, room := range rooms {
		roomIDs[i] = room.ID
	}
	items, err := s.summariesForRooms(ctx, wsUUID, rooms, roomIDs)
	if err != nil {
		return RoomListPage{}, err
	}
	return RoomListPage{Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: int64(page*pageSize) < total}, nil
}

func (s *Service) roomListScoped() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ROOM_LIST_SCOPED"))) {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

func (s *Service) scopeRoomSummaries(ctx context.Context, workspaceID, userID string, rooms []RoomSummary) ([]RoomSummary, error) {
	if !s.roomListScoped() || strings.TrimSpace(userID) == "" {
		return rooms, nil
	}
	if ok, err := s.isWorkspaceManager(ctx, pgUUID(workspaceID), userID); err != nil {
		return nil, err
	} else if ok {
		return rooms, nil
	}
	ids, err := s.queries.ListVisibleRoomIDsForUserInWorkspace(ctx, db.ListVisibleRoomIDsForUserInWorkspaceParams{
		WorkspaceID: pgUUID(workspaceID),
		UserID:      pgUUID(userID),
	})
	if err != nil {
		return nil, err
	}
	allow := make(map[[16]byte]struct{}, len(ids))
	for _, id := range ids {
		if id.Valid {
			allow[id.Bytes] = struct{}{}
		}
	}
	out := make([]RoomSummary, 0, len(rooms))
	for _, r := range rooms {
		if _, ok := allow[r.Room.ID.Bytes]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Service) loadCachedRoomSummaries(ctx context.Context, workspaceID string) ([]RoomSummary, error) {
	cacheKey := listCacheKey(workspaceID)
	if s.kvCache != nil {
		var cached []cachedRoomListItem
		if err := s.kvCache.Get(ctx, cacheKey, &cached); err == nil {
			return cachedToRoomSummaries(cached), nil
		}
	}

	flightCtx := context.WithoutCancel(ctx)
	v, err, _ := s.listFlight.Do(cacheKey, func() (interface{}, error) {
		if s.kvCache != nil {
			var cached []cachedRoomListItem
			if getErr := s.kvCache.Get(flightCtx, cacheKey, &cached); getErr == nil {
				return cachedToRoomSummaries(cached), nil
			}
		}
		out, loadErr := s.loadRoomSummaries(flightCtx, workspaceID)
		if loadErr != nil {
			return nil, loadErr
		}
		if s.kvCache != nil {
			if setErr := s.kvCache.Set(flightCtx, cacheKey, roomSummariesToCached(out), listCacheTTL); setErr != nil {
				logger.ErrorCtx(flightCtx, "cache data room list", setErr)
			}
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	out, ok := v.([]RoomSummary)
	if !ok {
		return nil, fmt.Errorf("unexpected list rooms cache type %T", v)
	}
	return out, nil
}

func (s *Service) loadRoomSummaries(ctx context.Context, workspaceID string) ([]RoomSummary, error) {
	wsUUID := pgUUID(workspaceID)
	rooms, err := s.queries.ListDealRoomsByWorkspace(ctx, wsUUID)
	if err != nil {
		return nil, err
	}
	return s.summariesForWorkspace(ctx, wsUUID, rooms)
}

func (s *Service) summariesForWorkspace(ctx context.Context, wsUUID pgtype.UUID, rooms []db.DealRoom) ([]RoomSummary, error) {
	aggregates, err := s.queries.GetDealRoomAggregatesByWorkspace(ctx, wsUUID)
	if err != nil {
		return nil, err
	}
	aggByRoom := make(map[string]db.GetDealRoomAggregatesByWorkspaceRow, len(aggregates))
	for _, a := range aggregates {
		aggByRoom[uuid.UUID(a.RoomID.Bytes).String()] = a
	}
	out := make([]RoomSummary, len(rooms))
	for i, room := range rooms {
		roomIDStr := uuid.UUID(room.ID.Bytes).String()
		agg, ok := aggByRoom[roomIDStr]
		if !ok {
			agg = db.GetDealRoomAggregatesByWorkspaceRow{}
		}
		out[i] = RoomSummary{
			Room:             room,
			DocumentCount:    agg.DocumentCount,
			MemberCount:      agg.MemberCount,
			PendingApprovals: agg.PendingCount,
			VisitorCount:     agg.VisitorCount,
			ViewCount:        agg.OpenCount,
			ActiveLinkCount:  agg.ActiveLinkCount,
			UnreadQuestions:  agg.PendingQuestionCount,
			LastAccessedAt:   agg.LastAccessedAt,
		}
	}
	if err := s.overlayRoomHeatScores(ctx, wsUUID, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) summariesForRooms(ctx context.Context, wsUUID pgtype.UUID, rooms []db.DealRoom, roomIDs []pgtype.UUID) ([]RoomSummary, error) {
	if len(rooms) == 0 {
		return []RoomSummary{}, nil
	}
	aggregates, err := s.queries.GetDealRoomAggregatesForRooms(ctx, db.GetDealRoomAggregatesForRoomsParams{
		WorkspaceID: wsUUID,
		RoomIds:     roomIDs,
	})
	if err != nil {
		return nil, err
	}
	aggByRoom := make(map[string]db.GetDealRoomAggregatesForRoomsRow, len(aggregates))
	for _, a := range aggregates {
		aggByRoom[uuid.UUID(a.RoomID.Bytes).String()] = a
	}
	out := make([]RoomSummary, len(rooms))
	for i, room := range rooms {
		roomIDStr := uuid.UUID(room.ID.Bytes).String()
		agg, ok := aggByRoom[roomIDStr]
		if !ok {
			agg = db.GetDealRoomAggregatesForRoomsRow{}
		}
		out[i] = RoomSummary{
			Room:             room,
			DocumentCount:    agg.DocumentCount,
			MemberCount:      agg.MemberCount,
			PendingApprovals: agg.PendingCount,
			VisitorCount:     agg.VisitorCount,
			ViewCount:        agg.OpenCount,
			ActiveLinkCount:  agg.ActiveLinkCount,
			UnreadQuestions:  agg.PendingQuestionCount,
			LastAccessedAt:   agg.LastAccessedAt,
		}
	}
	if err := s.overlayRoomHeatScores(ctx, wsUUID, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) invalidateListCache(ctx context.Context, workspaceID string) {
	if s == nil || s.kvCache == nil || strings.TrimSpace(workspaceID) == "" {
		return
	}
	if err := s.kvCache.Del(ctx, listCacheKey(workspaceID)); err != nil {
		logger.ErrorCtx(ctx, "invalidate data room list cache", err)
	}
}

const listInvalidateDebounce = 5 * time.Second

// SoftInvalidateListCache drops the workspace list cache, debounced so access-log
// write storms do not stampede Redis DEL. Structural writers should call
// invalidateListCache (hard) instead.
func (s *Service) SoftInvalidateListCache(ctx context.Context, workspaceID string) {
	if s == nil || s.kvCache == nil || strings.TrimSpace(workspaceID) == "" {
		return
	}
	if debouncer, ok := s.kvCache.(interface {
		TryAcquireDebounce(context.Context, string, time.Duration) bool
	}); ok {
		if !debouncer.TryAcquireDebounce(ctx, listCacheKey(workspaceID)+":inv", listInvalidateDebounce) {
			return
		}
	}
	s.invalidateListCache(ctx, workspaceID)
}

// InvalidateListCache exposes hard list-cache invalidation for sibling packages.
func (s *Service) InvalidateListCache(ctx context.Context, workspaceID string) {
	s.invalidateListCache(ctx, workspaceID)
}

// GetRoom returns a room scoped to a workspace.
func (s *Service) GetRoom(ctx context.Context, roomID, workspaceID string) (db.DealRoom, error) {
	id := pgUUID(roomID)
	room, err := s.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
		ID:          id,
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.DealRoom{}, ErrRoomNotFound
		}
		return db.DealRoom{}, err
	}
	return room, nil
}

// DeleteRoom soft-deletes a data room. Only active room owner/admin members may delete
// (workspace owner/admin is not sufficient unless they also hold that room role).
// Documents are detached and returned to the library as general when orphaned.
// Share links are marked deleted; knowledge Q&A rows are removed. External KB
// document deletes are enqueued only after the transaction commits.
func (s *Service) DeleteRoom(ctx context.Context, roomID, workspaceID, userID string) error {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.requireRoomOwnerOrAdmin(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return err
	}
	docs, err := s.queries.ListDealRoomDocuments(ctx, room.ID)
	if err != nil {
		return err
	}

	roomIDStr := uuid.UUID(room.ID.Bytes).String()
	wsIDStr := uuid.UUID(room.WorkspaceID.Bytes).String()

	if err := s.runInTx(ctx, func(q *db.Queries) error {
		for _, d := range docs {
			if delErr := q.DeleteDealRoomDocument(ctx, db.DeleteDealRoomDocumentParams{
				DocumentID: d.DocumentID,
				RoomID:     room.ID,
			}); delErr != nil {
				return delErr
			}
			if delErr := q.DeleteLinkDocumentsByDealRoomDocument(ctx, db.DeleteLinkDocumentsByDealRoomDocumentParams{
				DocumentID: d.DocumentID,
				DealRoomID: room.ID,
			}); delErr != nil {
				return delErr
			}
			if demoteErr := demoteDealRoomCategoryIfOrphanedQ(ctx, q, d.DocumentID, room.WorkspaceID); demoteErr != nil {
				return demoteErr
			}
		}
		if disErr := q.DisableLinksByDealRoom(ctx, db.DisableLinksByDealRoomParams{
			DealRoomID:  room.ID,
			WorkspaceID: room.WorkspaceID,
		}); disErr != nil {
			return disErr
		}
		if evalErr := q.DeleteKnowledgeQAEvalCandidatesByRoom(ctx, room.ID); evalErr != nil {
			return evalErr
		}
		if qaErr := q.DeleteKnowledgeQASessionsByRoom(ctx, room.ID); qaErr != nil {
			return qaErr
		}
		if archErr := q.DeleteKnowledgeQASessionArchivesByRoom(ctx, room.ID); archErr != nil {
			return archErr
		}
		if missionErr := q.DeleteKnowledgeQARoomMissionsByRoom(ctx, room.ID); missionErr != nil {
			return missionErr
		}
		if cancelErr := q.CancelPendingKnowledgeJobsForDeletedRoom(ctx, room.ID); cancelErr != nil {
			return cancelErr
		}
		n, delErr := q.SoftDeleteDealRoom(ctx, db.SoftDeleteDealRoomParams{
			ID:          room.ID,
			WorkspaceID: room.WorkspaceID,
		})
		if delErr != nil {
			return delErr
		}
		if n == 0 {
			return ErrRoomNotFound
		}
		return nil
	}); err != nil {
		return err
	}

	// Enqueue after commit so a rolled-back delete cannot purge a live room KB.
	if s.knowledgeEnqueuer != nil {
		for _, d := range docs {
			if kerr := s.knowledgeEnqueuer.EnqueueDeleteDocument(
				ctx,
				roomIDStr,
				wsIDStr,
				uuid.UUID(d.DocumentID.Bytes).String(),
			); kerr != nil {
				logger.ErrorCtx(ctx, "enqueue knowledge delete after delete room", kerr,
					logger.Attr("document_id", uuid.UUID(d.DocumentID.Bytes).String()),
				)
			}
		}
	}
	s.invalidateListCache(ctx, workspaceID)
	return nil
}

// AdminRoomIDsForUser returns room IDs in the workspace where the user is an
// active owner or admin. Used to overlay isAdmin on list responses without
// putting per-user flags into the workspace list cache.
func (s *Service) AdminRoomIDsForUser(ctx context.Context, workspaceID, userID string) (map[string]struct{}, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(userID) == "" {
		return map[string]struct{}{}, nil
	}
	ids, err := s.queries.ListAdminRoomIDsForUserInWorkspace(ctx, db.ListAdminRoomIDsForUserInWorkspaceParams{
		WorkspaceID: pgUUID(workspaceID),
		UserID:      pgUUID(userID),
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[uuid.UUID(id.Bytes).String()] = struct{}{}
	}
	return out, nil
}

const dealRoomRecentVisitorsLimit = 20

// RoomDailyView is one day in the deal-room views-over-time series.
type RoomDailyView struct {
	Day   string `json:"day"`
	Views int64  `json:"views"`
}

// RoomRecentVisitor is an aggregated visitor across all share links in a room.
type RoomRecentVisitor struct {
	VisitorID     string    `json:"visitorId"`
	VisitorEmail  string    `json:"visitorEmail,omitempty"`
	FirstAccessAt time.Time `json:"firstAccessAt"`
	LastAccessAt  time.Time `json:"lastAccessAt"`
	TotalViews    int64     `json:"totalViews"`
}

// RoomAnalytics is the deal-room analytics snapshot for the Analytics tab.
type RoomAnalytics struct {
	TotalViews      int64               `json:"totalViews"`
	UniqueVisitors  int64               `json:"uniqueVisitors"`
	ActiveLinkCount int64               `json:"activeLinkCount"`
	DocumentCount   int64               `json:"documentCount"`
	ViewsOverTime   []RoomDailyView     `json:"viewsOverTime"`
	RecentVisitors  []RoomRecentVisitor `json:"recentVisitors"`
}

const roomAnalyticsCacheTTL = 20 * time.Second

// GetRoomAnalytics returns aggregated view/visitor metrics for a deal room.
func (s *Service) GetRoomAnalytics(ctx context.Context, roomID, workspaceID, userID string) (RoomAnalytics, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return RoomAnalytics{}, err
	}
	if err := s.requireActiveRoomMember(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return RoomAnalytics{}, err
	}
	if s.planChecker != nil {
		if err := s.planChecker.AssertCanUseRoomAnalytics(ctx, workspaceID); err != nil {
			return RoomAnalytics{}, err
		}
	}

	cacheKey := roomAnalyticsCacheKey(workspaceID, roomID)
	if s.kvCache != nil {
		var cached RoomAnalytics
		if getErr := s.kvCache.Get(ctx, cacheKey, &cached); getErr == nil {
			return cached, nil
		}
	}

	row, err := s.queries.GetDealRoomAnalytics(ctx, db.GetDealRoomAnalyticsParams{
		WorkspaceID: pgUUID(workspaceID),
		DealRoomID:  room.ID,
	})
	if err != nil {
		return RoomAnalytics{}, fmt.Errorf("get data room analytics: %w", err)
	}

	out := RoomAnalytics{
		TotalViews:      row.TotalViews,
		UniqueVisitors:  row.UniqueVisitors,
		ActiveLinkCount: row.ActiveLinkCount,
		DocumentCount:   row.DocumentCount,
		ViewsOverTime:   []RoomDailyView{},
		RecentVisitors:  []RoomRecentVisitor{},
	}
	if len(row.ViewsOverTime) > 0 {
		if err := json.Unmarshal(row.ViewsOverTime, &out.ViewsOverTime); err != nil {
			return RoomAnalytics{}, fmt.Errorf("unmarshal views_over_time: %w", err)
		}
	}

	visitors, err := s.queries.ListRecentVisitorsByDealRoom(ctx, db.ListRecentVisitorsByDealRoomParams{
		WorkspaceID: pgUUID(workspaceID),
		DealRoomID:  room.ID,
		PageLimit:   dealRoomRecentVisitorsLimit,
	})
	if err != nil {
		return RoomAnalytics{}, fmt.Errorf("list data room recent visitors: %w", err)
	}
	out.RecentVisitors = make([]RoomRecentVisitor, 0, len(visitors))
	for _, v := range visitors {
		item := RoomRecentVisitor{
			VisitorID:  v.VisitorID,
			TotalViews: v.TotalViews,
		}
		if v.VisitorEmail != "" {
			item.VisitorEmail = v.VisitorEmail
		}
		if v.FirstAccessAt.Valid {
			item.FirstAccessAt = v.FirstAccessAt.Time
		}
		if v.LastAccessAt.Valid {
			item.LastAccessAt = v.LastAccessAt.Time
		}
		out.RecentVisitors = append(out.RecentVisitors, item)
	}
	if s.kvCache != nil {
		if setErr := s.kvCache.Set(ctx, cacheKey, out, roomAnalyticsCacheTTL); setErr != nil {
			logger.ErrorCtx(ctx, "cache data room analytics", setErr)
		}
	}
	return out, nil
}

// GetRoomSummary returns a room scoped to a workspace with computed aggregates.
func (s *Service) GetRoomSummary(ctx context.Context, roomID, workspaceID string) (RoomSummary, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return RoomSummary{}, err
	}

	docs, docErr := s.queries.ListDealRoomDocumentsWithMeta(ctx, room.ID)
	members, memErr := s.queries.ListRoomMembers(ctx, room.ID)
	requests, reqErr := s.queries.ListAccessRequestsByRoom(ctx, room.ID)
	if docErr != nil {
		logger.ErrorCtx(ctx, "list room documents failed", docErr)
	}
	if memErr != nil {
		logger.ErrorCtx(ctx, "list room members failed", memErr)
	}
	if reqErr != nil {
		logger.ErrorCtx(ctx, "list room access requests failed", reqErr)
	}

	var pending int64
	for _, r := range requests {
		if r.Status == "pending" {
			pending++
		}
	}

	var liveDocs int64
	for _, d := range docs {
		if IsArchivedDocumentStatus(d.Status) {
			continue
		}
		liveDocs++
	}

	return RoomSummary{
		Room:             room,
		DocumentCount:    liveDocs,
		MemberCount:      int64(len(members)),
		PendingApprovals: pending,
	}, nil
}

// GetRoomDetail returns a room with folders, documents, members and access requests.
// Folders and documents are visible to any active room member; members and access
// requests are only included for room admins. Invited pending members get a
// document-empty shell so they can sign NDA without gaining NeedView.
func (s *Service) GetRoomDetail(ctx context.Context, roomID, workspaceID, userID string) (RoomDetail, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return RoomDetail{}, err
	}
	caps, err := roomacl.Resolve(ctx, s.queries, room.WorkspaceID, room.ID, userID)
	if err != nil {
		return RoomDetail{}, err
	}
	if caps.InvitedPending {
		return pendingNDAShell(room, caps.RoomRole), nil
	}
	if !caps.View {
		return RoomDetail{}, ErrApprovalRequired
	}

	summary, err := s.GetRoomSummary(ctx, roomID, workspaceID)
	if err != nil {
		return RoomDetail{}, err
	}

	folders, err := s.loadFolders(summary.Room)
	if err != nil {
		return RoomDetail{}, err
	}

	docs, err := s.GetRoomDocuments(ctx, roomID, workspaceID, userID)
	if err != nil {
		return RoomDetail{}, err
	}
	var members []RoomMemberDetail
	var requests []db.RoomAccessRequest
	isAdmin := caps.Manage
	if caps.Manage || caps.Oversight {
		if rows, err := s.queries.ListRoomMembersWithUser(ctx, summary.Room.ID); err == nil {
			members = make([]RoomMemberDetail, len(rows))
			for i, r := range rows {
				members[i] = RoomMemberDetail{
					ID:          r.ID,
					TenantID:    r.TenantID,
					WorkspaceID: r.WorkspaceID,
					RoomID:      r.RoomID,
					Email:       r.Email,
					UserID:      r.UserID,
					Role:        r.Role,
					NdaStatus:   r.NdaStatus,
					NdaSignedAt: r.NdaSignedAt,
					Status:      r.Status,
					CreatedAt:   r.CreatedAt,
					UpdatedAt:   r.UpdatedAt,
					UserName:    r.UserName,
				}
			}
		}
		if caps.Manage {
			if reqs, reqErr := s.queries.ListAccessRequestsByRoom(ctx, summary.Room.ID); reqErr != nil {
				logger.ErrorCtx(ctx, "list room access requests failed", reqErr)
			} else {
				requests = reqs
			}
		}
	}

	return RoomDetail{
		Room:             summary.Room,
		DocumentCount:    summary.DocumentCount,
		MemberCount:      summary.MemberCount,
		PendingApprovals: summary.PendingApprovals,
		Folders:          folders,
		Documents:        docs,
		Members:          members,
		AccessRequests:   requests,
		IsAdmin:          isAdmin,
		Oversight:        caps.Oversight,
		CanContribute:    caps.Contribute,
		RoomRole:         caps.RoomRole,
		MemberStatus:     memberStatusFromCaps(caps),
	}, nil
}

func pendingNDAShell(room db.DealRoom, role string) RoomDetail {
	shell := room
	shell.Description = pgtype.Text{}
	shell.NdaTemplateID = pgtype.UUID{}
	shell.NdaDocumentID = pgtype.UUID{}
	return RoomDetail{
		Room:         shell,
		NdaRequired:  true,
		MemberStatus: "pending",
		RoomRole:     role,
	}
}

func memberStatusFromCaps(caps roomacl.Caps) string {
	if caps.InvitedPending {
		return "pending"
	}
	if caps.RoomRole != "" {
		return "active"
	}
	return ""
}

// AddMember adds a member to a room. Only room admins/owners can invite.
func (s *Service) AddMember(ctx context.Context, roomID, workspaceID, inviterUserID, email, role string) (db.RoomMember, error) {
	if _, err := mail.ParseAddress(email); err != nil {
		return db.RoomMember{}, ErrInvalidEmail
	}
	deliverTo := strings.ToLower(strings.TrimSpace(email))
	email = emailid.Canonical(email)
	if email == "" {
		email = deliverTo
	}
	role = roomacl.GrantableRole(role)
	if role == "" {
		return db.RoomMember{}, ErrInvalidRole
	}

	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.RoomMember{}, err
	}
	caps, err := roomacl.Require(ctx, s.queries, room.WorkspaceID, room.ID, inviterUserID, roomacl.NeedManage)
	if err != nil {
		return db.RoomMember{}, mapACL(err)
	}
	if err := canManageRoomRole(caps.RoomRole, role); err != nil {
		return db.RoomMember{}, err
	}

	if ndaStatusForRole(room.RequiresNda, role) == "pending" && !roomHasMemberNDA(room) {
		return db.RoomMember{}, ErrNDAAgreementRequired
	}

	user, userErr := lookupUserByMailbox(ctx, s.queries, deliverTo)
	if userErr != nil && !errors.Is(userErr, pgx.ErrNoRows) {
		return db.RoomMember{}, userErr
	}
	if err := s.ensureRoomMailboxAvailable(ctx, room.ID, deliverTo, user.ID); err != nil {
		return db.RoomMember{}, err
	}

	params := db.AddRoomMemberParams{
		TenantID:    room.TenantID,
		WorkspaceID: room.WorkspaceID,
		RoomID:      room.ID,
		Email:       email,
		Role:        role,
		NdaStatus:   ndaStatusForRole(room.RequiresNda, role),
		Status:      memberStatusForRole(room.RequiresNda, role),
	}
	if user.ID.Valid {
		if _, werr := s.queries.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
			WorkspaceID: room.WorkspaceID,
			UserID:      user.ID,
		}); errors.Is(werr, pgx.ErrNoRows) {
			if _, addErr := s.queries.AddWorkspaceMember(ctx, db.AddWorkspaceMemberParams{
				WorkspaceID: room.WorkspaceID,
				UserID:      user.ID,
				Role:        "guest",
			}); addErr != nil {
				return db.RoomMember{}, addErr
			}
		} else if werr != nil {
			return db.RoomMember{}, werr
		}
		params.UserID = user.ID
	}

	member, err := s.queries.AddRoomMember(ctx, params)
	if err != nil {
		return db.RoomMember{}, err
	}
	s.invalidateListCache(ctx, workspaceID)
	s.sendRoomInviteEmail(ctx, room, workspaceID, inviterUserID, deliverTo)
	return member, nil
}

func lookupUserByMailbox(ctx context.Context, q *db.Queries, email string) (db.User, error) {
	var last error
	for _, key := range emailid.Keys(email) {
		user, err := q.GetUserByEmail(ctx, key)
		if err == nil {
			return user, nil
		}
		last = err
		if !errors.Is(err, pgx.ErrNoRows) {
			return db.User{}, err
		}
	}
	if last == nil {
		last = pgx.ErrNoRows
	}
	return db.User{}, last
}

func (s *Service) findRoomMemberByMailbox(ctx context.Context, roomID pgtype.UUID, email string) (db.RoomMember, error) {
	rows, err := s.queries.ListRoomMembers(ctx, roomID)
	if err != nil {
		return db.RoomMember{}, err
	}
	if m, ok := roomacl.PickMailboxMember(rows, email); ok {
		return m, nil
	}
	return db.RoomMember{}, ErrMemberNotFound
}

func (s *Service) ensureRoomMailboxAvailable(ctx context.Context, roomID pgtype.UUID, email string, userID pgtype.UUID) error {
	rows, err := s.queries.ListRoomMembers(ctx, roomID)
	if err != nil {
		return err
	}
	for _, m := range rows {
		if emailid.SameMailbox(m.Email, email) {
			return errors.New("member already exists")
		}
		if userID.Valid && m.UserID.Valid && m.UserID == userID {
			return errors.New("member already exists")
		}
	}
	return nil
}

// SignMemberNDA lets the authenticated invitee activate their own pending row.
// When the room requires NDA, agreed must be true (informed consent). Already-active
// members remain idempotent and do not need to re-consent.
func (s *Service) SignMemberNDA(ctx context.Context, roomID, workspaceID, userID string, agreed bool, contentSHA256 string) (RoomDetail, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return RoomDetail{}, err
	}
	signer, err := s.resolveNDASigner(ctx, room, userID)
	if err != nil {
		return RoomDetail{}, err
	}
	if signer.alreadyActive {
		return s.GetRoomDetail(ctx, roomID, workspaceID, userID)
	}
	signEmail := signer.email
	uid := signer.userID

	if room.RequiresNda {
		if !agreed {
			return RoomDetail{}, ErrNDAConsentRequired
		}
		if err := s.matchMemberNDAContent(ctx, room, contentSHA256); err != nil {
			return RoomDetail{}, err
		}
		if err := s.RecordNDA(ctx, room.Slug, signEmail, "", ""); err != nil {
			return RoomDetail{}, err
		}
	} else if err := s.queries.UpdateRoomMemberStatus(ctx, db.UpdateRoomMemberStatusParams{
		Status: "active",
		RoomID: room.ID,
		Email:  signEmail,
	}); err != nil {
		return RoomDetail{}, err
	}
	if uid.Valid {
		for _, key := range emailid.Keys(signEmail) {
			_, _ = s.queries.BindRoomMembersUserByEmail(ctx, db.BindRoomMembersUserByEmailParams{
				UserID:      uid,
				WorkspaceID: room.WorkspaceID,
				Email:       key,
			})
		}
	}
	s.invalidateListCache(ctx, workspaceID)
	return s.GetRoomDetail(ctx, roomID, workspaceID, userID)
}

func (s *Service) sendRoomInviteEmail(ctx context.Context, room db.DealRoom, workspaceID, inviterUserID, email string) {
	if s.mailer == nil || s.cfg == nil {
		return
	}
	frontend := strings.TrimRight(s.cfg.FrontendURL, "/")
	if frontend == "" {
		return
	}
	ws, err := s.queries.GetWorkspaceByID(ctx, room.WorkspaceID)
	if err != nil || strings.TrimSpace(ws.Slug) == "" {
		return
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	roomURL := fmt.Sprintf("%s/%s/deal-rooms/%s", frontend, ws.Slug, roomID)
	if token, terr := mintRoomInviteToken(s.inviteTokenSecret(), roomID, email); terr == nil {
		roomURL = fmt.Sprintf("%s/room-invitations/%s/accept", frontend, token)
	}
	vars := map[string]string{
		"BrandName":      "DealSignal",
		"WorkspaceName":  ws.Name,
		"RoomName":       room.Name,
		"InvitationLink": roomURL,
	}
	if inviter := pgUUID(inviterUserID); inviter.Valid {
		if user, uerr := s.queries.GetUserByID(ctx, inviter); uerr == nil {
			vars["InviterEmail"] = user.Email
		}
	}
	if _, err := s.mailer.SendEmail(ctx, mailer.EmailJob{
		EmailType:         mailer.EmailTypeRoomInvite,
		Recipient:         email,
		WorkspaceID:       workspaceID,
		Locale:            locale.Normalize(locale.FromContext(ctx)),
		TemplateVariables: vars,
	}); err != nil {
		logger.ErrorCtx(ctx, "send room invite email failed", err)
	}
}

// ListMembers returns all members of a room. Any caller with NeedView can list.
func (s *Service) ListMembers(ctx context.Context, roomID, workspaceID, userID string) ([]RoomMemberDetail, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	if _, err := roomacl.Require(ctx, s.queries, room.WorkspaceID, room.ID, userID, roomacl.NeedView); err != nil {
		return nil, mapACL(err)
	}
	rows, err := s.queries.ListRoomMembersWithUser(ctx, room.ID)
	if err != nil {
		return nil, err
	}
	out := make([]RoomMemberDetail, len(rows))
	for i, r := range rows {
		out[i] = RoomMemberDetail{
			ID:          r.ID,
			TenantID:    r.TenantID,
			WorkspaceID: r.WorkspaceID,
			RoomID:      r.RoomID,
			Email:       r.Email,
			UserID:      r.UserID,
			Role:        r.Role,
			NdaStatus:   r.NdaStatus,
			NdaSignedAt: r.NdaSignedAt,
			Status:      r.Status,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
			UserName:    r.UserName,
		}
	}
	return out, nil
}

// UpdateRoomMemberRole changes a grantable room role. Owner rows cannot be changed.
func (s *Service) UpdateRoomMemberRole(ctx context.Context, roomID, workspaceID, actorID, memberID, role string) (db.RoomMember, error) {
	role = roomacl.GrantableRole(role)
	if role == "" {
		return db.RoomMember{}, ErrInvalidRole
	}
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.RoomMember{}, err
	}
	caps, err := roomacl.Require(ctx, s.queries, room.WorkspaceID, room.ID, actorID, roomacl.NeedManage)
	if err != nil {
		return db.RoomMember{}, mapACL(err)
	}
	member, err := s.queries.GetRoomMemberByID(ctx, db.GetRoomMemberByIDParams{
		ID:     pgUUID(memberID),
		RoomID: room.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.RoomMember{}, ErrMemberNotFound
		}
		return db.RoomMember{}, err
	}
	if member.Role == roomacl.RoleOwner {
		return db.RoomMember{}, ErrCannotManageMember
	}
	if member.UserID.Valid && uuid.UUID(member.UserID.Bytes).String() == actorID {
		return db.RoomMember{}, ErrCannotManageMember
	}
	if err := canManageRoomRole(caps.RoomRole, member.Role); err != nil {
		return db.RoomMember{}, err
	}
	if err := canManageRoomRole(caps.RoomRole, role); err != nil {
		return db.RoomMember{}, err
	}
	updated, err := s.queries.UpdateRoomMemberRole(ctx, db.UpdateRoomMemberRoleParams{
		Role:   role,
		ID:     member.ID,
		RoomID: room.ID,
	})
	if err != nil {
		return db.RoomMember{}, err
	}
	s.invalidateListCache(ctx, workspaceID)
	return updated, nil
}

// RemoveMember removes a grantable room member. Owner and self cannot be removed.
func (s *Service) RemoveMember(ctx context.Context, roomID, workspaceID, userID, memberID string) error {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	caps, err := roomacl.Require(ctx, s.queries, room.WorkspaceID, room.ID, userID, roomacl.NeedManage)
	if err != nil {
		return mapACL(err)
	}
	member, err := s.queries.GetRoomMemberByID(ctx, db.GetRoomMemberByIDParams{
		ID:     pgUUID(memberID),
		RoomID: room.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return err
	}
	if member.Role == roomacl.RoleOwner {
		return ErrCannotManageMember
	}
	if member.UserID.Valid && uuid.UUID(member.UserID.Bytes).String() == userID {
		return ErrCannotManageMember
	}
	if err := canManageRoomRole(caps.RoomRole, member.Role); err != nil {
		return err
	}
	if err := s.queries.DeleteRoomMember(ctx, db.DeleteRoomMemberParams{
		ID:     member.ID,
		RoomID: room.ID,
	}); err != nil {
		return err
	}
	s.invalidateListCache(ctx, workspaceID)
	return nil
}

// CreateAccessRequest creates a public access request for a room.
func (s *Service) CreateAccessRequest(ctx context.Context, roomSlug, email, reason, clientIP string) (db.RoomAccessRequest, error) {
	if _, err := mail.ParseAddress(email); err != nil {
		return db.RoomAccessRequest{}, ErrInvalidEmail
	}
	email = strings.ToLower(strings.TrimSpace(email))

	if err := s.checkPublicAccessRequestRateLimit(ctx, roomSlug, clientIP); err != nil {
		return db.RoomAccessRequest{}, err
	}

	room, err := s.queries.GetDealRoomBySlug(ctx, roomSlug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.RoomAccessRequest{}, ErrRoomNotFound
		}
		return db.RoomAccessRequest{}, err
	}

	existing, _ := s.queries.GetRoomMemberByEmail(ctx, db.GetRoomMemberByEmailParams{
		RoomID: room.ID,
		Email:  email,
	})
	if existing.Status == "active" {
		return db.RoomAccessRequest{}, errors.New("already a member")
	}

	if pending, perr := s.queries.GetPendingAccessRequestByRoomAndEmail(ctx, db.GetPendingAccessRequestByRoomAndEmailParams{
		RoomID: room.ID,
		Email:  email,
	}); perr == nil {
		return pending, nil
	} else if !errors.Is(perr, pgx.ErrNoRows) {
		return db.RoomAccessRequest{}, fmt.Errorf("lookup pending access request: %w", perr)
	}

	status := "pending"
	if !room.RequiresApproval {
		status = "approved"
	}

	reqParams := db.CreateAccessRequestParams{
		TenantID:    room.TenantID,
		WorkspaceID: room.WorkspaceID,
		RoomID:      room.ID,
		Email:       email,
		Reason:      pgtype.Text{String: reason, Valid: reason != ""},
		Status:      status,
	}

	if status == "approved" {
		memberParams := db.AddRoomMemberParams{
			TenantID:    room.TenantID,
			WorkspaceID: room.WorkspaceID,
			RoomID:      room.ID,
			Email:       email,
			Role:        roomacl.RoleGuest,
			NdaStatus:   ndaStatusFor(room.RequiresNda),
			Status:      memberStatusFor(room.RequiresNda),
		}
		var created db.RoomAccessRequest
		if err := s.runInTx(ctx, func(q *db.Queries) error {
			if _, err := q.AddRoomMember(ctx, memberParams); err != nil {
				return err
			}
			var err error
			created, err = q.CreateAccessRequest(ctx, reqParams)
			return err
		}); err != nil {
			return db.RoomAccessRequest{}, fmt.Errorf("create access request: %w", err)
		}
		s.invalidateListCache(ctx, uuid.UUID(room.WorkspaceID.Bytes).String())
		return created, nil
	}

	created, err := s.queries.CreateAccessRequest(ctx, reqParams)
	if err != nil {
		if strings.Contains(err.Error(), "idx_room_access_requests_pending_room_email") ||
			strings.Contains(err.Error(), "unique") {
			if pending, perr := s.queries.GetPendingAccessRequestByRoomAndEmail(ctx, db.GetPendingAccessRequestByRoomAndEmailParams{
				RoomID: room.ID,
				Email:  email,
			}); perr == nil {
				return pending, nil
			}
			return db.RoomAccessRequest{}, ErrAccessRequestExists
		}
		return db.RoomAccessRequest{}, fmt.Errorf("create access request: %w", err)
	}
	s.invalidateListCache(ctx, uuid.UUID(room.WorkspaceID.Bytes).String())
	return created, nil
}

func (s *Service) checkPublicAccessRequestRateLimit(ctx context.Context, roomSlug, clientIP string) error {
	if s.rateLimiter == nil {
		return nil
	}
	ip := strings.TrimSpace(clientIP)
	if ip == "" {
		ip = "unknown"
	}
	key := fmt.Sprintf("dealroom:access-request:%s:%s", roomSlug, ip)
	allowed, _, err := s.rateLimiter.RateLimitAllow(ctx, key, s.cfg.AccessRequestPerHour(), time.Hour)
	if err != nil {
		// Fail open: rate limiter outage must not block legitimate requests.
		logger.ErrorCtx(ctx, "dealroom access request rate limit check failed", err)
		return nil
	}
	if !allowed {
		return ErrRateLimited
	}
	return nil
}

// ApproveAccessRequest approves a pending request and activates the member.
func (s *Service) ApproveAccessRequest(ctx context.Context, requestID, roomID, workspaceID, approverUserID string) (db.RoomAccessRequest, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.RoomAccessRequest{}, err
	}
	if err := s.requireRoomAdmin(ctx, room.WorkspaceID, room.ID, approverUserID); err != nil {
		return db.RoomAccessRequest{}, err
	}

	reqUUID := pgUUID(requestID)
	req, err := s.queries.GetAccessRequestByID(ctx, db.GetAccessRequestByIDParams{
		ID:     reqUUID,
		RoomID: room.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.RoomAccessRequest{}, ErrRequestNotFound
		}
		return db.RoomAccessRequest{}, err
	}
	if req.Status != "pending" {
		return db.RoomAccessRequest{}, errors.New("request is not pending")
	}

	approverUUID := pgUUID(approverUserID)

	if err := s.runInTx(ctx, func(q *db.Queries) error {
		if err := q.UpdateAccessRequestStatus(ctx, db.UpdateAccessRequestStatusParams{
			Status:     "approved",
			ReviewedBy: approverUUID,
			ID:         req.ID,
		}); err != nil {
			return err
		}

		member, _ := q.GetRoomMemberByEmail(ctx, db.GetRoomMemberByEmailParams{
			RoomID: room.ID,
			Email:  req.Email,
		})
		if member.ID.Valid {
			return q.UpdateRoomMemberStatus(ctx, db.UpdateRoomMemberStatusParams{
				Status: "active",
				RoomID: room.ID,
				Email:  req.Email,
			})
		}
		_, err := q.AddRoomMember(ctx, db.AddRoomMemberParams{
			TenantID:    room.TenantID,
			WorkspaceID: room.WorkspaceID,
			RoomID:      room.ID,
			Email:       req.Email,
			Role:        roomacl.RoleGuest,
			NdaStatus:   ndaStatusFor(room.RequiresNda),
			Status:      memberStatusFor(room.RequiresNda),
		})
		return err
	}); err != nil {
		return db.RoomAccessRequest{}, fmt.Errorf("approve access request: %w", err)
	}

	req.Status = "approved"
	req.ReviewedBy = approverUUID
	s.resolveRoomAccessRequest(workspaceID, roomID)
	s.invalidateListCache(ctx, workspaceID)
	return req, nil
}

// RejectAccessRequest rejects a pending access request.
func (s *Service) RejectAccessRequest(ctx context.Context, requestID, roomID, workspaceID, userID string) (db.RoomAccessRequest, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.RoomAccessRequest{}, err
	}
	if err := s.requireRoomAdmin(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return db.RoomAccessRequest{}, err
	}

	req, err := s.queries.GetAccessRequestByID(ctx, db.GetAccessRequestByIDParams{
		ID:     pgUUID(requestID),
		RoomID: room.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.RoomAccessRequest{}, ErrRequestNotFound
		}
		return db.RoomAccessRequest{}, err
	}
	if req.Status != "pending" {
		return db.RoomAccessRequest{}, errors.New("request is not pending")
	}

	reviewerUUID := pgUUID(userID)
	if err := s.queries.UpdateAccessRequestStatus(ctx, db.UpdateAccessRequestStatusParams{
		Status:     "rejected",
		ReviewedBy: reviewerUUID,
		ID:         req.ID,
	}); err != nil {
		return db.RoomAccessRequest{}, err
	}
	req.Status = "rejected"
	req.ReviewedBy = reviewerUUID
	s.resolveRoomAccessRequest(workspaceID, roomID)
	s.invalidateListCache(ctx, workspaceID)
	return req, nil
}

// ListAccessRequests returns access requests for a room. Only admins can list.
func (s *Service) ListAccessRequests(ctx context.Context, roomID, workspaceID, userID string) ([]db.RoomAccessRequest, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireRoomAdmin(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return nil, err
	}
	return s.queries.ListAccessRequestsByRoom(ctx, room.ID)
}

// LoadMemberEmails returns workspace member emails for radar-honesty labeling
// on room membership access requests (same policy as link inbox / radar sync).
func (s *Service) LoadMemberEmails(ctx context.Context, workspaceID string) (action.MemberEmailSet, error) {
	wsUUID := pgUUID(workspaceID)
	if !wsUUID.Valid {
		return nil, fmt.Errorf("invalid workspace id")
	}
	return action.LoadMemberEmailSet(ctx, s.queries, wsUUID)
}

// PublicAccess checks if a visitor can access a public room.
func (s *Service) PublicAccess(ctx context.Context, slug, email string) (db.DealRoom, db.RoomMember, error) {
	room, err := s.queries.GetDealRoomBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.DealRoom{}, db.RoomMember{}, ErrRoomNotFound
		}
		return db.DealRoom{}, db.RoomMember{}, err
	}

	email = strings.ToLower(strings.TrimSpace(email))
	member, err := s.queries.GetRoomMemberByEmail(ctx, db.GetRoomMemberByEmailParams{
		RoomID: room.ID,
		Email:  email,
	})
	if err != nil || !member.ID.Valid {
		return db.DealRoom{}, db.RoomMember{}, ErrApprovalRequired
	}
	if member.Status != "active" {
		return db.DealRoom{}, db.RoomMember{}, ErrApprovalRequired
	}
	if room.RequiresNda && member.NdaStatus != "signed" {
		return room, member, ErrNDARequired
	}
	return room, member, nil
}

// RecordNDA records an NDA agreement for an existing room member and activates
// them. Callers must already be members (invited or approved); arbitrary emails
// cannot forge agreements or flip membership state.
func (s *Service) RecordNDA(ctx context.Context, roomSlug, email, ip, ua string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrInvalidEmail
	}
	room, err := s.queries.GetDealRoomBySlug(ctx, roomSlug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoomNotFound
		}
		return err
	}
	if !room.RequiresNda {
		return ErrNDANotRequired
	}

	member, err := s.queries.GetRoomMemberByEmail(ctx, db.GetRoomMemberByEmailParams{
		RoomID: room.ID,
		Email:  email,
	})
	if err != nil || !member.ID.Valid {
		return ErrMemberNotFound
	}

	tplID := room.NdaTemplateID
	if !tplID.Valid && room.NdaDocumentID.Valid {
		resolved, docID, err := s.ensureRoomNDATemplateFromDocument(
			ctx,
			room,
			"",
			uuid.UUID(room.NdaDocumentID.Bytes).String(),
		)
		if err != nil {
			return err
		}
		tplID = resolved
		if _, uerr := s.queries.UpdateDealRoomNDAAgreement(ctx, db.UpdateDealRoomNDAAgreementParams{
			NdaTemplateID: resolved,
			NdaDocumentID: docID,
			ID:            room.ID,
			WorkspaceID:   room.WorkspaceID,
		}); uerr != nil {
			logger.ErrorCtx(ctx, "record nda: backfill room template failed", uerr)
		}
	}

	if err := s.queries.CreateNDAAgreement(ctx, db.CreateNDAAgreementParams{
		RoomID:        room.ID,
		Email:         email,
		Ip:            hashIPText(s.cfg.IPHashKey, ip),
		UserAgent:     pgtype.Text{String: ua, Valid: ua != ""},
		NdaTemplateID: tplID,
		ContentSha256: s.boundMemberNDAContentSHA(ctx, room.WorkspaceID, tplID),
	}); err != nil {
		return fmt.Errorf("record nda: %w", err)
	}
	// Marks NDA signed and sets status=active so NDA-gated pending members
	// (auto-approved requests / post-approval NDA wait) can pass PublicAccess.
	if err := s.queries.UpdateRoomMemberNDA(ctx, db.UpdateRoomMemberNDAParams{
		RoomID: room.ID,
		Email:  email,
	}); err != nil {
		return fmt.Errorf("update room member NDA: %w", err)
	}
	s.resolveRoomNDA(ctx, uuid.UUID(room.WorkspaceID.Bytes).String(), room.ID, email)
	return nil
}

// SetFolderPermission sets a folder permission for a member email.
func (s *Service) SetFolderPermission(ctx context.Context, roomID, workspaceID, adminUserID, email, folderPath, permission string) (db.RoomMemberFolderPermission, error) {
	if _, err := mail.ParseAddress(email); err != nil {
		return db.RoomMemberFolderPermission{}, ErrInvalidEmail
	}
	email = strings.ToLower(strings.TrimSpace(email))

	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.RoomMemberFolderPermission{}, err
	}
	if err := s.requireRoomAdmin(ctx, room.WorkspaceID, room.ID, adminUserID); err != nil {
		return db.RoomMemberFolderPermission{}, err
	}
	member, err := s.queries.GetRoomMemberByEmail(ctx, db.GetRoomMemberByEmailParams{
		RoomID: room.ID,
		Email:  email,
	})
	if err != nil || !member.ID.Valid {
		return db.RoomMemberFolderPermission{}, ErrMemberNotFound
	}
	return s.queries.SetFolderPermission(ctx, db.SetFolderPermissionParams{
		TenantID:    room.TenantID,
		WorkspaceID: room.WorkspaceID,
		RoomID:      room.ID,
		Email:       email,
		FolderPath:  folderPath,
		Permission:  permission,
	})
}

// GetFolderPermission returns effective folder permission for a member.
func (s *Service) GetFolderPermission(ctx context.Context, roomID, email, folderPath string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	roomUUID := pgUUID(roomID)
	member, err := s.queries.GetRoomMemberByEmail(ctx, db.GetRoomMemberByEmailParams{
		RoomID: roomUUID,
		Email:  email,
	})
	if err != nil || !member.ID.Valid || member.Status != "active" {
		return "", ErrApprovalRequired
	}

	perm, err := s.queries.GetFolderPermission(ctx, db.GetFolderPermissionParams{
		RoomID:     roomUUID,
		Email:      email,
		FolderPath: folderPath,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "view", nil // default
		}
		return "", err
	}
	return perm.Permission, nil
}

// batchFolderPermissions returns a map of folderPath → permission for all folders
// accessible by a member in a single query, avoiding N+1 per-folder calls.
func (s *Service) batchFolderPermissions(ctx context.Context, roomID pgtype.UUID, email string) (map[string]string, error) {
	perms, err := s.queries.GetFolderPermissionsByRoomAndEmail(ctx, db.GetFolderPermissionsByRoomAndEmailParams{
		RoomID: roomID,
		Email:  email,
	})
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(perms))
	for _, p := range perms {
		m[p.FolderPath] = p.Permission
	}
	return m, nil
}

// AddDocument adds a document to a room folder.
func (s *Service) AddDocument(ctx context.Context, roomID, workspaceID, adminUserID, documentID, folderPath string, sortOrder int32) (db.DealRoomDocument, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.DealRoomDocument{}, err
	}
	if err := s.requireRoomContribute(ctx, room.WorkspaceID, room.ID, adminUserID); err != nil {
		return db.DealRoomDocument{}, err
	}
	folders, err := s.loadFolders(room)
	if err != nil {
		return db.DealRoomDocument{}, err
	}
	folderPath = normalizeFolderPath(folderPath)
	if folderPath == "/" || folderPath == "" {
		return db.DealRoomDocument{}, ErrFolderPathRequired
	}
	if !folderExists(folders, folderPath) {
		return db.DealRoomDocument{}, ErrFolderNotFound
	}
	if folderIsLocked(folders, folderPath) {
		return db.DealRoomDocument{}, ErrResourceLocked
	}
	docID, err := uuid.Parse(documentID)
	if err != nil {
		return db.DealRoomDocument{}, errors.New("invalid document id")
	}
	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		WorkspaceID: room.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.DealRoomDocument{}, errors.New("document not found")
		}
		return db.DealRoomDocument{}, err
	}
	if IsArchivedDocumentStatus(doc.Status) {
		return db.DealRoomDocument{}, ErrArchivedDocumentNotAllowed
	}
	if strings.EqualFold(doc.Category, upload.CategoryAgreement) {
		return db.DealRoomDocument{}, ErrAgreementNotAllowedInDealRoom
	}

	var row db.DealRoomDocument
	if err := s.runInTx(ctx, func(q *db.Queries) error {
		// Document lock first, then title — same order everywhere to avoid deadlock.
		if lockErr := upload.LockLiveDealRoomDocument(ctx, q, doc.ID); lockErr != nil {
			return lockErr
		}
		if strings.TrimSpace(doc.Title) != "" {
			if lockErr := upload.LockRoomTitle(ctx, q, room.ID, doc.Title); lockErr != nil {
				return lockErr
			}
		}

		// Idempotent: replacing an upload keeps the same document id; re-adding it
		// to the room should update folder placement instead of failing UNIQUE.
		if existing, getErr := q.GetDealRoomDocumentByDocumentID(ctx, db.GetDealRoomDocumentByDocumentIDParams{
			RoomID:     room.ID,
			DocumentID: doc.ID,
		}); getErr == nil {
			if existing.FolderPath != folderPath {
				if err := q.UpdateDealRoomDocumentFolder(ctx, db.UpdateDealRoomDocumentFolderParams{
					FolderPath: folderPath,
					ID:         existing.ID,
					RoomID:     room.ID,
				}); err != nil {
					return err
				}
				existing.FolderPath = folderPath
			}
			if existing.SortOrder != sortOrder {
				if err := q.UpdateDealRoomDocumentSortOrder(ctx, db.UpdateDealRoomDocumentSortOrderParams{
					SortOrder: sortOrder,
					ID:        existing.ID,
					RoomID:    room.ID,
				}); err != nil {
					return err
				}
				existing.SortOrder = sortOrder
			}
			if err := ensureDealRoomDocumentCategoryQ(ctx, q, doc); err != nil {
				return err
			}
			row = existing
			return nil
		} else if !errors.Is(getErr, pgx.ErrNoRows) {
			return getErr
		}

		liveMem, liveErr := q.GetLiveDealRoomMembershipByDocument(ctx, doc.ID)
		if liveErr == nil && liveMem.RoomID != room.ID {
			return ErrDocumentExistsOutsideRoom
		}
		if liveErr != nil && !errors.Is(liveErr, pgx.ErrNoRows) {
			return liveErr
		}
		if titleHit, titleErr := q.GetLiveDealRoomDocumentByTitle(ctx, db.GetLiveDealRoomDocumentByTitleParams{
			RoomID: room.ID,
			Title:  doc.Title,
		}); titleErr == nil && titleHit.ID != doc.ID {
			return ErrDocumentTitleExistsInRoom
		} else if titleErr != nil && !errors.Is(titleErr, pgx.ErrNoRows) {
			return titleErr
		}

		created, addErr := q.AddDealRoomDocument(ctx, db.AddDealRoomDocumentParams{
			TenantID:    room.TenantID,
			WorkspaceID: room.WorkspaceID,
			RoomID:      room.ID,
			DocumentID:  doc.ID,
			FolderPath:  folderPath,
			SortOrder:   sortOrder,
		})
		if addErr != nil {
			return addErr
		}
		if err := ensureDealRoomDocumentCategoryQ(ctx, q, doc); err != nil {
			return err
		}
		row = created
		return nil
	}); err != nil {
		return db.DealRoomDocument{}, err
	}
	s.invalidateListCache(ctx, workspaceID)
	return row, nil
}

func ensureDealRoomDocumentCategoryQ(ctx context.Context, q *db.Queries, doc db.GetDocumentByIDRow) error {
	if strings.EqualFold(doc.Category, upload.CategoryDealRoom) {
		return nil
	}
	if strings.EqualFold(doc.Category, upload.CategoryAgreement) {
		return ErrAgreementNotAllowedInDealRoom
	}
	return q.UpdateDocumentCategory(ctx, db.UpdateDocumentCategoryParams{
		Category:    upload.CategoryDealRoom,
		ID:          doc.ID,
		WorkspaceID: doc.WorkspaceID,
	})
}

// RemoveDocument removes a document from a room. Room owner/admin only.
func (s *Service) RemoveDocument(ctx context.Context, roomID, workspaceID, userID, documentID string) error {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.requireRoomAdmin(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return err
	}
	id, err := uuid.Parse(documentID)
	if err != nil {
		return errors.New("invalid document id")
	}
	existing, err := s.queries.GetDealRoomDocumentByDocumentID(ctx, db.GetDealRoomDocumentByDocumentIDParams{
		RoomID:     room.ID,
		DocumentID: pgtype.UUID{Bytes: id, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("document not found")
		}
		return err
	}
	if existing.Locked {
		return ErrResourceLocked
	}
	docUUID := pgtype.UUID{Bytes: id, Valid: true}
	if err := s.queries.DeleteDealRoomDocument(ctx, db.DeleteDealRoomDocumentParams{
		DocumentID: docUUID,
		RoomID:     room.ID,
	}); err != nil {
		return err
	}
	// Also remove the document from any deal-room share-link scopes so that
	// scoped links do not continue serving a document that is no longer in the room.
	if err := s.queries.DeleteLinkDocumentsByDealRoomDocument(ctx, db.DeleteLinkDocumentsByDealRoomDocumentParams{
		DocumentID: docUUID,
		DealRoomID: room.ID,
	}); err != nil {
		return err
	}
	if err := s.demoteDealRoomCategoryIfOrphaned(ctx, docUUID, room.WorkspaceID); err != nil {
		return err
	}
	if s.knowledgeEnqueuer != nil {
		if kerr := s.knowledgeEnqueuer.EnqueueDeleteDocument(
			ctx,
			uuid.UUID(room.ID.Bytes).String(),
			uuid.UUID(room.WorkspaceID.Bytes).String(),
			id.String(),
		); kerr != nil {
			logger.ErrorCtx(ctx, "enqueue knowledge delete after remove document", kerr)
		}
	}
	s.invalidateListCache(ctx, workspaceID)
	return nil
}

// demoteDealRoomCategoryIfOrphaned sets deal_room → general when no room membership remains.
func (s *Service) demoteDealRoomCategoryIfOrphaned(ctx context.Context, documentID, workspaceID pgtype.UUID) error {
	return demoteDealRoomCategoryIfOrphanedQ(ctx, s.queries, documentID, workspaceID)
}

func demoteDealRoomCategoryIfOrphanedQ(ctx context.Context, q *db.Queries, documentID, workspaceID pgtype.UUID) error {
	n, err := q.CountDealRoomMembershipsByDocument(ctx, documentID)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	doc, err := q.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          documentID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if !strings.EqualFold(doc.Category, upload.CategoryDealRoom) {
		return nil
	}
	general, genErr := q.GetDocumentByTitleInWorkspaceCategory(ctx, db.GetDocumentByTitleInWorkspaceCategoryParams{
		WorkspaceID: workspaceID,
		Title:       doc.Title,
		Category:    upload.CategoryGeneral,
	})
	if genErr != nil && !errors.Is(genErr, pgx.ErrNoRows) {
		return genErr
	}
	if genErr == nil && general.ID != documentID {
		renamed, renameErr := upload.UniqueRestoredTitle(ctx, q, workspaceID, doc.Title)
		if renameErr != nil {
			return renameErr
		}
		if err := q.UpdateDocumentTitle(ctx, db.UpdateDocumentTitleParams{
			Title:       renamed,
			ID:          documentID,
			WorkspaceID: workspaceID,
		}); err != nil {
			return err
		}
	}
	return q.UpdateDocumentCategory(ctx, db.UpdateDocumentCategoryParams{
		Category:    upload.CategoryGeneral,
		ID:          documentID,
		WorkspaceID: workspaceID,
	})
}

// MoveDocument moves a document to another folder. Only admins can move.
func (s *Service) MoveDocument(ctx context.Context, roomID, workspaceID, userID, documentID, folderPath string, sortOrder *int32) error {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.requireRoomContribute(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return err
	}
	folders, err := s.ListFolders(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if !folderExists(folders, folderPath) {
		return ErrFolderNotFound
	}
	if folderIsLocked(folders, folderPath) {
		return ErrResourceLocked
	}

	id, err := uuid.Parse(documentID)
	if err != nil {
		return errors.New("invalid document id")
	}
	existing, err := s.queries.GetDealRoomDocument(ctx, db.GetDealRoomDocumentParams{
		ID:     pgtype.UUID{Bytes: id, Valid: true},
		RoomID: room.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("document not found")
		}
		return err
	}
	if existing.Locked {
		return ErrResourceLocked
	}
	if err := s.queries.UpdateDealRoomDocumentFolder(ctx, db.UpdateDealRoomDocumentFolderParams{
		FolderPath: folderPath,
		ID:         existing.ID,
		RoomID:     room.ID,
	}); err != nil {
		return err
	}
	if sortOrder != nil {
		return s.queries.UpdateDealRoomDocumentSortOrder(ctx, db.UpdateDealRoomDocumentSortOrderParams{
			SortOrder: *sortOrder,
			ID:        existing.ID,
			RoomID:    room.ID,
		})
	}
	return nil
}

// ReorderDocuments updates sort orders for documents in a room. Only admins can reorder.
func (s *Service) ReorderDocuments(ctx context.Context, roomID, workspaceID, userID string, orders []DocumentOrder) error {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.requireRoomContribute(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return err
	}
	return s.runInTx(ctx, func(q *db.Queries) error {
		for _, o := range orders {
			id, err := uuid.Parse(o.DocumentID)
			if err != nil {
				return errors.New("invalid document id")
			}
			if err := q.UpdateDealRoomDocumentSortOrder(ctx, db.UpdateDealRoomDocumentSortOrderParams{
				SortOrder: o.SortOrder,
				ID:        pgtype.UUID{Bytes: id, Valid: true},
				RoomID:    room.ID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListDocuments returns documents in a room that the member can access,
// grouped by folder with the effective permission for each folder.
func (s *Service) ListDocuments(ctx context.Context, roomID, workspaceID, email string) ([]FolderDocs, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	member, err := s.queries.GetRoomMemberByEmail(ctx, db.GetRoomMemberByEmailParams{
		RoomID: room.ID,
		Email:  email,
	})
	if err != nil || !member.ID.Valid || member.Status != "active" {
		return nil, ErrApprovalRequired
	}

	folders, err := s.loadFolders(room)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListDealRoomDocumentsWithMeta(ctx, room.ID)
	if err != nil {
		return nil, err
	}

	docsByFolder := make(map[string][]RoomDocument)
	for _, r := range rows {
		if IsArchivedDocumentStatus(r.Status) {
			continue
		}
		var pageCount int32
		if r.PageCount.Valid {
			pageCount = r.PageCount.Int32
		}
		var fileSize int64
		if r.FileSize.Valid {
			fileSize = r.FileSize.Int64
		}
		d := RoomDocument{
			ID:         uuid.UUID(r.ID.Bytes).String(),
			DocumentID: uuid.UUID(r.DocumentID.Bytes).String(),
			Title:      r.DocumentTitle,
			PageCount:  pageCount,
			FileSize:   fileSize,
			SourceType: r.SourceType,
			Status:     r.Status,
			FolderPath: r.FolderPath,
			SortOrder:  r.SortOrder,
			Locked:     r.Locked,
			CreatedAt:  r.CreatedAt.Time,
		}
		docsByFolder[r.FolderPath] = append(docsByFolder[r.FolderPath], d)
	}

	// Single batch query replaces N per-folder GetFolderPermission calls.
	permMap, err := s.batchFolderPermissions(ctx, room.ID, email)
	if err != nil {
		return nil, err
	}
	// Default all real folders to view when no explicit permission is set.
	for _, f := range folders {
		if _, ok := permMap[f.Path]; !ok {
			permMap[f.Path] = "view"
		}
	}

	out := make([]FolderDocs, 0, len(folders))
	for _, f := range folders {
		perm := permMap[f.Path]
		if perm == "none" {
			continue
		}
		docs := docsByFolder[f.Path]
		if docs == nil {
			docs = []RoomDocument{}
		}
		sort.Slice(docs, func(i, j int) bool {
			if docs[i].SortOrder != docs[j].SortOrder {
				return docs[i].SortOrder < docs[j].SortOrder
			}
			return docs[i].CreatedAt.Before(docs[j].CreatedAt)
		})
		out = append(out, FolderDocs{
			Folder:     f,
			Permission: perm,
			Documents:  docs,
		})
	}
	return out, nil
}

// GetRoomDocuments returns room documents grouped by folder with metadata and the
// current member's effective permission for each folder.
func (s *Service) GetRoomDocuments(ctx context.Context, roomID, workspaceID, userID string) ([]FolderDocs, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	access, err := s.resolveRoomAccess(ctx, room.WorkspaceID, room.ID, userID)
	if err != nil {
		return nil, err
	}

	folders, err := s.loadFolders(room)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListDealRoomDocumentsWithMeta(ctx, room.ID)
	if err != nil {
		return nil, err
	}

	docsByFolder := make(map[string][]RoomDocument)
	for _, r := range rows {
		if IsArchivedDocumentStatus(r.Status) {
			continue
		}
		var pageCount int32
		if r.PageCount.Valid {
			pageCount = r.PageCount.Int32
		}
		var fileSize int64
		if r.FileSize.Valid {
			fileSize = r.FileSize.Int64
		}
		d := RoomDocument{
			ID:         uuid.UUID(r.ID.Bytes).String(),
			DocumentID: uuid.UUID(r.DocumentID.Bytes).String(),
			Title:      r.DocumentTitle,
			PageCount:  pageCount,
			FileSize:   fileSize,
			SourceType: r.SourceType,
			Status:     r.Status,
			FolderPath: r.FolderPath,
			SortOrder:  r.SortOrder,
			Locked:     r.Locked,
			CreatedAt:  r.CreatedAt.Time,
		}
		docsByFolder[r.FolderPath] = append(docsByFolder[r.FolderPath], d)
	}

	// Single batch query replaces N per-folder GetFolderPermission calls.
	var permMap map[string]string
	if access.manager {
		permMap = map[string]string{}
	} else {
		permMap, err = s.batchFolderPermissions(ctx, room.ID, access.memberEmail)
		if err != nil {
			return nil, err
		}
	}
	// Default all real folders to view when no explicit permission is set.
	for _, f := range folders {
		if _, ok := permMap[f.Path]; !ok {
			permMap[f.Path] = "view"
		}
	}

	out := make([]FolderDocs, 0, len(folders))
	for _, f := range folders {
		perm := permMap[f.Path]
		if perm == "none" {
			continue
		}
		docs := docsByFolder[f.Path]
		if docs == nil {
			docs = []RoomDocument{}
		}
		sort.Slice(docs, func(i, j int) bool {
			if docs[i].SortOrder != docs[j].SortOrder {
				return docs[i].SortOrder < docs[j].SortOrder
			}
			return docs[i].CreatedAt.Before(docs[j].CreatedAt)
		})
		out = append(out, FolderDocs{
			Folder:     f,
			Permission: perm,
			Documents:  docs,
		})
	}
	return out, nil
}

// ListFolders returns the folder structure stored in a room's settings.
// Callers that expose this over HTTP must use ListFoldersForMember instead.
func (s *Service) ListFolders(ctx context.Context, roomID, workspaceID string) ([]Folder, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.loadFolders(room)
}

// ListFoldersForMember returns folders only for an active room member.
func (s *Service) ListFoldersForMember(ctx context.Context, roomID, workspaceID, userID string) ([]Folder, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActiveRoomMember(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return nil, err
	}
	return s.loadFolders(room)
}

// CreateFolder adds a folder to a room. Only admins can create folders.
func (s *Service) CreateFolder(ctx context.Context, roomID, workspaceID, userID, name, parentPath string) ([]Folder, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireRoomAdmin(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("folder name is required")
	}
	if strings.Contains(name, "/") {
		return nil, errors.New("folder name cannot contain slashes")
	}
	if slug := slugify(name); slug == "" {
		return nil, errors.New("folder name must contain valid characters")
	}
	folders, err := s.loadFolders(room)
	if err != nil {
		return nil, err
	}
	if parentPath == "" {
		if len(folders) > 0 {
			parentPath = folders[0].Path
		} else {
			parentPath = generalFolder().Path
		}
	}
	parentPath = normalizeFolderPath(parentPath)
	// parentPath == "/" means creating a top-level folder; "/" itself is not a folder.
	if parentPath != "/" && !folderExists(folders, parentPath) {
		return nil, ErrFolderNotFound
	}
	if parentPath != "/" && folderIsLocked(folders, parentPath) {
		return nil, ErrResourceLocked
	}

	newPath := joinFolderPath(parentPath, slugify(name))
	if folderExists(folders, newPath) {
		return nil, ErrFolderExists
	}

	maxOrder := -1
	for _, f := range folders {
		if f.SortOrder > maxOrder {
			maxOrder = f.SortOrder
		}
	}
	folders = append(folders, Folder{
		Path:      newPath,
		Name:      name,
		SortOrder: maxOrder + 1,
	})

	if err := s.saveFolders(ctx, room, folders); err != nil {
		return nil, err
	}
	return folders, nil
}

// RenameFolder renames a folder and cascades the path to documents and permissions.
// Only admins can rename folders.
func (s *Service) RenameFolder(ctx context.Context, roomID, workspaceID, userID, oldPath, newName string) ([]Folder, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireRoomAdmin(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return nil, err
	}
	oldPath = normalizeFolderPath(oldPath)
	if oldPath == "/" {
		return nil, errors.New("folder not found")
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, errors.New("folder name is required")
	}
	if strings.Contains(newName, "/") {
		return nil, errors.New("folder name cannot contain slashes")
	}
	if slug := slugify(newName); slug == "" {
		return nil, errors.New("folder name must contain valid characters")
	}

	folders, err := s.loadFolders(room)
	if err != nil {
		return nil, err
	}
	idx := folderIndex(folders, oldPath)
	if idx < 0 {
		return nil, ErrFolderNotFound
	}
	if folders[idx].Locked {
		return nil, ErrResourceLocked
	}

	parentPath := parentFolder(oldPath)
	newPath := joinFolderPath(parentPath, slugify(newName))
	if newPath != oldPath && folderExists(folders, newPath) {
		return nil, ErrFolderExists
	}

	// Build a folder path mapping for cascade updates.
	pathMap := make(map[string]string)
	pathMap[oldPath] = newPath
	for i := range folders {
		if strings.HasPrefix(folders[i].Path, oldPath+"/") {
			suffix := strings.TrimPrefix(folders[i].Path, oldPath)
			pathMap[folders[i].Path] = newPath + suffix
		}
	}

	// Update folder structures in memory.
	for i := range folders {
		if folders[i].Path == oldPath {
			folders[i].Path = newPath
			folders[i].Name = newName
		} else if strings.HasPrefix(folders[i].Path, oldPath+"/") {
			folders[i].Path = pathMap[folders[i].Path]
			// Keep the last segment as the displayed name.
			folders[i].Name = folderName(folders[i].Path)
		}
	}

	if err := s.runInTx(ctx, func(q *db.Queries) error {
		// Cascade update documents and permissions for the renamed folder and its descendants.
		for oldP, newP := range pathMap {
			if err := q.UpdateDealRoomDocumentsFolderPath(ctx, db.UpdateDealRoomDocumentsFolderPathParams{
				FolderPath:   newP,
				RoomID:       room.ID,
				FolderPath_2: oldP,
			}); err != nil {
				return err
			}
			if err := q.UpdateRoomFolderPermissionsFolderPath(ctx, db.UpdateRoomFolderPermissionsFolderPathParams{
				FolderPath:   newP,
				RoomID:       room.ID,
				FolderPath_2: oldP,
			}); err != nil {
				return err
			}
		}

		// Cascade update deal-room link folder scopes.
		links, err := q.ListLinksByDealRoomID(ctx, room.ID)
		if err != nil {
			return err
		}
		for _, link := range links {
			newScopes := make([]string, len(link.FolderScopePaths))
			for i, p := range link.FolderScopePaths {
				if p == oldPath {
					newScopes[i] = newPath
				} else if strings.HasPrefix(p, oldPath+"/") {
					newScopes[i] = newPath + strings.TrimPrefix(p, oldPath)
				} else {
					newScopes[i] = p
				}
			}
			if err := q.UpdateLinkFolderScopePaths(ctx, db.UpdateLinkFolderScopePathsParams{
				FolderScopePaths: newScopes,
				ID:               link.ID,
				WorkspaceID:      link.WorkspaceID,
			}); err != nil {
				return err
			}
		}

		return s.saveFoldersWithQueries(ctx, q, room, folders)
	}); err != nil {
		return nil, err
	}
	return folders, nil
}

// SetResourceLocks locks or unlocks folders and documents in a room (admin only).
// Folder locks live in settings JSON; document locks live on deal_room_documents.locked.
func (s *Service) SetResourceLocks(
	ctx context.Context,
	roomID, workspaceID, userID string,
	req SetResourceLocksRequest,
	locked bool,
) error {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.requireRoomAdmin(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return err
	}
	if len(req.FolderPaths) == 0 && len(req.DocumentIDs) == 0 {
		return errors.New("folder_paths or document_ids required")
	}

	knowledgeDocIDs := make([]string, 0)
	seenKnowledgeDocs := map[string]bool{}
	enqueueKnowledgeDoc := func(docID string) {
		if !locked || docID == "" || seenKnowledgeDocs[docID] {
			return
		}
		seenKnowledgeDocs[docID] = true
		knowledgeDocIDs = append(knowledgeDocIDs, docID)
	}

	if len(req.FolderPaths) > 0 {
		folders, err := s.loadFolders(room)
		if err != nil {
			return err
		}
		normalizedFolders := make([]string, 0, len(req.FolderPaths))
		for _, raw := range req.FolderPaths {
			p := normalizeFolderPath(raw)
			idx := folderIndex(folders, p)
			if idx < 0 {
				return ErrFolderNotFound
			}
			folders[idx].Locked = locked
			normalizedFolders = append(normalizedFolders, p)
		}
		if err := s.saveFolders(ctx, room, folders); err != nil {
			return err
		}
		// Folder lock cascades to knowledge corpus for docs in that folder tree.
		roomDocs, err := s.queries.ListDealRoomDocumentsWithMeta(ctx, room.ID)
		if err != nil {
			return err
		}
		for _, d := range roomDocs {
			if !documentUnderLockedFolders(d.FolderPath, normalizedFolders) {
				continue
			}
			enqueueKnowledgeDoc(uuid.UUID(d.DocumentID.Bytes).String())
		}
	}

	if len(req.DocumentIDs) > 0 {
		ids := make([]pgtype.UUID, 0, len(req.DocumentIDs))
		for _, raw := range req.DocumentIDs {
			parsed, err := uuid.Parse(raw)
			if err != nil {
				return errors.New("invalid document id")
			}
			ids = append(ids, pgtype.UUID{Bytes: parsed, Valid: true})
			enqueueKnowledgeDoc(parsed.String())
		}
		if err := s.queries.SetDealRoomDocumentsLocked(ctx, db.SetDealRoomDocumentsLockedParams{
			Locked:      locked,
			RoomID:      room.ID,
			DocumentIds: ids,
		}); err != nil {
			return err
		}
	}

	// Knowledge corpus: lock purges withdrawn files when a corpus already exists.
	// Unlock does not ingest; the next Knowledge Sync is the re-index.
	if s.knowledgeEnqueuer != nil && len(knowledgeDocIDs) > 0 {
		roomIDStr := uuid.UUID(room.ID.Bytes).String()
		wsIDStr := uuid.UUID(room.WorkspaceID.Bytes).String()
		for _, docID := range knowledgeDocIDs {
			if kerr := s.knowledgeEnqueuer.EnqueueDeleteDocument(ctx, roomIDStr, wsIDStr, docID); kerr != nil {
				logger.ErrorCtx(ctx, "enqueue knowledge delete after resource lock", kerr,
					logger.Attr("document_id", docID),
				)
			}
		}
	}
	return nil
}

// documentUnderLockedFolders reports whether folderPath equals or is under any path.
func documentUnderLockedFolders(folderPath string, folderPaths []string) bool {
	p := normalizeFolderPath(folderPath)
	for _, raw := range folderPaths {
		locked := normalizeFolderPath(raw)
		if locked == "/" {
			continue
		}
		if p == locked || strings.HasPrefix(p, locked+"/") {
			return true
		}
	}
	return false
}

// DeleteFolder removes a folder from a room. Only admins can delete folders.
// Rejects deletion if the folder or its descendants contain documents.
func (s *Service) DeleteFolder(ctx context.Context, roomID, workspaceID, userID, path string) ([]Folder, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireRoomAdmin(ctx, room.WorkspaceID, room.ID, userID); err != nil {
		return nil, err
	}
	path = normalizeFolderPath(path)
	if path == "/" {
		return nil, errors.New("folder not found")
	}

	folders, err := s.loadFolders(room)
	if err != nil {
		return nil, err
	}
	if !folderExists(folders, path) {
		return nil, ErrFolderNotFound
	}
	if folderIsLocked(folders, path) {
		return nil, ErrResourceLocked
	}

	count, err := s.queries.CountDocumentsInFolder(ctx, db.CountDocumentsInFolderParams{
		RoomID:     room.ID,
		FolderPath: path,
	})
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrFolderNotEmpty
	}

	newFolders := make([]Folder, 0, len(folders))
	for _, f := range folders {
		if f.Path == path || strings.HasPrefix(f.Path, path+"/") {
			continue
		}
		newFolders = append(newFolders, f)
	}

	if err := s.runInTx(ctx, func(q *db.Queries) error {
		if err := q.DeleteRoomFolderPermissionsPrefix(ctx, db.DeleteRoomFolderPermissionsPrefixParams{
			RoomID:     room.ID,
			FolderPath: path,
		}); err != nil {
			return err
		}
		if err := s.saveFoldersWithQueries(ctx, q, room, newFolders); err != nil {
			return err
		}
		// Remove the deleted folder (and its descendants) from any deal-room link scopes.
		links, err := q.ListLinksByDealRoomID(ctx, room.ID)
		if err != nil {
			return err
		}
		for _, l := range links {
			if len(l.FolderScopePaths) == 0 {
				continue
			}
			filtered := make([]string, 0, len(l.FolderScopePaths))
			for _, p := range l.FolderScopePaths {
				if p == path || strings.HasPrefix(p, path+"/") {
					continue
				}
				filtered = append(filtered, p)
			}
			if len(filtered) != len(l.FolderScopePaths) {
				if err := q.UpdateLinkFolderScopePaths(ctx, db.UpdateLinkFolderScopePathsParams{
					FolderScopePaths: filtered,
					ID:               l.ID,
					WorkspaceID:      l.WorkspaceID,
				}); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return newFolders, nil
}

func (s *Service) loadFolders(room db.DealRoom) ([]Folder, error) {
	var loaded []Folder
	if len(room.Settings) > 0 && string(room.Settings) != "{}" {
		var settings struct {
			Folders []Folder `json:"folders"`
		}
		if err := json.Unmarshal(room.Settings, &settings); err != nil {
			return nil, fmt.Errorf("parse room settings: %w", err)
		}
		loaded = settings.Folders
	}
	// Remove legacy root folder and fall back to a general folder when none remain.
	filtered := make([]Folder, 0, len(loaded))
	for _, f := range loaded {
		if f.Path == "/" {
			continue
		}
		filtered = append(filtered, f)
	}
	if len(filtered) == 0 {
		return []Folder{generalFolder()}, nil
	}
	return filtered, nil
}

func (s *Service) saveFolders(ctx context.Context, room db.DealRoom, folders []Folder) error {
	return s.saveFoldersWithQueries(ctx, s.queries, room, folders)
}

func (s *Service) saveFoldersWithQueries(ctx context.Context, q *db.Queries, room db.DealRoom, folders []Folder) error {
	var settings map[string]interface{}
	if len(room.Settings) > 0 && string(room.Settings) != "{}" {
		if err := json.Unmarshal(room.Settings, &settings); err != nil {
			return fmt.Errorf("parse room settings: %w", err)
		}
	} else {
		settings = make(map[string]interface{})
	}
	settings["folders"] = folders
	settingsBytes, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal room settings: %w", err)
	}
	return q.UpdateDealRoomSettings(ctx, db.UpdateDealRoomSettingsParams{
		Column1:     settingsBytes,
		ID:          room.ID,
		WorkspaceID: room.WorkspaceID,
	})
}

func (s *Service) isWorkspaceManager(ctx context.Context, workspaceID pgtype.UUID, userID string) (bool, error) {
	wm, err := s.queries.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      pgUUID(userID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return wm.Role == workspaceRoleOwner || wm.Role == workspaceRoleAdmin, nil
}

// roomAccess is the caller's effective deal-room access after ACL resolution.
type roomAccess struct {
	manager     bool
	memberEmail string
}

// resolveRoomAccess allows an active room member, or workspace owner/admin
// oversight without a room row. Folder ACL uses memberEmail for non-managers.
func (s *Service) resolveRoomAccess(ctx context.Context, workspaceID, roomID pgtype.UUID, userID string) (roomAccess, error) {
	caps, err := roomacl.Resolve(ctx, s.queries, workspaceID, roomID, userID)
	if err != nil {
		return roomAccess{}, err
	}
	if !caps.View {
		return roomAccess{}, ErrApprovalRequired
	}
	return roomAccess{manager: caps.Manage || caps.Oversight, memberEmail: caps.MemberEmail}, nil
}

func (s *Service) requireRoomAdmin(ctx context.Context, workspaceID, roomID pgtype.UUID, userID string) error {
	_, err := roomacl.Require(ctx, s.queries, workspaceID, roomID, userID, roomacl.NeedManage)
	return mapACL(err)
}

func (s *Service) requireRoomContribute(ctx context.Context, workspaceID, roomID pgtype.UUID, userID string) error {
	_, err := roomacl.Require(ctx, s.queries, workspaceID, roomID, userID, roomacl.NeedContribute)
	return mapACL(err)
}

func (s *Service) requireRoomOwnerOrAdmin(ctx context.Context, workspaceID, roomID pgtype.UUID, userID string) error {
	_, err := roomacl.Require(ctx, s.queries, workspaceID, roomID, userID, roomacl.NeedDelete)
	return mapACL(err)
}

func mapACL(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, roomacl.ErrDenied) {
		return ErrApprovalRequired
	}
	if errors.Is(err, roomacl.ErrNotAdmin) {
		return ErrNotRoomAdmin
	}
	return err
}

func canManageRoomRole(actorRoomRole, target string) error {
	if target == roomacl.RoleOwner {
		return ErrCannotManageMember
	}
	if actorRoomRole == roomacl.RoleOwner {
		return nil
	}
	if actorRoomRole == roomacl.RoleAdmin && (target == roomacl.RoleMember || target == roomacl.RoleGuest) {
		return nil
	}
	return ErrCannotManageMember
}

// IsWorkspaceOversight reports whether the user is a workspace owner/admin.
func (s *Service) IsWorkspaceOversight(ctx context.Context, workspaceID, userID string) bool {
	ok, err := s.isWorkspaceManager(ctx, pgUUID(workspaceID), userID)
	return err == nil && ok
}

// PendingNdaRoomIDsForUser returns rooms where Resolve would set InvitedPending.
// Workspace owner/admin oversight is View, not an NDA wall — keep the list overlay
// aligned so aggregates and badges are not stripped for those callers.
func (s *Service) PendingNdaRoomIDsForUser(ctx context.Context, workspaceID, userID string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if strings.TrimSpace(userID) == "" {
		return out, nil
	}
	if s.IsWorkspaceOversight(ctx, workspaceID, userID) {
		return out, nil
	}
	rows, err := s.queries.ListRoomMembershipsForUserInWorkspace(ctx, db.ListRoomMembershipsForUserInWorkspaceParams{
		WorkspaceID: pgUUID(workspaceID),
		UserID:      pgUUID(userID),
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Status == "pending" && row.RoomID.Valid {
			out[uuid.UUID(row.RoomID.Bytes).String()] = struct{}{}
		}
	}
	return out, nil
}

func (s *Service) requireActiveRoomMember(ctx context.Context, workspaceID, roomID pgtype.UUID, userID string) error {
	_, err := s.resolveRoomAccess(ctx, workspaceID, roomID, userID)
	return err
}

func (s *Service) runInTx(ctx context.Context, fn func(*db.Queries) error) error {
	if s.pool == nil {
		return fn(s.queries)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) getTenantForWorkspace(ctx context.Context, workspaceID pgtype.UUID) (pgtype.UUID, error) {
	ws, err := s.queries.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, errors.New("workspace not found")
		}
		return pgtype.UUID{}, err
	}
	return ws.TenantID, nil
}

func defaultFolders() []Folder {
	return []Folder{}
}

const maxCreateFolders = 40

func normalizeCreateFolders(input []Folder) ([]Folder, error) {
	out := make([]Folder, 0, len(input))
	used := make(map[string]bool, len(input))
	for _, f := range input {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			continue
		}
		if strings.Contains(name, "/") {
			return nil, fmt.Errorf("%w: folder name cannot contain slashes", ErrInvalidFolder)
		}
		path := normalizeFolderPath(f.Path)
		if path == "/" {
			slug := slugify(name)
			if slug == "" {
				slug = fmt.Sprintf("folder-%d", len(out)+1)
			}
			path = "/" + slug
		}
		base := path
		n := 2
		for used[path] {
			path = fmt.Sprintf("%s-%d", base, n)
			n++
		}
		used[path] = true
		out = append(out, Folder{
			Path:        path,
			Name:        name,
			Description: strings.TrimSpace(f.Description),
			SortOrder:   len(out),
		})
		if len(out) >= maxCreateFolders {
			break
		}
	}
	return out, nil
}

func generalFolder() Folder {
	return Folder{Path: "/general", Name: "General", SortOrder: 0}
}

func folderExists(folders []Folder, path string) bool {
	return folderIndex(folders, path) >= 0
}

func folderIndex(folders []Folder, path string) int {
	for i, f := range folders {
		if f.Path == path {
			return i
		}
	}
	return -1
}

func folderIsLocked(folders []Folder, path string) bool {
	idx := folderIndex(folders, path)
	if idx < 0 {
		return false
	}
	return folders[idx].Locked
}

func normalizeFolderPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return "/"
	}
	return p
}

func parentFolder(path string) string {
	path = normalizeFolderPath(path)
	if path == "/" {
		return "/"
	}
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

func folderName(path string) string {
	path = normalizeFolderPath(path)
	if path == "/" {
		return ""
	}
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return path
	}
	return path[idx+1:]
}

func slugify(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	// Remove characters that are not lowercase alphanumeric or hyphen.
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

func joinFolderPath(parent, name string) string {
	parent = normalizeFolderPath(parent)
	if parent == "/" {
		return "/" + name
	}
	return parent + "/" + name
}

func normalizeRole(role string) string {
	return roomacl.GrantableRole(role)
}

func ndaStatusFor(required bool) string {
	if required {
		return "pending"
	}
	return "not_required"
}

func roomHasMemberNDA(room db.DealRoom) bool {
	return room.NdaTemplateID.Valid || room.NdaDocumentID.Valid
}

// SetMemberNDAAgreement binds the room-level NDA that invited members must sign.
func (s *Service) SetMemberNDAAgreement(ctx context.Context, roomID, workspaceID, adminUserID, templateID, documentID string) (db.DealRoom, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.DealRoom{}, err
	}
	if err := s.requireRoomAdmin(ctx, room.WorkspaceID, room.ID, adminUserID); err != nil {
		return db.DealRoom{}, err
	}
	tplID, docID, err := s.resolveRoomNDABinding(ctx, room, adminUserID, templateID, documentID)
	if err != nil {
		return db.DealRoom{}, err
	}
	if room.RequiresNda && !tplID.Valid && !docID.Valid {
		return db.DealRoom{}, ErrNDAAgreementRequired
	}
	updated, err := s.queries.UpdateDealRoomNDAAgreement(ctx, db.UpdateDealRoomNDAAgreementParams{
		NdaTemplateID: tplID,
		NdaDocumentID: docID,
		ID:            room.ID,
		WorkspaceID:   room.WorkspaceID,
	})
	if err != nil {
		return db.DealRoom{}, fmt.Errorf("update room nda agreement: %w", err)
	}
	s.invalidateListCache(ctx, workspaceID)
	return updated, nil
}

func (s *Service) resolveRoomNDABinding(ctx context.Context, room db.DealRoom, createdBy, templateID, documentID string) (pgtype.UUID, pgtype.UUID, error) {
	templateID = strings.TrimSpace(templateID)
	documentID = strings.TrimSpace(documentID)
	if templateID == "" && documentID == "" {
		return pgtype.UUID{}, pgtype.UUID{}, nil
	}
	if templateID != "" {
		id := pgUUID(templateID)
		if !id.Valid {
			return pgtype.UUID{}, pgtype.UUID{}, ErrNDAAgreementRequired
		}
		tpl, err := s.queries.GetNDATemplateByID(ctx, db.GetNDATemplateByIDParams{
			ID:          id,
			WorkspaceID: room.WorkspaceID,
		})
		if err == nil {
			if tpl.Status != "active" {
				return pgtype.UUID{}, pgtype.UUID{}, ErrNDAAgreementRequired
			}
			return tpl.ID, tpl.SourceDocumentID, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("get nda template: %w", err)
		}
		// Picker value is often the agreement-library document UUID.
		if documentID == "" {
			documentID = templateID
		}
	}
	return s.ensureRoomNDATemplateFromDocument(ctx, room, createdBy, documentID)
}

func (s *Service) ensureRoomNDATemplateFromDocument(ctx context.Context, room db.DealRoom, createdBy, documentID string) (pgtype.UUID, pgtype.UUID, error) {
	docID := pgUUID(documentID)
	if !docID.Valid {
		return pgtype.UUID{}, pgtype.UUID{}, ErrNDAAgreementRequired
	}
	tpl, err := s.queries.GetNDATemplateBySourceDocument(ctx, db.GetNDATemplateBySourceDocumentParams{
		WorkspaceID:      room.WorkspaceID,
		SourceDocumentID: docID,
	})
	if err == nil {
		if tpl.Status != "active" {
			return pgtype.UUID{}, pgtype.UUID{}, ErrNDAAgreementRequired
		}
		return tpl.ID, tpl.SourceDocumentID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("get nda template by document: %w", err)
	}

	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          docID,
		WorkspaceID: room.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, pgtype.UUID{}, ErrNDAAgreementRequired
		}
		return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("get nda document: %w", err)
	}
	if !strings.EqualFold(doc.Category, upload.CategoryAgreement) || !strings.EqualFold(doc.Status, "ready") {
		return pgtype.UUID{}, pgtype.UUID{}, ErrNDAAgreementRequired
	}

	name := strings.TrimSpace(doc.Title)
	if name == "" {
		name = "NDA Agreement"
	}
	created := pgUUID(createdBy)
	tpl, err = s.queries.CreateNDATemplate(ctx, db.CreateNDATemplateParams{
		TenantID:          room.TenantID,
		WorkspaceID:       room.WorkspaceID,
		Name:              name,
		SourceDocumentID:  docID,
		ContentSha256:     "",
		RequireSignerName: true,
		Status:            "active",
		CreatedBy:         created,
	})
	if err != nil {
		existing, gerr := s.queries.GetNDATemplateBySourceDocument(ctx, db.GetNDATemplateBySourceDocumentParams{
			WorkspaceID:      room.WorkspaceID,
			SourceDocumentID: docID,
		})
		if gerr == nil && existing.Status == "active" {
			return existing.ID, existing.SourceDocumentID, nil
		}
		return pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("create nda template: %w", err)
	}
	return tpl.ID, tpl.SourceDocumentID, nil
}

// ndaStatusForRole: room operators never owe a visitor NDA; external roles do
// when the room requires one.
func ndaStatusForRole(required bool, role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner", "admin":
		return "not_required"
	default:
		return ndaStatusFor(required)
	}
}

func memberStatusFor(requiresApprovalOrNDA bool) string {
	if requiresApprovalOrNDA {
		return "pending"
	}
	return "active"
}

func memberStatusForRole(requiresNDA bool, role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner", "admin":
		return "active"
	default:
		return memberStatusFor(requiresNDA)
	}
}

func pgUUID(id string) pgtype.UUID {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

func (s *Service) resolveRoomAccessRequest(workspaceID, roomID string) {
	if s.actionSyncer == nil {
		return
	}
	// Actions are keyed by room ID. Only clear when no pending membership
	// requests remain for this room.
	id, err := uuid.Parse(roomID)
	if err != nil {
		return
	}
	rows, err := s.queries.ListAccessRequestsByRoom(context.Background(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		return
	}
	for _, r := range rows {
		if r.Status == "pending" {
			return
		}
	}
	s.actionSyncer.ResolveBySource(context.Background(), workspaceID, action.SourceTypeRoomAccessRequest, roomID)
}

func (s *Service) resolveRoomNDA(ctx context.Context, workspaceID string, roomID pgtype.UUID, email string) {
	if s.actionSyncer == nil {
		return
	}
	if !roomID.Valid {
		return
	}
	email = strings.ToLower(strings.TrimSpace(email))
	// Member-keyed actions (source_id = room_members.id, target_id = room).
	if email != "" {
		if member, err := s.queries.GetRoomMemberByEmail(ctx, db.GetRoomMemberByEmailParams{
			RoomID: roomID,
			Email:  email,
		}); err == nil && member.ID.Valid {
			s.actionSyncer.ResolveBySource(ctx, workspaceID, action.SourceTypeRoomNDA, uuid.UUID(member.ID.Bytes).String())
		}
	}
	// Legacy room-keyed actions: clear only when no external pending NDA remains.
	members, err := s.queries.ListRoomMembers(ctx, roomID)
	if err != nil {
		return
	}
	for _, m := range members {
		if m.NdaStatus != "pending" {
			continue
		}
		if m.Role == "owner" || m.Role == "admin" {
			continue
		}
		if strings.TrimSpace(m.Email) == "" {
			continue
		}
		return
	}
	s.actionSyncer.ResolveBySource(ctx, workspaceID, action.SourceTypeRoomNDA, uuid.UUID(roomID.Bytes).String())
}
