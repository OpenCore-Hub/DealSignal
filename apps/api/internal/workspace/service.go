package workspace

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/billing"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidSlug             = errors.New("the workspace URL can only contain lowercase letters, numbers, and hyphens")
	ErrSlugExists              = errors.New("a workspace with this URL already exists. please choose a different name")
	ErrNotMember               = errors.New("user is not a member of this workspace")
	ErrAlreadyMember           = errors.New("user is already a member")
	ErrInvalidEmail            = errors.New("invalid email")
	ErrInvalidRole             = errors.New("invalid role")
	ErrNotManager              = errors.New("only owner or admin can manage members")
	ErrMemberNotFound          = errors.New("member not found")
	ErrCannotModifyOwner       = errors.New("cannot modify the workspace owner")
	ErrCannotModifySelf        = errors.New("cannot change your own membership here")
	ErrCannotManageMember      = errors.New("cannot manage this member")
	ErrInvitationNotFound      = errors.New("invitation not found")
	ErrInvitationExpired       = errors.New("invitation expired")
	ErrInvitationUsed          = errors.New("invitation already used")
	ErrInvitationEmailMismatch = errors.New("email does not match invitation")
	ErrLogoStorageUnavailable  = errors.New("logo storage is not configured")
	ErrInvalidLogoType         = errors.New("unsupported logo image type")
	ErrLogoTooLarge            = errors.New("logo must be smaller than 5 MB")
	ErrInvalidPlanCode         = errors.New("invalid plan code")
	ErrInvalidBillingPeriod    = errors.New("invalid billing period")
	ErrPlanPaymentRequired     = errors.New("plan change requires payment")
	ErrPlanSalesAssisted       = errors.New("enterprise plan requires sales")
	ErrEmailUnverified         = errors.New("verify your email before creating another workspace")
	ErrPlanManageViaPortal     = errors.New("manage this subscription in the billing portal")
	ErrStripeNoCustomer        = errors.New("no stripe customer for this workspace")
	ErrInvalidCheckoutPlan     = errors.New("that plan cannot be purchased at checkout")
	slugRegex                  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	emailRegex                 = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleGuest  = "guest"

	// trialGrantLockNamespace is int4 key1 for pg_advisory_xact_lock when
	// granting the first-owner 14-day trial. key2 is hashtext(userID).
	trialGrantLockNamespace int32 = 881726
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

func canManageTargetRole(actorRole, targetRole string) error {
	if targetRole == RoleOwner {
		return ErrCannotModifyOwner
	}
	if actorRole == RoleOwner {
		return nil
	}
	if actorRole == RoleAdmin && (targetRole == RoleMember || targetRole == RoleGuest) {
		return nil
	}
	return ErrCannotManageMember
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
	queries               *db.Queries
	dbPool                Beginner
	mailer                mailer.Mailer
	frontendURL           string
	storage               *storage.Client
	cnameTarget           string
	cnameLookup           CNAMELookup
	allowUnpaidPlanChange bool
	stripe                *stripeRuntime
}

type stripeRuntime struct {
	gateway billing.Gateway
	prices  billing.Prices
}

// ServiceOption configures the workspace service.
type ServiceOption func(*Service)

// WithDBPool enables transactional operations like AcceptInvitation.
func WithDBPool(pool Beginner) ServiceOption {
	return func(s *Service) { s.dbPool = pool }
}

// WithAllowUnpaidPlanChange lets ChangePlan persist pro/business without checkout.
// Production must leave this false. Tests and local/dev set it true.
func WithAllowUnpaidPlanChange(allow bool) ServiceOption {
	return func(s *Service) { s.allowUnpaidPlanChange = allow }
}

// WithStripe attaches Checkout/Portal. The webhook is the only paid plan writer.
func WithStripe(gateway billing.Gateway, prices billing.Prices) ServiceOption {
	return func(s *Service) {
		if gateway == nil {
			return
		}
		s.stripe = &stripeRuntime{gateway: gateway, prices: prices}
	}
}

// WithMailer sets the transactional mailer used for invitation emails.
func WithMailer(m mailer.Mailer) ServiceOption {
	return func(s *Service) { s.mailer = m }
}

