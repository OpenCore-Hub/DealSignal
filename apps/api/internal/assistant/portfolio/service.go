package portfolio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	// ErrDisabled is returned when ASK_DOCS_PORTFOLIO is off.
	ErrDisabled = errors.New("portfolio disabled")
	// ErrForbidden is returned when the caller is not a workspace admin/owner.
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound is returned when a view or room is missing.
	ErrNotFound = errors.New("not found")
	// ErrInvalidInput is returned for malformed requests.
	ErrInvalidInput = errors.New("invalid input")
	// ErrQuotaExceeded is returned when soft quotas are hit.
	ErrQuotaExceeded = errors.New("quota exceeded")
)

// Querier is the DB surface used by portfolio.
type Querier interface {
	GetWorkspaceMember(ctx context.Context, arg db.GetWorkspaceMemberParams) (db.WorkspaceMember, error)
	ListDealRoomsByIDs(ctx context.Context, arg db.ListDealRoomsByIDsParams) ([]db.DealRoom, error)
	ListAskDocsDDSnapshotsForRooms(ctx context.Context, arg db.ListAskDocsDDSnapshotsForRoomsParams) ([]db.AskDocsDdSnapshot, error)
	CreateAskDocsPortfolioView(ctx context.Context, arg db.CreateAskDocsPortfolioViewParams) (db.AskDocsPortfolioView, error)
	GetAskDocsPortfolioView(ctx context.Context, arg db.GetAskDocsPortfolioViewParams) (db.AskDocsPortfolioView, error)
	ListAskDocsPortfolioViews(ctx context.Context, workspaceID pgtype.UUID) ([]db.AskDocsPortfolioView, error)
	CountAskDocsPortfolioViews(ctx context.Context, workspaceID pgtype.UUID) (int32, error)
	UpdateAskDocsPortfolioView(ctx context.Context, arg db.UpdateAskDocsPortfolioViewParams) (db.AskDocsPortfolioView, error)
	DeleteAskDocsPortfolioView(ctx context.Context, arg db.DeleteAskDocsPortfolioViewParams) (int64, error)
	ListAskDocsPortfolioViewRooms(ctx context.Context, viewID pgtype.UUID) ([]db.ListAskDocsPortfolioViewRoomsRow, error)
	DeleteAskDocsPortfolioViewRooms(ctx context.Context, viewID pgtype.UUID) error
	InsertAskDocsPortfolioViewRoom(ctx context.Context, arg db.InsertAskDocsPortfolioViewRoomParams) error
}

// Service implements portfolio view CRUD + snapshot aggregation.
type Service struct {
	queries Querier
	opts    Options
}

// NewService creates a portfolio service.
func NewService(q Querier, opts Options) *Service {
	return &Service{queries: q, opts: opts}
}

// CreateViewRequest is the Owner API body for creating a portfolio view.
type CreateViewRequest struct {
	Name    string   `json:"name"`
	PackID  string   `json:"pack_id,omitempty"`
	RoomIDs []string `json:"room_ids"`
}

// UpdateViewRequest is the Owner API body for updating a portfolio view.
type UpdateViewRequest struct {
	Name    *string  `json:"name,omitempty"`
	PackID  *string  `json:"pack_id,omitempty"`
	RoomIDs []string `json:"room_ids,omitempty"`
}

// AbsentItem is a top absent checklist item for summary display.
type AbsentItem struct {
	ItemID string `json:"item_id"`
	Label  string `json:"label"`
}

// RoomSummary is the read-only coverage snapshot summary for one room.
type RoomSummary struct {
	DealRoomID   string       `json:"deal_room_id"`
	DealRoomName string       `json:"deal_room_name"`
	HasSnapshot  bool         `json:"has_snapshot"`
	Stale        bool         `json:"stale,omitempty"`
	Supported    int          `json:"supported"`
	Absent       int          `json:"absent"`
	Insufficient int          `json:"insufficient"`
	Total        int          `json:"total"`
	TopAbsent    []AbsentItem `json:"top_absent,omitempty"`
	UpdatedAt    *time.Time   `json:"updated_at,omitempty"`
}

