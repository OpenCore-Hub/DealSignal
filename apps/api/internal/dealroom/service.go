// Package dealroom implements data-room CRUD, membership, approvals and permissions.
package dealroom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

	ErrRoomNotFound       = errors.New("room not found")
	ErrInvalidSlug        = errors.New("slug must be lowercase alphanumeric with hyphens")
	ErrDuplicateSlug      = errors.New("slug already exists")
	ErrNotRoomAdmin       = errors.New("not a room admin")
	ErrMemberNotFound     = errors.New("member not found")
	ErrRequestNotFound    = errors.New("access request not found")
	ErrNDARequired        = errors.New("nda required")
	ErrApprovalRequired   = errors.New("access not approved")
	ErrFolderAccessDenied = errors.New("folder access denied")
	ErrInvalidEmail       = errors.New("invalid email")
)

// Beginner starts a database transaction.
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Service handles data rooms.
type Service struct {
	queries *db.Queries
	pool    Beginner
}

// NewService creates a deal room service.
func NewService(q *db.Queries, pool Beginner) *Service {
	return &Service{queries: q, pool: pool}
}

// CreateRoomRequest is the input for creating a room.
type CreateRoomRequest struct {
	Slug             string
	Name             string
	Description      string
	TemplateType     string
	Settings         map[string]interface{}
	RequiresNDA      bool
	RequiresApproval bool
}

// CreateRoom creates a data room in a workspace.
func (s *Service) CreateRoom(ctx context.Context, userID, workspaceID string, req CreateRoomRequest) (db.DealRoom, error) {
	if strings.TrimSpace(req.Name) == "" {
		return db.DealRoom{}, errors.New("name is required")
	}
	if !slugRegex.MatchString(req.Slug) {
		return db.DealRoom{}, ErrInvalidSlug
	}

	workspaceUUID := pgUUID(workspaceID)
	userUUID := pgUUID(userID)

	tenant, err := s.getTenantForWorkspace(ctx, workspaceUUID)
	if err != nil {
		return db.DealRoom{}, err
	}

	settings := []byte("{}")
	if len(req.Settings) > 0 {
		settings, _ = json.Marshal(req.Settings)
	}

	room, err := s.queries.CreateDealRoom(ctx, db.CreateDealRoomParams{
		TenantID:         tenant,
		WorkspaceID:      workspaceUUID,
		Slug:             req.Slug,
		Name:             req.Name,
		Description:      pgtype.Text{String: req.Description, Valid: req.Description != ""},
		TemplateType:     pgtype.Text{String: req.TemplateType, Valid: req.TemplateType != ""},
		Settings:         settings,
		RequiresNda:      req.RequiresNDA,
		RequiresApproval: req.RequiresApproval,
		Status:           "active",
		CreatedBy:        userUUID,
	})
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return db.DealRoom{}, ErrDuplicateSlug
		}
		return db.DealRoom{}, fmt.Errorf("create room: %w", err)
	}

	// creator becomes owner
	_, _ = s.queries.AddRoomMember(ctx, db.AddRoomMemberParams{
		TenantID:    tenant,
		WorkspaceID: workspaceUUID,
		RoomID:      room.ID,
		Email:       "",
		UserID:      userUUID,
		Role:        "owner",
		NdaStatus:   ndaStatusFor(room.RequiresNda),
		Status:      "active",
	})
	return room, nil
}

// RoomSummary enriches a deal room with computed aggregates.
type RoomSummary struct {
	Room             db.DealRoom
	DocumentCount    int64
	MemberCount      int64
	PendingApprovals int64
}