// WithFrontendURL sets the public frontend URL used in invitation links.
func WithFrontendURL(url string) ServiceOption {
	return func(s *Service) { s.frontendURL = url }
}

// WithStorage enables workspace logo upload and presigned logo URLs.
func WithStorage(c *storage.Client) ServiceOption {
	return func(s *Service) { s.storage = c }
}

// SetStorage attaches object storage after the service is constructed.
func (s *Service) SetStorage(c *storage.Client) {
	s.storage = c
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
// The whole operation runs in a transaction when a database pool is available,
// so failures like an invalid owner user ID do not leave orphaned tenants/
// workspaces behind. The non-transactional branch is kept for unit tests that
// do not provide a pool.
func (s *Service) Create(ctx context.Context, userID, name, slug, brandColor string) (Workspace, error) {
	if !slugRegex.MatchString(slug) {
		return Workspace{}, ErrInvalidSlug
	}
	slug = strings.ToLower(slug)

	uid, err := pgUUID(userID)
	if err != nil {
		return Workspace{}, err
	}

	create := func(q *db.Queries) (Workspace, error) {
		if err := q.LockUserWriterCap(ctx, db.LockUserWriterCapParams{
			LockNs: trialGrantLockNamespace,
			UserID: userID,
		}); err != nil {
			return Workspace{}, fmt.Errorf("lock owned workspace cap: %w", err)
		}
		user, err := q.GetUserByID(ctx, uid)
		if err != nil {
			return Workspace{}, err
		}
		if err := s.assertCanAddOwnedWorkspace(ctx, q, uid, true); err != nil {
			return Workspace{}, err
		}
		ownedCount, err := q.CountOwnedWorkspacesByUser(ctx, uid)
		if err != nil {
			return Workspace{}, fmt.Errorf("count owned workspaces: %w", err)
		}

		tenant, err := q.CreateTenant(ctx, db.CreateTenantParams{Name: name, Slug: pgtype.Text{String: slug, Valid: true}})
		if err != nil {
			if isUniqueViolation(err) {
				// fallback to a unique slug if the workspace slug is already a tenant slug
				tenant, err = q.CreateTenant(ctx, db.CreateTenantParams{Name: name, Slug: pgtype.Text{String: uuid.NewString(), Valid: true}})
			}
			if err != nil {
				return Workspace{}, err
			}
		}

		tenantUUID, _ := pgUUID(uuidToString(tenant.ID))
		ws, err := q.CreateWorkspace(ctx, db.CreateWorkspaceParams{
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
		_, err = q.AddWorkspaceMember(ctx, db.AddWorkspaceMemberParams{
			WorkspaceID: wsUUID,
			UserID:      uid,
			Role:        RoleOwner,
		})
		if err != nil {
			return Workspace{}, err
		}

		planCode := plan.CodeFree
		trialEnds := pgtype.Timestamptz{}
		if ownedCount == 0 && !user.TrialGrantedAt.Valid {
			if _, grantErr := q.GrantUserTrial(ctx, uid); grantErr != nil {
				if !errors.Is(grantErr, pgx.ErrNoRows) {
					return Workspace{}, fmt.Errorf("grant trial: %w", grantErr)
				}
			} else {
				planCode = plan.CodeTrial
				trialEnds = pgtype.Timestamptz{Time: time.Now().UTC().Add(plan.TrialDuration), Valid: true}
			}
		}

		_, err = q.InsertWorkspaceBilling(ctx, db.InsertWorkspaceBillingParams{
			WorkspaceID: wsUUID,
			PlanCode:    planCode,
			Period:      plan.PeriodMonthly,
			TrialEndsAt: trialEnds,
		})
		if err != nil {
			return Workspace{}, fmt.Errorf("insert workspace billing: %w", err)
		}

		return workspaceFromRow(ws), nil
	}

	if s.dbPool == nil {
		return create(s.queries)
	}

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return Workspace{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Serialize owned-workspace grant per user so parallel Create cannot
	// slip under the cap or mint two 14-day trials.
	ws, err := create(s.queries.WithTx(tx))
	if err != nil {
		return Workspace{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Workspace{}, fmt.Errorf("commit tx: %w", err)
	}

	return ws, nil
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
	member, err := s.requireMember(ctx, userID, wsID)
	if err != nil {
		return Workspace{}, err
	}
	out := workspaceFromRow(ws)
	out.Role = member.Role
	return out, nil
}

// Get returns a workspace if the user is a member.
func (s *Service) Get(ctx context.Context, userID, workspaceID, tenantID string) (Workspace, error) {
	ws, err := s.getWorkspaceByID(ctx, workspaceID, tenantID)
	if err != nil {
		return Workspace{}, err
	}
	member, err := s.requireMember(ctx, userID, workspaceID)
	if err != nil {
		return Workspace{}, err
	}
	out := workspaceFromRow(ws)
	out.Role = member.Role
	return out, nil
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

// CreateInvitation creates or refreshes an invitation for a new member. Only owner/admin can call.
// Pending invites are resent (new token + expiry). Used invites for non-members are deleted then recreated.
// Active members are rejected with ErrAlreadyMember.
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
	if err := canManageTargetRole(actor.Role, role); err != nil {
		return Invitation{}, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !emailRegex.MatchString(email) {
		return Invitation{}, ErrInvalidEmail
	}
	if tenantID != "" {
		if err := s.requireWorkspaceInTenant(ctx, workspaceID, tenantID); err != nil {
			return Invitation{}, err
		}
	}

	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Invitation{}, err
	}

	_, err = s.queries.GetWorkspaceMemberByEmail(ctx, db.GetWorkspaceMemberByEmailParams{
		WorkspaceID: wsUUID,
		Email:       email,
	})
	if err == nil {
		return Invitation{}, ErrAlreadyMember
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Invitation{}, err
	}

	if expiresDays <= 0 {
		expiresDays = 7
	}
	if expiresDays > 30 {
		expiresDays = 30
	}
	expiresAt := pgtype.Timestamptz{Time: time.Now().UTC().AddDate(0, 0, expiresDays), Valid: true}

	existing, err := s.queries.GetWorkspaceInvitationByEmail(ctx, db.GetWorkspaceInvitationByEmailParams{
		WorkspaceID: wsUUID,
		Email:       email,
	})
	switch {
	case err == nil && !existing.UsedAt.Valid:
		// Resend: only consume a seat when promoting a guest pending invite to an internal role.
		consumesSeat := isInternalSeatRole(role) && !isInternalSeatRole(existing.Role)
		var inv Invitation
		run := func(q *db.Queries) error {
			if consumesSeat {
				if seatErr := s.assertCanAddInternalSeatQ(ctx, q, workspaceID); seatErr != nil {
					return seatErr
				}
			}
			i, resendErr := q.ResendPendingWorkspaceInvitation(ctx, db.ResendPendingWorkspaceInvitationParams{
				WorkspaceID: wsUUID,
				Email:       email,
				Role:        role,
				ExpiresAt:   expiresAt,
			})
			if resendErr != nil {
				return resendErr
			}
			inv = invitationFromDB(i)
			return nil
		}
		if consumesSeat {
			if seatErr := s.withSeatMutation(ctx, workspaceID, run); seatErr != nil {
				return Invitation{}, seatErr
			}
		} else if runErr := run(s.queries); runErr != nil {
			return Invitation{}, runErr
		}
		s.sendInvitationEmail(ctx, inv, actorID, expiresDays)
		return inv, nil
	case err == nil && existing.UsedAt.Valid:
		if delErr := s.queries.DeleteWorkspaceInvitationByEmail(ctx, db.DeleteWorkspaceInvitationByEmailParams{
			WorkspaceID: wsUUID,
			Email:       email,
		}); delErr != nil {
			return Invitation{}, delErr
		}
	case errors.Is(err, pgx.ErrNoRows):
		// first invite for this email
	case err != nil:
		return Invitation{}, err
	}

	var inv Invitation
	create := func(q *db.Queries) error {
		if isInternalSeatRole(role) {
			if seatErr := s.assertCanAddInternalSeatQ(ctx, q, workspaceID); seatErr != nil {
				return seatErr
			}
		}
		i, createErr := q.CreateInvitation(ctx, db.CreateInvitationParams{
			WorkspaceID: wsUUID,
			Email:       email,
			Role:        role,
			ExpiresAt:   expiresAt,
		})
		if createErr != nil {
			return createErr
		}
		inv = invitationFromDB(i)
		return nil
	}
	if isInternalSeatRole(role) {
		if seatErr := s.withSeatMutation(ctx, workspaceID, create); seatErr != nil {
			return Invitation{}, seatErr
		}
	} else if createErr := create(s.queries); createErr != nil {
		return Invitation{}, createErr
	}

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

// InvitationPreview is the public (unauthenticated) view of an invitation token.
// Token holders already received the invite email; exposing email enables lock-to-invite on register/login.
type InvitationPreview struct {
	Email         string `json:"email"`
	Role          string `json:"role"`
	Status        string `json:"status"` // pending | expired | used
	ExpiresAt     string `json:"expires_at"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceSlug string `json:"workspace_slug"`
	WorkspaceName string `json:"workspace_name"`
}

const (
	InvitationStatusPending = "pending"
	InvitationStatusExpired = "expired"
	InvitationStatusUsed    = "used"
)

// PreviewInvitation returns invitation + workspace context for the accept UX.
// Does not require auth. Invalid tokens map to ErrInvitationNotFound.
func (s *Service) PreviewInvitation(ctx context.Context, token string) (InvitationPreview, error) {
	tokenUUID, err := pgUUID(token)
	if err != nil {
		return InvitationPreview{}, ErrInvitationNotFound
	}

	inv, err := s.queries.GetInvitationByToken(ctx, tokenUUID)
	if err != nil {
		return InvitationPreview{}, ErrInvitationNotFound
	}

	ws, err := s.getWorkspaceByID(ctx, uuidToString(inv.WorkspaceID), "")
	if err != nil {
		return InvitationPreview{}, ErrInvitationNotFound
	}

	status := InvitationStatusPending
	now := time.Now().UTC()
	switch {
	case inv.UsedAt.Valid:
		status = InvitationStatusUsed
	case inv.ExpiresAt.Valid && inv.ExpiresAt.Time.Before(now):
		status = InvitationStatusExpired
	}

	return InvitationPreview{
		Email:         strings.TrimSpace(inv.Email),
		Role:          inv.Role,
		Status:        status,
		ExpiresAt:     inv.ExpiresAt.Time.UTC().Format(time.RFC3339),
		WorkspaceID:   uuidToString(inv.WorkspaceID),
		WorkspaceSlug: ws.Slug,
		WorkspaceName: ws.Name,
	}, nil
}

// AcceptInvitationResult is returned after a successful invitation acceptance.
type AcceptInvitationResult struct {
	UserID        string `json:"user_id"`
	Role          string `json:"role"`
	JoinedAt      string `json:"joined_at"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceSlug string `json:"workspace_slug"`
	WorkspaceName string `json:"workspace_name"`
}

// AcceptInvitation uses a token to add a user to a workspace.
// Runs inside a transaction to prevent TOCTOU races on invitation usage.
// Concurrent accepts (e.g. React Strict Mode double-mount) are serialized with
// FOR UPDATE and treated as idempotent success when the caller is already a member.
func (s *Service) AcceptInvitation(ctx context.Context, token, userID string) (AcceptInvitationResult, error) {
	tokenUUID, err := pgUUID(token)
	if err != nil {
		return AcceptInvitationResult{}, ErrInvitationNotFound
	}

	if s.dbPool == nil {
		return AcceptInvitationResult{}, errors.New("accept invitation requires a database pool")
	}

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return AcceptInvitationResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.queries.WithTx(tx)

	inv, err := qtx.GetInvitationByTokenForUpdate(ctx, tokenUUID)
	if err != nil {
		return AcceptInvitationResult{}, ErrInvitationNotFound
	}
	if inv.ExpiresAt.Time.Before(time.Now().UTC()) {
		return AcceptInvitationResult{}, ErrInvitationExpired
	}

	workspaceID := uuidToString(inv.WorkspaceID)
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return AcceptInvitationResult{}, ErrInvitationNotFound
	}
	uUUID, err := pgUUID(userID)
	if err != nil {
		return AcceptInvitationResult{}, err
	}

	user, err := qtx.GetUserByID(ctx, uUUID)
	if err != nil {
		return AcceptInvitationResult{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(user.Email), strings.TrimSpace(inv.Email)) {
		return AcceptInvitationResult{}, ErrInvitationEmailMismatch
	}

	ws, err := qtx.GetWorkspaceByID(ctx, wsUUID)
	if err != nil {
		return AcceptInvitationResult{}, err
	}

	toResult := func(m Member) AcceptInvitationResult {
		return AcceptInvitationResult{
			UserID:        m.UserID,
			Role:          m.Role,
			JoinedAt:      m.JoinedAt,
			WorkspaceID:   workspaceID,
			WorkspaceSlug: ws.Slug,
			WorkspaceName: ws.Name,
		}
	}

	existing, err := qtx.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: wsUUID,
		UserID:      uUUID,
	})
	if err == nil {
		if !inv.UsedAt.Valid {
			if err := qtx.MarkInvitationUsed(ctx, tokenUUID); err != nil {
				return AcceptInvitationResult{}, fmt.Errorf("mark invitation used: %w", err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return AcceptInvitationResult{}, fmt.Errorf("commit tx: %w", err)
		}
		return toResult(memberFromDB(existing)), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AcceptInvitationResult{}, err
	}

	if inv.UsedAt.Valid {
		return AcceptInvitationResult{}, ErrInvitationUsed
	}

	// Serialize with CreateInvitation/AddMember so MarkInvitationUsed cannot
	// briefly drop the reserved seat count and let another invite squeeze in.
	if err := s.lockWorkspaceBillingSeatQuota(ctx, qtx, wsUUID); err != nil {
		return AcceptInvitationResult{}, err
	}

	// Pending internal invites already reserve a seat (counted in SeatsUsed).
	// Accept is net-zero when under/at cap; oversubscribed workspaces (e.g. after
	// downgrade) must free seats before the invitee can join as internal.
	if err := s.assertReservedInternalSeatAcceptable(ctx, qtx, workspaceID, inv.Role); err != nil {
		return AcceptInvitationResult{}, err
	}

	m, err := qtx.AddWorkspaceMember(ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: wsUUID,
		UserID:      uUUID,
		Role:        inv.Role,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			existing, getErr := qtx.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
				WorkspaceID: wsUUID,
				UserID:      uUUID,
			})
			if getErr != nil {
				return AcceptInvitationResult{}, err
			}
			if markErr := qtx.MarkInvitationUsed(ctx, tokenUUID); markErr != nil {
				return AcceptInvitationResult{}, fmt.Errorf("mark invitation used: %w", markErr)
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return AcceptInvitationResult{}, fmt.Errorf("commit tx: %w", commitErr)
			}
			return toResult(memberFromDB(existing)), nil
		}
		return AcceptInvitationResult{}, err
	}

	if err := qtx.MarkInvitationUsed(ctx, tokenUUID); err != nil {
		return AcceptInvitationResult{}, fmt.Errorf("mark invitation used: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return AcceptInvitationResult{}, fmt.Errorf("commit tx: %w", err)
	}
	return toResult(memberFromDB(m)), nil
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

	var m db.WorkspaceMember
	add := func(q *db.Queries) error {
		if isInternalSeatRole(role) {
			if seatErr := s.assertCanAddInternalSeatQ(ctx, q, workspaceID); seatErr != nil {
				return seatErr
			}
		}
		row, addErr := q.AddWorkspaceMember(ctx, db.AddWorkspaceMemberParams{
			WorkspaceID: wsUUID,
			UserID:      uUUID,
			Role:        role,
		})
		if addErr != nil {
			return addErr
		}
		m = row
		return nil
	}
	if isInternalSeatRole(role) {
		if seatErr := s.withSeatMutation(ctx, workspaceID, add); seatErr != nil {
			return Member{}, seatErr
		}
	} else if addErr := add(s.queries); addErr != nil {
		return Member{}, addErr
	}
	return memberFromDB(m), nil
}

// UpdateMemberRole changes an active member's role. Owner/admin only; owner rows and self are protected.
func (s *Service) UpdateMemberRole(ctx context.Context, actorID, workspaceID, tenantID, userID, role string) (Member, error) {
	actor, err := s.requireManager(ctx, actorID, workspaceID, tenantID)
	if err != nil {
		return Member{}, err
	}
	if !validMemberRole(role) {
		return Member{}, ErrInvalidRole
	}
	if actorID == userID {
		return Member{}, ErrCannotModifySelf
	}

	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Member{}, err
	}
	uUUID, err := pgUUID(userID)
	if err != nil {
		return Member{}, ErrMemberNotFound
	}

	target, err := s.queries.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: wsUUID,
		UserID:      uUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Member{}, ErrMemberNotFound
		}
		return Member{}, err
	}
	if err := canManageTargetRole(actor.Role, target.Role); err != nil {
		return Member{}, err
	}
	if err := canManageTargetRole(actor.Role, role); err != nil {
		return Member{}, err
	}
	consumesSeat := isInternalSeatRole(role) && !isInternalSeatRole(target.Role)
	freesSeat := !isInternalSeatRole(role) && isInternalSeatRole(target.Role)
	var m db.WorkspaceMember
	update := func(q *db.Queries) error {
		if consumesSeat {
			if seatErr := s.assertCanAddInternalSeatQ(ctx, q, workspaceID); seatErr != nil {
				return seatErr
			}
		}
		row, updErr := q.UpdateWorkspaceMemberRole(ctx, db.UpdateWorkspaceMemberRoleParams{
			WorkspaceID: wsUUID,
			UserID:      uUUID,
			Role:        role,
		})
		if updErr != nil {
			return updErr
		}
		m = row
		return nil
	}
	if consumesSeat || freesSeat {
		if seatErr := s.withSeatMutation(ctx, workspaceID, update); seatErr != nil {
			return Member{}, seatErr
		}
	} else if updErr := update(s.queries); updErr != nil {
		return Member{}, updErr
	}
	return memberFromDB(m), nil
}

// RemoveMember removes an active member. Owner/admin only; owner rows and self are protected.
func (s *Service) RemoveMember(ctx context.Context, actorID, workspaceID, tenantID, userID string) error {
	actor, err := s.requireManager(ctx, actorID, workspaceID, tenantID)
	if err != nil {
		return err
	}
	if actorID == userID {
		return ErrCannotModifySelf
	}

	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	uUUID, err := pgUUID(userID)
	if err != nil {
		return ErrMemberNotFound
	}

	target, err := s.queries.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: wsUUID,
		UserID:      uUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return err
	}
	if err := canManageTargetRole(actor.Role, target.Role); err != nil {
		return err
	}

	run := func(q *db.Queries) error {
		return q.DeleteWorkspaceMember(ctx, db.DeleteWorkspaceMemberParams{
			WorkspaceID: wsUUID,
			UserID:      uUUID,
		})
	}
	// Serialize seat-freeing removes with CreateInvitation/Accept so a freed
	// seat is visible to the next locked consumer (no false plan_limit_seats).
	if isInternalSeatRole(target.Role) {
		return s.withSeatMutation(ctx, workspaceID, run)
	}
	return run(s.queries)
}