// ViewSummary is a list-row projection without room breakdown.
type ViewSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	PackID    string    `json:"pack_id"`
	RoomCount int       `json:"room_count"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ViewDetail is the Owner-facing portfolio view with room snapshot summaries.
type ViewDetail struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	PackID    string        `json:"pack_id"`
	CreatedBy string        `json:"created_by"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Rooms     []RoomSummary `json:"rooms"`
}

type coverageRowLite struct {
	ItemID string `json:"item_id"`
	Label  string `json:"label"`
	Status string `json:"status"`
}

// CreateView creates a portfolio view (workspace admin/owner only).
func (s *Service) CreateView(ctx context.Context, workspaceID, userID string, req CreateViewRequest) (ViewDetail, error) {
	if !s.opts.Enabled {
		return ViewDetail{}, ErrDisabled
	}
	ws, uid, err := s.requireAdmin(ctx, workspaceID, userID)
	if err != nil {
		return ViewDetail{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ViewDetail{}, fmt.Errorf("%w: name required", ErrInvalidInput)
	}
	packID, err := normalizePackID(req.PackID)
	if err != nil {
		return ViewDetail{}, err
	}
	roomIDs, err := s.validateRoomIDs(ctx, ws, req.RoomIDs)
	if err != nil {
		return ViewDetail{}, err
	}
	count, err := s.queries.CountAskDocsPortfolioViews(ctx, ws)
	if err != nil {
		return ViewDetail{}, err
	}
	if int(count) >= s.opts.MaxViews {
		return ViewDetail{}, fmt.Errorf("%w: max views is %d", ErrQuotaExceeded, s.opts.MaxViews)
	}
	row, err := s.queries.CreateAskDocsPortfolioView(ctx, db.CreateAskDocsPortfolioViewParams{
		WorkspaceID: ws,
		Name:        name,
		PackID:      packID,
		CreatedBy:   uid,
	})
	if err != nil {
		return ViewDetail{}, err
	}
	if err := s.replaceRooms(ctx, row.ID, roomIDs); err != nil {
		_, _ = s.queries.DeleteAskDocsPortfolioView(ctx, db.DeleteAskDocsPortfolioViewParams{
			ID:          row.ID,
			WorkspaceID: ws,
		})
		return ViewDetail{}, err
	}
	return s.buildDetail(ctx, row)
}

// ListViews lists portfolio views for the workspace.
func (s *Service) ListViews(ctx context.Context, workspaceID, userID string) ([]ViewSummary, error) {
	if !s.opts.Enabled {
		return nil, ErrDisabled
	}
	ws, _, err := s.requireAdmin(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListAskDocsPortfolioViews(ctx, ws)
	if err != nil {
		return nil, err
	}
	out := make([]ViewSummary, 0, len(rows))
	for _, row := range rows {
		rooms, err := s.queries.ListAskDocsPortfolioViewRooms(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, ViewSummary{
			ID:        uuid.UUID(row.ID.Bytes).String(),
			Name:      row.Name,
			PackID:    row.PackID,
			RoomCount: len(rooms),
			CreatedBy: uuid.UUID(row.CreatedBy.Bytes).String(),
			CreatedAt: row.CreatedAt.Time,
			UpdatedAt: row.UpdatedAt.Time,
		})
	}
	return out, nil
}

// GetView returns a portfolio view with room snapshot summaries.
func (s *Service) GetView(ctx context.Context, workspaceID, viewID, userID string) (ViewDetail, error) {
	if !s.opts.Enabled {
		return ViewDetail{}, ErrDisabled
	}
	ws, _, err := s.requireAdmin(ctx, workspaceID, userID)
	if err != nil {
		return ViewDetail{}, err
	}
	vid, err := uuid.Parse(viewID)
	if err != nil {
		return ViewDetail{}, ErrNotFound
	}
	row, err := s.queries.GetAskDocsPortfolioView(ctx, db.GetAskDocsPortfolioViewParams{
		ID:          pgtype.UUID{Bytes: vid, Valid: true},
		WorkspaceID: ws,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ViewDetail{}, ErrNotFound
		}
		return ViewDetail{}, err
	}
	return s.buildDetail(ctx, row)
}

// UpdateView updates name/pack/rooms for a portfolio view.
func (s *Service) UpdateView(ctx context.Context, workspaceID, viewID, userID string, req UpdateViewRequest) (ViewDetail, error) {
	if !s.opts.Enabled {
		return ViewDetail{}, ErrDisabled
	}
	ws, _, err := s.requireAdmin(ctx, workspaceID, userID)
	if err != nil {
		return ViewDetail{}, err
	}
	vid, err := uuid.Parse(viewID)
	if err != nil {
		return ViewDetail{}, ErrNotFound
	}
	existing, err := s.queries.GetAskDocsPortfolioView(ctx, db.GetAskDocsPortfolioViewParams{
		ID:          pgtype.UUID{Bytes: vid, Valid: true},
		WorkspaceID: ws,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ViewDetail{}, ErrNotFound
		}
		return ViewDetail{}, err
	}
	name := existing.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			return ViewDetail{}, fmt.Errorf("%w: name required", ErrInvalidInput)
		}
	}
	packID := existing.PackID
	if req.PackID != nil {
		packID, err = normalizePackID(*req.PackID)
		if err != nil {
			return ViewDetail{}, err
		}
	}
	row, err := s.queries.UpdateAskDocsPortfolioView(ctx, db.UpdateAskDocsPortfolioViewParams{
		ID:          existing.ID,
		WorkspaceID: ws,
		Name:        name,
		PackID:      packID,
	})
	if err != nil {
		return ViewDetail{}, err
	}
	if req.RoomIDs != nil {
		roomIDs, err := s.validateRoomIDs(ctx, ws, req.RoomIDs)
		if err != nil {
			return ViewDetail{}, err
		}
		if err := s.replaceRooms(ctx, row.ID, roomIDs); err != nil {
			return ViewDetail{}, err
		}
	}
	return s.buildDetail(ctx, row)
}

