// Package dealroom implements data-room CRUD, membership, approvals and permissions.
package dealroom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"sort"
	"strings"
	"time"

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
	ErrFolderNotEmpty     = errors.New("folder is not empty")
	ErrFolderNotFound     = errors.New("folder not found")
	ErrFolderExists       = errors.New("folder already exists")
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
	Slug             string                 `json:"slug"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description,omitempty"`
	TemplateType     string                 `json:"template_type,omitempty"`
	Settings         map[string]interface{} `json:"settings,omitempty"`
	RequiresNDA      bool                   `json:"requires_nda,omitempty"`
	RequiresApproval bool                   `json:"requires_approval,omitempty"`
}

// Folder describes a folder stored in deal_rooms.settings.
type Folder struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	SortOrder   int    `json:"sort_order"`
}

// DocumentOrder is used to reorder documents within a folder.
type DocumentOrder struct {
	DocumentID string `json:"document_id"`
	SortOrder  int32  `json:"sort_order"`
}

// RoomDocument is a deal room document enriched with document metadata.
type RoomDocument struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Title      string    `json:"title"`
	PageCount  int32     `json:"page_count"`
	FileSize   int64     `json:"file_size"`
	SourceType string    `json:"source_type"`
	Status     string    `json:"status"`
	FolderPath string    `json:"folder_path"`
	SortOrder  int32     `json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
}

// FolderDocs groups documents by folder for the room detail response.
type FolderDocs struct {
	Folder     Folder         `json:"folder"`
	Permission string         `json:"permission"`
	Documents  []RoomDocument `json:"documents"`
}

// RoomMemberDetail is a room member with optional user name.
type RoomMemberDetail struct {
	ID          pgtype.UUID        `json:"id"`
	TenantID    pgtype.UUID        `json:"tenant_id"`
	WorkspaceID pgtype.UUID        `json:"workspace_id"`
	RoomID      pgtype.UUID        `json:"room_id"`
	Email       string             `json:"email"`
	UserID      pgtype.UUID        `json:"user_id"`
	Role        string             `json:"role"`
	NdaStatus   string             `json:"nda_status"`
	NdaSignedAt pgtype.Timestamptz `json:"nda_signed_at"`
	Status      string             `json:"status"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
	UpdatedAt   pgtype.Timestamptz `json:"updated_at"`
	UserName    string             `json:"user_name"`
}