// UpdateInvitationRole changes a pending invitation role. Owner/admin only.
func (s *Service) UpdateInvitationRole(ctx context.Context, actorID, workspaceID, tenantID, token, role string) (Invitation, error) {
	actor, err := s.requireManager(ctx, actorID, workspaceID, tenantID)
	if err != nil {
		return Invitation{}, err
	}
	if !validInvitationRole(role) {
		return Invitation{}, ErrInvalidRole
	}

	inv, err := s.pendingInvitationInWorkspace(ctx, workspaceID, token)
	if err != nil {
		return Invitation{}, err
	}
	if err := canManageTargetRole(actor.Role, inv.Role); err != nil {
		return Invitation{}, err
	}
	if err := canManageTargetRole(actor.Role, role); err != nil {
		return Invitation{}, err
	}
	consumesSeat := isInternalSeatRole(role) && !isInternalSeatRole(inv.Role)
	freesSeat := !isInternalSeatRole(role) && isInternalSeatRole(inv.Role)
	wsUUID, _ := pgUUID(workspaceID)
	tokenUUID, _ := pgUUID(token)
	var updated db.WorkspaceInvitation
	run := func(q *db.Queries) error {
		if consumesSeat {
			if seatErr := s.assertCanAddInternalSeatQ(ctx, q, workspaceID); seatErr != nil {
				return seatErr
			}
		}
		row, updErr := q.UpdatePendingWorkspaceInvitationRole(ctx, db.UpdatePendingWorkspaceInvitationRoleParams{
			WorkspaceID: wsUUID,
			Token:       tokenUUID,
			Role:        role,
		})
		if updErr != nil {
			return updErr
		}
		updated = row
		return nil
	}
	if consumesSeat || freesSeat {
		if seatErr := s.withSeatMutation(ctx, workspaceID, run); seatErr != nil {
			if errors.Is(seatErr, pgx.ErrNoRows) {
				return Invitation{}, ErrInvitationNotFound
			}
			return Invitation{}, seatErr
		}
	} else if runErr := run(s.queries); runErr != nil {
		if errors.Is(runErr, pgx.ErrNoRows) {
			return Invitation{}, ErrInvitationNotFound
		}
		return Invitation{}, runErr
	}
	return invitationFromDB(updated), nil
}