// ListRooms returns all rooms in a workspace with computed aggregates.
func (s *Service) ListRooms(ctx context.Context, workspaceID string) ([]RoomSummary, error) {
	wsUUID := pgUUID(workspaceID)
	rooms, err := s.queries.ListDealRoomsByWorkspace(ctx, wsUUID)
	if err != nil {
		return nil, err
	}

	out := make([]RoomSummary, len(rooms))
	for i, room := range rooms {
		docs, _ := s.queries.ListDealRoomDocuments(ctx, room.ID)
		members, _ := s.queries.ListRoomMembers(ctx, room.ID)
		requests, _ := s.queries.ListAccessRequestsByRoom(ctx, room.ID)

		var pending int64
		for _, r := range requests {
			if r.Status == "pending" {
				pending++
			}
		}

		out[i] = RoomSummary{
			Room:             room,
			DocumentCount:    int64(len(docs)),
			MemberCount:      int64(len(members)),
			PendingApprovals: pending,
		}
	}
	return out, nil
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

// GetRoomSummary returns a room scoped to a workspace with computed aggregates.
func (s *Service) GetRoomSummary(ctx context.Context, roomID, workspaceID string) (RoomSummary, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return RoomSummary{}, err
	}

	docs, _ := s.queries.ListDealRoomDocuments(ctx, room.ID)
	members, _ := s.queries.ListRoomMembers(ctx, room.ID)
	requests, _ := s.queries.ListAccessRequestsByRoom(ctx, room.ID)

	var pending int64
	for _, r := range requests {
		if r.Status == "pending" {
			pending++
		}
	}

	return RoomSummary{
		Room:             room,
		DocumentCount:    int64(len(docs)),
		MemberCount:      int64(len(members)),
		PendingApprovals: pending,
	}, nil
}

// AddMember adds a member to a room. Only room admins/owners can invite.
func (s *Service) AddMember(ctx context.Context, roomID, workspaceID, inviterUserID, email, role string) (db.RoomMember, error) {
	if _, err := mail.ParseAddress(email); err != nil {
		return db.RoomMember{}, ErrInvalidEmail
	}
	email = strings.ToLower(strings.TrimSpace(email))
	role = normalizeRole(role)
	if role == "" {
		return db.RoomMember{}, errors.New("invalid role")
	}

	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.RoomMember{}, err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, inviterUserID); err != nil {
		return db.RoomMember{}, err
	}

	existing, _ := s.queries.GetRoomMemberByEmail(ctx, db.GetRoomMemberByEmailParams{
		RoomID: room.ID,
		Email:  email,
	})
	if existing.ID.Valid {
		return db.RoomMember{}, errors.New("member already exists")
	}

	return s.queries.AddRoomMember(ctx, db.AddRoomMemberParams{
		TenantID:    room.TenantID,
		WorkspaceID: room.WorkspaceID,
		RoomID:      room.ID,
		Email:       email,
		Role:        role,
		NdaStatus:   ndaStatusFor(room.RequiresNda),
		Status:      "active",
	})
}

// CreateAccessRequest creates a public access request for a room.
func (s *Service) CreateAccessRequest(ctx context.Context, roomSlug, email, reason string) (db.RoomAccessRequest, error) {
	if _, err := mail.ParseAddress(email); err != nil {
		return db.RoomAccessRequest{}, ErrInvalidEmail
	}
	email = strings.ToLower(strings.TrimSpace(email))

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
			Role:        "viewer",
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
		return created, nil
	}

	return s.queries.CreateAccessRequest(ctx, reqParams)
}

// ApproveAccessRequest approves a pending request and activates the member.
func (s *Service) ApproveAccessRequest(ctx context.Context, requestID, roomID, workspaceID, approverUserID string) (db.RoomAccessRequest, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.RoomAccessRequest{}, err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, approverUserID); err != nil {
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
			Role:        "viewer",
			NdaStatus:   ndaStatusFor(room.RequiresNda),
			Status:      memberStatusFor(room.RequiresNda),
		})
		return err
	}); err != nil {
		return db.RoomAccessRequest{}, fmt.Errorf("approve access request: %w", err)
	}

	req.Status = "approved"
	req.ReviewedBy = approverUUID
	return req, nil
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
		return db.DealRoom{}, db.RoomMember{}, ErrNDARequired
	}
	return room, member, nil
}

