package portfolio

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakePortfolioDB struct {
	wsRole   string
	rooms    map[uuid.UUID]db.DealRoom
	views    map[uuid.UUID]db.AskDocsPortfolioView
	viewRooms map[uuid.UUID][]db.ListAskDocsPortfolioViewRoomsRow
	snaps    map[string]db.AskDocsDdSnapshot // roomID|packID
}

func newFakePortfolioDB() *fakePortfolioDB {
	ws := uuid.New()
	r1, r2 := uuid.New(), uuid.New()
	rows, _ := json.Marshal([]coverageRowLite{
		{ItemID: "cap_table", Label: "Cap table", Status: "supported"},
		{ItemID: "option_pool", Label: "Option pool", Status: "absent_in_scope"},
		{ItemID: "nda", Label: "NDA", Status: "absent_in_scope"},
		{ItemID: "financials", Label: "Financials", Status: "insufficient"},
	})
	f := &fakePortfolioDB{
		wsRole: "admin",
		rooms: map[uuid.UUID]db.DealRoom{
			r1: {ID: pgtype.UUID{Bytes: r1, Valid: true}, WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true}, Name: "Room A"},
			r2: {ID: pgtype.UUID{Bytes: r2, Valid: true}, WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true}, Name: "Room B"},
		},
		views:     map[uuid.UUID]db.AskDocsPortfolioView{},
		viewRooms: map[uuid.UUID][]db.ListAskDocsPortfolioViewRoomsRow{},
		snaps:     map[string]db.AskDocsDdSnapshot{},
	}
	f.snaps[r1.String()+"|"+jobs.FinancingDDV1] = db.AskDocsDdSnapshot{
		DealRoomID:   pgtype.UUID{Bytes: r1, Valid: true},
		PackID:       jobs.FinancingDDV1,
		Stale:        false,
		CoverageRows: rows,
		UpdatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	return f
}

func (f *fakePortfolioDB) workspaceID() pgtype.UUID {
	for _, r := range f.rooms {
		return r.WorkspaceID
	}
	return pgtype.UUID{}
}