// RoomDetail is the full enriched response for a single room.
type RoomDetail struct {
	Room             db.DealRoom
	DocumentCount    int64
	MemberCount      int64
	PendingApprovals int64
	Folders          []Folder
	Documents        []FolderDocs
	Members          []RoomMemberDetail
	AccessRequests   []db.RoomAccessRequest
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

	settings := make(map[string]interface{})
	for k, v := range req.Settings {
		settings[k] = v
	}

	folders := defaultFolders()
	if req.TemplateType != "" {
		if tmplFolders := templateFolders(req.TemplateType); len(tmplFolders) > 0 {
			for _, f := range tmplFolders {
				folders = append(folders, Folder(f))
			}
		}
	}
	settings["folders"] = folders

	settingsBytes, err := json.Marshal(settings)
	if err != nil {
		return db.DealRoom{}, fmt.Errorf("marshal settings: %w", err)
	}

	room, err := s.queries.CreateDealRoom(ctx, db.CreateDealRoomParams{
		TenantID:         tenant,
		WorkspaceID:      workspaceUUID,
		Slug:             req.Slug,
		Name:             req.Name,
		Description:      pgtype.Text{String: req.Description, Valid: req.Description != ""},
		TemplateType:     pgtype.Text{String: req.TemplateType, Valid: req.TemplateType != ""},
		Settings:         settingsBytes,
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

// GetRoomDetail returns a room with folders, documents, members and access requests.
// Folders and documents are visible to any active room member; members and access
// requests are only included for room admins.
func (s *Service) GetRoomDetail(ctx context.Context, roomID, workspaceID, userID string) (RoomDetail, error) {
	summary, err := s.GetRoomSummary(ctx, roomID, workspaceID)
	if err != nil {
		return RoomDetail{}, err
	}

	folders, err := s.ListFolders(ctx, roomID, workspaceID)
	if err != nil {
		return RoomDetail{}, err
	}

	docs, err := s.GetRoomDocuments(ctx, roomID, workspaceID, userID)
	if err != nil {
		return RoomDetail{}, err
	}

	var members []RoomMemberDetail
	var requests []db.RoomAccessRequest
	if err := s.requireRoomAdmin(ctx, summary.Room.ID, userID); err == nil {
		if rows, err := s.queries.ListRoomMembersWithUser(ctx, summary.Room.ID); err == nil {
			members = make([]RoomMemberDetail, len(rows))
			for i, r := range rows {
				members[i] = RoomMemberDetail{
					ID:          r.ID,
					TenantID:    r.TenantID,
					WorkspaceID: r.WorkspaceID,
					RoomID:      r.RoomID,
					Email:       r.Email,
					UserID:      r.UserID,
					Role:        r.Role,
					NdaStatus:   r.NdaStatus,
					NdaSignedAt: r.NdaSignedAt,
					Status:      r.Status,
					CreatedAt:   r.CreatedAt,
					UpdatedAt:   r.UpdatedAt,
					UserName:    r.UserName,
				}
			}
		}
		requests, _ = s.queries.ListAccessRequestsByRoom(ctx, summary.Room.ID)
	}

	return RoomDetail{
		Room:             summary.Room,
		DocumentCount:    summary.DocumentCount,
		MemberCount:      summary.MemberCount,
		PendingApprovals: summary.PendingApprovals,
		Folders:          folders,
		Documents:        docs,
		Members:          members,
		AccessRequests:   requests,
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

// ListMembers returns all members of a room. Only admins can list members.
func (s *Service) ListMembers(ctx context.Context, roomID, workspaceID, userID string) ([]RoomMemberDetail, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, userID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListRoomMembersWithUser(ctx, room.ID)
	if err != nil {
		return nil, err
	}
	out := make([]RoomMemberDetail, len(rows))
	for i, r := range rows {
		out[i] = RoomMemberDetail{
			ID:          r.ID,
			TenantID:    r.TenantID,
			WorkspaceID: r.WorkspaceID,
			RoomID:      r.RoomID,
			Email:       r.Email,
			UserID:      r.UserID,
			Role:        r.Role,
			NdaStatus:   r.NdaStatus,
			NdaSignedAt: r.NdaSignedAt,
			Status:      r.Status,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
			UserName:    r.UserName,
		}
	}
	return out, nil
}

// RemoveMember removes a member from a room. Only admins can remove members.
func (s *Service) RemoveMember(ctx context.Context, roomID, workspaceID, userID, memberID string) error {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, userID); err != nil {
		return err
	}
	member, err := s.queries.GetRoomMemberByID(ctx, db.GetRoomMemberByIDParams{
		ID:     pgUUID(memberID),
		RoomID: room.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return err
	}
	if member.Role == "owner" {
		return errors.New("cannot remove room owner")
	}
	return s.queries.DeleteRoomMember(ctx, db.DeleteRoomMemberParams{
		ID:     member.ID,
		RoomID: room.ID,
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

// RejectAccessRequest rejects a pending access request.
func (s *Service) RejectAccessRequest(ctx context.Context, requestID, roomID, workspaceID, userID string) (db.RoomAccessRequest, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return db.RoomAccessRequest{}, err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, userID); err != nil {
		return db.RoomAccessRequest{}, err
	}

	req, err := s.queries.GetAccessRequestByID(ctx, db.GetAccessRequestByIDParams{
		ID:     pgUUID(requestID),
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

	reviewerUUID := pgUUID(userID)
	if err := s.queries.UpdateAccessRequestStatus(ctx, db.UpdateAccessRequestStatusParams{
		Status:     "rejected",
		ReviewedBy: reviewerUUID,
		ID:         req.ID,
	}); err != nil {
		return db.RoomAccessRequest{}, err
	}
	req.Status = "rejected"
	req.ReviewedBy = reviewerUUID
	return req, nil
}

// ListAccessRequests returns access requests for a room. Only admins can list.
func (s *Service) ListAccessRequests(ctx context.Context, roomID, workspaceID, userID string) ([]db.RoomAccessRequest, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, userID); err != nil {
		return nil, err
	}
	return s.queries.ListAccessRequestsByRoom(ctx, room.ID)
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

// RemoveDocument removes a document from a room. Only admins can remove.
func (s *Service) RemoveDocument(ctx context.Context, roomID, workspaceID, userID, documentID string) error {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, userID); err != nil {
		return err
	}
	id, err := uuid.Parse(documentID)
	if err != nil {
		return errors.New("invalid document id")
	}
	return s.queries.DeleteDealRoomDocument(ctx, db.DeleteDealRoomDocumentParams{
		ID:     pgtype.UUID{Bytes: id, Valid: true},
		RoomID: room.ID,
	})
}

// MoveDocument moves a document to another folder. Only admins can move.
func (s *Service) MoveDocument(ctx context.Context, roomID, workspaceID, userID, documentID, folderPath string, sortOrder *int32) error {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, userID); err != nil {
		return err
	}
	folders, err := s.ListFolders(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if !folderExists(folders, folderPath) {
		return ErrFolderNotFound
	}

	id, err := uuid.Parse(documentID)
	if err != nil {
		return errors.New("invalid document id")
	}
	if err := s.queries.UpdateDealRoomDocumentFolder(ctx, db.UpdateDealRoomDocumentFolderParams{
		FolderPath: folderPath,
		ID:         pgtype.UUID{Bytes: id, Valid: true},
		RoomID:     room.ID,
	}); err != nil {
		return err
	}
	if sortOrder != nil {
		return s.queries.UpdateDealRoomDocumentSortOrder(ctx, db.UpdateDealRoomDocumentSortOrderParams{
			SortOrder: *sortOrder,
			ID:        pgtype.UUID{Bytes: id, Valid: true},
			RoomID:    room.ID,
		})
	}
	return nil
}

// ReorderDocuments updates sort orders for documents in a room. Only admins can reorder.
func (s *Service) ReorderDocuments(ctx context.Context, roomID, workspaceID, userID string, orders []DocumentOrder) error {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, userID); err != nil {
		return err
	}
	return s.runInTx(ctx, func(q *db.Queries) error {
		for _, o := range orders {
			id, err := uuid.Parse(o.DocumentID)
			if err != nil {
				return errors.New("invalid document id")
			}
			if err := q.UpdateDealRoomDocumentSortOrder(ctx, db.UpdateDealRoomDocumentSortOrderParams{
				SortOrder: o.SortOrder,
				ID:        pgtype.UUID{Bytes: id, Valid: true},
				RoomID:    room.ID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListDocuments returns documents in a room that the member can access,
// grouped by folder with the effective permission for each folder.
func (s *Service) ListDocuments(ctx context.Context, roomID, workspaceID, email string) ([]FolderDocs, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	member, err := s.queries.GetRoomMemberByEmail(ctx, db.GetRoomMemberByEmailParams{
		RoomID: room.ID,
		Email:  email,
	})
	if err != nil || !member.ID.Valid || member.Status != "active" {
		return nil, ErrApprovalRequired
	}

	folders, err := s.loadFolders(room)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListDealRoomDocumentsWithMeta(ctx, room.ID)
	if err != nil {
		return nil, err
	}

	docsByFolder := make(map[string][]RoomDocument)
	for _, r := range rows {
		var pageCount int32
		if r.PageCount.Valid {
			pageCount = r.PageCount.Int32
		}
		var fileSize int64
		if r.FileSize.Valid {
			fileSize = r.FileSize.Int64
		}
		d := RoomDocument{
			ID:         uuid.UUID(r.ID.Bytes).String(),
			DocumentID: uuid.UUID(r.DocumentID.Bytes).String(),
			Title:      r.DocumentTitle,
			PageCount:  pageCount,
			FileSize:   fileSize,
			SourceType: r.SourceType,
			Status:     r.Status,
			FolderPath: r.FolderPath,
			SortOrder:  r.SortOrder,
			CreatedAt:  r.CreatedAt.Time,
		}
		docsByFolder[r.FolderPath] = append(docsByFolder[r.FolderPath], d)
	}

	out := make([]FolderDocs, 0, len(folders))
	for _, f := range folders {
		perm, _ := s.GetFolderPermission(ctx, roomID, email, f.Path)
		if perm == "none" {
			continue
		}
		docs := docsByFolder[f.Path]
		if docs == nil {
			docs = []RoomDocument{}
		}
		sort.Slice(docs, func(i, j int) bool {
			if docs[i].SortOrder != docs[j].SortOrder {
				return docs[i].SortOrder < docs[j].SortOrder
			}
			return docs[i].CreatedAt.Before(docs[j].CreatedAt)
		})
		out = append(out, FolderDocs{
			Folder:     f,
			Permission: perm,
			Documents:  docs,
		})
	}
	return out, nil
}

// GetRoomDocuments returns room documents grouped by folder with metadata and the
// current member's effective permission for each folder.
func (s *Service) GetRoomDocuments(ctx context.Context, roomID, workspaceID, userID string) ([]FolderDocs, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	member, err := s.queries.GetRoomMemberByUserID(ctx, db.GetRoomMemberByUserIDParams{
		RoomID: room.ID,
		UserID: pgUUID(userID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrApprovalRequired
		}
		return nil, err
	}
	if member.Status != "active" {
		return nil, ErrApprovalRequired
	}

	folders, err := s.loadFolders(room)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListDealRoomDocumentsWithMeta(ctx, room.ID)
	if err != nil {
		return nil, err
	}

	docsByFolder := make(map[string][]RoomDocument)
	for _, r := range rows {
		var pageCount int32
		if r.PageCount.Valid {
			pageCount = r.PageCount.Int32
		}
		var fileSize int64
		if r.FileSize.Valid {
			fileSize = r.FileSize.Int64
		}
		d := RoomDocument{
			ID:         uuid.UUID(r.ID.Bytes).String(),
			DocumentID: uuid.UUID(r.DocumentID.Bytes).String(),
			Title:      r.DocumentTitle,
			PageCount:  pageCount,
			FileSize:   fileSize,
			SourceType: r.SourceType,
			Status:     r.Status,
			FolderPath: r.FolderPath,
			SortOrder:  r.SortOrder,
			CreatedAt:  r.CreatedAt.Time,
		}
		docsByFolder[r.FolderPath] = append(docsByFolder[r.FolderPath], d)
	}

	out := make([]FolderDocs, 0, len(folders))
	for _, f := range folders {
		perm, _ := s.GetFolderPermission(ctx, roomID, member.Email, f.Path)
		if perm == "none" {
			continue
		}
		docs := docsByFolder[f.Path]
		if docs == nil {
			docs = []RoomDocument{}
		}
		sort.Slice(docs, func(i, j int) bool {
			if docs[i].SortOrder != docs[j].SortOrder {
				return docs[i].SortOrder < docs[j].SortOrder
			}
			return docs[i].CreatedAt.Before(docs[j].CreatedAt)
		})
		out = append(out, FolderDocs{
			Folder:     f,
			Permission: perm,
			Documents:  docs,
		})
	}
	return out, nil
}

// ListFolders returns the folder structure stored in a room's settings.
func (s *Service) ListFolders(ctx context.Context, roomID, workspaceID string) ([]Folder, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.loadFolders(room)
}

// CreateFolder adds a folder to a room. Only admins can create folders.
func (s *Service) CreateFolder(ctx context.Context, roomID, workspaceID, userID, name, parentPath string) ([]Folder, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, userID); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("folder name is required")
	}
	if strings.Contains(name, "/") {
		return nil, errors.New("folder name cannot contain slashes")
	}
	if slug := slugify(name); slug == "" {
		return nil, errors.New("folder name must contain valid characters")
	}
	if parentPath == "" {
		parentPath = "/"
	}
	parentPath = normalizeFolderPath(parentPath)

	folders, err := s.loadFolders(room)
	if err != nil {
		return nil, err
	}
	if parentPath != "/" && !folderExists(folders, parentPath) {
		return nil, ErrFolderNotFound
	}

	newPath := joinFolderPath(parentPath, slugify(name))
	if folderExists(folders, newPath) {
		return nil, ErrFolderExists
	}

	maxOrder := -1
	for _, f := range folders {
		if f.SortOrder > maxOrder {
			maxOrder = f.SortOrder
		}
	}
	folders = append(folders, Folder{
		Path:      newPath,
		Name:      name,
		SortOrder: maxOrder + 1,
	})

	if err := s.saveFolders(ctx, room, folders); err != nil {
		return nil, err
	}
	return folders, nil
}

// RenameFolder renames a folder and cascades the path to documents and permissions.
// Only admins can rename folders.
func (s *Service) RenameFolder(ctx context.Context, roomID, workspaceID, userID, oldPath, newName string) ([]Folder, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, userID); err != nil {
		return nil, err
	}
	oldPath = normalizeFolderPath(oldPath)
	if oldPath == "/" {
		return nil, errors.New("cannot rename root folder")
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, errors.New("folder name is required")
	}
	if strings.Contains(newName, "/") {
		return nil, errors.New("folder name cannot contain slashes")
	}
	if slug := slugify(newName); slug == "" {
		return nil, errors.New("folder name must contain valid characters")
	}

	folders, err := s.loadFolders(room)
	if err != nil {
		return nil, err
	}
	idx := folderIndex(folders, oldPath)
	if idx < 0 {
		return nil, ErrFolderNotFound
	}

	parentPath := parentFolder(oldPath)
	newPath := joinFolderPath(parentPath, slugify(newName))
	if newPath != oldPath && folderExists(folders, newPath) {
		return nil, ErrFolderExists
	}

	// Build a folder path mapping for cascade updates.
	pathMap := make(map[string]string)
	pathMap[oldPath] = newPath
	for i := range folders {
		if strings.HasPrefix(folders[i].Path, oldPath+"/") {
			suffix := strings.TrimPrefix(folders[i].Path, oldPath)
			pathMap[folders[i].Path] = newPath + suffix
		}
	}

	// Update folder structures in memory.
	for i := range folders {
		if folders[i].Path == oldPath {
			folders[i].Path = newPath
			folders[i].Name = newName
		} else if strings.HasPrefix(folders[i].Path, oldPath+"/") {
			folders[i].Path = pathMap[folders[i].Path]
			// Keep the last segment as the displayed name.
			folders[i].Name = folderName(folders[i].Path)
		}
	}

	if err := s.runInTx(ctx, func(q *db.Queries) error {
		// Cascade update documents and permissions for the renamed folder and its descendants.
		for oldP, newP := range pathMap {
			if err := q.UpdateDealRoomDocumentsFolderPath(ctx, db.UpdateDealRoomDocumentsFolderPathParams{
				FolderPath:   newP,
				RoomID:       room.ID,
				FolderPath_2: oldP,
			}); err != nil {
				return err
			}
			if err := q.UpdateRoomFolderPermissionsFolderPath(ctx, db.UpdateRoomFolderPermissionsFolderPathParams{
				FolderPath:   newP,
				RoomID:       room.ID,
				FolderPath_2: oldP,
			}); err != nil {
				return err
			}
		}
		return s.saveFoldersWithQueries(ctx, q, room, folders)
	}); err != nil {
		return nil, err
	}
	return folders, nil
}

// DeleteFolder removes a folder from a room. Only admins can delete folders.
// Rejects deletion if the folder or its descendants contain documents.
func (s *Service) DeleteFolder(ctx context.Context, roomID, workspaceID, userID, path string) ([]Folder, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, userID); err != nil {
		return nil, err
	}
	path = normalizeFolderPath(path)
	if path == "/" {
		return nil, errors.New("cannot delete root folder")
	}

	folders, err := s.loadFolders(room)
	if err != nil {
		return nil, err
	}
	if !folderExists(folders, path) {
		return nil, ErrFolderNotFound
	}

	count, err := s.queries.CountDocumentsInFolder(ctx, db.CountDocumentsInFolderParams{
		RoomID:     room.ID,
		FolderPath: path,
	})
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrFolderNotEmpty
	}

	newFolders := make([]Folder, 0, len(folders))
	for _, f := range folders {
		if f.Path == path || strings.HasPrefix(f.Path, path+"/") {
			continue
		}
		newFolders = append(newFolders, f)
	}

	if err := s.runInTx(ctx, func(q *db.Queries) error {
		if err := q.DeleteRoomFolderPermissionsPrefix(ctx, db.DeleteRoomFolderPermissionsPrefixParams{
			RoomID:     room.ID,
			FolderPath: path,
		}); err != nil {
			return err
		}
		return s.saveFoldersWithQueries(ctx, q, room, newFolders)
	}); err != nil {
		return nil, err
	}
	return newFolders, nil
}

func (s *Service) loadFolders(room db.DealRoom) ([]Folder, error) {
	if len(room.Settings) == 0 || string(room.Settings) == "{}" {
		return defaultFolders(), nil
	}
	var settings struct {
		Folders []Folder `json:"folders"`
	}
	if err := json.Unmarshal(room.Settings, &settings); err != nil {
		return nil, fmt.Errorf("parse room settings: %w", err)
	}
	if len(settings.Folders) == 0 {
		return defaultFolders(), nil
	}
	// Ensure the root folder always exists so documents uploaded to "/" are visible.
	if !folderExists(settings.Folders, "/") {
		settings.Folders = append(defaultFolders(), settings.Folders...)
	}
	return settings.Folders, nil
}

func (s *Service) saveFolders(ctx context.Context, room db.DealRoom, folders []Folder) error {
	return s.saveFoldersWithQueries(ctx, s.queries, room, folders)
}

func (s *Service) saveFoldersWithQueries(ctx context.Context, q *db.Queries, room db.DealRoom, folders []Folder) error {
	var settings map[string]interface{}
	if len(room.Settings) > 0 && string(room.Settings) != "{}" {
		if err := json.Unmarshal(room.Settings, &settings); err != nil {
			return fmt.Errorf("parse room settings: %w", err)
		}
	} else {
		settings = make(map[string]interface{})
	}
	settings["folders"] = folders
	settingsBytes, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal room settings: %w", err)
	}
	return q.UpdateDealRoomSettings(ctx, db.UpdateDealRoomSettingsParams{
		Column1:     settingsBytes,
		ID:          room.ID,
		WorkspaceID: room.WorkspaceID,
	})
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

func defaultFolders() []Folder {
	return []Folder{{Path: "/", Name: "Root", SortOrder: 0}}
}

func folderExists(folders []Folder, path string) bool {
	return folderIndex(folders, path) >= 0
}

func folderIndex(folders []Folder, path string) int {
	for i, f := range folders {
		if f.Path == path {
			return i
		}
	}
	return -1
}

func normalizeFolderPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return "/"
	}
	return p
}

func parentFolder(path string) string {
	path = normalizeFolderPath(path)
	if path == "/" {
		return "/"
	}
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

func folderName(path string) string {
	path = normalizeFolderPath(path)
	if path == "/" {
		return "Root"
	}
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return path
	}
	return path[idx+1:]
}

func slugify(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	// Remove characters that are not lowercase alphanumeric or hyphen.
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

func joinFolderPath(parent, name string) string {
	parent = normalizeFolderPath(parent)
	if parent == "/" {
		return "/" + name
	}
	return parent + "/" + name
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
