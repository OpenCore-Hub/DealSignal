package link

import (
	"context"
	"errors"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/roomacl"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrAskHostForbidden is returned when the caller cannot view or answer Ask Host.
var ErrAskHostForbidden = errors.New("ask host forbidden")

// ErrLinkMutateForbidden is returned when the caller cannot edit, enable, or archive a link.
var ErrLinkMutateForbidden = errors.New("link mutate forbidden")

// ErrLinkViewForbidden is returned when the caller cannot view a deal-room link.
var ErrLinkViewForbidden = errors.New("link view forbidden")

// authorizeAskHostOwnerView allows room owner/admin on deal-room links.
// Document share links (no deal room) keep workspace owner/admin.
func authorizeAskHostOwnerView(ctx context.Context, q roomacl.Querier, workspaceID, dealRoomID pgtype.UUID, userID string) error {
	if !dealRoomID.Valid {
		return authorizeDocumentLinkAskHost(ctx, q, workspaceID, userID)
	}
	if _, err := roomacl.Require(ctx, q, workspaceID, dealRoomID, userID, roomacl.NeedManage); err != nil {
		if errors.Is(err, roomacl.ErrDenied) || errors.Is(err, roomacl.ErrNotAdmin) {
			return ErrAskHostForbidden
		}
		return err
	}
	return nil
}

func authorizeDocumentLinkAskHost(ctx context.Context, q roomacl.Querier, workspaceID pgtype.UUID, userID string) error {
	id, err := uuid.Parse(userID)
	if err != nil {
		return ErrAskHostForbidden
	}
	ws, err := q.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      pgtype.UUID{Bytes: id, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAskHostForbidden
		}
		return err
	}
	if ws.Role != "owner" && ws.Role != "admin" {
		return ErrAskHostForbidden
	}
	return nil
}

// authorizeLinkMutate gates edit / enable / archive / delete.
// Deal-room links require NeedManage (oversight cannot mutate).
// Document share links keep workspace write: RBAC already requires
// owner|admin|member, except guest pass-through on /links/:id which we reject here.
func authorizeLinkMutate(ctx context.Context, q roomacl.Querier, workspaceID, dealRoomID pgtype.UUID, userID, workspaceRole string) error {
	if dealRoomID.Valid {
		if _, err := roomacl.Require(ctx, q, workspaceID, dealRoomID, userID, roomacl.NeedManage); err != nil {
			if errors.Is(err, roomacl.ErrDenied) || errors.Is(err, roomacl.ErrNotAdmin) {
				return ErrLinkMutateForbidden
			}
			return err
		}
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(workspaceRole), "guest") {
		return ErrLinkMutateForbidden
	}
	return nil
}

// authorizeLinkView gates GET of a deal-room share link (tokens, analytics, rules).
// Oversight and room members may view. Document links stay workspace-scoped.
func authorizeLinkView(ctx context.Context, q roomacl.Querier, workspaceID, dealRoomID pgtype.UUID, userID string) error {
	if !dealRoomID.Valid {
		return nil
	}
	if _, err := roomacl.Require(ctx, q, workspaceID, dealRoomID, userID, roomacl.NeedView); err != nil {
		if errors.Is(err, roomacl.ErrDenied) || errors.Is(err, roomacl.ErrNotAdmin) {
			return ErrLinkViewForbidden
		}
		return err
	}
	return nil
}

// canReviewLinkRequests reports whether reviewerID may see/act on applicant PII.
// Deal-room links: NeedManage. Document links stay creator-only.
func canReviewLinkRequests(ctx context.Context, q roomacl.Querier, link db.Link, reviewerID string) bool {
	if link.DealRoomID.Valid {
		return authorizeLinkMutate(ctx, q, link.WorkspaceID, link.DealRoomID, reviewerID, "") == nil
	}
	return link.CreatedBy.Valid && uuid.UUID(link.CreatedBy.Bytes).String() == reviewerID
}
