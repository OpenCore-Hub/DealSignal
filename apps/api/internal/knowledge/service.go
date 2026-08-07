package knowledge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/docling"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrUnavailable  = errors.New("knowledge base unavailable")
	ErrForbidden    = errors.New("knowledge base forbidden")
	ErrNotFound     = errors.New("knowledge base not found")
	ErrInvalidInput = errors.New("invalid input")
	// ErrAnswerRequiresSession is returned when legacy /knowledge/query asks for Answer=true.
	// Metered answers must go through the session query path so turns are audited.
	ErrAnswerRequiresSession = errors.New("answer requires session query")
)

// ObjectStore reads/writes document and diligence-archive bytes in object storage.
type ObjectStore interface {
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
}

// RoomAccess checks deal-room membership for knowledge endpoints.
type RoomAccess interface {
	GetRoom(ctx context.Context, roomID, workspaceID string) (db.DealRoom, error)
	RequireActiveRoomMember(ctx context.Context, roomID, workspaceID, userID string) error
}

type roomAccessAdapter struct {
	queries *db.Queries
}

func (a roomAccessAdapter) GetRoom(ctx context.Context, roomID, workspaceID string) (db.DealRoom, error) {
	room, err := a.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
		ID:          pgUUID(roomID),
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.DealRoom{}, ErrNotFound
		}
		return db.DealRoom{}, err
	}
	return room, nil
}