// DeleteView deletes a portfolio view.
func (s *Service) DeleteView(ctx context.Context, workspaceID, viewID, userID string) error {
	if !s.opts.Enabled {
		return ErrDisabled
	}
	ws, _, err := s.requireAdmin(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	vid, err := uuid.Parse(viewID)
	if err != nil {
		return ErrNotFound
	}
	n, err := s.queries.DeleteAskDocsPortfolioView(ctx, db.DeleteAskDocsPortfolioViewParams{
		ID:          pgtype.UUID{Bytes: vid, Valid: true},
		WorkspaceID: ws,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) requireAdmin(ctx context.Context, workspaceID, userID string) (pgtype.UUID, pgtype.UUID, error) {
	wid, err := uuid.Parse(workspaceID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, ErrNotFound
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, ErrForbidden
	}
	ws := pgtype.UUID{Bytes: wid, Valid: true}
	user := pgtype.UUID{Bytes: uid, Valid: true}
	member, err := s.queries.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: ws,
		UserID:      user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, pgtype.UUID{}, ErrForbidden
		}
		return pgtype.UUID{}, pgtype.UUID{}, err
	}
	if member.Role != "owner" && member.Role != "admin" {
		return pgtype.UUID{}, pgtype.UUID{}, ErrForbidden
	}
	return ws, user, nil
}

func normalizePackID(packID string) (string, error) {
	id := strings.TrimSpace(packID)
	if id == "" {
		id = jobs.FinancingDDV1
	}
	if !jobs.IsBuiltinPackID(id) {
		return "", fmt.Errorf("%w: unknown pack_id", ErrInvalidInput)
	}
	return id, nil
}

func (s *Service) validateRoomIDs(ctx context.Context, workspaceID pgtype.UUID, raw []string) ([]uuid.UUID, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: room_ids required", ErrInvalidInput)
	}
	if len(raw) > s.opts.MaxRooms {
		return nil, fmt.Errorf("%w: max rooms per view is %d", ErrQuotaExceeded, s.opts.MaxRooms)
	}
	ids := make([]uuid.UUID, 0, len(raw))
	seen := make(map[uuid.UUID]struct{}, len(raw))
	for _, r := range raw {
		id, err := uuid.Parse(strings.TrimSpace(r))
		if err != nil {
			return nil, fmt.Errorf("%w: room_id", ErrInvalidInput)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: room_ids required", ErrInvalidInput)
	}
	rooms, err := s.queries.ListDealRoomsByIDs(ctx, db.ListDealRoomsByIDsParams{
		WorkspaceID: workspaceID,
		Ids:         toPgUUIDs(ids),
	})
	if err != nil {
		return nil, err
	}
	if len(rooms) != len(ids) {
		return nil, fmt.Errorf("%w: one or more rooms not in workspace", ErrInvalidInput)
	}
	return ids, nil
}

func toPgUUIDs(ids []uuid.UUID) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		out = append(out, pgtype.UUID{Bytes: id, Valid: true})
	}
	return out
}

