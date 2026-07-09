package workspace

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidSlug        = errors.New("the workspace URL can only contain lowercase letters, numbers, and hyphens")
	ErrSlugExists         = errors.New("a workspace with this URL already exists. please choose a different name")
	ErrNotMember          = errors.New("user is not a member of this workspace")
	ErrAlreadyMember      = errors.New("user is already a member")
	ErrInvalidRole        = errors.New("invalid role")
	ErrNotManager         = errors.New("only owner or admin can manage members")
	ErrInvitationNotFound = errors.New("invitation not found")
	ErrInvitationExpired  = errors.New("invitation expired")
	ErrInvitationUsed     = errors.New("invitation already used")
	slugRegex             = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleGuest  = "guest"
)

func validMemberRole(role string) bool {
	return role == RoleAdmin || role == RoleMember || role == RoleGuest
}

func validManagerRole(role string) bool {
	return role == RoleOwner || role == RoleAdmin
}

func validInvitationRole(role string) bool {
	return role == RoleAdmin || role == RoleMember || role == RoleGuest
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
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	JoinedAt string `json:"joined_at"`
}

// Invitation is the public view of a db.WorkspaceInvitation.
type Invitation struct {
	Token       string `json:"token"`
	WorkspaceID string `json:"workspace_id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	ExpiresAt   string `json:"expires_at"`
	UsedAt      string `json:"used_at,omitempty"`
	CreatedAt   string `json:"created_at"`
}

func invitationFromDB(i db.WorkspaceInvitation) Invitation {
	return Invitation{
		Token:       uuidToString(i.Token),
		WorkspaceID: uuidToString(i.WorkspaceID),
		Email:       i.Email,
		Role:        i.Role,
		ExpiresAt:   i.ExpiresAt.Time.Format(time.RFC3339),
		CreatedAt:   i.CreatedAt.Time.Format(time.RFC3339),
	}
}

// Beginner starts a database transaction.
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Service handles workspace operations.
type Service struct {
	queries     *db.Queries
	dbPool      Beginner
	mailer      mailer.Mailer
	frontendURL string
}

// ServiceOption configures the workspace service.
type ServiceOption func(*Service)

// WithDBPool enables transactional operations like AcceptInvitation.
func WithDBPool(pool Beginner) ServiceOption {
	return func(s *Service) { s.dbPool = pool }
}

// WithMailer sets the transactional mailer used for invitation emails.
func WithMailer(m mailer.Mailer) ServiceOption {
	return func(s *Service) { s.mailer = m }
}

// WithFrontendURL sets the public frontend URL used in invitation links.
func WithFrontendURL(url string) ServiceOption {
	return func(s *Service) { s.frontendURL = url }
}

// NewService creates a workspace service.
func NewService(q *db.Queries, opts ...ServiceOption) *Service {
	s := &Service{queries: q}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
		ID:         uuidToString(w.ID),
		TenantID:   uuidToString(w.TenantID),
		Name:       w.Name,
		Slug:       w.Slug,
		BrandColor: w.BrandColor.String,
		CreatedAt:  w.CreatedAt.Time.Format(time.RFC3339),
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

	tenant, err := s.queries.CreateTenant(ctx, db.CreateTenantParams{Name: name, Slug: pgtype.Text{String: slug, Valid: true}})
	if err != nil {
		if isUniqueViolation(err) {
			// fallback to a unique slug if the workspace slug is already a tenant slug
			tenant, err = s.queries.CreateTenant(ctx, db.CreateTenantParams{Name: name, Slug: pgtype.Text{String: uuid.NewString(), Valid: true}})
		}
		if err != nil {
			return Workspace{}, err
		}
	}

	tenantUUID, _ := pgUUID(uuidToString(tenant.ID))
	ws, err := s.queries.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		TenantID:   tenantUUID,
		Name:       name,
		Slug:       slug,
		BrandColor: pgtype.Text{String: brandColor, Valid: brandColor != ""},
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Workspace{}, ErrSlugExists
		}
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

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "unique constraint")
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
func (s *Service) IsTenantAdmin(ctx context.Context, userID, tenantID string) bool {
	uid, err := pgUUID(userID)
	if err != nil {
		return false
	}
	tid, err := pgUUID(tenantID)
	if err != nil {
		return false
	}
	rows, err := s.queries.ListWorkspacesByUserAndTenant(ctx, db.ListWorkspacesByUserAndTenantParams{
		UserID:   uid,
		TenantID: tid,
	})
	if err != nil {
		return false
	}
	for _, r := range rows {
		if r.Role == RoleOwner || r.Role == RoleAdmin {
			return true
		}
	}
	return false
}

func (s *Service) GetBySlug(ctx context.Context, userID, slug, tenantID string) (Workspace, error) {
	var tenantUUID pgtype.UUID
	if tenantID != "" {
		var err error
		tenantUUID, err = pgUUID(tenantID)
		if err != nil {
			return Workspace{}, err
		}
	}
	return s.getByTenantAndSlug(ctx, userID, tenantUUID, slug)
}

// GetByTenantAndSlug returns a workspace scoped to a tenant when available.
func (s *Service) GetByTenantAndSlug(ctx context.Context, userID, tenantID, slug string) (Workspace, error) {
	var tenantUUID pgtype.UUID
	if tenantID != "" {
		var err error
		tenantUUID, err = pgUUID(tenantID)
		if err != nil {
			return Workspace{}, err
		}
	}
	return s.getByTenantAndSlug(ctx, userID, tenantUUID, slug)
}

func (s *Service) getByTenantAndSlug(ctx context.Context, userID string, tenantUUID pgtype.UUID, slug string) (Workspace, error) {
	var ws db.Workspace
	var err error
	if tenantUUID.Valid {
		ws, err = s.queries.GetWorkspaceByTenantAndSlug(ctx, db.GetWorkspaceByTenantAndSlugParams{
			TenantID: tenantUUID,
			Slug:     slug,
		})
	} else {
		ws, err = s.queries.GetWorkspaceBySlug(ctx, slug)
	}
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
func (s *Service) Get(ctx context.Context, userID, workspaceID, tenantID string) (Workspace, error) {
	ws, err := s.getWorkspaceByID(ctx, workspaceID, tenantID)
	if err != nil {
		return Workspace{}, err
	}
	if _, err := s.requireMember(ctx, userID, workspaceID); err != nil {
		return Workspace{}, err
	}
	return workspaceFromRow(ws), nil
}

func (s *Service) getWorkspaceByID(ctx context.Context, workspaceID, tenantID string) (db.Workspace, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return db.Workspace{}, err
	}
	if tenantID != "" {
		tenantUUID, err := pgUUID(tenantID)
		if err != nil {
			return db.Workspace{}, err
		}
		return s.queries.GetWorkspaceByIDAndTenant(ctx, db.GetWorkspaceByIDAndTenantParams{
			ID:       wsUUID,
			TenantID: tenantUUID,
		})
	}
	return s.queries.GetWorkspaceByID(ctx, wsUUID)
}

func (s *Service) requireWorkspaceInTenant(ctx context.Context, workspaceID, tenantID string) error {
	_, err := s.getWorkspaceByID(ctx, workspaceID, tenantID)
	return err
}

// CreateInvitation creates an invitation token for a new member. Only owner/admin can call.
func (s *Service) CreateInvitation(ctx context.Context, actorID, workspaceID, tenantID, email, role string, expiresDays int) (Invitation, error) {
	actor, err := s.requireMember(ctx, actorID, workspaceID)
	if err != nil {
		return Invitation{}, err
	}
	if !validManagerRole(actor.Role) {
		return Invitation{}, ErrNotManager
	}
	if !validInvitationRole(role) {
		return Invitation{}, ErrInvalidRole
	}
	if tenantID != "" {
		if err := s.requireWorkspaceInTenant(ctx, workspaceID, tenantID); err != nil {
			return Invitation{}, err
		}
	}

	wsUUID, _ := pgUUID(workspaceID)
	if expiresDays <= 0 {
		expiresDays = 7
	}
	if expiresDays > 30 {
		expiresDays = 30
	}
	expiresAt := time.Now().UTC().AddDate(0, 0, expiresDays)

	i, err := s.queries.CreateInvitation(ctx, db.CreateInvitationParams{
		WorkspaceID: wsUUID,
		Email:       email,
		Role:        role,
		ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return Invitation{}, err
	}

	inv := invitationFromDB(i)
	s.sendInvitationEmail(ctx, inv, actorID, expiresDays)
	return inv, nil
}

// sendInvitationEmail sends the workspace invitation email. Failures are logged
// and suppressed; the invitation token has already been created.
func (s *Service) sendInvitationEmail(ctx context.Context, inv Invitation, actorID string, expiresDays int) {
	if s.mailer == nil || s.frontendURL == "" {
		return
	}

	vars := map[string]string{
		"BrandName":      "DealSignal",
		"WorkspaceName":  "",
		"InviterEmail":   "",
		"Role":           inv.Role,
		"InvitationLink": fmt.Sprintf("%s/invitations/%s/accept", strings.TrimRight(s.frontendURL, "/"), inv.Token),
		"ExpiryDays":     strconv.Itoa(expiresDays),
	}

	if ws, err := s.getWorkspaceByID(ctx, inv.WorkspaceID, ""); err == nil {
		vars["WorkspaceName"] = ws.Name
	}
	if actorUUID, err := pgUUID(actorID); err == nil {
		if user, err := s.queries.GetUserByID(ctx, actorUUID); err == nil {
			vars["InviterEmail"] = user.Email
		}
	}

	_, _ = s.mailer.SendEmail(ctx, mailer.EmailJob{
		EmailType:         mailer.EmailTypeInvitation,
		Recipient:         inv.Email,
		WorkspaceID:       inv.WorkspaceID,
		Locale:            locale.Normalize(locale.FromContext(ctx)),
		TemplateVariables: vars,
	})
}

// AcceptInvitation uses a token to add a user to a workspace.
// Runs inside a transaction to prevent TOCTOU races on invitation usage.
func (s *Service) AcceptInvitation(ctx context.Context, token, userID string) (Member, error) {
	tokenUUID, err := pgUUID(token)
	if err != nil {
		return Member{}, ErrInvitationNotFound
	}

	if s.dbPool == nil {
		return Member{}, errors.New("accept invitation requires a database pool")
	}

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return Member{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.queries.WithTx(tx)

	inv, err := qtx.GetInvitationByToken(ctx, tokenUUID)
	if err != nil {
		return Member{}, ErrInvitationNotFound
	}
	if inv.UsedAt.Valid {
		return Member{}, ErrInvitationUsed
	}
	if inv.ExpiresAt.Time.Before(time.Now().UTC()) {
		return Member{}, ErrInvitationExpired
	}

	workspaceID := uuidToString(inv.WorkspaceID)
	wsUUID, _ := pgUUID(workspaceID)
	uUUID, _ := pgUUID(userID)

	// Idempotent: if already a member, mark invitation used and return existing membership.
	existing, err := qtx.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: wsUUID,
		UserID:      uUUID,
	})
	if err == nil {
		if err := qtx.MarkInvitationUsed(ctx, tokenUUID); err != nil {
			return Member{}, fmt.Errorf("mark invitation used: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return Member{}, fmt.Errorf("commit tx: %w", err)
		}
		return memberFromDB(existing), nil
	}

	m, err := qtx.AddWorkspaceMember(ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: wsUUID,
		UserID:      uUUID,
		Role:        inv.Role,
	})
	if err != nil {
		return Member{}, err
	}

	if err := qtx.MarkInvitationUsed(ctx, tokenUUID); err != nil {
		return Member{}, fmt.Errorf("mark invitation used: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Member{}, fmt.Errorf("commit tx: %w", err)
	}
	return memberFromDB(m), nil
}

// AddMember adds an existing user to a workspace. Only owner/admin can call.
func (s *Service) AddMember(ctx context.Context, actorID, workspaceID, tenantID, userID, role string) (Member, error) {
	actor, err := s.requireMember(ctx, actorID, workspaceID)
	if err != nil {
		return Member{}, err
	}
	if !validManagerRole(actor.Role) {
		return Member{}, ErrNotManager
	}
	if !validMemberRole(role) {
		return Member{}, ErrInvalidRole
	}

	if tenantID != "" {
		if err := s.requireWorkspaceInTenant(ctx, workspaceID, tenantID); err != nil {
			return Member{}, err
		}
	}

	wsUUID, _ := pgUUID(workspaceID)
	uUUID, _ := pgUUID(userID)

	_, err = s.queries.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
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

// IsManager returns true if the user is an owner or admin of the workspace.
func (s *Service) IsManager(ctx context.Context, userID, workspaceID string) bool {
	m, err := s.requireMember(ctx, userID, workspaceID)
	if err != nil {
		return false
	}
	return validManagerRole(m.Role)
}