func (a roomAccessAdapter) RequireActiveRoomMember(ctx context.Context, roomID, workspaceID, userID string) error {
	room, err := a.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	member, err := a.queries.GetRoomMemberByUserID(ctx, db.GetRoomMemberByUserIDParams{
		RoomID: room.ID,
		UserID: pgUUID(userID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return err
	}
	if member.Status != "active" {
		return ErrForbidden
	}
	return nil
}

// Beginner starts a database transaction (pgx pool).
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Service orchestrates local mapping + docling-rag sync/query.
type Service struct {
	queries          *db.Queries
	pool             Beginner
	cfg              config.DoclingRAGConfig
	client           *docling.Client
	store            ObjectStore
	preview          PreviewPDFConverter
	access           RoomAccess
	secretKey        string // used to seal tenant API keys at rest
	followUpLLM      followUpChatCompleter
	rewriteEnabled   bool // conversational retrieve rewrite (independent of follow-up chips)
	rewriteCache     rewriteCache
	retentionDays    int  // hot retention window surfaced on ops board
	tableLaneEnabled bool // merge local table_row chunks into Query (Phase I2)
	multiHopEnabled  bool // deterministic clause→definition→attachment hop (Phase I3)
}

// NewService constructs a knowledge service. client may be nil/disabled.
// secretKey should be a long-lived server secret (e.g. URL_SIGNING_SECRET).
func NewService(queries *db.Queries, cfg config.DoclingRAGConfig, client *docling.Client, store ObjectStore, secretKey string) *Service {
	return &Service{
		queries:          queries,
		cfg:              cfg,
		client:           client,
		store:            store,
		access:           roomAccessAdapter{queries: queries},
		secretKey:        secretKey,
		rewriteEnabled:   true,
		tableLaneEnabled: true,
		multiHopEnabled:  true,
	}
}

// WithQueryRewrite enables/disables elliptical retrieve-query rewrite.
// Follow-up chip LLM is unaffected. Default is enabled.
func (s *Service) WithQueryRewrite(enabled bool) *Service {
	if s != nil {
		s.rewriteEnabled = enabled
	}
	return s
}

// WithRewriteCache attaches a provenanced rewrite cache (memory or Redis).
// Cache hits are always re-validated with rewriteIsGrounded (ceiling Phase P).
func (s *Service) WithRewriteCache(cache rewriteCache) *Service {
	if s != nil {
		s.rewriteCache = cache
	}
	return s
}

// WithTableLane enables/disables the local table_row retrieve lane (Phase I2).
func (s *Service) WithTableLane(enabled bool) *Service {
	if s != nil {
		s.tableLaneEnabled = enabled
	}
	return s
}

// WithMultiHop enables/disables deterministic second-hop retrieve (Phase I3).
// Probe Query leaves MultiHop=false on the request; session path opts in.
func (s *Service) WithMultiHop(enabled bool) *Service {
	if s != nil {
		s.multiHopEnabled = enabled
	}
	return s
}

// WithDBPool enables transactional session/turn writes.
func (s *Service) WithDBPool(pool Beginner) *Service {
	if s != nil {
		s.pool = pool
	}
	return s
}

// WithPreviewPDFConverter sets the OnlyOffice converter used for Word/PowerPoint
// knowledge ingest (preview-page locus). Required for DOCX/PPTX sync.
func (s *Service) WithPreviewPDFConverter(c PreviewPDFConverter) *Service {
	if s != nil {
		s.preview = c
	}
	return s
}

// Enabled reports whether docling-rag integration is configured.
func (s *Service) Enabled() bool {
	return s != nil && s.client != nil && s.client.Enabled()
}

// doclingTimeout returns upstream RAG HTTP timeout for long-running write budgets.
func (s *Service) doclingTimeout() time.Duration {
	if s == nil {
		return 0
	}
	return s.cfg.HTTPTimeout
}

// QuotaPair is used/limit for one entitlement dimension.
type QuotaPair struct {
	Used  int `json:"used"`
	Limit int `json:"limit"`
}

// CorpusQuota is the plan entitlement snapshot for the vector library card.
type CorpusQuota struct {
	PlanCode       string    `json:"planCode,omitempty"`
	KnowledgeBases QuotaPair `json:"knowledgeBases"`
	Documents      QuotaPair `json:"documents"`
	Answers        QuotaPair `json:"answers"`
}

// CorpusStatus is the owner-facing knowledge snapshot.
type CorpusStatus struct {
	Enabled      bool               `json:"enabled"`
	Status       string             `json:"status"`
	LastSyncedAt *time.Time         `json:"lastSyncedAt,omitempty"`
	ErrorMessage string             `json:"errorMessage,omitempty"`
	Progress     SyncProgress       `json:"progress"`
	Documents    []DocumentSyncItem `json:"documents"`
	Quota        *CorpusQuota       `json:"quota,omitempty"`
}

// SyncProgress summarizes room corpus sync for the UI.
type SyncProgress struct {
	Total     int    `json:"total"`
	Pending   int    `json:"pending"`
	Syncing   int    `json:"syncing"`
	Synced    int    `json:"synced"`
	Failed    int    `json:"failed"`
	JobStatus string `json:"jobStatus,omitempty"` // pending|running|failed|done
}

// DocumentSyncItem is one room document's sync state.
type DocumentSyncItem struct {
	DocumentID string `json:"documentId"`
	Title      string `json:"title,omitempty"`
	Status     string `json:"status"`
	ChunkCount int32  `json:"chunkCount"`
	LastError  string `json:"lastError,omitempty"`
}

// QueryRequest is the BFF search body.
type QueryRequest struct {
	Query  string `json:"query"`
	Answer bool   `json:"answer"`
	TopK   int    `json:"top_k"`
	// SessionState / MultiHop are set only by queryWithSession (not HTTP probe JSON).
	SessionState SessionState `json:"-"`
	MultiHop     bool         `json:"-"`
}

// QueryResponse is sanitized search output for the UI.
type QueryResponse struct {
	Query   string     `json:"query"`
	Mode    string     `json:"mode"`
	Answer  string     `json:"answer,omitempty"`
	Results []QueryHit `json:"results"`
	// MultiHop is session-internal audit; persisted on bound_answer, not probe JSON.
	MultiHop *MultiHopAudit `json:"-"`
}

// QueryHit is one citation-friendly hit.
type QueryHit struct {
	ChunkID    string  `json:"chunkId"`
	DocumentID string  `json:"documentId,omitempty"`
	Text       string  `json:"text"`
	Score      float64 `json:"score"`
	SourceName string  `json:"sourceName,omitempty"`
	Pages      []int   `json:"pages,omitempty"`
	Sheet      string  `json:"sheet,omitempty"`
	ViewerPage *int    `json:"viewerPage,omitempty"`
}

// GetCorpus returns local sync state for a room.
func (s *Service) GetCorpus(ctx context.Context, roomID, workspaceID, userID string) (CorpusStatus, error) {
	if !s.Enabled() {
		return CorpusStatus{Enabled: false, Status: "none", Documents: []DocumentSyncItem{}}, nil
	}
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return CorpusStatus{}, err
	}
	out, err := s.loadRoomCorpusSnapshot(ctx, pgUUID(workspaceID), pgUUID(roomID), true)
	if err != nil {
		return CorpusStatus{}, err
	}
	room, roomErr := s.access.GetRoom(ctx, roomID, workspaceID)
	if roomErr != nil {
		return CorpusStatus{}, roomErr
	}
	out.Quota = s.loadCorpusQuota(ctx, room.WorkspaceID, out.Progress)
	return out, nil
}

// loadCorpusQuota best-effort fills plan limits + usage for the vector library card.
func (s *Service) loadCorpusQuota(ctx context.Context, workspaceID pgtype.UUID, progress SyncProgress) *CorpusQuota {
	def := docling.DefaultPartnerEntitlements()
	answers := s.answersQuotaSnapshot(ctx, workspaceID)
	q := &CorpusQuota{
		PlanCode:       def.PlanCode,
		KnowledgeBases: QuotaPair{Limit: int(def.MaxKBs)},
		Documents:      QuotaPair{Used: progress.Synced, Limit: int(def.MaxDocs)},
		Answers:        QuotaPair{Used: answers.Used, Limit: answers.Limit},
	}
	tenant, err := s.queries.GetWorkspaceRagTenant(ctx, workspaceID)
	if err != nil {
		return q
	}
	tenantSlug := tenant.ExternalTenantSlug
	if s.cfg.PlatformAdminKey != "" && s.client != nil {
		if ent, eerr := s.client.GetEntitlements(ctx, tenantSlug); eerr == nil {
			q.PlanCode = ent.PlanCode
			q.KnowledgeBases.Limit = int(ent.Entitlements.MaxKBs)
			q.Documents.Limit = int(ent.Entitlements.MaxDocs)
			// Answers limit/used already resolved in answersQuotaSnapshot (same entitlements).
			q.Answers.Limit = answers.Limit
			q.Answers.Used = answers.Used
		}
	}
	apiKey, oerr := openSecret(s.secretKey, tenant.TenantApiKey)
	if oerr != nil || apiKey == "" {
		return q
	}
	if kbs, lerr := s.client.ListKnowledgeBases(ctx, tenantSlug, apiKey); lerr == nil {
		q.KnowledgeBases.Used = len(kbs)
	}
	return q
}

// reconcileCorpusStatus derives a truthful badge when the corpus row lags document rows.
func reconcileCorpusStatus(status string, p SyncProgress) string {
	if status != "provisioning" && status != "syncing" {
		return status
	}
	if p.Total <= 0 {
		return status
	}
	if p.Pending > 0 || p.Syncing > 0 {
		return status
	}
	if p.Failed > 0 {
		return "degraded"
	}
	if p.Synced == p.Total {
		return "ready"
	}
	return status
}

// EnqueueRoomSync queues a full room sync job and marks pending docs.
// Tenant/KB provisioning happens in the worker so the BFF returns quickly
// and does not fail the click when docling-rag is temporarily unreachable.
func (s *Service) EnqueueRoomSync(ctx context.Context, roomID, workspaceID, userID string) error {
	if !s.Enabled() {
		return ErrUnavailable
	}
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return err
	}
	room, err := s.access.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.ensureLocalCorpusRow(ctx, room); err != nil {
		return err
	}
	if err := s.alignRoomDocuments(ctx, room); err != nil {
		return err
	}
	_, err = s.queries.UpdateDealRoomRagCorpusStatus(ctx, db.UpdateDealRoomRagCorpusStatusParams{
		RoomID:       room.ID,
		Status:       "syncing",
		ErrorMessage: pgtype.Text{},
	})
	if err != nil {
		return err
	}
	_, err = s.queries.EnqueueKnowledgeSyncJob(ctx, db.EnqueueKnowledgeSyncJobParams{
		WorkspaceID: room.WorkspaceID,
		RoomID:      room.ID,
		DocumentID:  pgtype.UUID{},
		JobType:     "sync_room",
	})
	return err
}

