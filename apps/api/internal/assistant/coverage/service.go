package coverage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	linkpkg "github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Run / Snapshot status constants.
const (
	RunQueued    = "queued"
	RunRunning   = "running"
	RunSucceeded = "succeeded"
	RunFailed    = "failed"

	ScopeRoom = "room"
	ScopeLink = "link"
)

var (
	// ErrDisabled is returned when ASK_DOCS_DD_COVERAGE is off.
	ErrDisabled = errors.New("dd coverage disabled")
	// ErrForbidden is returned when the caller cannot access the room.
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound is returned when room/run/snapshot is missing.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned when a scan is already queued/running for the scope.
	ErrConflict = errors.New("scan already in progress")
	// ErrQueueUnavailable is returned when Redis scan queue is not configured.
	ErrQueueUnavailable = errors.New("dd scan queue unavailable")
	// ErrInvalidInput is returned for bad pack_id / link_id.
	ErrInvalidInput = errors.New("invalid input")
)

// StreamName is the Redis Streams key for DD scan jobs (D11).
const StreamName = "askdocs:dd_scan"

// Querier is the DB surface for DD coverage.
type Querier interface {
	GetDealRoomByID(ctx context.Context, arg db.GetDealRoomByIDParams) (db.DealRoom, error)
	GetWorkspaceMember(ctx context.Context, arg db.GetWorkspaceMemberParams) (db.WorkspaceMember, error)
	GetRoomMemberByUserID(ctx context.Context, arg db.GetRoomMemberByUserIDParams) (db.RoomMember, error)
	GetDealRoomKnowledgeBaseByRoom(ctx context.Context, roomID pgtype.UUID) (db.DealRoomKnowledgeBasis, error)
	GetLinkByIDAndWorkspace(ctx context.Context, arg db.GetLinkByIDAndWorkspaceParams) (db.Link, error)
	CreateAskDocsDDRun(ctx context.Context, arg db.CreateAskDocsDDRunParams) (db.AskDocsDdRun, error)
	GetAskDocsDDRun(ctx context.Context, arg db.GetAskDocsDDRunParams) (db.AskDocsDdRun, error)
	ListActiveAskDocsDDRunsForRoom(ctx context.Context, dealRoomID pgtype.UUID) ([]db.AskDocsDdRun, error)
	UpdateAskDocsDDRunStatus(ctx context.Context, arg db.UpdateAskDocsDDRunStatusParams) (db.AskDocsDdRun, error)
	UpsertAskDocsDDSnapshot(ctx context.Context, arg db.UpsertAskDocsDDSnapshotParams) (db.AskDocsDdSnapshot, error)
	UpsertAskDocsDDSnapshotForLink(ctx context.Context, arg db.UpsertAskDocsDDSnapshotForLinkParams) (db.AskDocsDdSnapshot, error)
	GetAskDocsDDSnapshotRoom(ctx context.Context, arg db.GetAskDocsDDSnapshotRoomParams) (db.AskDocsDdSnapshot, error)
	GetAskDocsDDSnapshotLink(ctx context.Context, arg db.GetAskDocsDDSnapshotLinkParams) (db.AskDocsDdSnapshot, error)
	MarkAskDocsDDSnapshotsStaleForRoom(ctx context.Context, arg db.MarkAskDocsDDSnapshotsStaleForRoomParams) error
	MarkAllAskDocsDDSnapshotsStaleForRoom(ctx context.Context, dealRoomID pgtype.UUID) error
	GetAskDocsDDRoomPack(ctx context.Context, dealRoomID pgtype.UUID) (db.AskDocsDdRoomPack, error)
	UpsertAskDocsDDRoomPack(ctx context.Context, arg db.UpsertAskDocsDDRoomPackParams) (db.AskDocsDdRoomPack, error)
	DeleteAskDocsDDRoomPack(ctx context.Context, arg db.DeleteAskDocsDDRoomPackParams) (int64, error)
	UpsertAskDocsDDCrossCheck(ctx context.Context, arg db.UpsertAskDocsDDCrossCheckParams) (db.AskDocsDdCrossCheck, error)
	GetAskDocsDDCrossCheckLatest(ctx context.Context, arg db.GetAskDocsDDCrossCheckLatestParams) (db.AskDocsDdCrossCheck, error)
	ListDealRoomDocumentsWithMeta(ctx context.Context, roomID pgtype.UUID) ([]db.ListDealRoomDocumentsWithMetaRow, error)
	ListLinkDocumentsByPublicToken(ctx context.Context, publicToken string) ([]db.ListLinkDocumentsByPublicTokenRow, error)
	GetDocumentByID(ctx context.Context, arg db.GetDocumentByIDParams) (db.GetDocumentByIDRow, error)
}

