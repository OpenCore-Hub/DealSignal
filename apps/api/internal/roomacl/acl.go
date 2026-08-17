// Package roomacl is the single deal-room capability resolver.
// It depends only on db.Queries-shaped methods so dealroom, link, knowledge,
// and workspace can share it without import cycles.
package roomacl

import (
	"context"
	"errors"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth/emailid"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrDenied   = errors.New("room access denied")
	ErrNotAdmin = errors.New("not a room admin")
)

const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleGuest  = "guest"

	wsOwner = "owner"
	wsAdmin = "admin"
)

// Need is a required room capability.
type Need int

const (
	NeedView Need = iota
	NeedContribute
	NeedManage
	NeedDelete
)

// Caps is the caller's effective access to one room.
type Caps struct {
	View        bool
	Contribute  bool
	Manage      bool
	Delete      bool
	Oversight   bool
	RoomRole    string
	MemberEmail string
	// InvitedPending is a room_members row that is not yet active (typically NDA).
	// It is not View: documents, knowledge, and share links stay closed.
	InvitedPending bool
}

// Can reports whether the caller satisfies need.
func (c Caps) Can(need Need) bool {
	switch need {
	case NeedView:
		return c.View
	case NeedContribute:
		return c.Contribute
	case NeedManage:
		return c.Manage
	case NeedDelete:
		return c.Delete
	default:
		return false
	}
}

// Querier is the db surface Resolve needs.
type Querier interface {
	GetWorkspaceMember(ctx context.Context, arg db.GetWorkspaceMemberParams) (db.WorkspaceMember, error)
	GetRoomMemberByUserID(ctx context.Context, arg db.GetRoomMemberByUserIDParams) (db.RoomMember, error)
}

type binder interface {
	BindRoomMembersUserByEmail(ctx context.Context, arg db.BindRoomMembersUserByEmailParams) (int64, error)
}

type userLookup interface {
	GetUserByID(ctx context.Context, id pgtype.UUID) (db.User, error)
}

type roomMemberLister interface {
	ListRoomMembers(ctx context.Context, roomID pgtype.UUID) ([]db.RoomMember, error)
}

// PickMailboxMember returns the pending row for email's mailbox, else active.
// Suspended and other statuses are ignored.
func PickMailboxMember(rows []db.RoomMember, email string) (db.RoomMember, bool) {
	var active db.RoomMember
	var hasActive bool
	for _, m := range rows {
		if !emailid.SameMailbox(m.Email, email) {
			continue
		}
		switch m.Status {
		case "pending":
			return m, true
		case "active":
			active = m
			hasActive = true
		}
	}
	return active, hasActive
}

// Resolve computes caps. Fail-closed: missing workspace membership is empty.
func Resolve(ctx context.Context, q Querier, workspaceID, roomID pgtype.UUID, userID string) (Caps, error) {
	uid, err := parseUser(userID)
	if err != nil {
		return Caps{}, nil
	}

	ws, wsErr := q.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      uid,
	})
	if wsErr != nil && !errors.Is(wsErr, pgx.ErrNoRows) {
		return Caps{}, wsErr
	}

	member, err := q.GetRoomMemberByUserID(ctx, db.GetRoomMemberByUserIDParams{
		RoomID: roomID,
		UserID: uid,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Caps{}, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		// Oversight must not SameMailbox-bind an invitee row (would consume it).
		allowMailbox := wsErr == nil && ws.Role != wsOwner && ws.Role != wsAdmin
		if healed, herr := tryBind(ctx, q, workspaceID, roomID, uid, allowMailbox); herr != nil {
			return Caps{}, herr
		} else if healed != nil {
			member = *healed
			err = nil
		}
	}

	if err == nil && member.Status == "active" {
		return capsForRoomRole(canonicalRole(member.Role), member.Email), nil
	}

	if wsErr == nil && (ws.Role == wsOwner || ws.Role == wsAdmin) {
		return Caps{View: true, Oversight: true}, nil
	}
	if err == nil && member.Status == "pending" {
		return Caps{
			RoomRole:       canonicalRole(member.Role),
			MemberEmail:    member.Email,
			InvitedPending: true,
		}, nil
	}
	return Caps{}, nil
}