func (s *Service) replaceRooms(ctx context.Context, viewID pgtype.UUID, roomIDs []uuid.UUID) error {
	if err := s.queries.DeleteAskDocsPortfolioViewRooms(ctx, viewID); err != nil {
		return err
	}
	for i, id := range roomIDs {
		if err := s.queries.InsertAskDocsPortfolioViewRoom(ctx, db.InsertAskDocsPortfolioViewRoomParams{
			ViewID:     viewID,
			DealRoomID: pgtype.UUID{Bytes: id, Valid: true},
			SortOrder:  int32(i),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) buildDetail(ctx context.Context, row db.AskDocsPortfolioView) (ViewDetail, error) {
	roomRows, err := s.queries.ListAskDocsPortfolioViewRooms(ctx, row.ID)
	if err != nil {
		return ViewDetail{}, err
	}
	roomIDs := make([]uuid.UUID, 0, len(roomRows))
	for _, r := range roomRows {
		roomIDs = append(roomIDs, uuid.UUID(r.DealRoomID.Bytes))
	}
	nameByID := map[uuid.UUID]string{}
	if len(roomIDs) > 0 {
		rooms, err := s.queries.ListDealRoomsByIDs(ctx, db.ListDealRoomsByIDsParams{
			WorkspaceID: row.WorkspaceID,
			Ids:         toPgUUIDs(roomIDs),
		})
		if err != nil {
			return ViewDetail{}, err
		}
		for _, room := range rooms {
			nameByID[uuid.UUID(room.ID.Bytes)] = room.Name
		}
	}
	snapByRoom := map[uuid.UUID]db.AskDocsDdSnapshot{}
	if len(roomIDs) > 0 {
		snaps, err := s.queries.ListAskDocsDDSnapshotsForRooms(ctx, db.ListAskDocsDDSnapshotsForRoomsParams{
			WorkspaceID: row.WorkspaceID,
			PackID:      row.PackID,
			DealRoomIds: toPgUUIDs(roomIDs),
		})
		if err != nil {
			return ViewDetail{}, err
		}
		for _, snap := range snaps {
			snapByRoom[uuid.UUID(snap.DealRoomID.Bytes)] = snap
		}
	}
	summaries := make([]RoomSummary, 0, len(roomIDs))
	for _, id := range roomIDs {
		sum := RoomSummary{
			DealRoomID:   id.String(),
			DealRoomName: nameByID[id],
			TopAbsent:    []AbsentItem{},
		}
		if sum.DealRoomName == "" {
			sum.DealRoomName = id.String()
		}
		if snap, ok := snapByRoom[id]; ok {
			sum.HasSnapshot = true
			sum.Stale = snap.Stale
			t := snap.UpdatedAt.Time
			sum.UpdatedAt = &t
			applySnapshotCounts(&sum, snap.CoverageRows)
		}
		summaries = append(summaries, sum)
	}
	return ViewDetail{
		ID:        uuid.UUID(row.ID.Bytes).String(),
		Name:      row.Name,
		PackID:    row.PackID,
		CreatedBy: uuid.UUID(row.CreatedBy.Bytes).String(),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
		Rooms:     summaries,
	}, nil
}

func applySnapshotCounts(sum *RoomSummary, raw []byte) {
	rows := []coverageRowLite{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rows)
	}
	const topN = 5
	for _, r := range rows {
		sum.Total++
		switch r.Status {
		case "supported":
			sum.Supported++
		case "absent_in_scope":
			sum.Absent++
			if len(sum.TopAbsent) < topN {
				sum.TopAbsent = append(sum.TopAbsent, AbsentItem{
					ItemID: r.ItemID,
					Label:  r.Label,
				})
			}
		case "insufficient":
			sum.Insufficient++
		}
	}
}