// RecordNDA records an NDA agreement and updates member status.
func (s *Service) RecordNDA(ctx context.Context, roomSlug, email, ip, ua string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	room, err := s.queries.GetDealRoomBySlug(ctx, roomSlug)
	if err != nil {
		return ErrRoomNotFound
	}
	if err := s.queries.CreateNDAAgreement(ctx, db.CreateNDAAgreementParams{
		RoomID:    room.ID,
		Email:     email,
		Ip:        parseIP(ip),
		UserAgent: pgtype.Text{String: ua, Valid: ua != ""},
	}); err != nil {
		return fmt.Errorf("record nda: %w", err)
	}
	_ = s.queries.UpdateRoomMemberNDA(ctx, db.UpdateRoomMemberNDAParams{
		RoomID: room.ID,
		Email:  email,
	})
	return nil
}

// SetFolderPermission sets a folder permission for a member email.
func (s *Service) SetFolderPermission(ctx context.Context, roomID, workspaceID, adminUserID, email, folderPath, permission string) (db.RoomMemberFolderPermission, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.RoomMemberFolderPermission{}, err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, adminUserID); err != nil {
		return db.RoomMemberFolderPermission{}, err
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

// AddDocument adds a document to a room folder.
func (s *Service) AddDocument(ctx context.Context, roomID, workspaceID, adminUserID, documentID, folderPath string, sortOrder int32) (db.DealRoomDocument, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.DealRoomDocument{}, err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, adminUserID); err != nil {
		return db.DealRoomDocument{}, err
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
	return s.queries.AddDealRoomDocument(ctx, db.AddDealRoomDocumentParams{
		TenantID:    room.TenantID,
		WorkspaceID: room.WorkspaceID,
		RoomID:      room.ID,
		DocumentID:  doc.ID,
		FolderPath:  folderPath,
		SortOrder:   sortOrder,
	})
}

// ListDocuments returns documents in a room that the member can access.
func (s *Service) ListDocuments(ctx context.Context, roomID, email string) ([]db.DealRoomDocument, error) {
	roomUUID := pgUUID(roomID)
	member, err := s.queries.GetRoomMemberByEmail(ctx, db.GetRoomMemberByEmailParams{
		RoomID: roomUUID,
		Email:  email,
	})
	if err != nil || !member.ID.Valid || member.Status != "active" {
		return nil, ErrApprovalRequired
	}

	docs, err := s.queries.ListDealRoomDocuments(ctx, roomUUID)
	if err != nil {
		return nil, err
	}

	out := make([]db.DealRoomDocument, 0, len(docs))
	for _, d := range docs {
		perm, err := s.queries.GetFolderPermission(ctx, db.GetFolderPermissionParams{
			RoomID:     roomUUID,
			Email:      email,
			FolderPath: d.FolderPath,
		})
		if err == nil && perm.Permission == "none" {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

func (s *Service) requireRoomAdmin(ctx context.Context, roomID pgtype.UUID, userID string) error {
	members, err := s.queries.ListRoomMembers(ctx, roomID)
	if err != nil {
		return err
	}
	uid := pgUUID(userID)
	for _, m := range members {
		if m.UserID == uid && (m.Role == "owner" || m.Role == "admin") && m.Status == "active" {
			return nil
		}
	}
	return ErrNotRoomAdmin
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

func normalizeRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "", "viewer":
		return "viewer"
	case "contributor", "admin":
		return role
	case "owner":
		// only creation can assign owner
		return ""
	default:
		return ""
	}
}

func ndaStatusFor(required bool) string {
	if required {
		return "pending"
	}
	return "not_required"
}

func memberStatusFor(requiresApprovalOrNDA bool) string {
	if requiresApprovalOrNDA {
		return "pending"
	}
	return "active"
}

func pgUUID(id string) pgtype.UUID {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}