// StartScanRequest is the Owner API body for enqueueing a scan.
type StartScanRequest struct {
	PackID string `json:"pack_id,omitempty"`
	LinkID string `json:"link_id,omitempty"`
	Lang   string `json:"lang,omitempty"`
}

// ScanJob is the Redis stream payload.
type ScanJob struct {
	RunID       string          `json:"run_id"`
	WorkspaceID string          `json:"workspace_id"`
	DealRoomID  string          `json:"deal_room_id"`
	LinkID      string          `json:"link_id,omitempty"`
	PackID      string          `json:"pack_id"`
	PackVersion string          `json:"pack_version,omitempty"`
	PackItems   []jobs.PackItem `json:"pack_items,omitempty"` // pinned fork/builtin copy at enqueue
	Lang        string          `json:"lang"`
	Attempt     int             `json:"attempt"`
}

// RunView is the Owner-facing run projection.
type RunView struct {
	ID           string     `json:"id"`
	PackID       string     `json:"pack_id"`
	PackVersion  string     `json:"pack_version"`
	Scope        string     `json:"scope"`
	LinkID       string     `json:"link_id,omitempty"`
	Status       string     `json:"status"`
	TriggeredBy  string     `json:"triggered_by"`
	ErrorMessage string     `json:"error_message,omitempty"`
	KBGeneration *int32     `json:"kb_generation,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// SnapshotView is the Owner-facing latest ClaimPack snapshot.
type SnapshotView struct {
	ID           string        `json:"id"`
	PackID       string        `json:"pack_id"`
	PackVersion  string        `json:"pack_version"`
	Scope        string        `json:"scope"`
	LinkID       string        `json:"link_id,omitempty"`
	RunID        string        `json:"run_id"`
	KBGeneration *int32        `json:"kb_generation,omitempty"`
	Stale        bool          `json:"stale"`
	CoverageRows []CoverageRow `json:"coverage_rows"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// CrossCheckRequest is the Owner API body for dual-document comparison.
type CrossCheckRequest struct {
	PackID      string `json:"pack_id,omitempty"`
	DocumentAID string `json:"document_a_id"`
	DocumentBID string `json:"document_b_id"`
	Lang        string `json:"lang,omitempty"`
}

// CrossCheckView is the Owner-facing dual-document ClaimPack.
type CrossCheckView struct {
	ID           string            `json:"id"`
	PackID       string            `json:"pack_id"`
	PackVersion  string            `json:"pack_version"`
	DocumentAID  string            `json:"document_a_id"`
	DocumentBID  string            `json:"document_b_id"`
	TriggeredBy  string            `json:"triggered_by"`
	Claims       []CrossCheckClaim `json:"claims"`
	CreatedAt    time.Time         `json:"created_at"`
}

// Queue enqueues DD scan jobs.
type Queue interface {
	Enqueue(ctx context.Context, job ScanJob) error
	Dequeue(ctx context.Context, consumerGroup, consumerName string) (job ScanJob, ackID string, err error)
	Ack(ctx context.Context, consumerGroup, ackID string) error
	EnsureConsumerGroup(ctx context.Context, consumerGroup string) error
}

// Service orchestrates Owner DD coverage scans.
type Service struct {
	queries   Querier
	searcher  Searcher
	packs     *jobs.PackRegistry
	queue     Queue
	opts      Options
	completer Completer
}

// NewService constructs a coverage service. queue may be nil (StartScan → ErrQueueUnavailable).
func NewService(q Querier, searcher Searcher, packs *jobs.PackRegistry, queue Queue, opts Options) *Service {
	if packs == nil {
		packs = jobs.MustLoadBuiltinPacks()
	}
	return &Service{queries: q, searcher: searcher, packs: packs, queue: queue, opts: opts}
}

// WithCompleter wires an optional LLM for P2.1a boundary row reclassification.
func (s *Service) WithCompleter(c Completer) *Service {
	s.completer = c
	return s
}

// StartScan authorizes, creates a queued run, and enqueues the Redis job.
func (s *Service) StartScan(ctx context.Context, workspaceID, roomID, userID string, req StartScanRequest) (RunView, error) {
	if !s.opts.Enabled {
		return RunView{}, ErrDisabled
	}
	if s.queue == nil {
		return RunView{}, ErrQueueUnavailable
	}
	room, err := s.loadRoom(ctx, workspaceID, roomID)
	if err != nil {
		return RunView{}, err
	}
	if err := s.authorize(ctx, room, userID); err != nil {
		return RunView{}, err
	}

	packID := req.PackID
	if packID == "" {
		packID = jobs.FinancingDDV1
	}
	if !jobs.IsBuiltinPackID(packID) {
		return RunView{}, fmt.Errorf("%w: unknown pack_id", ErrInvalidInput)
	}
	pack, err := s.resolveEffectivePack(ctx, room.ID, packID)
	if err != nil {
		return RunView{}, err
	}

	var linkID pgtype.UUID
	if req.LinkID != "" {
		lid, err := uuid.Parse(req.LinkID)
		if err != nil {
			return RunView{}, fmt.Errorf("%w: link_id", ErrInvalidInput)
		}
		link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
			ID:          pgtype.UUID{Bytes: lid, Valid: true},
			WorkspaceID: room.WorkspaceID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return RunView{}, ErrNotFound
			}
			return RunView{}, err
		}
		if !link.DealRoomID.Valid || link.DealRoomID.Bytes != room.ID.Bytes {
			return RunView{}, fmt.Errorf("%w: link not in room", ErrInvalidInput)
		}
		linkID = link.ID
	}

	active, err := s.queries.ListActiveAskDocsDDRunsForRoom(ctx, room.ID)
	if err != nil {
		return RunView{}, err
	}
	if len(active) > 0 {
		return RunView{}, ErrConflict
	}

	kbGen := pgtype.Int4{}
	if kb, err := s.queries.GetDealRoomKnowledgeBaseByRoom(ctx, room.ID); err == nil {
		kbGen = pgtype.Int4{Int32: kb.ActiveGeneration, Valid: true}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return RunView{}, err
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		return RunView{}, ErrForbidden
	}
	run, err := s.queries.CreateAskDocsDDRun(ctx, db.CreateAskDocsDDRunParams{
		WorkspaceID:  room.WorkspaceID,
		DealRoomID:   room.ID,
		LinkID:       linkID,
		PackID:       pack.PackID,
		PackVersion:  pack.PackVersion,
		Status:       RunQueued,
		TriggeredBy:  pgtype.UUID{Bytes: uid, Valid: true},
		KbGeneration: kbGen,
	})
	if err != nil {
		return RunView{}, err
	}

	lang := req.Lang
	if lang == "" {
		lang = "en"
	}
	job := ScanJob{
		RunID:       uuid.UUID(run.ID.Bytes).String(),
		WorkspaceID: uuid.UUID(room.WorkspaceID.Bytes).String(),
		DealRoomID:  uuid.UUID(room.ID.Bytes).String(),
		PackID:      pack.PackID,
		PackVersion: pack.PackVersion,
		PackItems:   append([]jobs.PackItem(nil), pack.Items...),
		Lang:        lang,
		Attempt:     0,
	}
	if linkID.Valid {
		job.LinkID = uuid.UUID(linkID.Bytes).String()
	}
	if err := s.queue.Enqueue(ctx, job); err != nil {
		_, _ = s.queries.UpdateAskDocsDDRunStatus(ctx, db.UpdateAskDocsDDRunStatusParams{
			ID:           run.ID,
			Status:       RunFailed,
			ErrorMessage: pgtype.Text{String: "enqueue failed: " + err.Error(), Valid: true},
			FinishedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		return RunView{}, fmt.Errorf("enqueue dd scan: %w", err)
	}
	return toRunView(run), nil
}

// GetRun returns run metadata for the Owner.
func (s *Service) GetRun(ctx context.Context, workspaceID, roomID, runID, userID string) (RunView, error) {
	if !s.opts.Enabled {
		return RunView{}, ErrDisabled
	}
	room, err := s.loadRoom(ctx, workspaceID, roomID)
	if err != nil {
		return RunView{}, err
	}
	if err := s.authorize(ctx, room, userID); err != nil {
		return RunView{}, err
	}
	rid, err := uuid.Parse(runID)
	if err != nil {
		return RunView{}, ErrNotFound
	}
	run, err := s.queries.GetAskDocsDDRun(ctx, db.GetAskDocsDDRunParams{
		ID:          pgtype.UUID{Bytes: rid, Valid: true},
		WorkspaceID: room.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RunView{}, ErrNotFound
		}
		return RunView{}, err
	}
	if run.DealRoomID.Bytes != room.ID.Bytes {
		return RunView{}, ErrNotFound
	}
	return toRunView(run), nil
}

// GetSnapshot returns the latest ClaimPack snapshot for room or link scope.
func (s *Service) GetSnapshot(ctx context.Context, workspaceID, roomID, userID, packID, linkID string) (SnapshotView, error) {
	if !s.opts.Enabled {
		return SnapshotView{}, ErrDisabled
	}
	room, err := s.loadRoom(ctx, workspaceID, roomID)
	if err != nil {
		return SnapshotView{}, err
	}
	if err := s.authorize(ctx, room, userID); err != nil {
		return SnapshotView{}, err
	}
	if packID == "" {
		packID = jobs.FinancingDDV1
	}
	if _, ok := s.packs.Get(packID); !ok {
		return SnapshotView{}, fmt.Errorf("%w: unknown pack_id", ErrInvalidInput)
	}

	var snap db.AskDocsDdSnapshot
	if linkID == "" {
		snap, err = s.queries.GetAskDocsDDSnapshotRoom(ctx, db.GetAskDocsDDSnapshotRoomParams{
			DealRoomID: room.ID,
			PackID:     packID,
		})
	} else {
		lid, parseErr := uuid.Parse(linkID)
		if parseErr != nil {
			return SnapshotView{}, fmt.Errorf("%w: link_id", ErrInvalidInput)
		}
		snap, err = s.queries.GetAskDocsDDSnapshotLink(ctx, db.GetAskDocsDDSnapshotLinkParams{
			DealRoomID: room.ID,
			LinkID:     pgtype.UUID{Bytes: lid, Valid: true},
			PackID:     packID,
		})
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SnapshotView{}, ErrNotFound
		}
		return SnapshotView{}, err
	}
	return toSnapshotView(snap)
}