// EnqueueIngestDocument queues ingest for one room document (lifecycle hook).
// Locked room documents are never enqueued for ingest.
func (s *Service) EnqueueIngestDocument(ctx context.Context, roomID, workspaceID, documentID string) error {
	if !s.Enabled() {
		return nil
	}
	room, err := s.access.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	docUUID := pgUUID(documentID)
	binding, err := s.queries.GetDealRoomDocumentByDocumentID(ctx, db.GetDealRoomDocumentByDocumentIDParams{
		RoomID:     room.ID,
		DocumentID: docUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	lockedFolders := lockedFolderPathSet(room.Settings)
	if knowledgeExcluded(binding.Locked, binding.FolderPath, lockedFolders) {
		return nil
	}
	if err := s.ensureLocalCorpusRow(ctx, room); err != nil {
		logger.ErrorCtx(ctx, "knowledge ensure local corpus", err)
		return nil
	}
	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          docUUID,
		WorkspaceID: room.WorkspaceID,
	})
	if err != nil {
		return err
	}
	extName := externalDocName(documentID, doc.SourceType, doc.Title, doc.StorageKey)
	_, err = s.queries.UpsertDealRoomRagDocument(ctx, db.UpsertDealRoomRagDocumentParams{
		RoomID:       room.ID,
		DocumentID:   doc.ID,
		WorkspaceID:  room.WorkspaceID,
		ExternalName: extName,
		Status:       "pending",
		LastError:    pgtype.Text{},
	})
	if err != nil {
		return err
	}
	_, err = s.queries.EnqueueKnowledgeSyncJob(ctx, db.EnqueueKnowledgeSyncJobParams{
		WorkspaceID: room.WorkspaceID,
		RoomID:      room.ID,
		DocumentID:  doc.ID,
		JobType:     "ingest_doc",
	})
	return err
}