// RevokeInvitation deletes a pending invitation. Owner/admin only.
func (s *Service) RevokeInvitation(ctx context.Context, actorID, workspaceID, tenantID, token string) error {
	actor, err := s.requireManager(ctx, actorID, workspaceID, tenantID)
	if err != nil {
		return err
	}
	inv, err := s.pendingInvitationInWorkspace(ctx, workspaceID, token)
	if err != nil {
		return err
	}
	if err := canManageTargetRole(actor.Role, inv.Role); err != nil {
		return err
	}

	wsUUID, _ := pgUUID(workspaceID)
	tokenUUID, _ := pgUUID(token)
	run := func(q *db.Queries) error {
		return q.DeletePendingWorkspaceInvitation(ctx, db.DeletePendingWorkspaceInvitationParams{
			WorkspaceID: wsUUID,
			Token:       tokenUUID,
		})
	}
	// Pending internal invites reserve seats; lock so revoke + create serialize.
	if isInternalSeatRole(inv.Role) {
		return s.withSeatMutation(ctx, workspaceID, run)
	}
	return run(s.queries)
}

func (s *Service) requireManager(ctx context.Context, actorID, workspaceID, tenantID string) (db.WorkspaceMember, error) {
	actor, err := s.requireMember(ctx, actorID, workspaceID)
	if err != nil {
		return db.WorkspaceMember{}, err
	}
	if !validManagerRole(actor.Role) {
		return db.WorkspaceMember{}, ErrNotManager
	}
	if tenantID != "" {
		if err := s.requireWorkspaceInTenant(ctx, workspaceID, tenantID); err != nil {
			return db.WorkspaceMember{}, err
		}
	}
	return actor, nil
}

func (s *Service) pendingInvitationInWorkspace(ctx context.Context, workspaceID, token string) (db.WorkspaceInvitation, error) {
	tokenUUID, err := pgUUID(token)
	if err != nil {
		return db.WorkspaceInvitation{}, ErrInvitationNotFound
	}
	inv, err := s.queries.GetInvitationByToken(ctx, tokenUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.WorkspaceInvitation{}, ErrInvitationNotFound
		}
		return db.WorkspaceInvitation{}, err
	}
	if uuidToString(inv.WorkspaceID) != workspaceID {
		return db.WorkspaceInvitation{}, ErrInvitationNotFound
	}
	if inv.UsedAt.Valid {
		return db.WorkspaceInvitation{}, ErrInvitationUsed
	}
	return inv, nil
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
