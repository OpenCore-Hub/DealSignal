package dealroom

import (
	"context"
	"errors"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth/emailid"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	RoomInviteStatusPending = "pending"
	RoomInviteStatusUsed    = "used"
)

// RoomInvitePreview is the public view of a data-room invite token.
type RoomInvitePreview struct {
	Email         string `json:"email"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceSlug string `json:"workspace_slug"`
	WorkspaceName string `json:"workspace_name"`
	RoomID        string `json:"room_id"`
	RoomName      string `json:"room_name"`
}

// RoomInviteAcceptResult is returned after a successful room-invite accept.
type RoomInviteAcceptResult struct {
	UserID        string `json:"user_id"`
	Role          string `json:"role"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceSlug string `json:"workspace_slug"`
	WorkspaceName string `json:"workspace_name"`
	RoomID        string `json:"room_id"`
	RoomName      string `json:"room_name"`
	MemberStatus  string `json:"member_status"`
}

func (s *Service) inviteTokenSecret() string {
	if s == nil || s.cfg == nil {
		return ""
	}
	if k := strings.TrimSpace(s.cfg.InviteTokenHashKey); k != "" {
		return k
	}
	return strings.TrimSpace(s.cfg.JWTSecret)
}

func (s *Service) lookupRoomInvite(ctx context.Context, token string) (db.RoomMember, db.DealRoom, db.Workspace, error) {
	secret := s.inviteTokenSecret()
	roomID, mac, err := parseRoomInviteToken(token)
	if err != nil || secret == "" {
		return db.RoomMember{}, db.DealRoom{}, db.Workspace{}, ErrRoomInviteNotFound
	}
	members, err := s.queries.ListRoomMembers(ctx, pgUUID(roomID.String()))
	if err != nil {
		return db.RoomMember{}, db.DealRoom{}, db.Workspace{}, err
	}
	member, ok := matchRoomInviteMember(secret, roomID, mac, members)
	if !ok {
		return db.RoomMember{}, db.DealRoom{}, db.Workspace{}, ErrRoomInviteNotFound
	}
	wsID := uuid.UUID(member.WorkspaceID.Bytes).String()
	room, err := s.GetRoom(ctx, roomID.String(), wsID)
	if err != nil {
		if errors.Is(err, ErrRoomNotFound) {
			return db.RoomMember{}, db.DealRoom{}, db.Workspace{}, ErrRoomInviteNotFound
		}
		return db.RoomMember{}, db.DealRoom{}, db.Workspace{}, err
	}
	ws, err := s.queries.GetWorkspaceByID(ctx, room.WorkspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.RoomMember{}, db.DealRoom{}, db.Workspace{}, ErrRoomInviteNotFound
		}
		return db.RoomMember{}, db.DealRoom{}, db.Workspace{}, err
	}
	return member, room, ws, nil
}

func roomInvitePreviewOf(member db.RoomMember, room db.DealRoom, ws db.Workspace) RoomInvitePreview {
	status := RoomInviteStatusPending
	if member.UserID.Valid {
		status = RoomInviteStatusUsed
	}
	return RoomInvitePreview{
		Email:         strings.TrimSpace(member.Email),
		Role:          member.Role,
		Status:        status,
		WorkspaceID:   uuid.UUID(ws.ID.Bytes).String(),
		WorkspaceSlug: ws.Slug,
		WorkspaceName: ws.Name,
		RoomID:        uuid.UUID(room.ID.Bytes).String(),
		RoomName:      room.Name,
	}
}

func roomInviteAcceptResult(userID string, member db.RoomMember, room db.DealRoom, ws db.Workspace) RoomInviteAcceptResult {
	return RoomInviteAcceptResult{
		UserID:        userID,
		Role:          member.Role,
		WorkspaceID:   uuid.UUID(ws.ID.Bytes).String(),
		WorkspaceSlug: ws.Slug,
		WorkspaceName: ws.Name,
		RoomID:        uuid.UUID(room.ID.Bytes).String(),
		RoomName:      room.Name,
		MemberStatus:  member.Status,
	}
}

// PreviewRoomInvite returns mailbox-locked invite context. Invalid tokens are not found.
func (s *Service) PreviewRoomInvite(ctx context.Context, token string) (RoomInvitePreview, error) {
	member, room, ws, err := s.lookupRoomInvite(ctx, token)
	if err != nil {
		if errors.Is(err, ErrRoomInviteNotFound) {
			return RoomInvitePreview{}, ErrRoomInviteNotFound
		}
		return RoomInvitePreview{}, err
	}
	return roomInvitePreviewOf(member, room, ws), nil
}

// AcceptRoomInvite binds the signed-in user to the invited mailbox. Matching
// users who are already bound succeed idempotently. NDA remains a room-page gate.
func (s *Service) AcceptRoomInvite(ctx context.Context, token, userID string) (RoomInviteAcceptResult, error) {
	member, room, ws, err := s.lookupRoomInvite(ctx, token)
	if err != nil {
		return RoomInviteAcceptResult{}, err
	}
	uid := pgUUID(userID)
	if !uid.Valid {
		return RoomInviteAcceptResult{}, ErrRoomInviteNotFound
	}
	user, err := s.queries.GetUserByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RoomInviteAcceptResult{}, ErrRoomInviteNotFound
		}
		return RoomInviteAcceptResult{}, err
	}
	if !emailid.SameMailbox(user.Email, member.Email) {
		return RoomInviteAcceptResult{}, ErrRoomInviteEmailMismatch
	}
	if member.UserID.Valid {
		if member.UserID.Bytes != uid.Bytes {
			return RoomInviteAcceptResult{}, ErrRoomInviteUsed
		}
		return roomInviteAcceptResult(userID, member, room, ws), nil
	}

	for _, key := range emailid.Keys(member.Email) {
		if _, bindErr := s.queries.BindRoomMembersUserByEmail(ctx, db.BindRoomMembersUserByEmailParams{
			UserID:      uid,
			WorkspaceID: room.WorkspaceID,
			Email:       key,
		}); bindErr != nil {
			return RoomInviteAcceptResult{}, bindErr
		}
	}
	if _, werr := s.queries.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: room.WorkspaceID,
		UserID:      uid,
	}); errors.Is(werr, pgx.ErrNoRows) {
		if _, addErr := s.queries.AddWorkspaceMember(ctx, db.AddWorkspaceMemberParams{
			WorkspaceID: room.WorkspaceID,
			UserID:      uid,
			Role:        workspaceRoleGuest,
		}); addErr != nil {
			return RoomInviteAcceptResult{}, addErr
		}
	} else if werr != nil {
		return RoomInviteAcceptResult{}, werr
	}

	bound, err := s.queries.GetRoomMemberByUserID(ctx, db.GetRoomMemberByUserIDParams{
		RoomID: room.ID,
		UserID: uid,
	})
	if err != nil {
		return RoomInviteAcceptResult{}, err
	}
	s.invalidateListCache(ctx, uuid.UUID(room.WorkspaceID.Bytes).String())
	return roomInviteAcceptResult(userID, bound, room, ws), nil
}