// GetPack returns the effective checklist pack for the room (fork or builtin).
// packID defaults to financing_dd_v1; ma_redflag_v1 is always built-in (no fork).
func (s *Service) GetPack(ctx context.Context, workspaceID, roomID, userID, packID string) (PackView, error) {
	if !s.opts.Enabled {
		return PackView{}, ErrDisabled
	}
	room, err := s.loadRoom(ctx, workspaceID, roomID)
	if err != nil {
		return PackView{}, err
	}
	if err := s.authorize(ctx, room, userID); err != nil {
		return PackView{}, err
	}
	if packID == "" {
		packID = jobs.FinancingDDV1
	}
	if !jobs.IsBuiltinPackID(packID) {
		return PackView{}, fmt.Errorf("%w: unknown pack_id", ErrInvalidInput)
	}
	return s.packViewForRoom(ctx, room.ID, packID)
}

// ListPacks returns built-in pack summaries available for scanning.
func (s *Service) ListPacks(ctx context.Context, workspaceID, roomID, userID string) ([]PackView, error) {
	if !s.opts.Enabled {
		return nil, ErrDisabled
	}
	room, err := s.loadRoom(ctx, workspaceID, roomID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, room, userID); err != nil {
		return nil, err
	}
	out := make([]PackView, 0, 2)
	for _, p := range s.packs.List() {
		view, err := s.packViewForRoom(ctx, room.ID, p.PackID)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

// PutPack upserts a room-level fork of financing_dd_v1 and marks snapshots stale.
func (s *Service) PutPack(ctx context.Context, workspaceID, roomID, userID string, req PutPackRequest) (PackView, error) {
	if !s.opts.Enabled {
		return PackView{}, ErrDisabled
	}
	room, err := s.loadRoom(ctx, workspaceID, roomID)
	if err != nil {
		return PackView{}, err
	}
	if err := s.authorize(ctx, room, userID); err != nil {
		return PackView{}, err
	}
	if !jobs.ForkAllowed(jobs.FinancingDDV1) {
		return PackView{}, fmt.Errorf("%w: fork not allowed", ErrInvalidInput)
	}
	base, ok := s.packs.Get(jobs.FinancingDDV1)
	if !ok {
		return PackView{}, fmt.Errorf("%w: builtin pack missing", ErrInvalidInput)
	}
	items := NormalizeForkItems(req.Items)
	if err := ValidateForkItems(items); err != nil {
		return PackView{}, err
	}
	revision := int32(1)
	if existing, err := s.queries.GetAskDocsDDRoomPack(ctx, room.ID); err == nil {
		revision = existing.ForkRevision + 1
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return PackView{}, err
	}
	version := forkVersion(base.PackVersion, int(revision))
	payload, err := json.Marshal(items)
	if err != nil {
		return PackView{}, err
	}
	uid := pgtype.UUID{}
	if parsed, err := uuid.Parse(userID); err == nil {
		uid = pgtype.UUID{Bytes: parsed, Valid: true}
	}
	row, err := s.queries.UpsertAskDocsDDRoomPack(ctx, db.UpsertAskDocsDDRoomPackParams{
		DealRoomID:   room.ID,
		WorkspaceID:  room.WorkspaceID,
		BasePackID:   base.PackID,
		PackVersion:  version,
		ForkRevision: revision,
		Items:        payload,
		UpdatedBy:    uid,
	})
	if err != nil {
		return PackView{}, err
	}
	_ = s.queries.MarkAllAskDocsDDSnapshotsStaleForRoom(ctx, room.ID)
	return PackView{
		PackID:       base.PackID,
		PackVersion:  row.PackVersion,
		BasePackID:   row.BasePackID,
		Forked:       true,
		ForkRevision: int(row.ForkRevision),
		Items:        items,
	}, nil
}

// ResetPack deletes the room fork (back to builtin) and marks snapshots stale.
func (s *Service) ResetPack(ctx context.Context, workspaceID, roomID, userID string) (PackView, error) {
	if !s.opts.Enabled {
		return PackView{}, ErrDisabled
	}
	room, err := s.loadRoom(ctx, workspaceID, roomID)
	if err != nil {
		return PackView{}, err
	}
	if err := s.authorize(ctx, room, userID); err != nil {
		return PackView{}, err
	}
	if _, err := s.queries.DeleteAskDocsDDRoomPack(ctx, db.DeleteAskDocsDDRoomPackParams{
		DealRoomID:  room.ID,
		WorkspaceID: room.WorkspaceID,
	}); err != nil {
		return PackView{}, err
	}
	_ = s.queries.MarkAllAskDocsDDSnapshotsStaleForRoom(ctx, room.ID)
	return s.packViewForRoom(ctx, room.ID, jobs.FinancingDDV1)
}

// StartCrossCheck runs Owner dual-document comparison and persists the latest ClaimPack.
func (s *Service) StartCrossCheck(ctx context.Context, workspaceID, roomID, userID string, req CrossCheckRequest) (CrossCheckView, error) {
	if !s.opts.Enabled {
		return CrossCheckView{}, ErrDisabled
	}
	room, err := s.loadRoom(ctx, workspaceID, roomID)
	if err != nil {
		return CrossCheckView{}, err
	}
	if err := s.authorize(ctx, room, userID); err != nil {
		return CrossCheckView{}, err
	}
	packID := req.PackID
	if packID == "" {
		packID = jobs.FinancingDDV1
	}
	if !jobs.IsBuiltinPackID(packID) {
		return CrossCheckView{}, fmt.Errorf("%w: unknown pack_id", ErrInvalidInput)
	}
	docA, err := uuid.Parse(req.DocumentAID)
	if err != nil {
		return CrossCheckView{}, fmt.Errorf("%w: document_a_id", ErrInvalidInput)
	}
	docB, err := uuid.Parse(req.DocumentBID)
	if err != nil {
		return CrossCheckView{}, fmt.Errorf("%w: document_b_id", ErrInvalidInput)
	}
	if docA == docB {
		return CrossCheckView{}, fmt.Errorf("%w: document ids must differ", ErrInvalidInput)
	}
	if err := s.ensureDocsInRoomKB(ctx, room.ID, docA, docB); err != nil {
		return CrossCheckView{}, err
	}
	pack, err := s.resolveEffectivePack(ctx, room.ID, packID)
	if err != nil {
		return CrossCheckView{}, err
	}
	lang := req.Lang
	if lang == "" {
		lang = "en"
	}
	claims, err := CrossCheckPack(ctx, s.searcher, room.WorkspaceID, docA, docB, pack, lang)
	if err != nil {
		return CrossCheckView{}, err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return CrossCheckView{}, err
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return CrossCheckView{}, ErrForbidden
	}
	row, err := s.queries.UpsertAskDocsDDCrossCheck(ctx, db.UpsertAskDocsDDCrossCheckParams{
		WorkspaceID: room.WorkspaceID,
		DealRoomID:  room.ID,
		PackID:      pack.PackID,
		PackVersion: pack.PackVersion,
		DocumentAID: pgtype.UUID{Bytes: docA, Valid: true},
		DocumentBID: pgtype.UUID{Bytes: docB, Valid: true},
		TriggeredBy: pgtype.UUID{Bytes: uid, Valid: true},
		Claims:      payload,
	})
	if err != nil {
		return CrossCheckView{}, err
	}
	return toCrossCheckView(row)
}

// GetLatestCrossCheck returns the latest dual-document ClaimPack for the room+pack.
func (s *Service) GetLatestCrossCheck(ctx context.Context, workspaceID, roomID, userID, packID string) (CrossCheckView, error) {
	if !s.opts.Enabled {
		return CrossCheckView{}, ErrDisabled
	}
	room, err := s.loadRoom(ctx, workspaceID, roomID)
	if err != nil {
		return CrossCheckView{}, err
	}
	if err := s.authorize(ctx, room, userID); err != nil {
		return CrossCheckView{}, err
	}
	if packID == "" {
		packID = jobs.FinancingDDV1
	}
	if !jobs.IsBuiltinPackID(packID) {
		return CrossCheckView{}, fmt.Errorf("%w: unknown pack_id", ErrInvalidInput)
	}
	row, err := s.queries.GetAskDocsDDCrossCheckLatest(ctx, db.GetAskDocsDDCrossCheckLatestParams{
		DealRoomID: room.ID,
		PackID:     packID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CrossCheckView{}, ErrNotFound
		}
		return CrossCheckView{}, err
	}
	return toCrossCheckView(row)
}

func (s *Service) ensureDocsInRoomKB(ctx context.Context, dealRoomID pgtype.UUID, docs ...uuid.UUID) error {
	kb, err := s.queries.GetDealRoomKnowledgeBaseByRoom(ctx, dealRoomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: knowledge base has no active documents", ErrInvalidInput)
		}
		return err
	}
	allowed := make(map[uuid.UUID]struct{}, len(kb.ActiveDocumentIds))
	for _, id := range kb.ActiveDocumentIds {
		if id.Valid {
			allowed[uuid.UUID(id.Bytes)] = struct{}{}
		}
	}
	for _, d := range docs {
		if _, ok := allowed[d]; !ok {
			return fmt.Errorf("%w: document %s not in room knowledge base", ErrInvalidInput, d)
		}
	}
	return nil
}

// EffectivePackForRoom returns the scan/chip pack for a deal room (no auth; caller must gate).
func (s *Service) EffectivePackForRoom(ctx context.Context, dealRoomID pgtype.UUID) (jobs.Pack, error) {
	return s.resolveEffectivePack(ctx, dealRoomID, jobs.FinancingDDV1)
}

func (s *Service) resolveEffectivePack(ctx context.Context, dealRoomID pgtype.UUID, packID string) (jobs.Pack, error) {
	base, ok := s.packs.Get(packID)
	if !ok {
		return jobs.Pack{}, fmt.Errorf("%w: unknown pack_id", ErrInvalidInput)
	}
	if !jobs.ForkAllowed(packID) {
		return base, nil
	}
	row, err := s.queries.GetAskDocsDDRoomPack(ctx, dealRoomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return base, nil
		}
		return jobs.Pack{}, err
	}
	if row.BasePackID != "" && row.BasePackID != packID {
		return base, nil
	}
	items, err := decodePackItems(row.Items)
	if err != nil {
		return jobs.Pack{}, err
	}
	return packFromItems(base, row.PackVersion, items), nil
}

