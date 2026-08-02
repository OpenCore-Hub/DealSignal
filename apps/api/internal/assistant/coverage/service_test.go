package coverage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type memQueue struct {
	jobs []ScanJob
}

func (q *memQueue) Enqueue(_ context.Context, job ScanJob) error {
	q.jobs = append(q.jobs, job)
	return nil
}
func (q *memQueue) Dequeue(context.Context, string, string) (ScanJob, string, error) {
	return ScanJob{}, "", ErrQueueEmpty
}
func (q *memQueue) Ack(context.Context, string, string) error { return nil }
func (q *memQueue) EnsureConsumerGroup(context.Context, string) error {
	return nil
}

type fakeCoverageDB struct {
	room     db.DealRoom
	wsRole   string
	kb       db.DealRoomKnowledgeBasis
	kbErr    error
	active   []db.AskDocsDdRun
	runs     map[string]db.AskDocsDdRun
	snaps    map[string]db.AskDocsDdSnapshot
	roomPack *db.AskDocsDdRoomPack
	cross    map[string]db.AskDocsDdCrossCheck
	createN  int
}

func newFakeCoverageDB() *fakeCoverageDB {
	ws := uuid.New()
	room := uuid.New()
	return &fakeCoverageDB{
		room: db.DealRoom{
			ID:          pgtype.UUID{Bytes: room, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		},
		wsRole: "owner",
		kb: db.DealRoomKnowledgeBasis{
			Status:           "ready",
			ActiveGeneration: 3,
			ActiveDocumentIds: []pgtype.UUID{
				{Bytes: uuid.New(), Valid: true},
				{Bytes: uuid.New(), Valid: true},
			},
		},
		runs:  map[string]db.AskDocsDdRun{},
		snaps: map[string]db.AskDocsDdSnapshot{},
		cross: map[string]db.AskDocsDdCrossCheck{},
	}
}

func (f *fakeCoverageDB) GetDealRoomByID(_ context.Context, arg db.GetDealRoomByIDParams) (db.DealRoom, error) {
	if arg.ID.Bytes != f.room.ID.Bytes || arg.WorkspaceID.Bytes != f.room.WorkspaceID.Bytes {
		return db.DealRoom{}, pgx.ErrNoRows
	}
	return f.room, nil
}
func (f *fakeCoverageDB) GetWorkspaceMember(_ context.Context, _ db.GetWorkspaceMemberParams) (db.WorkspaceMember, error) {
	return db.WorkspaceMember{Role: f.wsRole}, nil
}
func (f *fakeCoverageDB) GetRoomMemberByUserID(context.Context, db.GetRoomMemberByUserIDParams) (db.RoomMember, error) {
	return db.RoomMember{}, pgx.ErrNoRows
}
func (f *fakeCoverageDB) GetDealRoomKnowledgeBaseByRoom(context.Context, pgtype.UUID) (db.DealRoomKnowledgeBasis, error) {
	if f.kbErr != nil {
		return db.DealRoomKnowledgeBasis{}, f.kbErr
	}
	return f.kb, nil
}
func (f *fakeCoverageDB) GetLinkByIDAndWorkspace(context.Context, db.GetLinkByIDAndWorkspaceParams) (db.Link, error) {
	return db.Link{}, pgx.ErrNoRows
}
func (f *fakeCoverageDB) CreateAskDocsDDRun(_ context.Context, arg db.CreateAskDocsDDRunParams) (db.AskDocsDdRun, error) {
	f.createN++
	id := uuid.New()
	run := db.AskDocsDdRun{
		ID:           pgtype.UUID{Bytes: id, Valid: true},
		WorkspaceID:  arg.WorkspaceID,
		DealRoomID:   arg.DealRoomID,
		LinkID:       arg.LinkID,
		PackID:       arg.PackID,
		PackVersion:  arg.PackVersion,
		Status:       arg.Status,
		TriggeredBy:  arg.TriggeredBy,
		KbGeneration: arg.KbGeneration,
		CreatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	f.runs[id.String()] = run
	return run, nil
}
func (f *fakeCoverageDB) GetAskDocsDDRun(_ context.Context, arg db.GetAskDocsDDRunParams) (db.AskDocsDdRun, error) {
	run, ok := f.runs[uuid.UUID(arg.ID.Bytes).String()]
	if !ok {
		return db.AskDocsDdRun{}, pgx.ErrNoRows
	}
	return run, nil
}
func (f *fakeCoverageDB) ListActiveAskDocsDDRunsForRoom(context.Context, pgtype.UUID) ([]db.AskDocsDdRun, error) {
	return f.active, nil
}
func (f *fakeCoverageDB) UpdateAskDocsDDRunStatus(_ context.Context, arg db.UpdateAskDocsDDRunStatusParams) (db.AskDocsDdRun, error) {
	key := uuid.UUID(arg.ID.Bytes).String()
	run, ok := f.runs[key]
	if !ok {
		return db.AskDocsDdRun{}, pgx.ErrNoRows
	}
	run.Status = arg.Status
	if arg.ErrorMessage.Valid {
		run.ErrorMessage = arg.ErrorMessage.String
	}
	if arg.StartedAt.Valid {
		run.StartedAt = arg.StartedAt
	}
	if arg.FinishedAt.Valid {
		run.FinishedAt = arg.FinishedAt
	}
	f.runs[key] = run
	return run, nil
}
func (f *fakeCoverageDB) UpsertAskDocsDDSnapshot(_ context.Context, arg db.UpsertAskDocsDDSnapshotParams) (db.AskDocsDdSnapshot, error) {
	id := uuid.New()
	snap := db.AskDocsDdSnapshot{
		ID:           pgtype.UUID{Bytes: id, Valid: true},
		WorkspaceID:  arg.WorkspaceID,
		DealRoomID:   arg.DealRoomID,
		PackID:       arg.PackID,
		PackVersion:  arg.PackVersion,
		RunID:        arg.RunID,
		KbGeneration: arg.KbGeneration,
		CoverageRows: arg.CoverageRows,
		CreatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	f.snaps[arg.PackID] = snap
	return snap, nil
}
func (f *fakeCoverageDB) UpsertAskDocsDDSnapshotForLink(context.Context, db.UpsertAskDocsDDSnapshotForLinkParams) (db.AskDocsDdSnapshot, error) {
	return db.AskDocsDdSnapshot{}, errors.New("unused")
}
func (f *fakeCoverageDB) GetAskDocsDDSnapshotRoom(_ context.Context, arg db.GetAskDocsDDSnapshotRoomParams) (db.AskDocsDdSnapshot, error) {
	snap, ok := f.snaps[arg.PackID]
	if !ok {
		return db.AskDocsDdSnapshot{}, pgx.ErrNoRows
	}
	return snap, nil
}
func (f *fakeCoverageDB) GetAskDocsDDSnapshotLink(context.Context, db.GetAskDocsDDSnapshotLinkParams) (db.AskDocsDdSnapshot, error) {
	return db.AskDocsDdSnapshot{}, pgx.ErrNoRows
}
func (f *fakeCoverageDB) MarkAskDocsDDSnapshotsStaleForRoom(context.Context, db.MarkAskDocsDDSnapshotsStaleForRoomParams) error {
	return nil
}
func (f *fakeCoverageDB) MarkAllAskDocsDDSnapshotsStaleForRoom(_ context.Context, roomID pgtype.UUID) error {
	for k, snap := range f.snaps {
		snap.Stale = true
		f.snaps[k] = snap
	}
	return nil
}
func (f *fakeCoverageDB) GetAskDocsDDRoomPack(_ context.Context, dealRoomID pgtype.UUID) (db.AskDocsDdRoomPack, error) {
	if f.roomPack == nil || dealRoomID.Bytes != f.room.ID.Bytes {
		return db.AskDocsDdRoomPack{}, pgx.ErrNoRows
	}
	return *f.roomPack, nil
}
func (f *fakeCoverageDB) UpsertAskDocsDDRoomPack(_ context.Context, arg db.UpsertAskDocsDDRoomPackParams) (db.AskDocsDdRoomPack, error) {
	row := db.AskDocsDdRoomPack{
		DealRoomID:   arg.DealRoomID,
		WorkspaceID:  arg.WorkspaceID,
		BasePackID:   arg.BasePackID,
		PackVersion:  arg.PackVersion,
		ForkRevision: arg.ForkRevision,
		Items:        arg.Items,
		UpdatedBy:    arg.UpdatedBy,
		CreatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	f.roomPack = &row
	return row, nil
}
func (f *fakeCoverageDB) DeleteAskDocsDDRoomPack(_ context.Context, arg db.DeleteAskDocsDDRoomPackParams) (int64, error) {
	if f.roomPack == nil || arg.DealRoomID.Bytes != f.room.ID.Bytes {
		return 0, nil
	}
	f.roomPack = nil
	return 1, nil
}
func (f *fakeCoverageDB) UpsertAskDocsDDCrossCheck(_ context.Context, arg db.UpsertAskDocsDDCrossCheckParams) (db.AskDocsDdCrossCheck, error) {
	id := uuid.New()
	if existing, ok := f.cross[arg.PackID]; ok {
		id = uuid.UUID(existing.ID.Bytes)
	}
	row := db.AskDocsDdCrossCheck{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		WorkspaceID: arg.WorkspaceID,
		DealRoomID:  arg.DealRoomID,
		PackID:      arg.PackID,
		PackVersion: arg.PackVersion,
		DocumentAID: arg.DocumentAID,
		DocumentBID: arg.DocumentBID,
		TriggeredBy: arg.TriggeredBy,
		Claims:      arg.Claims,
		CreatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	f.cross[arg.PackID] = row
	return row, nil
}
func (f *fakeCoverageDB) GetAskDocsDDCrossCheckLatest(_ context.Context, arg db.GetAskDocsDDCrossCheckLatestParams) (db.AskDocsDdCrossCheck, error) {
	row, ok := f.cross[arg.PackID]
	if !ok || arg.DealRoomID.Bytes != f.room.ID.Bytes {
		return db.AskDocsDdCrossCheck{}, pgx.ErrNoRows
	}
	return row, nil
}
func (f *fakeCoverageDB) ListDealRoomDocumentsWithMeta(context.Context, pgtype.UUID) ([]db.ListDealRoomDocumentsWithMetaRow, error) {
	return nil, nil
}
func (f *fakeCoverageDB) ListLinkDocumentsByPublicToken(context.Context, string) ([]db.ListLinkDocumentsByPublicTokenRow, error) {
	return nil, nil
}
func (f *fakeCoverageDB) GetDocumentByID(context.Context, db.GetDocumentByIDParams) (db.GetDocumentByIDRow, error) {
	return db.GetDocumentByIDRow{}, pgx.ErrNoRows
}

func TestStartScan_Disabled(t *testing.T) {
	f := newFakeCoverageDB()
	svc := NewService(f, &stubSearcher{}, jobs.MustLoadBuiltinPacks(), &memQueue{}, Options{Enabled: false})
	_, err := svc.StartScan(context.Background(), uuid.UUID(f.room.WorkspaceID.Bytes).String(), uuid.UUID(f.room.ID.Bytes).String(), uuid.NewString(), StartScanRequest{})
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("got %v", err)
	}
}

func TestStartScan_Conflict(t *testing.T) {
	f := newFakeCoverageDB()
	f.active = []db.AskDocsDdRun{{Status: RunQueued}}
	svc := NewService(f, &stubSearcher{}, jobs.MustLoadBuiltinPacks(), &memQueue{}, Options{Enabled: true})
	_, err := svc.StartScan(context.Background(), uuid.UUID(f.room.WorkspaceID.Bytes).String(), uuid.UUID(f.room.ID.Bytes).String(), uuid.NewString(), StartScanRequest{})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("got %v", err)
	}
}

func TestStartScan_QueueUnavailable(t *testing.T) {
	f := newFakeCoverageDB()
	svc := NewService(f, &stubSearcher{}, jobs.MustLoadBuiltinPacks(), nil, Options{Enabled: true})
	_, err := svc.StartScan(context.Background(), uuid.UUID(f.room.WorkspaceID.Bytes).String(), uuid.UUID(f.room.ID.Bytes).String(), uuid.NewString(), StartScanRequest{})
	if !errors.Is(err, ErrQueueUnavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestStartScan_AndExecute(t *testing.T) {
	f := newFakeCoverageDB()
	q := &memQueue{}
	doc := uuid.UUID(f.kb.ActiveDocumentIds[0].Bytes)
	pack := jobs.MustLoadBuiltinPacks()
	p, _ := pack.Get(jobs.FinancingDDV1)
	searcher := &stubSearcher{
		hitsByQuery: map[string][]search.Evidence{
			p.Items[0].QueryFor("en"): {{
				ChunkID: uuid.NewString(), DocumentID: doc.String(), PageNumber: 2,
				Quote: "cap table", Score: 0.8, MatchType: "vector",
			}},
		},
	}
	svc := NewService(f, searcher, pack, q, Options{Enabled: true})
	userID := uuid.NewString()
	run, err := svc.StartScan(context.Background(), uuid.UUID(f.room.WorkspaceID.Bytes).String(), uuid.UUID(f.room.ID.Bytes).String(), userID, StartScanRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != RunQueued || len(q.jobs) != 1 {
		t.Fatalf("run=%+v jobs=%d", run, len(q.jobs))
	}
	if err := svc.ExecuteRun(context.Background(), q.jobs[0]); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetRun(context.Background(), uuid.UUID(f.room.WorkspaceID.Bytes).String(), uuid.UUID(f.room.ID.Bytes).String(), run.ID, userID)
	if err != nil || got.Status != RunSucceeded {
		t.Fatalf("status=%s err=%v", got.Status, err)
	}
	snap, err := svc.GetSnapshot(context.Background(), uuid.UUID(f.room.WorkspaceID.Bytes).String(), uuid.UUID(f.room.ID.Bytes).String(), userID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.CoverageRows) != 20 {
		t.Fatalf("rows=%d", len(snap.CoverageRows))
	}
	if snap.CoverageRows[0].Status != StatusSupported {
		t.Fatalf("row0=%+v", snap.CoverageRows[0])
	}
	var decoded []CoverageRow
	if err := json.Unmarshal(f.snaps[jobs.FinancingDDV1].CoverageRows, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded[0].Status != StatusSupported {
		t.Fatalf("persisted row0=%+v", decoded[0])
	}
}

func TestStartScan_MARedflagPack(t *testing.T) {
	f := newFakeCoverageDB()
	q := &memQueue{}
	svc := NewService(f, &stubSearcher{}, jobs.MustLoadBuiltinPacks(), q, Options{Enabled: true})
	userID := uuid.NewString()
	run, err := svc.StartScan(
		context.Background(),
		uuid.UUID(f.room.WorkspaceID.Bytes).String(),
		uuid.UUID(f.room.ID.Bytes).String(),
		userID,
		StartScanRequest{PackID: jobs.MARedflagV1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if run.PackID != jobs.MARedflagV1 {
		t.Fatalf("pack_id=%s", run.PackID)
	}
	if len(q.jobs) != 1 || q.jobs[0].PackID != jobs.MARedflagV1 {
		t.Fatalf("job=%+v", q.jobs)
	}
	if err := svc.ExecuteRun(context.Background(), q.jobs[0]); err != nil {
		t.Fatal(err)
	}
	snap, err := svc.GetSnapshot(
		context.Background(),
		uuid.UUID(f.room.WorkspaceID.Bytes).String(),
		uuid.UUID(f.room.ID.Bytes).String(),
		userID,
		jobs.MARedflagV1,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.CoverageRows) != 18 {
		t.Fatalf("rows=%d", len(snap.CoverageRows))
	}
}

func TestStartCrossCheck_ConflictAndPersist(t *testing.T) {
	f := newFakeCoverageDB()
	docA := uuid.UUID(f.kb.ActiveDocumentIds[0].Bytes)
	docB := uuid.UUID(f.kb.ActiveDocumentIds[1].Bytes)
	pack := jobs.MustLoadBuiltinPacks()
	p, _ := pack.Get(jobs.MARedflagV1)
	query := p.Items[0].QueryFor("en")
	searcher := &docScopedSearcher{hits: map[string]map[string][]search.Evidence{
		docA.String(): {
			query: {{
				ChunkID: uuid.NewString(), DocumentID: docA.String(), PageNumber: 1,
				Quote: "Seller indemnifies Buyer for all tax liabilities indefinitely.", Score: 0.9,
			}},
		},
		docB.String(): {
			query: {{
				ChunkID: uuid.NewString(), DocumentID: docB.String(), PageNumber: 1,
				Quote: "No indemnity; buyer assumes all closing risk solely.", Score: 0.88,
			}},
		},
	}}
	svc := NewService(f, searcher, pack, &memQueue{}, Options{Enabled: true})
	userID := uuid.NewString()
	view, err := svc.StartCrossCheck(
		context.Background(),
		uuid.UUID(f.room.WorkspaceID.Bytes).String(),
		uuid.UUID(f.room.ID.Bytes).String(),
		userID,
		CrossCheckRequest{
			PackID:      jobs.MARedflagV1,
			DocumentAID: docA.String(),
			DocumentBID: docB.String(),
			Lang:        "en",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if view.PackID != jobs.MARedflagV1 || len(view.Claims) != 18 {
		t.Fatalf("view=%+v", view)
	}
	if view.Claims[0].Status != ClaimConflict {
		t.Fatalf("claim0=%+v", view.Claims[0])
	}
	latest, err := svc.GetLatestCrossCheck(
		context.Background(),
		uuid.UUID(f.room.WorkspaceID.Bytes).String(),
		uuid.UUID(f.room.ID.Bytes).String(),
		userID,
		jobs.MARedflagV1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != view.ID || latest.Claims[0].Status != ClaimConflict {
		t.Fatalf("latest=%+v", latest)
	}
}

func TestStartCrossCheck_DocNotInKB(t *testing.T) {
	f := newFakeCoverageDB()
	svc := NewService(f, &stubSearcher{}, jobs.MustLoadBuiltinPacks(), &memQueue{}, Options{Enabled: true})
	_, err := svc.StartCrossCheck(
		context.Background(),
		uuid.UUID(f.room.WorkspaceID.Bytes).String(),
		uuid.UUID(f.room.ID.Bytes).String(),
		uuid.NewString(),
		CrossCheckRequest{
			DocumentAID: uuid.UUID(f.kb.ActiveDocumentIds[0].Bytes).String(),
			DocumentBID: uuid.NewString(),
		},
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

type docScopedSearcher struct {
	hits map[string]map[string][]search.Evidence
}

func (s *docScopedSearcher) SearchInDocuments(_ context.Context, _ pgtype.UUID, documentIDs []uuid.UUID, query string, _ int, _ ...search.SearchOptions) ([]search.Evidence, error) {
	if len(documentIDs) == 0 {
		return nil, nil
	}
	byQuery, ok := s.hits[documentIDs[0].String()]
	if !ok {
		return nil, nil
	}
	return byQuery[query], nil
}