// EnqueueDeleteDocument queues deletion from the external KB.
// Always creates/updates a local binding so the worker can resolve the remote
// document by stable external_name even if ingest mapping was missing.
func (s *Service) EnqueueDeleteDocument(ctx context.Context, roomID, workspaceID, documentID string) error {
	if !s.Enabled() {
		return nil
	}
	room, err := s.access.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.ensureLocalCorpusRow(ctx, room); err != nil {
		logger.ErrorCtx(ctx, "knowledge ensure local corpus for delete", err)
		return err
	}
	docUUID := pgUUID(documentID)
	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          docUUID,
		WorkspaceID: room.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	extName := externalDocName(documentID, doc.SourceType, doc.Title, doc.StorageKey)
	_, err = s.queries.UpsertDealRoomRagDocument(ctx, db.UpsertDealRoomRagDocumentParams{
		RoomID:       room.ID,
		DocumentID:   doc.ID,
		WorkspaceID:  room.WorkspaceID,
		ExternalName: extName,
		Status:       "deleted",
		LastError:    pgtype.Text{},
	})
	if err != nil {
		return err
	}
	// Prefer delete over any still-pending ingest for the same document.
	_ = s.queries.CancelPendingKnowledgeIngestJobs(ctx, db.CancelPendingKnowledgeIngestJobsParams{
		RoomID:     room.ID,
		DocumentID: doc.ID,
	})
	_, err = s.queries.EnqueueKnowledgeSyncJob(ctx, db.EnqueueKnowledgeSyncJobParams{
		WorkspaceID: room.WorkspaceID,
		RoomID:      room.ID,
		DocumentID:  doc.ID,
		JobType:     "delete_doc",
	})
	return err
}

// Query proxies search/answer to docling-rag.
func (s *Service) Query(ctx context.Context, roomID, workspaceID, userID string, req QueryRequest) (QueryResponse, error) {
	if !s.Enabled() {
		return QueryResponse{}, ErrUnavailable
	}
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return QueryResponse{}, err
	}
	q := strings.TrimSpace(req.Query)
	if q == "" {
		return QueryResponse{}, fmt.Errorf("query is required")
	}
	room, err := s.access.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return QueryResponse{}, err
	}
	cred, err := s.ensureProvisioned(ctx, room)
	if err != nil {
		return QueryResponse{}, err
	}
	topK := req.TopK
	if topK <= 0 {
		topK = s.cfg.DefaultTopK
	}
	mode := s.cfg.DefaultMode
	res, err := s.client.Search(ctx, cred.tenantSlug, cred.kbSlug, cred.apiKey, docling.SearchRequest{
		Query:  q,
		Mode:   mode,
		TopK:   topK,
		Answer: req.Answer,
	})
	if err != nil {
		var apiErr *docling.APIError
		if errors.As(err, &apiErr) && (apiErr.Code == "INDEX_NOT_READY" || apiErr.Status == http.StatusServiceUnavailable) {
			_, _ = s.queries.UpdateDealRoomRagCorpusStatus(ctx, db.UpdateDealRoomRagCorpusStatusParams{
				RoomID:       room.ID,
				Status:       "syncing",
				ErrorMessage: pgtype.Text{},
			})
		}
		return QueryResponse{}, mapUpstream(err)
	}

	lockedIDs, err := s.lockedDocumentIDs(ctx, room)
	if err != nil {
		return QueryResponse{}, err
	}
	bindings, err := s.queries.ListDealRoomRagDocuments(ctx, room.ID)
	if err != nil {
		return QueryResponse{}, err
	}
	byExtID := map[string]string{}
	byName := map[string]string{}
	for _, b := range bindings {
		docID := uuid.UUID(b.DocumentID.Bytes).String()
		byName[b.ExternalName] = docID
		if b.ExternalDocumentID.Valid {
			byExtID[b.ExternalDocumentID.String] = docID
		}
	}
	out := applyLockedSearchFilter(res, byExtID, byName, lockedIDs)

	// Phase I2: local table_row lane (numeric/table intent) merged before locus enrich.
	if tableHits, terr := s.retrieveTableLaneHits(ctx, room, lockedIDs, q, topK); terr != nil {
		logger.InfoCtx(ctx, "knowledge table lane unavailable; continuing with hybrid only",
			logger.Attr("error", terr.Error()),
		)
	} else if n := applyTableLane(&out, tableHits, topK); n > 0 {
		recordKnowledgeQATableLaneHits(n)
	}

	// Phase I3: deterministic multi-hop (session path only; Answer:false on hop Search).
	if req.MultiHop {
		if audit := s.runMultiHop(ctx, cred, byExtID, byName, lockedIDs, req.SessionState, &out, topK, mode); audit != nil {
			out.MultiHop = audit
		}
	}

	s.enrichViewerPages(ctx, &out)
	s.enrichSourceDisplayNames(ctx, room.ID, &out)
	return out, nil
}