func (s *Service) packViewForRoom(ctx context.Context, dealRoomID pgtype.UUID, packID string) (PackView, error) {
	base, ok := s.packs.Get(packID)
	if !ok {
		return PackView{}, fmt.Errorf("%w: unknown pack_id", ErrInvalidInput)
	}
	if !jobs.ForkAllowed(packID) {
		return PackView{
			PackID:      base.PackID,
			PackVersion: base.PackVersion,
			BasePackID:  base.PackID,
			Forked:      false,
			Items:       append([]jobs.PackItem(nil), base.Items...),
		}, nil
	}
	row, err := s.queries.GetAskDocsDDRoomPack(ctx, dealRoomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PackView{
				PackID:      base.PackID,
				PackVersion: base.PackVersion,
				BasePackID:  base.PackID,
				Forked:      false,
				Items:       append([]jobs.PackItem(nil), base.Items...),
			}, nil
		}
		return PackView{}, err
	}
	items, err := decodePackItems(row.Items)
	if err != nil {
		return PackView{}, err
	}
	return PackView{
		PackID:       base.PackID,
		PackVersion:  row.PackVersion,
		BasePackID:   row.BasePackID,
		Forked:       true,
		ForkRevision: int(row.ForkRevision),
		Items:        items,
	}, nil
}

