package link

import (
	"context"
	"errors"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrAskHostForbidden is returned when the caller cannot view or answer Ask Host.
var ErrAskHostForbidden = errors.New("ask host forbidden")

// askHostAuthQuerier is the DB surface for Ask Host owner-inbox authorization.
type askHostAuthQuerier interface {
	GetWorkspaceMember(ctx context.Context, arg db.GetWorkspaceMemberParams) (db.WorkspaceMember, error)
	GetRoomMemberByUserID(ctx context.Context, arg db.GetRoomMemberByUserIDParams) (db.RoomMember, error)
}

// authorizeAskHostOwnerView allows workspace owner/admin or an active deal-room member.
// Mirrors Ask Docs audit / security-events authorization (SPEC US#26–27 / PLAN B10).
func authorizeAskHostOwnerView(ctx context.Context, q askHostAuthQuerier, workspaceID, dealRoomID pgtype.UUID, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrAskHostForbidden
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	ws, err := q.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      userUUID,
	})
	if err == nil && (ws.Role == "owner" || ws.Role == "admin") {
		return nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if dealRoomID.Valid {
		member, err := q.GetRoomMemberByUserID(ctx, db.GetRoomMemberByUserIDParams{
			RoomID: dealRoomID,
			UserID: userUUID,
		})
		if err == nil && member.Status == "active" {
			return nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}

	return ErrAskHostForbidden
}