// enrichViewerPages fills ViewerPage from locus.pages or sheet→page map.
// Sheet-map load failures degrade (leave viewerPage unset) so search still returns.
func (s *Service) enrichViewerPages(ctx context.Context, out *QueryResponse) {
	sheetStart := s.loadSheetPageStarts(ctx, out.Results)
	applyViewerPages(out.Results, sheetStart)
}

// enrichSourceDisplayNames replaces opaque ingest names (UUID.ext) with the
// room document title. Failures degrade — hits still return with stamped names.
func (s *Service) enrichSourceDisplayNames(ctx context.Context, roomID pgtype.UUID, out *QueryResponse) {
	if len(out.Results) == 0 {
		return
	}
	need := false
	for _, hit := range out.Results {
		if hit.DocumentID != "" {
			need = true
			break
		}
	}
	if !need {
		return
	}
	rows, err := s.queries.ListDealRoomDocumentsWithMeta(ctx, roomID)
	if err != nil {
		logger.InfoCtx(ctx, "document titles unavailable; citation source names degraded",
			logger.Attr("error", err.Error()),
		)
		return
	}
	titleByDoc := make(map[string]string, len(rows))
	for _, d := range rows {
		docID := uuid.UUID(d.DocumentID.Bytes).String()
		if title := strings.TrimSpace(d.DocumentTitle); title != "" {
			titleByDoc[docID] = title
		}
	}
	applyDisplaySourceNames(out.Results, titleByDoc)
}

// applyDisplaySourceNames prefers DealSignal document titles over RAG upload
// identities (externalDocName = "{uuid}.{ext}"). Does not invent titles.
func applyDisplaySourceNames(hits []QueryHit, titleByDoc map[string]string) {
	for i := range hits {
		title := strings.TrimSpace(titleByDoc[hits[i].DocumentID])
		if title == "" {
			continue
		}
		hits[i].SourceName = title
	}
}

func (s *Service) loadSheetPageStarts(ctx context.Context, hits []QueryHit) map[string]map[string]int {
	need := map[string]bool{}
	for _, hit := range hits {
		if hit.DocumentID != "" && len(hit.Pages) == 0 && hit.Sheet != "" {
			need[hit.DocumentID] = true
		}
	}
	docIDs := make([]pgtype.UUID, 0, len(need))
	for id := range need {
		docIDs = append(docIDs, pgUUID(id))
	}
	out := map[string]map[string]int{}
	if len(docIDs) == 0 {
		return out
	}
	rows, err := s.queries.ListSheetPageRangesByDocuments(ctx, docIDs)
	if err != nil {
		logger.InfoCtx(ctx, "sheet page ranges unavailable; citation jumps degraded",
			logger.Attr("error", err.Error()),
		)
		return out
	}
	for _, row := range rows {
		docID := uuid.UUID(row.DocumentID.Bytes).String()
		if out[docID] == nil {
			out[docID] = map[string]int{}
		}
		out[docID][row.SheetName] = int(row.PageStart)
	}
	return out
}

// applyViewerPages sets ViewerPage from pages (min) or sheet map page_start.
// Missing map entries leave ViewerPage nil — never invent a page.
func applyViewerPages(hits []QueryHit, sheetStart map[string]map[string]int) {
	for i := range hits {
		hit := &hits[i]
		hit.ViewerPage = nil
		if len(hit.Pages) > 0 {
			min := hit.Pages[0]
			for _, p := range hit.Pages[1:] {
				if p > 0 && p < min {
					min = p
				}
			}
			if min > 0 {
				hit.ViewerPage = &min
			}
			continue
		}
		if hit.Sheet == "" || hit.DocumentID == "" {
			continue
		}
		if start, ok := sheetStart[hit.DocumentID][hit.Sheet]; ok && start > 0 {
			hit.ViewerPage = &start
		}
	}
}

type ragCredentials struct {
	tenantSlug string
	kbSlug     string
	apiKey     string
}

