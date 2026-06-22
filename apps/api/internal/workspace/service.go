package workspace

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidSlug  = errors.New("slug must be lowercase alphanumeric with hyphens")
	ErrNotMember    = errors.New("user is not a member of this workspace")
	ErrAlreadyMember = errors.New("user is already a member")
	slugRegex       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

func validMemberRole(role string) bool {
	return role == RoleAdmin || role == RoleMember
}

// Workspace is the public view of a db.Workspace.
type Workspace struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	BrandColor string `json:"brand_color,omitempty"`
	Role       string `json:"role,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// Member is the public view of a db.WorkspaceMember.
type Member struct {
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	JoinedAt  string `json:"joined_at"`
}

// Service handles workspace operations.
type Service struct {
	queries *db.Queries
}

// NewService creates a workspace service.
func NewService(q *db.Queries) *Service {
	return &Service{queries: q}
}

func workspaceFromDB(w db.ListWorkspacesByUserRow) Workspace {
	return Workspace{
		ID:         uuidToString(w.ID),
		TenantID:   uuidToString(w.TenantID),
		Name:       w.Name,
		Slug:       w.Slug,
		BrandColor: w.BrandColor.String,
		Role:       w.Role,
		CreatedAt:  w.CreatedAt.Time.Format(time.RFC3339),
	}
}

func workspaceFromRow(w db.Workspace) Workspace {
	return Workspace{
		ID:        uuidToString(w.ID),
		TenantID:  uuidToString(w.TenantID),
		Name:      w.Name,
		Slug:      w.Slug,
		BrandColor: w.BrandColor.String,
		CreatedAt: w.CreatedAt.Time.Format(time.RFC3339),
	}
}

func memberFromDB(m db.WorkspaceMember) Member {
	return Member{
		UserID:   uuidToString(m.UserID),
		Role:     m.Role,
		JoinedAt: m.JoinedAt.Time.Format(time.RFC3339),
	}
}

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

func pgUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

// Create creates a tenant, workspace and makes the user owner.
func (s *Service) Create(ctx context.Context, userID, name, slug, brandColor string) (Workspace, error) {
	if !slugRegex.MatchString(slug) {
		return Workspace{}, ErrInvalidSlug
	}
	slug = strings.ToLower(slug)

	uid, err := pgUUID(userID)
	if err != nil {
		return Workspace{}, err
	}

	tenant, err := s.queries.CreateTenant(ctx, name)
	if err != nil {
		return Workspace{}, err
	}

	tenantUUID, _ := pgUUID(uuidToString(tenant.ID))
	ws, err := s.queries.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		TenantID:   tenantUUID,
		Name:       name,
		Slug:       slug,
		BrandColor: pgtype.Text{String: brandColor, Valid: brandColor != ""},
	})
	if err != nil {
		return Workspace{}, err
	}

	wsUUID, _ := pgUUID(uuidToString(ws.ID))
	_, err = s.queries.AddWorkspaceMember(ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: wsUUID,
		UserID:      uid,
		Role:        RoleOwner,
	})
	if err != nil {
		return Workspace{}, err
	}

	return workspaceFromRow(ws), nil
}

// List returns workspaces the user belongs to.
func (s *Service) List(ctx context.Context, userID string) ([]Workspace, error) {
	uid, err := pgUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListWorkspacesByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]Workspace, len(rows))
	for i, r := range rows {
		out[i] = workspaceFromDB(r)
	}
	return out, nil
}

// GetBySlug returns a workspace by slug if the user is a member.
func (s *Service) GetBySlug(ctx context.Context, userID, slug string) (Workspace, error) {
	ws, err := s.queries.GetWorkspaceBySlug(ctx, slug)
	if err != nil {
		return Workspace{}, err
	}
	wsID := uuidToString(ws.ID)
	if _, err := s.requireMember(ctx, userID, wsID); err != nil {
		return Workspace{}, err
	}
	return workspaceFromRow(ws), nil
}

// Get returns a workspace if the user is a member.
func (s *Service) Get(ctx context.Context, userID, workspaceID string) (Workspace, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Workspace{}, err
	}
	ws, err := s.queries.GetWorkspaceByID(ctx, wsUUID)
	if err != nil {
		return Workspace{}, err
	}
	if _, err := s.requireMember(ctx, userID, workspaceID); err != nil {
		return Workspace{}, err
	}
	return workspaceFromRow(ws), nil
}

// AddMember adds a user to a workspace.
func (s *Service) AddMember(ctx context.Context, actorID, workspaceID, userID, role string) (Member, error) {
	if _, err := s.requireMember(ctx, actorID, workspaceID); err != nil {
		return Member{}, err
	}
	if !validMemberRole(role) {
		return Member{}, errors.New("invalid role")
	}

	wsUUID, _ := pgUUID(workspaceID)
	uUUID, _ := pgUUID(userID)

	_, err := s.queries.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: wsUUID,
		UserID:      uUUID,
	})
	if err == nil {
		return Member{}, ErrAlreadyMember
	}

	m, err := s.queries.AddWorkspaceMember(ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: wsUUID,
		UserID:      uUUID,
		Role:        role,
	})
	if err != nil {
		return Member{}, err
	}
	return memberFromDB(m), nil
}

func (s *Service) requireMember(ctx context.Context, userID, workspaceID string) (db.WorkspaceMember, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return db.WorkspaceMember{}, err
	}
	uUUID, err := pgUUID(userID)
	if err != nil {
		return db.WorkspaceMember{}, err
	}
	m, err := s.queries.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: wsUUID,
		UserID:      uUUID,
	})
	if err != nil {
		return db.WorkspaceMember{}, ErrNotMember
	}
	return m, nil
}