// Require resolves and rejects when need is missing.
func Require(ctx context.Context, q Querier, workspaceID, roomID pgtype.UUID, userID string, need Need) (Caps, error) {
	caps, err := Resolve(ctx, q, workspaceID, roomID, userID)
	if err != nil {
		return Caps{}, err
	}
	if !caps.Can(need) {
		if need == NeedView {
			return caps, ErrDenied
		}
		return caps, ErrNotAdmin
	}
	return caps, nil
}

func tryBind(ctx context.Context, q Querier, workspaceID, roomID, uid pgtype.UUID, allowMailbox bool) (*db.RoomMember, error) {
	b, ok := q.(binder)
	u, uok := q.(userLookup)
	if !ok || !uok {
		return nil, nil
	}
	user, err := u.GetUserByID(ctx, uid)
	if err != nil {
		return nil, nil
	}
	email := strings.TrimSpace(user.Email)
	if email == "" {
		return nil, nil
	}
	for _, key := range emailid.Keys(email) {
		if _, err := b.BindRoomMembersUserByEmail(ctx, db.BindRoomMembersUserByEmailParams{
			WorkspaceID: workspaceID,
			Email:       key,
			UserID:      uid,
		}); err != nil {
			return nil, nil
		}
	}
	if member, ok, err := roomMemberByUser(ctx, q, roomID, uid); err != nil {
		return nil, err
	} else if ok {
		return &member, nil
	}
	if !allowMailbox {
		return nil, nil
	}
	lister, ok := q.(roomMemberLister)
	if !ok {
		return nil, nil
	}
	rows, err := lister.ListRoomMembers(ctx, roomID)
	if err != nil {
		return nil, nil
	}
	unbound := make([]db.RoomMember, 0, len(rows))
	for _, m := range rows {
		if !m.UserID.Valid {
			unbound = append(unbound, m)
		}
	}
	match, ok := PickMailboxMember(unbound, email)
	if !ok {
		return nil, nil
	}
	if _, err := b.BindRoomMembersUserByEmail(ctx, db.BindRoomMembersUserByEmailParams{
		WorkspaceID: workspaceID,
		Email:       match.Email,
		UserID:      uid,
	}); err != nil {
		return nil, nil
	}
	if member, found, err := roomMemberByUser(ctx, q, roomID, uid); err != nil {
		return nil, err
	} else if found {
		return &member, nil
	}
	return nil, nil
}

func roomMemberByUser(ctx context.Context, q Querier, roomID, uid pgtype.UUID) (db.RoomMember, bool, error) {
	member, err := q.GetRoomMemberByUserID(ctx, db.GetRoomMemberByUserIDParams{
		RoomID: roomID,
		UserID: uid,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.RoomMember{}, false, nil
		}
		return db.RoomMember{}, false, err
	}
	return member, true, nil
}

func capsForRoomRole(role, email string) Caps {
	c := Caps{View: true, RoomRole: role, MemberEmail: email}
	switch role {
	case RoleOwner, RoleAdmin:
		c.Contribute = true
		c.Manage = true
		c.Delete = true
	case RoleMember:
		c.Contribute = true
	}
	return c
}

// CanonicalRole maps legacy aliases to the grantable/inherited room roles.
func CanonicalRole(role string) string {
	return canonicalRole(role)
}

func canonicalRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleOwner:
		return RoleOwner
	case RoleAdmin:
		return RoleAdmin
	case RoleMember, "contributor":
		return RoleMember
	case RoleGuest, "viewer", "":
		return RoleGuest
	default:
		return ""
	}
}

// GrantableRole accepts invite/update input. Owner is never grantable.
func GrantableRole(role string) string {
	r := canonicalRole(role)
	if r == RoleOwner || r == "" {
		return ""
	}
	if r == RoleGuest && strings.TrimSpace(role) == "" {
		return RoleGuest
	}
	return r
}

func parseUser(userID string) (pgtype.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}