// ensureLocalCorpusRow creates a placeholder corpus mapping without calling docling-rag.
func (s *Service) ensureLocalCorpusRow(ctx context.Context, room db.DealRoom) error {
	_, err := s.queries.GetDealRoomRagCorpus(ctx, room.ID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	wsID := uuid.UUID(room.WorkspaceID.Bytes).String()
	roomID := uuid.UUID(room.ID.Bytes).String()
	_, err = s.queries.UpsertDealRoomRagCorpus(ctx, db.UpsertDealRoomRagCorpusParams{
		RoomID:             room.ID,
		WorkspaceID:        room.WorkspaceID,
		ExternalTenantSlug: "pending-ds-ws-" + strings.ReplaceAll(wsID, "-", ""),
		ExternalKbSlug:     roomID,
		Status:             "provisioning",
		ErrorMessage:       pgtype.Text{},
	})
	return err
}

// verifyOrReissueTenantKey ensures the stored tenant key can call the control/data API.
// On 401/403 it mints a new key with the platform admin credential and persists it sealed.
func (s *Service) verifyOrReissueTenantKey(ctx context.Context, workspaceID pgtype.UUID, tenantSlug string, apiKey *string) error {
	if apiKey == nil || *apiKey == "" {
		return ErrUnavailable
	}
	if _, err := s.client.ListKnowledgeBases(ctx, tenantSlug, *apiKey); err == nil {
		return nil
	} else {
		var apiErr *docling.APIError
		if !errors.As(err, &apiErr) || (apiErr.Status != http.StatusUnauthorized && apiErr.Status != http.StatusForbidden) {
			return mapUpstream(err)
		}
		logger.ErrorCtx(ctx, "knowledge tenant key rejected; reissuing", err,
			logger.Attr("tenant", tenantSlug),
			logger.Attr("upstream_status", apiErr.Status),
			logger.Attr("upstream_code", apiErr.Code),
		)
	}
	if s.cfg.PlatformAdminKey == "" {
		return ErrUnavailable
	}
	issued, err := s.client.CreateAPIKey(ctx, tenantSlug, "dealsignal-"+uuid.NewString(), []docling.APIKeyGrant{{
		KB:      "*",
		Actions: []string{"read", "ingest", "answer", "admin"},
	}})
	if err != nil || issued.Key == "" {
		if err == nil {
			err = errors.New("docling-rag did not return api key on reissue")
		}
		return mapUpstream(err)
	}
	*apiKey = issued.Key
	sealed, serr := sealSecret(s.secretKey, *apiKey)
	if serr != nil {
		return fmt.Errorf("seal tenant api key: %w", serr)
	}
	_, err = s.queries.UpsertWorkspaceRagTenant(ctx, db.UpsertWorkspaceRagTenantParams{
		WorkspaceID:        workspaceID,
		ExternalTenantSlug: tenantSlug,
		TenantApiKey:       sealed,
	})
	return err
}

func (s *Service) ensureProvisioned(ctx context.Context, room db.DealRoom) (ragCredentials, error) {
	wsID := uuid.UUID(room.WorkspaceID.Bytes).String()
	roomID := uuid.UUID(room.ID.Bytes).String()

	tenantRow, err := s.queries.GetWorkspaceRagTenant(ctx, room.WorkspaceID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ragCredentials{}, err
	}
	apiKey := ""
	tenantSlug := ""
	if err == nil {
		opened, oerr := openSecret(s.secretKey, tenantRow.TenantApiKey)
		if oerr != nil {
			return ragCredentials{}, fmt.Errorf("decrypt tenant api key: %w", oerr)
		}
		apiKey = opened
		tenantSlug = tenantRow.ExternalTenantSlug
	}
	if apiKey == "" {
		if s.cfg.PlatformAdminKey == "" {
			return ragCredentials{}, ErrUnavailable
		}
		ws, werr := s.queries.GetWorkspaceByID(ctx, room.WorkspaceID)
		if werr != nil {
			return ragCredentials{}, werr
		}
		slug := "ds-ws-" + strings.ReplaceAll(wsID, "-", "")
		created, cerr := s.client.CreateTenant(ctx, docling.CreateTenantRequest{
			Name:        ws.Name,
			Slug:        slug,
			ExternalRef: "dealsignal-ws-" + wsID,
			IssueAPIKey: true,
		})
		if cerr != nil {
			var apiErr *docling.APIError
			if errors.As(cerr, &apiErr) && apiErr.Status == 409 {
				tenantSlug = slug
				issued, kerr := s.client.CreateAPIKey(ctx, tenantSlug, "dealsignal-"+wsID, []docling.APIKeyGrant{{
					KB:      "*",
					Actions: []string{"read", "ingest", "answer", "admin"},
				}})
				if kerr != nil || issued.Key == "" {
					return ragCredentials{}, mapUpstream(cerr)
				}
				apiKey = issued.Key
			} else {
				return ragCredentials{}, mapUpstream(cerr)
			}
		} else {
			tenantSlug = created.TenantSlug
			if tenantSlug == "" {
				tenantSlug = slug
			}
			if created.APIKey == nil || created.APIKey.Key == "" {
				return ragCredentials{}, fmt.Errorf("docling-rag did not issue api key")
			}
			apiKey = created.APIKey.Key
		}
		sealed, serr := sealSecret(s.secretKey, apiKey)
		if serr != nil {
			return ragCredentials{}, fmt.Errorf("seal tenant api key: %w", serr)
		}
		_, err = s.queries.UpsertWorkspaceRagTenant(ctx, db.UpsertWorkspaceRagTenantParams{
			WorkspaceID:        room.WorkspaceID,
			ExternalTenantSlug: tenantSlug,
			TenantApiKey:       sealed,
		})
		if err != nil {
			return ragCredentials{}, err
		}
	}

	if err := s.verifyOrReissueTenantKey(ctx, room.WorkspaceID, tenantSlug, &apiKey); err != nil {
		return ragCredentials{}, err
	}

	// CreateTenant always creates a "default" KB under trial max_kbs=1; raise
	// quotas before creating the per-room KB (1 workspace → 1 tenant, 1 room → 1 KB).
	if bumpErr := s.client.EnsureMinEntitlements(ctx, tenantSlug, docling.DefaultPartnerEntitlements()); bumpErr != nil {
		logger.ErrorCtx(ctx, "knowledge ensure entitlements failed", bumpErr,
			logger.Attr("tenant", tenantSlug),
		)
		return ragCredentials{}, mapUpstream(bumpErr)
	}

	kbSlug := roomID
	_, err = s.client.EnsureKnowledgeBase(ctx, tenantSlug, apiKey, kbSlug, room.Name)
	if err != nil {
		logger.ErrorCtx(ctx, "knowledge ensure KB failed", err,
			logger.Attr("tenant", tenantSlug),
			logger.Attr("kb", kbSlug),
		)
		return ragCredentials{}, mapUpstream(err)
	}
	_, err = s.queries.UpsertDealRoomRagCorpus(ctx, db.UpsertDealRoomRagCorpusParams{
		RoomID:             room.ID,
		WorkspaceID:        room.WorkspaceID,
		ExternalTenantSlug: tenantSlug,
		ExternalKbSlug:     kbSlug,
		Status:             "provisioning",
		ErrorMessage:       pgtype.Text{},
	})
	if err != nil {
		return ragCredentials{}, err
	}
	return ragCredentials{tenantSlug: tenantSlug, kbSlug: kbSlug, apiKey: apiKey}, nil
}

func (s *Service) alignRoomDocuments(ctx context.Context, room db.DealRoom) error {
	roomDocs, err := s.queries.ListDealRoomDocumentsWithMeta(ctx, room.ID)
	if err != nil {
		return err
	}
	lockedFolders := lockedFolderPathSet(room.Settings)
	// Only searchable documents remain active. Locked document/folder bindings
	// fall out of active and are marked deleted for remote purge.
	active := make([]pgtype.UUID, 0, len(roomDocs))
	for _, d := range roomDocs {
		if knowledgeExcluded(d.Locked, d.FolderPath, lockedFolders) {
			continue
		}
		active = append(active, d.DocumentID)
		docID := uuid.UUID(d.DocumentID.Bytes).String()
		doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
			ID:          d.DocumentID,
			WorkspaceID: room.WorkspaceID,
		})
		if err != nil {
			return err
		}
		_, err = s.queries.UpsertDealRoomRagDocument(ctx, db.UpsertDealRoomRagDocumentParams{
			RoomID:       room.ID,
			DocumentID:   d.DocumentID,
			WorkspaceID:  room.WorkspaceID,
			ExternalName: externalDocName(docID, doc.SourceType, doc.Title, doc.StorageKey),
			Status:       "pending",
			LastError:    pgtype.Text{},
		})
		if err != nil {
			return err
		}
	}
	return s.queries.MarkMissingRagDocumentsDeleted(ctx, db.MarkMissingRagDocumentsDeletedParams{
		RoomID:            room.ID,
		ActiveDocumentIds: active,
	})
}