func (f *fakePortfolioDB) GetWorkspaceMember(context.Context, db.GetWorkspaceMemberParams) (db.WorkspaceMember, error) {
	return db.WorkspaceMember{Role: f.wsRole}, nil
}
func (f *fakePortfolioDB) ListDealRoomsByIDs(_ context.Context, arg db.ListDealRoomsByIDsParams) ([]db.DealRoom, error) {
	out := make([]db.DealRoom, 0, len(arg.Ids))
	for _, id := range arg.Ids {
		room, ok := f.rooms[uuid.UUID(id.Bytes)]
		if !ok || room.WorkspaceID.Bytes != arg.WorkspaceID.Bytes {
			continue
		}
		out = append(out, room)
	}
	return out, nil
}
func (f *fakePortfolioDB) ListAskDocsDDSnapshotsForRooms(_ context.Context, arg db.ListAskDocsDDSnapshotsForRoomsParams) ([]db.AskDocsDdSnapshot, error) {
	out := make([]db.AskDocsDdSnapshot, 0)
	for _, id := range arg.DealRoomIds {
		key := uuid.UUID(id.Bytes).String() + "|" + arg.PackID
		if snap, ok := f.snaps[key]; ok {
			out = append(out, snap)
		}
	}
	return out, nil
}
func (f *fakePortfolioDB) CreateAskDocsPortfolioView(_ context.Context, arg db.CreateAskDocsPortfolioViewParams) (db.AskDocsPortfolioView, error) {
	id := uuid.New()
	row := db.AskDocsPortfolioView{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		WorkspaceID: arg.WorkspaceID,
		Name:        arg.Name,
		PackID:      arg.PackID,
		CreatedBy:   arg.CreatedBy,
		CreatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	f.views[id] = row
	return row, nil
}
func (f *fakePortfolioDB) GetAskDocsPortfolioView(_ context.Context, arg db.GetAskDocsPortfolioViewParams) (db.AskDocsPortfolioView, error) {
	row, ok := f.views[uuid.UUID(arg.ID.Bytes)]
	if !ok || row.WorkspaceID.Bytes != arg.WorkspaceID.Bytes {
		return db.AskDocsPortfolioView{}, pgx.ErrNoRows
	}
	return row, nil
}
func (f *fakePortfolioDB) ListAskDocsPortfolioViews(_ context.Context, workspaceID pgtype.UUID) ([]db.AskDocsPortfolioView, error) {
	out := make([]db.AskDocsPortfolioView, 0)
	for _, v := range f.views {
		if v.WorkspaceID.Bytes == workspaceID.Bytes {
			out = append(out, v)
		}
	}
	return out, nil
}
func (f *fakePortfolioDB) CountAskDocsPortfolioViews(_ context.Context, workspaceID pgtype.UUID) (int32, error) {
	var n int32
	for _, v := range f.views {
		if v.WorkspaceID.Bytes == workspaceID.Bytes {
			n++
		}
	}
	return n, nil
}
func (f *fakePortfolioDB) UpdateAskDocsPortfolioView(_ context.Context, arg db.UpdateAskDocsPortfolioViewParams) (db.AskDocsPortfolioView, error) {
	row, ok := f.views[uuid.UUID(arg.ID.Bytes)]
	if !ok {
		return db.AskDocsPortfolioView{}, pgx.ErrNoRows
	}
	row.Name = arg.Name
	row.PackID = arg.PackID
	row.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	f.views[uuid.UUID(arg.ID.Bytes)] = row
	return row, nil
}
func (f *fakePortfolioDB) DeleteAskDocsPortfolioView(_ context.Context, arg db.DeleteAskDocsPortfolioViewParams) (int64, error) {
	id := uuid.UUID(arg.ID.Bytes)
	row, ok := f.views[id]
	if !ok || row.WorkspaceID.Bytes != arg.WorkspaceID.Bytes {
		return 0, nil
	}
	delete(f.views, id)
	delete(f.viewRooms, id)
	return 1, nil
}
func (f *fakePortfolioDB) ListAskDocsPortfolioViewRooms(_ context.Context, viewID pgtype.UUID) ([]db.ListAskDocsPortfolioViewRoomsRow, error) {
	return f.viewRooms[uuid.UUID(viewID.Bytes)], nil
}
func (f *fakePortfolioDB) DeleteAskDocsPortfolioViewRooms(_ context.Context, viewID pgtype.UUID) error {
	f.viewRooms[uuid.UUID(viewID.Bytes)] = nil
	return nil
}
func (f *fakePortfolioDB) InsertAskDocsPortfolioViewRoom(_ context.Context, arg db.InsertAskDocsPortfolioViewRoomParams) error {
	id := uuid.UUID(arg.ViewID.Bytes)
	f.viewRooms[id] = append(f.viewRooms[id], db.ListAskDocsPortfolioViewRoomsRow{
		DealRoomID: arg.DealRoomID,
		SortOrder:  arg.SortOrder,
	})
	return nil
}

func TestOptionsFromEnv_ProdDefaultOff(t *testing.T) {
	t.Setenv("ASK_DOCS_PORTFOLIO", "")
	o := OptionsFromEnv("production")
	if o.Enabled {
		t.Fatal("prod must default off")
	}
	o = OptionsFromEnv("development")
	if !o.Enabled {
		t.Fatal("non-prod must default on")
	}
}

func TestCreateView_AggregatesSnapshotOnly(t *testing.T) {
	f := newFakePortfolioDB()
	svc := NewService(f, Options{Enabled: true, MaxViews: 5, MaxRooms: 20})
	ws := uuid.UUID(f.workspaceID().Bytes).String()
	var roomIDs []string
	for id := range f.rooms {
		roomIDs = append(roomIDs, id.String())
	}
	view, err := svc.CreateView(context.Background(), ws, uuid.NewString(), CreateViewRequest{
		Name:    "Series A portfolio",
		RoomIDs: roomIDs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Rooms) != 2 {
		t.Fatalf("rooms=%d", len(view.Rooms))
	}
	var withSnap *RoomSummary
	for i := range view.Rooms {
		if view.Rooms[i].HasSnapshot {
			withSnap = &view.Rooms[i]
		}
	}
	if withSnap == nil {
		t.Fatal("expected one snapshot summary")
	}
	if withSnap.Supported != 1 || withSnap.Absent != 2 || withSnap.Insufficient != 1 {
		t.Fatalf("counts=%+v", withSnap)
	}
	if len(withSnap.TopAbsent) != 2 || withSnap.TopAbsent[0].ItemID != "option_pool" {
		t.Fatalf("top_absent=%+v", withSnap.TopAbsent)
	}
}

func TestCreateView_Disabled(t *testing.T) {
	f := newFakePortfolioDB()
	svc := NewService(f, Options{Enabled: false, MaxViews: 5, MaxRooms: 20})
	_, err := svc.CreateView(context.Background(), uuid.UUID(f.workspaceID().Bytes).String(), uuid.NewString(), CreateViewRequest{
		Name: "x", RoomIDs: []string{uuid.NewString()},
	})
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateView_ForbiddenMember(t *testing.T) {
	f := newFakePortfolioDB()
	f.wsRole = "member"
	svc := NewService(f, Options{Enabled: true, MaxViews: 5, MaxRooms: 20})
	_, err := svc.CreateView(context.Background(), uuid.UUID(f.workspaceID().Bytes).String(), uuid.NewString(), CreateViewRequest{
		Name: "x", RoomIDs: []string{uuid.NewString()},
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateView_Quota(t *testing.T) {
	f := newFakePortfolioDB()
	svc := NewService(f, Options{Enabled: true, MaxViews: 1, MaxRooms: 20})
	ws := uuid.UUID(f.workspaceID().Bytes).String()
	var roomID string
	for id := range f.rooms {
		roomID = id.String()
		break
	}
	_, err := svc.CreateView(context.Background(), ws, uuid.NewString(), CreateViewRequest{Name: "one", RoomIDs: []string{roomID}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreateView(context.Background(), ws, uuid.NewString(), CreateViewRequest{Name: "two", RoomIDs: []string{roomID}})
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("got %v", err)
	}
}