// ExecuteRun runs the row engine for a queued job (worker entrypoint).
func (s *Service) ExecuteRun(ctx context.Context, job ScanJob) error {
	rid, err := uuid.Parse(job.RunID)
	if err != nil {
		return fmt.Errorf("invalid run_id: %w", err)
	}
	wsID, err := uuid.Parse(job.WorkspaceID)
	if err != nil {
		return fmt.Errorf("invalid workspace_id: %w", err)
	}
	run, err := s.queries.GetAskDocsDDRun(ctx, db.GetAskDocsDDRunParams{
		ID:          pgtype.UUID{Bytes: rid, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
	})
	if err != nil {
		return err
	}
	if run.Status != RunQueued && run.Status != RunRunning {
		return nil // already terminal
	}

	now := time.Now().UTC()
	if _, err := s.queries.UpdateAskDocsDDRunStatus(ctx, db.UpdateAskDocsDDRunStatusParams{
		ID:        run.ID,
		Status:    RunRunning,
		StartedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		return err
	}

	fail := func(msg string) error {
		_, _ = s.queries.UpdateAskDocsDDRunStatus(ctx, db.UpdateAskDocsDDRunStatusParams{
			ID:           run.ID,
			Status:       RunFailed,
			ErrorMessage: pgtype.Text{String: msg, Valid: true},
			FinishedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		return errors.New(msg)
	}

	pack, ok := s.packs.Get(run.PackID)
	if !ok {
		return fail("unknown pack_id")
	}
	if len(job.PackItems) > 0 {
		version := job.PackVersion
		if version == "" {
			version = run.PackVersion
		}
		pack = packFromItems(pack, version, job.PackItems)
	} else if versioned, err := s.resolveEffectivePack(ctx, run.DealRoomID, run.PackID); err == nil {
		pack = versioned
	}

	docIDs, kbGen, err := s.resolveDocumentIDs(ctx, run)
	if err != nil {
		return fail(err.Error())
	}

	lang := job.Lang
	if lang == "" {
		lang = "en"
	}
	rows, err := ScanPack(ctx, s.searcher, run.WorkspaceID, docIDs, pack, lang)
	if err != nil {
		return fail(err.Error())
	}
	rows = RefineBoundaryRows(ctx, s.completer, pack, lang, rows, s.opts)
	rows = AttachExtractedValues(rows, pack)
	payload, err := json.Marshal(rows)
	if err != nil {
		return fail("marshal coverage_rows: " + err.Error())
	}

	if run.LinkID.Valid {
		_, err = s.queries.UpsertAskDocsDDSnapshotForLink(ctx, db.UpsertAskDocsDDSnapshotForLinkParams{
			WorkspaceID:  run.WorkspaceID,
			DealRoomID:   run.DealRoomID,
			LinkID:       run.LinkID,
			PackID:       pack.PackID,
			PackVersion:  pack.PackVersion,
			RunID:        run.ID,
			KbGeneration: kbGen,
			CoverageRows: payload,
		})
	} else {
		_, err = s.queries.UpsertAskDocsDDSnapshot(ctx, db.UpsertAskDocsDDSnapshotParams{
			WorkspaceID:  run.WorkspaceID,
			DealRoomID:   run.DealRoomID,
			LinkID:       pgtype.UUID{},
			PackID:       pack.PackID,
			PackVersion:  pack.PackVersion,
			RunID:        run.ID,
			KbGeneration: kbGen,
			CoverageRows: payload,
		})
	}
	if err != nil {
		return fail("persist snapshot: " + err.Error())
	}

	_, err = s.queries.UpdateAskDocsDDRunStatus(ctx, db.UpdateAskDocsDDRunStatusParams{
		ID:         run.ID,
		Status:     RunSucceeded,
		FinishedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	return err
}

func (s *Service) resolveDocumentIDs(ctx context.Context, run db.AskDocsDdRun) ([]uuid.UUID, pgtype.Int4, error) {
	kb, err := s.queries.GetDealRoomKnowledgeBaseByRoom(ctx, run.DealRoomID)
	kbGen := pgtype.Int4{}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, kbGen, nil
		}
		return nil, kbGen, err
	}
	kbGen = pgtype.Int4{Int32: kb.ActiveGeneration, Valid: true}
	kbDocs := pgUUIDsToUUIDs(kb.ActiveDocumentIds)
	switch kb.Status {
	case "ready", "stale", "building":
		// ok
	default:
		return nil, kbGen, nil
	}

	if !run.LinkID.Valid {
		return kbDocs, kbGen, nil
	}
	link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          run.LinkID,
		WorkspaceID: run.WorkspaceID,
	})
	if err != nil {
		return nil, kbGen, err
	}
	authorized, err := linkpkg.AuthorizedDocumentIDs(ctx, s.queries, link)
	if err != nil {
		return nil, kbGen, err
	}
	return intersectUUIDs(authorized, kbDocs), kbGen, nil
}

func (s *Service) loadRoom(ctx context.Context, workspaceID, roomID string) (db.DealRoom, error) {
	wid, err := uuid.Parse(workspaceID)
	if err != nil {
		return db.DealRoom{}, ErrNotFound
	}
	rid, err := uuid.Parse(roomID)
	if err != nil {
		return db.DealRoom{}, ErrNotFound
	}
	room, err := s.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
		ID:          pgtype.UUID{Bytes: rid, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wid, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.DealRoom{}, ErrNotFound
		}
		return db.DealRoom{}, err
	}
	return room, nil
}

func (s *Service) authorize(ctx context.Context, room db.DealRoom, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrForbidden
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}
	ws, err := s.queries.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: room.WorkspaceID,
		UserID:      userUUID,
	})
	if err == nil && (ws.Role == "owner" || ws.Role == "admin") {
		return nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	member, err := s.queries.GetRoomMemberByUserID(ctx, db.GetRoomMemberByUserIDParams{
		RoomID: room.ID,
		UserID: userUUID,
	})
	if err == nil && member.Status == "active" {
		return nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return ErrForbidden
}

func toRunView(run db.AskDocsDdRun) RunView {
	v := RunView{
		ID:           uuid.UUID(run.ID.Bytes).String(),
		PackID:       run.PackID,
		PackVersion:  run.PackVersion,
		Scope:        ScopeRoom,
		Status:       run.Status,
		TriggeredBy:  uuid.UUID(run.TriggeredBy.Bytes).String(),
		ErrorMessage: run.ErrorMessage,
		CreatedAt:    run.CreatedAt.Time,
	}
	if run.LinkID.Valid {
		v.Scope = ScopeLink
		v.LinkID = uuid.UUID(run.LinkID.Bytes).String()
	}
	if run.KbGeneration.Valid {
		g := run.KbGeneration.Int32
		v.KBGeneration = &g
	}
	if run.StartedAt.Valid {
		t := run.StartedAt.Time
		v.StartedAt = &t
	}
	if run.FinishedAt.Valid {
		t := run.FinishedAt.Time
		v.FinishedAt = &t
	}
	return v
}

func toSnapshotView(snap db.AskDocsDdSnapshot) (SnapshotView, error) {
	rows := []CoverageRow{}
	if len(snap.CoverageRows) > 0 {
		if err := json.Unmarshal(snap.CoverageRows, &rows); err != nil {
			return SnapshotView{}, fmt.Errorf("decode coverage_rows: %w", err)
		}
	}
	v := SnapshotView{
		ID:           uuid.UUID(snap.ID.Bytes).String(),
		PackID:       snap.PackID,
		PackVersion:  snap.PackVersion,
		Scope:        ScopeRoom,
		RunID:        uuid.UUID(snap.RunID.Bytes).String(),
		Stale:        snap.Stale,
		CoverageRows: rows,
		CreatedAt:    snap.CreatedAt.Time,
		UpdatedAt:    snap.UpdatedAt.Time,
	}
	if snap.LinkID.Valid {
		v.Scope = ScopeLink
		v.LinkID = uuid.UUID(snap.LinkID.Bytes).String()
	}
	if snap.KbGeneration.Valid {
		g := snap.KbGeneration.Int32
		v.KBGeneration = &g
	}
	return v, nil
}

func toCrossCheckView(row db.AskDocsDdCrossCheck) (CrossCheckView, error) {
	claims := []CrossCheckClaim{}
	if len(row.Claims) > 0 {
		if err := json.Unmarshal(row.Claims, &claims); err != nil {
			return CrossCheckView{}, fmt.Errorf("decode claims: %w", err)
		}
	}
	return CrossCheckView{
		ID:           uuid.UUID(row.ID.Bytes).String(),
		PackID:       row.PackID,
		PackVersion:  row.PackVersion,
		DocumentAID:  uuid.UUID(row.DocumentAID.Bytes).String(),
		DocumentBID:  uuid.UUID(row.DocumentBID.Bytes).String(),
		TriggeredBy:  uuid.UUID(row.TriggeredBy.Bytes).String(),
		Claims:       claims,
		CreatedAt:    row.CreatedAt.Time,
	}, nil
}

func pgUUIDsToUUIDs(ids []pgtype.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !id.Valid {
			continue
		}
		out = append(out, uuid.UUID(id.Bytes))
	}
	return out
}

func intersectUUIDs(a, b []uuid.UUID) []uuid.UUID {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	allowed := make(map[uuid.UUID]struct{}, len(b))
	for _, id := range b {
		allowed[id] = struct{}{}
	}
	out := make([]uuid.UUID, 0, len(a))
	seen := make(map[uuid.UUID]struct{}, len(a))
	for _, id := range a {
		if _, ok := allowed[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