func (s *Service) lockedDocumentIDs(ctx context.Context, room db.DealRoom) (map[string]bool, error) {
	rows, err := s.queries.ListDealRoomDocuments(ctx, room.ID)
	if err != nil {
		return nil, err
	}
	lockedFolders := lockedFolderPathSet(room.Settings)
	out := make(map[string]bool, len(rows))
	for _, row := range rows {
		if knowledgeExcluded(row.Locked, row.FolderPath, lockedFolders) {
			out[uuid.UUID(row.DocumentID.Bytes).String()] = true
		}
	}
	return out, nil
}

// applyLockedSearchFilter drops hits from locked documents and discards grounded
// answers that may have been produced from those passages.
func applyLockedSearchFilter(
	res docling.SearchResponse,
	byExtID, byName map[string]string,
	lockedIDs map[string]bool,
) QueryResponse {
	out := QueryResponse{
		Query:   res.Query,
		Mode:    res.Mode,
		Results: make([]QueryHit, 0, len(res.Results)),
	}
	sawLockedHit := false
	sawUnmappedWhileLocks := false
	for _, hit := range res.Results {
		localDoc := byExtID[hit.Chunk.DocID]
		if localDoc == "" {
			if name, _ := hit.Chunk.Metadata["name"].(string); name != "" {
				localDoc = byName[name]
			}
			if localDoc == "" {
				if src, _ := hit.Chunk.Metadata["source_uri"].(string); src != "" {
					localDoc = byName[strings.TrimPrefix(src, "upload:///")]
				}
			}
		}
		if localDoc != "" && lockedIDs[localDoc] {
			sawLockedHit = true
			continue
		}
		if localDoc == "" && len(lockedIDs) > 0 {
			// Cannot prove the hit is unlocked while the room has excluded docs.
			sawUnmappedWhileLocks = true
			continue
		}
		qh := QueryHit{
			ChunkID:    hit.Chunk.ID,
			DocumentID: localDoc,
			Text:       hit.Chunk.Text,
			Score:      hit.Score,
		}
		fillHitLocus(&qh, hit.Chunk.Metadata)
		out.Results = append(out.Results, qh)
	}
	if res.Answer != "" && !sawLockedHit && !sawUnmappedWhileLocks {
		out.Answer = res.Answer
	}
	return out
}

// isRoomDocumentKnowledgeExcluded reports whether a room document must not be ingested.
func (s *Service) isRoomDocumentKnowledgeExcluded(ctx context.Context, room db.DealRoom, documentID pgtype.UUID) (bool, error) {
	row, err := s.queries.GetDealRoomDocumentByDocumentID(ctx, db.GetDealRoomDocumentByDocumentIDParams{
		RoomID:     room.ID,
		DocumentID: documentID,
	})
	if err != nil {
		return false, err
	}
	return knowledgeExcluded(row.Locked, row.FolderPath, lockedFolderPathSet(room.Settings)), nil
}

func mapUpstream(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *docling.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Status == 401 || apiErr.Status == 403:
			return fmt.Errorf("%w: upstream denied", ErrUnavailable)
		case apiErr.Status == 404:
			return ErrNotFound
		default:
			return fmt.Errorf("%w: %s", ErrUnavailable, apiErr.Code)
		}
	}
	// Dial/timeout failures from an unreachable docling-rag host.
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: upstream unreachable", ErrUnavailable)
	}
	msg := err.Error()
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "Client.Timeout") {
		return fmt.Errorf("%w: upstream unreachable", ErrUnavailable)
	}
	return err
}

// externalDocName is the stable RAG upload identity. Word/PowerPoint use
// "{id}.pdf" because knowledge ingest uploads the OnlyOffice preview PDF (page
// locus = viewer pages). Other types keep "{id}.{sourceExt}".
func externalDocName(documentID, sourceType, title, storageKey string) string {
	return documentID + "." + knowledgeExternalExt(sourceType, title, storageKey)
}

func fillHitLocus(hit *QueryHit, metadata map[string]any) {
	if metadata == nil {
		return
	}
	// Prefer stamped provenance fields (prov-v1); do not invent from chunk text.
	if name, ok := metadata["source_name"].(string); ok && name != "" {
		hit.SourceName = name
	} else if src, ok := metadata["source_uri"].(string); ok && src != "" {
		hit.SourceName = strings.TrimPrefix(src, "upload:///")
	}
	locus, _ := metadata["locus"].(map[string]any)
	if locus == nil {
		return
	}
	if sheet, ok := locus["sheet"].(string); ok {
		hit.Sheet = sheet
	}
	switch pages := locus["pages"].(type) {
	case []any:
		for _, p := range pages {
			switch v := p.(type) {
			case float64:
				if v >= 1 {
					hit.Pages = append(hit.Pages, int(v))
				}
			case int:
				if v >= 1 {
					hit.Pages = append(hit.Pages, v)
				}
			}
		}
	case []float64:
		for _, v := range pages {
			if v >= 1 {
				hit.Pages = append(hit.Pages, int(v))
			}
		}
	case []int:
		for _, v := range pages {
			if v >= 1 {
				hit.Pages = append(hit.Pages, v)
			}
		}
	}
}

func pgUUID(id string) pgtype.UUID {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func contentTypeForName(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return "application/octet-stream"
	}
}
