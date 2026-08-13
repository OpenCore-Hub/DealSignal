package workspace

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/billing"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCreateWorkspaceInvalidSlug(t *testing.T) {
	svc := NewService(db.New(&fakeDB{t: t}))
	_, err := svc.Create(context.Background(), uuid.NewString(), "Demo", "my workspace!", "")
	if !errors.Is(err, ErrInvalidSlug) {
		t.Fatalf("expected ErrInvalidSlug, got %v", err)
	}
}

func TestCreateWorkspace(t *testing.T) {
	fake := &fakeDB{t: t}
	svc := NewService(db.New(fake))
	userID := uuid.NewString()

	ws, err := svc.Create(context.Background(), userID, "Demo Capital", "demo-capital", "#ff0000")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if ws.Name != "Demo Capital" {
		t.Fatalf("expected name Demo Capital, got %s", ws.Name)
	}
	if ws.Slug != "demo-capital" {
		t.Fatalf("expected slug demo-capital, got %s", ws.Slug)
	}
	if ws.BrandColor != "#ff0000" {
		t.Fatalf("expected brand color #ff0000, got %s", ws.BrandColor)
	}
	if fake.billing.PlanCode != "trial" || fake.billing.Period != "monthly" {
		t.Fatalf("expected trial billing row, got %+v", fake.billing)
	}
	if !fake.billing.TrialEndsAt.Valid {
		t.Fatal("expected trial_ends_at on new workspace")
	}
	if !fake.trialGrantedAt.Valid {
		t.Fatal("first owned workspace must stamp trial_granted_at")
	}
}

func TestCreateSecondWorkspaceHitsFreeCap(t *testing.T) {
	fake := &fakeDB{t: t, ownedCount: 1}
	svc := NewService(db.New(fake))
	if _, err := svc.Create(context.Background(), uuid.NewString(), "Second", "second-ws", ""); !errors.Is(err, plan.ErrLimitWorkspaces) {
		t.Fatalf("expected ErrLimitWorkspaces, got %v", err)
	}
	if fake.billing.PlanCode != "" {
		t.Fatal("denied create must not insert billing")
	}
}

func TestCreateSecondWorkspaceOnProIsFree(t *testing.T) {
	fake := &fakeDB{
		t:          t,
		ownedCount: 1,
		ownedBilling: []db.WorkspaceBilling{{
			PlanCode: plan.CodePro,
			Period:   plan.PeriodMonthly,
		}},
	}
	svc := NewService(db.New(fake))
	if _, err := svc.Create(context.Background(), uuid.NewString(), "Second", "second-pro-ws", ""); err != nil {
		t.Fatalf("pro second workspace: %v", err)
	}
	if fake.billing.PlanCode != plan.CodeFree {
		t.Fatalf("extra owned workspace must be free, got %+v", fake.billing)
	}
	if fake.billing.TrialEndsAt.Valid {
		t.Fatal("extra workspace must not receive a trial clock")
	}
}

func TestCreateSecondWorkspaceOnProUnverified(t *testing.T) {
	fake := &fakeDB{
		t:               t,
		ownedCount:      1,
		unverifiedEmail: true,
		ownedBilling: []db.WorkspaceBilling{{
			PlanCode: plan.CodePro,
			Period:   plan.PeriodMonthly,
		}},
	}
	svc := NewService(db.New(fake))
	if _, err := svc.Create(context.Background(), uuid.NewString(), "Second", "second-unverified", ""); !errors.Is(err, ErrEmailUnverified) {
		t.Fatalf("expected ErrEmailUnverified, got %v", err)
	}
}

func TestCreateAfterTrialGrantedIsFree(t *testing.T) {
	fake := &fakeDB{
		t:              t,
		trialGrantedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	svc := NewService(db.New(fake))
	if _, err := svc.Create(context.Background(), uuid.NewString(), "Remint", "remint-ws", ""); err != nil {
		t.Fatalf("create after trial grant: %v", err)
	}
	if fake.billing.PlanCode != plan.CodeFree {
		t.Fatalf("remint after leave must be free, got %+v", fake.billing)
	}
	if fake.billing.TrialEndsAt.Valid {
		t.Fatal("remint after leave must not receive a trial clock")
	}
}

func TestCreateFirstOwnedIgnoresMembershipElsewhere(t *testing.T) {
	// Invite-join as member/admin must not consume OwnedWorkspaces; first owned still trials.
	fake := &fakeDB{t: t, ownedCount: 0}
	svc := NewService(db.New(fake))
	if _, err := svc.Create(context.Background(), uuid.NewString(), "Own", "own-ws", ""); err != nil {
		t.Fatalf("first owned after invite-join must succeed: %v", err)
	}
	if fake.billing.PlanCode != plan.CodeTrial {
		t.Fatalf("first owned after invite-join must be trial, got %+v", fake.billing)
	}
}

func TestAddMemberDoesNotConsumeInviteeOwnedCap(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{
		t:           t,
		memberRole:  RoleOwner,
		actorUserID: actorID,
		ownedCount:  1,
		billing:     db.WorkspaceBilling{PlanCode: plan.CodePro, Period: plan.PeriodMonthly},
	}
	svc := NewService(db.New(fake))
	if _, err := svc.AddMember(context.Background(), actorID, uuid.NewString(), "", uuid.NewString(), RoleMember); err != nil {
		t.Fatalf("AddMember must use inviter seats, not invitee owned cap: %v", err)
	}
	if _, err := svc.AddMember(context.Background(), actorID, uuid.NewString(), "", uuid.NewString(), RoleGuest); err != nil {
		t.Fatalf("guest must stay unlimited: %v", err)
	}
}

func TestUpdateMemberRolePromoteDoesNotConsumeOwnedCap(t *testing.T) {
	actorID := uuid.NewString()
	targetID := uuid.NewString()
	fake := &fakeDB{
		t:            t,
		memberRole:   RoleOwner,
		actorUserID:  actorID,
		targetUserID: targetID,
		targetRole:   RoleGuest,
		ownedCount:   1,
		billing:      db.WorkspaceBilling{PlanCode: plan.CodePro, Period: plan.PeriodMonthly},
	}
	svc := NewService(db.New(fake))
	if _, err := svc.UpdateMemberRole(context.Background(), actorID, uuid.NewString(), "", targetID, RoleMember); err != nil {
		t.Fatalf("promote guest to member must use workspace seats, not invitee owned cap: %v", err)
	}
}

func TestCreateInvitationDoesNotConsumeInviteeOwnedCap(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{
		t:            t,
		memberRole:   RoleOwner,
		actorUserID:  actorID,
		ownedCount:   1,
		inviteeEmail: "taken@example.test",
		billing:      db.WorkspaceBilling{PlanCode: plan.CodePro, Period: plan.PeriodMonthly},
	}
	svc := NewService(db.New(fake))
	if _, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "taken@example.test", RoleMember, 7); err != nil {
		t.Fatalf("invite existing owner as member must succeed: %v", err)
	}
	if _, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7); err != nil {
		t.Fatalf("guest invite must stay unlimited: %v", err)
	}
}

func TestCreateAtProOwnedCap(t *testing.T) {
	fake := &fakeDB{
		t:          t,
		ownedCount: 3,
		ownedBilling: []db.WorkspaceBilling{{
			PlanCode: plan.CodePro,
			Period:   plan.PeriodMonthly,
		}},
	}
	svc := NewService(db.New(fake))
	if _, err := svc.Create(context.Background(), uuid.NewString(), "Fourth", "fourth-ws", ""); !errors.Is(err, plan.ErrLimitWorkspaces) {
		t.Fatalf("expected ErrLimitWorkspaces at pro cap, got %v", err)
	}
}

func TestChangePlanGates(t *testing.T) {
	svc := NewService(db.New(&fakeDB{t: t}))
	_, err := svc.ChangePlan(context.Background(), uuid.NewString(), plan.CodePro, plan.PeriodMonthly)
	if !errors.Is(err, ErrPlanPaymentRequired) {
		t.Fatalf("unpaid pro: %v", err)
	}
	_, err = svc.ChangePlan(context.Background(), uuid.NewString(), plan.CodeEnterprise, plan.PeriodMonthly)
	if !errors.Is(err, ErrPlanSalesAssisted) {
		t.Fatalf("enterprise: %v", err)
	}
	_, err = svc.ChangePlan(context.Background(), uuid.NewString(), plan.CodeTrial, plan.PeriodMonthly)
	if !errors.Is(err, ErrInvalidPlanCode) {
		t.Fatalf("trial select: %v", err)
	}

	paid := NewService(db.New(&fakeDB{t: t, billing: db.WorkspaceBilling{
		WorkspaceID:          newPGUUID(),
		PlanCode:             plan.CodePro,
		Period:               plan.PeriodMonthly,
		StripeSubscriptionID: pgtype.Text{String: "sub_live", Valid: true},
		BillingStatus:        pgtype.Text{String: billing.StatusActive, Valid: true},
	}}))
	_, err = paid.ChangePlan(context.Background(), uuid.NewString(), plan.CodeFree, plan.PeriodMonthly)
	if !errors.Is(err, ErrPlanManageViaPortal) {
		t.Fatalf("active stripe free: %v", err)
	}
}

func TestListWorkspaces(t *testing.T) {
	userID := uuid.NewString()
	wsID := uuid.New()
	tenantID := uuid.New()
	now := time.Now()
	fake := &fakeDB{
		t: t,
		listRows: []db.ListWorkspacesByUserRow{
			{
				ID:         pgtype.UUID{Bytes: wsID, Valid: true},
				TenantID:   pgtype.UUID{Bytes: tenantID, Valid: true},
				Name:       "Demo Capital",
				Slug:       "demo-capital",
				BrandColor: pgtype.Text{String: "#ff0000", Valid: true},
				CreatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
				Role:       RoleOwner,
			},
		},
	}
	svc := NewService(db.New(fake))

	workspaces, err := svc.List(context.Background(), userID)
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(workspaces))
	}
	if workspaces[0].Role != RoleOwner {
		t.Fatalf("expected role owner, got %s", workspaces[0].Role)
	}
}

func TestGetWorkspaceNotMember(t *testing.T) {
	fake := &fakeDB{t: t}
	svc := NewService(db.New(fake))

	_, err := svc.Get(context.Background(), uuid.NewString(), uuid.NewString(), "")
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestAddMemberInvalidRole(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	_, err := svc.AddMember(context.Background(), actorID, uuid.NewString(), "", uuid.NewString(), "superuser")
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if err.Error() != "invalid role" {
		t.Fatalf("expected invalid role error, got %v", err)
	}
}

func TestAddMemberSuccess(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	member, err := svc.AddMember(context.Background(), actorID, uuid.NewString(), "", uuid.NewString(), RoleAdmin)
	if err != nil {
		t.Fatalf("add member: %v", err)
	}
	if member.Role != RoleAdmin {
		t.Fatalf("expected role admin, got %s", member.Role)
	}
}

// fakeDB is a minimal in-memory implementation of db.DBTX for service tests.
type fakeDB struct {
	t               *testing.T
	memberRole      string
	actorUserID     string
	actorEmail      string
	lookupUserEmail string
	memberEmail     string
	memberUserID    string
	targetUserID    string
	targetRole      string
	listRows        []db.ListWorkspacesByUserRow

	tenant           db.Tenant
	workspace        db.Workspace
	member           db.WorkspaceMember
	invitation       db.WorkspaceInvitation
	viewerDomain     db.WorkspaceViewerDomain
	billing          db.WorkspaceBilling
	billingLookupErr error
	storageUsage     int64
	linksCount       int
	roomsCount       int
	seatsCount       int
	docsCount        int64
	askTurns         int32
	knowledgeAnswers int64
	ownedCount       int64
	ownedBilling     []db.WorkspaceBilling
	unverifiedEmail  bool
	trialGrantedAt   pgtype.Timestamptz
	inviteeEmail     string
}

func (f *fakeDB) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	sqlLower := strings.ToLower(sql)
	if strings.Contains(sqlLower, "delete from workspace_invitations") {
		f.invitation = db.WorkspaceInvitation{}
	}
	if strings.Contains(sqlLower, "delete from workspace_members") {
		f.targetUserID = ""
		f.targetRole = ""
	}
	if strings.Contains(sqlLower, "delete from workspace_viewer_domains") {
		f.viewerDomain = db.WorkspaceViewerDomain{}
	}
	if strings.Contains(sqlLower, "update workspace_invitations") && strings.Contains(sqlLower, "used_at") {
		f.invitation.UsedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	sqlLower := strings.ToLower(sql)

	if strings.Contains(sqlLower, "from links") {
		row := make([]interface{}, 22)
		rows := make([][]interface{}, f.linksCount)
		for i := range rows {
			rows[i] = row
		}
		return &fakeRows{rows: rows}, nil
	}

	if strings.Contains(sqlLower, "from deal_rooms") {
		row := make([]interface{}, 15)
		rows := make([][]interface{}, f.roomsCount)
		for i := range rows {
			rows[i] = row
		}
		return &fakeRows{rows: rows}, nil
	}

	if strings.Contains(sqlLower, "from workspace_members wm") && strings.Contains(sqlLower, "workspace_billing") {
		rows := make([][]interface{}, len(f.ownedBilling))
		for i, r := range f.ownedBilling {
			rows[i] = billingScanValues(r)
		}
		return &fakeRows{rows: rows}, nil
	}

	rows := make([][]interface{}, len(f.listRows))
	for i, r := range f.listRows {
		rows[i] = []interface{}{
			r.ID, r.TenantID, r.Name, r.Slug, r.BrandColor, r.ForceEmailVerification, r.WatermarkDownloads, r.TwoFactorEnabled, r.CreatedAt, r.Role,
		}
	}
	return &fakeRows{rows: rows}, nil
}

func (f *fakeDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	sqlLower := strings.ToLower(sql)

	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	switch {
	case strings.Contains(sqlLower, "from tenant_domains"):
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "insert into workspace_viewer_domains"):
		f.viewerDomain = db.WorkspaceViewerDomain{
			WorkspaceID: argUUID(args, 0),
			Hostname:    argString(args, 1),
			Status:      "pending",
			CnameTarget: argString(args, 2),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		return fakeRow{values: []interface{}{
			f.viewerDomain.WorkspaceID, f.viewerDomain.Hostname, f.viewerDomain.Status,
			f.viewerDomain.CnameTarget, f.viewerDomain.VerifiedAt, f.viewerDomain.CreatedAt,
			f.viewerDomain.UpdatedAt,
		}}

	case strings.Contains(sqlLower, "update workspace_viewer_domains"):
		if !f.viewerDomain.WorkspaceID.Valid {
			return fakeRow{err: pgx.ErrNoRows}
		}
		f.viewerDomain.Status = "verified"
		f.viewerDomain.VerifiedAt = now
		f.viewerDomain.UpdatedAt = now
		return fakeRow{values: []interface{}{
			f.viewerDomain.WorkspaceID, f.viewerDomain.Hostname, f.viewerDomain.Status,
			f.viewerDomain.CnameTarget, f.viewerDomain.VerifiedAt, f.viewerDomain.CreatedAt,
			f.viewerDomain.UpdatedAt,
		}}

	case strings.Contains(sqlLower, "from workspace_viewer_domains"):
		if strings.Contains(sqlLower, "join workspaces") {
			if f.viewerDomain.Hostname == "" || !strings.EqualFold(f.viewerDomain.Hostname, argString(args, 0)) {
				return fakeRow{err: pgx.ErrNoRows}
			}
			tenantID := f.workspace.TenantID
			if !tenantID.Valid {
				tenantID = newPGUUID()
				f.workspace.TenantID = tenantID
			}
			return fakeRow{values: []interface{}{
				f.viewerDomain.WorkspaceID, f.viewerDomain.Hostname, f.viewerDomain.Status,
				f.viewerDomain.CnameTarget, f.viewerDomain.VerifiedAt, f.viewerDomain.CreatedAt,
				f.viewerDomain.UpdatedAt, tenantID,
			}}
		}
		if !f.viewerDomain.WorkspaceID.Valid {
			return fakeRow{err: pgx.ErrNoRows}
		}
		return fakeRow{values: []interface{}{
			f.viewerDomain.WorkspaceID, f.viewerDomain.Hostname, f.viewerDomain.Status,
			f.viewerDomain.CnameTarget, f.viewerDomain.VerifiedAt, f.viewerDomain.CreatedAt,
			f.viewerDomain.UpdatedAt,
		}}

	case strings.Contains(sqlLower, "insert into tenants"):
		f.tenant = db.Tenant{
			ID:        newPGUUID(),
			Name:      argString(args, 0),
			Slug:      pgtype.Text{String: argString(args, 1), Valid: true},
			CreatedAt: now,
		}
		return fakeRow{values: []interface{}{f.tenant.ID, f.tenant.Name, f.tenant.Slug, f.tenant.CreatedAt}}

	case strings.Contains(sqlLower, "insert into workspaces"):
		f.workspace = db.Workspace{
			ID:                     newPGUUID(),
			TenantID:               argUUID(args, 0),
			Name:                   argString(args, 1),
			Slug:                   argString(args, 2),
			BrandColor:             argText(args, 3),
			ForceEmailVerification: false,
			WatermarkDownloads:     false,
			TwoFactorEnabled:       false,
			CreatedAt:              now,
		}
		return fakeRow{values: []interface{}{f.workspace.ID, f.workspace.TenantID, f.workspace.Name, f.workspace.Slug, f.workspace.BrandColor, f.workspace.CreatedAt, f.workspace.ForceEmailVerification, f.workspace.WatermarkDownloads, f.workspace.TwoFactorEnabled, f.workspace.CrmConfig, f.workspace.WebhookSecret}}

	case strings.Contains(sqlLower, "insert into workspace_billing"):
		f.billing = db.WorkspaceBilling{
			WorkspaceID: argUUID(args, 0),
			PlanCode:    argString(args, 1),
			Period:      argString(args, 2),
			TrialEndsAt: argTimestamptz(args, 3),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		return fakeRow{values: billingScanValues(f.billing)}

	case strings.Contains(sqlLower, "from workspace_billing"):
		if f.billingLookupErr != nil {
			return fakeRow{err: f.billingLookupErr}
		}
		if f.billing.PlanCode == "" {
			return fakeRow{err: pgx.ErrNoRows}
		}
		return fakeRow{values: billingScanValues(f.billing)}

	case strings.Contains(sqlLower, "from workspace_members") && strings.Contains(sqlLower, "role = 'owner'") && strings.Contains(sqlLower, "count(") && !strings.Contains(sqlLower, "workspace_invitations"):
		return fakeRow{values: []interface{}{f.ownedCount}}

	case strings.Contains(sqlLower, "from workspace_members wm") && strings.Contains(sqlLower, "from workspace_invitations wi"):
		return fakeRow{values: []interface{}{int64(f.seatsCount)}}

	case strings.Contains(sqlLower, "insert into workspace_members"):
		f.member = db.WorkspaceMember{
			WorkspaceID: argUUID(args, 0),
			UserID:      argUUID(args, 1),
			Role:        argString(args, 2),
			JoinedAt:    now,
		}
		return fakeRow{values: []interface{}{f.member.WorkspaceID, f.member.UserID, f.member.Role, f.member.JoinedAt}}

	case strings.Contains(sqlLower, "insert into workspace_invitations"):
		f.invitation = db.WorkspaceInvitation{
			Token:       newPGUUID(),
			WorkspaceID: argUUID(args, 0),
			Email:       argString(args, 1),
			Role:        argString(args, 2),
			ExpiresAt:   argTimestamptz(args, 3),
			CreatedAt:   now,
		}
		return fakeRow{values: []interface{}{f.invitation.Token, f.invitation.WorkspaceID, f.invitation.Email, f.invitation.Role, f.invitation.ExpiresAt, f.invitation.UsedAt, f.invitation.CreatedAt}}

	case strings.Contains(sqlLower, "update workspace_invitations") && strings.Contains(sqlLower, "used_at is null") && !strings.Contains(sqlLower, "gen_random_uuid"):
		if !f.invitation.Token.Valid || f.invitation.UsedAt.Valid {
			return fakeRow{err: pgx.ErrNoRows}
		}
		f.invitation.Role = argString(args, 2)
		return fakeRow{values: []interface{}{f.invitation.Token, f.invitation.WorkspaceID, f.invitation.Email, f.invitation.Role, f.invitation.ExpiresAt, f.invitation.UsedAt, f.invitation.CreatedAt}}

	case strings.Contains(sqlLower, "update workspace_invitations") && strings.Contains(sqlLower, "used_at is null"):
		if f.invitation.Email == "" || f.invitation.UsedAt.Valid || !strings.EqualFold(f.invitation.Email, argString(args, 1)) {
			return fakeRow{err: pgx.ErrNoRows}
		}
		f.invitation.Role = argString(args, 2)
		f.invitation.ExpiresAt = argTimestamptz(args, 3)
		f.invitation.Token = newPGUUID()
		f.invitation.CreatedAt = now
		return fakeRow{values: []interface{}{f.invitation.Token, f.invitation.WorkspaceID, f.invitation.Email, f.invitation.Role, f.invitation.ExpiresAt, f.invitation.UsedAt, f.invitation.CreatedAt}}

	case strings.Contains(sqlLower, "update workspace_members"):
		if f.targetUserID == "" || !bytesEqual(argUUID(args, 1).Bytes, pgUUIDFromString(f.targetUserID).Bytes) {
			return fakeRow{err: pgx.ErrNoRows}
		}
		f.targetRole = argString(args, 2)
		return fakeRow{values: []interface{}{argUUID(args, 0), argUUID(args, 1), f.targetRole, now}}

	case strings.Contains(sqlLower, "from workspace_invitations") && strings.Contains(sqlLower, "email = $2"):
		if f.invitation.Email == "" || !strings.EqualFold(f.invitation.Email, argString(args, 1)) {
			return fakeRow{err: pgx.ErrNoRows}
		}
		return fakeRow{values: []interface{}{f.invitation.Token, f.invitation.WorkspaceID, f.invitation.Email, f.invitation.Role, f.invitation.ExpiresAt, f.invitation.UsedAt, f.invitation.CreatedAt}}

	case strings.Contains(sqlLower, "from workspace_invitations"):
		if !f.invitation.Token.Valid {
			return fakeRow{err: pgx.ErrNoRows}
		}
		return fakeRow{values: []interface{}{f.invitation.Token, f.invitation.WorkspaceID, f.invitation.Email, f.invitation.Role, f.invitation.ExpiresAt, f.invitation.UsedAt, f.invitation.CreatedAt}}

	case strings.Contains(sqlLower, "from workspaces") && strings.Contains(sqlLower, "where id = $1 limit"):
		return fakeRow{values: []interface{}{f.workspace.ID, f.workspace.TenantID, f.workspace.Name, f.workspace.Slug, f.workspace.BrandColor, f.workspace.CreatedAt, f.workspace.ForceEmailVerification, f.workspace.WatermarkDownloads, f.workspace.TwoFactorEnabled, f.workspace.CrmConfig, f.workspace.WebhookSecret}}

	case strings.Contains(sqlLower, "update workspaces") && strings.Contains(sqlLower, "watermark_downloads"):
		f.workspace.ForceEmailVerification = argBool(args, 0)
		f.workspace.WatermarkDownloads = argBool(args, 1)
		f.workspace.TwoFactorEnabled = argBool(args, 2)
		if !f.workspace.ID.Valid {
			f.workspace.ID = argUUID(args, 3)
		}
		return fakeRow{values: []interface{}{f.workspace.ID, f.workspace.TenantID, f.workspace.Name, f.workspace.Slug, f.workspace.BrandColor, f.workspace.CreatedAt, f.workspace.ForceEmailVerification, f.workspace.WatermarkDownloads, f.workspace.TwoFactorEnabled, f.workspace.CrmConfig, f.workspace.WebhookSecret}}

	case strings.Contains(sqlLower, "from workspaces") && strings.Contains(sqlLower, "where id = $1 and tenant_id"):
		return fakeRow{values: []interface{}{f.workspace.ID, f.workspace.TenantID, f.workspace.Name, f.workspace.Slug, f.workspace.BrandColor, f.workspace.CreatedAt, f.workspace.ForceEmailVerification, f.workspace.WatermarkDownloads, f.workspace.TwoFactorEnabled, f.workspace.CrmConfig, f.workspace.WebhookSecret}}

	case strings.Contains(sqlLower, "from workspace_logos"), strings.Contains(sqlLower, "into workspace_logos"):
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from workspaces") && strings.Contains(sqlLower, "where slug"):
		return fakeRow{values: []interface{}{f.workspace.ID, f.workspace.TenantID, f.workspace.Name, f.workspace.Slug, f.workspace.BrandColor, f.workspace.CreatedAt, f.workspace.ForceEmailVerification, f.workspace.WatermarkDownloads, f.workspace.TwoFactorEnabled, f.workspace.CrmConfig, f.workspace.WebhookSecret}}

	case strings.Contains(sqlLower, "from workspace_members wm") && strings.Contains(sqlLower, "u.email = $2"):
		if f.memberEmail == "" || !strings.EqualFold(argString(args, 1), f.memberEmail) {
			return fakeRow{err: pgx.ErrNoRows}
		}
		userID := pgUUIDFromString(f.memberUserID)
		if f.memberUserID == "" {
			userID = newPGUUID()
		}
		return fakeRow{values: []interface{}{
			argUUID(args, 0),
			userID,
			RoleMember,
			now,
			f.memberEmail,
		}}

	case strings.Contains(sqlLower, "from workspace_members") && strings.Contains(sqlLower, "where workspace_id"):
		userArg := argUUID(args, 1)
		if f.memberRole != "" && bytesEqual(userArg.Bytes, pgUUIDFromString(f.actorUserID).Bytes) {
			f.member = db.WorkspaceMember{
				WorkspaceID: argUUID(args, 0),
				UserID:      userArg,
				Role:        f.memberRole,
				JoinedAt:    now,
			}
			return fakeRow{values: []interface{}{f.member.WorkspaceID, f.member.UserID, f.member.Role, f.member.JoinedAt}}
		}
		if f.targetUserID != "" && bytesEqual(userArg.Bytes, pgUUIDFromString(f.targetUserID).Bytes) {
			role := f.targetRole
			if role == "" {
				role = RoleMember
			}
			return fakeRow{values: []interface{}{argUUID(args, 0), userArg, role, now}}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "count(*)") && strings.Contains(sqlLower, "from links"):
		return fakeRow{values: []interface{}{int64(f.linksCount)}}

	case strings.Contains(sqlLower, "count(*)") && strings.Contains(sqlLower, "from deal_rooms"):
		return fakeRow{values: []interface{}{int64(f.roomsCount)}}

	case strings.Contains(sqlLower, "count(*)") && strings.Contains(sqlLower, "from documents"):
		return fakeRow{values: []interface{}{f.docsCount}}

	case strings.Contains(sqlLower, "from link_ask_turns"):
		return fakeRow{values: []interface{}{f.askTurns}}

	case strings.Contains(sqlLower, "from knowledge_qa_turns"):
		return fakeRow{values: []interface{}{f.knowledgeAnswers}}

	case strings.Contains(sqlLower, "sum(d.file_size)"):
		return fakeRow{values: []interface{}{f.storageUsage}}

	case strings.Contains(sqlLower, "update users") && strings.Contains(sqlLower, "trial_granted_at"):
		if f.trialGrantedAt.Valid {
			return fakeRow{err: pgx.ErrNoRows}
		}
		f.trialGrantedAt = now
		return fakeRow{values: []interface{}{argUUID(args, 0), f.trialGrantedAt}}

	case strings.Contains(sqlLower, "from users") && strings.Contains(sqlLower, "where email"):
		email := argString(args, 0)
		if f.inviteeEmail == "" || !strings.EqualFold(f.inviteeEmail, email) {
			return fakeRow{err: pgx.ErrNoRows}
		}
		return fakeRow{values: []interface{}{
			newPGUUID(),
			email,
			"",
			now,
			true,
			pgtype.Timestamptz{},
		}}

	case strings.Contains(sqlLower, "from users") && strings.Contains(sqlLower, "where id = $1 limit"):
		email := f.actorEmail
		if f.lookupUserEmail != "" {
			email = f.lookupUserEmail
		} else if email == "" {
			email = "actor@example.test"
		}
		return fakeRow{values: []interface{}{
			argUUID(args, 0),
			email,
			"",
			now,
			!f.unverifiedEmail,
			f.trialGrantedAt,
		}}
	}

	return fakeRow{err: errors.New("unexpected query")}
}

func billingScanValues(row db.WorkspaceBilling) []interface{} {
	return []interface{}{
		row.WorkspaceID, row.PlanCode, row.Period, row.TrialEndsAt, row.CreatedAt, row.UpdatedAt,
		row.StripeCustomerID, row.StripeSubscriptionID, row.StripePriceID,
		row.BillingStatus, row.CurrentPeriodEnd, row.PastDueAt,
	}
}

type fakeRow struct {
	values []interface{}
	err    error
}

func (r fakeRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan count mismatch: got %d, want %d", len(dest), len(r.values))
	}
	for i, v := range r.values {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr {
			return fmt.Errorf("destination is not a pointer")
		}
		sv := reflect.ValueOf(v)
		if !sv.Type().AssignableTo(dv.Elem().Type()) {
			return fmt.Errorf("cannot assign %s to %s", sv.Type(), dv.Elem().Type())
		}
		dv.Elem().Set(sv)
	}
	return nil
}

// fakeTx wraps fakeDB as a pgx.Tx implementation for AcceptInvitation tests.
type fakeTx struct {
	*fakeDB
}

func (ft *fakeTx) Begin(ctx context.Context) (pgx.Tx, error) { return ft, nil }
func (ft *fakeTx) Commit(ctx context.Context) error          { return nil }
func (ft *fakeTx) Rollback(ctx context.Context) error        { return nil }
func (ft *fakeTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (ft *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (ft *fakeTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (ft *fakeTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (ft *fakeTx) Conn() *pgx.Conn { return nil }

// Begin satisfies the Beginner interface for transaction-based tests.
func (f *fakeDB) Begin(ctx context.Context) (pgx.Tx, error) {
	return &fakeTx{fakeDB: f}, nil
}

type fakeRows struct {
	rows [][]interface{}
	pos  int
}

func (r *fakeRows) Next() bool                                   { return r.pos < len(r.rows) }
func (r *fakeRows) Err() error                                   { return nil }
func (r *fakeRows) Close()                                       {}
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeRows) Scan(dest ...interface{}) error {
	if r.pos >= len(r.rows) {
		return pgx.ErrNoRows
	}
	row := r.rows[r.pos]
	r.pos++
	if len(dest) != len(row) {
		return fmt.Errorf("scan count mismatch: got %d, want %d", len(dest), len(row))
	}
	for i, v := range row {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr {
			return fmt.Errorf("destination is not a pointer")
		}
		sv := reflect.ValueOf(v)
		if !sv.Type().AssignableTo(dv.Elem().Type()) {
			return fmt.Errorf("cannot assign %s to %s", sv.Type(), dv.Elem().Type())
		}
		dv.Elem().Set(sv)
	}
	return nil
}

func newPGUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.New(), Valid: true}
}

func argString(args []interface{}, i int) string {
	if i >= len(args) {
		return ""
	}
	if s, ok := args[i].(string); ok {
		return s
	}
	if t, ok := args[i].(pgtype.Text); ok {
		return t.String
	}
	return ""
}

func argBool(args []interface{}, i int) bool {
	if i >= len(args) {
		return false
	}
	if b, ok := args[i].(bool); ok {
		return b
	}
	return false
}

func argUUID(args []interface{}, i int) pgtype.UUID {
	if i >= len(args) {
		return pgtype.UUID{}
	}
	if u, ok := args[i].(pgtype.UUID); ok {
		return u
	}
	return pgtype.UUID{}
}

func argText(args []interface{}, i int) pgtype.Text {
	if i >= len(args) {
		return pgtype.Text{}
	}
	if t, ok := args[i].(pgtype.Text); ok {
		return t
	}
	return pgtype.Text{}
}

func pgUUIDFromString(s string) pgtype.UUID {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

func activeTrialBilling() db.WorkspaceBilling {
	return db.WorkspaceBilling{
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour), Valid: true},
	}
}

func bytesEqual(a, b [16]byte) bool {
	return a == b
}

func TestAddMemberRequiresManager(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleMember, actorUserID: actorID}
	svc := NewService(db.New(fake))

	_, err := svc.AddMember(context.Background(), actorID, uuid.NewString(), "", uuid.NewString(), RoleAdmin)
	if !errors.Is(err, ErrNotManager) {
		t.Fatalf("expected ErrNotManager, got %v", err)
	}
}

func TestGuestRoleValid(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	member, err := svc.AddMember(context.Background(), actorID, uuid.NewString(), "", uuid.NewString(), RoleGuest)
	if err != nil {
		t.Fatalf("add guest member: %v", err)
	}
	if member.Role != RoleGuest {
		t.Fatalf("expected role guest, got %s", member.Role)
	}
}

func TestCreateInvitationRequiresManager(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleMember, actorUserID: actorID}
	svc := NewService(db.New(fake))

	_, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7)
	if !errors.Is(err, ErrNotManager) {
		t.Fatalf("expected ErrNotManager, got %v", err)
	}
}

func TestCreateInvitationInvalidRole(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	_, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", "superuser", 7)
	if !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
}

func TestCreateInvitationSuccessNormalizesEmail(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	inv, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "  Guest@Example.TEST ", RoleMember, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	if inv.Email != "guest@example.test" {
		t.Fatalf("expected normalized email, got %q", inv.Email)
	}
	if inv.Role != RoleMember {
		t.Fatalf("expected role member, got %s", inv.Role)
	}
}

func TestCreateInvitationAlreadyMember(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{
		t:           t,
		memberRole:  RoleOwner,
		actorUserID: actorID,
		memberEmail: "member@example.test",
	}
	svc := NewService(db.New(fake))

	_, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "member@example.test", RoleMember, 7)
	if !errors.Is(err, ErrAlreadyMember) {
		t.Fatalf("expected ErrAlreadyMember, got %v", err)
	}
}

func TestCreateInvitationInvalidEmail(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	_, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "not-an-email", RoleMember, 7)
	if !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}
}

func TestCreateInvitationResendsPending(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))
	wsID := uuid.NewString()

	first, err := svc.CreateInvitation(context.Background(), actorID, wsID, "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("first invite: %v", err)
	}

	second, err := svc.CreateInvitation(context.Background(), actorID, wsID, "", "guest@example.test", RoleMember, 14)
	if err != nil {
		t.Fatalf("resend invite: %v", err)
	}
	if second.Role != RoleMember {
		t.Fatalf("expected resent role member, got %s", second.Role)
	}
	if second.Token == first.Token {
		t.Fatal("expected resent invitation to rotate token")
	}
	if fake.invitation.UsedAt.Valid {
		t.Fatal("pending resend must not clear or set used_at")
	}
}

func TestCreateInvitationReplacesUsedInvitation(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))
	wsID := uuid.NewString()

	first, err := svc.CreateInvitation(context.Background(), actorID, wsID, "", "alumni@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("first invite: %v", err)
	}
	fake.invitation.UsedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}

	second, err := svc.CreateInvitation(context.Background(), actorID, wsID, "", "alumni@example.test", RoleMember, 7)
	if err != nil {
		t.Fatalf("re-invite after used: %v", err)
	}
	if second.Token == first.Token {
		t.Fatal("expected new invitation token after used invite was replaced")
	}
	if second.Role != RoleMember {
		t.Fatalf("expected role member, got %s", second.Role)
	}
	if fake.invitation.UsedAt.Valid {
		t.Fatal("replacement invitation must be unused")
	}
}

func TestUpdateInvitationRole(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))
	wsID := uuid.NewString()

	inv, err := svc.CreateInvitation(context.Background(), actorID, wsID, "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	updated, err := svc.UpdateInvitationRole(context.Background(), actorID, wsID, "", inv.Token, RoleAdmin)
	if err != nil {
		t.Fatalf("update invitation role: %v", err)
	}
	if updated.Role != RoleAdmin {
		t.Fatalf("expected admin, got %s", updated.Role)
	}
	if updated.Token != inv.Token {
		t.Fatal("editing role should not rotate the invitation token")
	}
}

func TestRevokeInvitation(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))
	wsID := uuid.NewString()

	inv, err := svc.CreateInvitation(context.Background(), actorID, wsID, "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	if err := svc.RevokeInvitation(context.Background(), actorID, wsID, "", inv.Token); err != nil {
		t.Fatalf("revoke invitation: %v", err)
	}
	if fake.invitation.Token.Valid {
		t.Fatal("expected invitation to be deleted")
	}
}

func TestRevokeMemberInvitationUnderSeatLock(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{
		t:           t,
		memberRole:  RoleOwner,
		actorUserID: actorID,
		seatsCount:  1,
		billing:     db.WorkspaceBilling{PlanCode: "pro", Period: "monthly"},
	}
	svc := NewService(db.New(fake), WithDBPool(fake))
	wsID := uuid.NewString()

	inv, err := svc.CreateInvitation(context.Background(), actorID, wsID, "", "member@example.test", RoleMember, 7)
	if err != nil {
		t.Fatalf("create member invitation: %v", err)
	}
	if err := svc.RevokeInvitation(context.Background(), actorID, wsID, "", inv.Token); err != nil {
		t.Fatalf("revoke member invitation under seat lock: %v", err)
	}
	if fake.invitation.Token.Valid {
		t.Fatal("expected member invitation to be deleted")
	}
}

func TestUpdateMemberRoleDemoteUnderSeatLock(t *testing.T) {
	actorID := uuid.NewString()
	targetID := uuid.NewString()
	fake := &fakeDB{
		t:            t,
		memberRole:   RoleOwner,
		actorUserID:  actorID,
		targetUserID: targetID,
		targetRole:   RoleMember,
		seatsCount:   2,
		billing:      db.WorkspaceBilling{PlanCode: "pro", Period: "monthly"},
	}
	svc := NewService(db.New(fake), WithDBPool(fake))
	wsID := uuid.NewString()

	member, err := svc.UpdateMemberRole(context.Background(), actorID, wsID, "", targetID, RoleGuest)
	if err != nil {
		t.Fatalf("demote member→guest under seat lock: %v", err)
	}
	if member.Role != RoleGuest {
		t.Fatalf("role=%q want guest", member.Role)
	}
}

func TestUpdateMemberRole(t *testing.T) {
	actorID := uuid.NewString()
	targetID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID, targetUserID: targetID, targetRole: RoleMember}
	svc := NewService(db.New(fake))
	wsID := uuid.NewString()

	member, err := svc.UpdateMemberRole(context.Background(), actorID, wsID, "", targetID, RoleAdmin)
	if err != nil {
		t.Fatalf("update member role: %v", err)
	}
	if member.Role != RoleAdmin {
		t.Fatalf("expected admin, got %s", member.Role)
	}
}

func TestUpdateMemberRoleCannotModifyOwner(t *testing.T) {
	actorID := uuid.NewString()
	targetID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID, targetUserID: targetID, targetRole: RoleOwner}
	svc := NewService(db.New(fake))

	_, err := svc.UpdateMemberRole(context.Background(), actorID, uuid.NewString(), "", targetID, RoleAdmin)
	if !errors.Is(err, ErrCannotModifyOwner) {
		t.Fatalf("expected ErrCannotModifyOwner, got %v", err)
	}
}

func TestRemoveMember(t *testing.T) {
	actorID := uuid.NewString()
	targetID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID, targetUserID: targetID, targetRole: RoleGuest}
	svc := NewService(db.New(fake))

	if err := svc.RemoveMember(context.Background(), actorID, uuid.NewString(), "", targetID); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	if fake.targetUserID != "" {
		t.Fatal("expected target member to be deleted")
	}
}

func TestRemoveMemberCannotRemoveSelf(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	if err := svc.RemoveMember(context.Background(), actorID, uuid.NewString(), "", actorID); !errors.Is(err, ErrCannotModifySelf) {
		t.Fatalf("expected ErrCannotModifySelf, got %v", err)
	}
}

func TestAdminCannotInviteAdmin(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleAdmin, actorUserID: actorID}
	svc := NewService(db.New(fake))

	_, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "admin@example.test", RoleAdmin, 7)
	if !errors.Is(err, ErrCannotManageMember) {
		t.Fatalf("expected ErrCannotManageMember, got %v", err)
	}
}

func TestAdminCannotRevokeAdminInvite(t *testing.T) {
	actorID := uuid.NewString()
	wsID := uuid.NewString()
	token := newPGUUID()
	fake := &fakeDB{
		t:           t,
		memberRole:  RoleAdmin,
		actorUserID: actorID,
		invitation: db.WorkspaceInvitation{
			Token:       token,
			WorkspaceID: pgUUIDFromString(wsID),
			Email:       "admin@example.test",
			Role:        RoleAdmin,
			ExpiresAt:   pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour), Valid: true},
			CreatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		},
	}
	svc := NewService(db.New(fake))

	if err := svc.RevokeInvitation(context.Background(), actorID, wsID, "", uuidToString(token)); !errors.Is(err, ErrCannotManageMember) {
		t.Fatalf("expected ErrCannotManageMember, got %v", err)
	}
}

func argTimestamptz(args []interface{}, i int) pgtype.Timestamptz {
	if i >= len(args) {
		return pgtype.Timestamptz{}
	}
	if t, ok := args[i].(pgtype.Timestamptz); ok {
		return t
	}
	return pgtype.Timestamptz{}
}

func TestAcceptInvitationSuccess(t *testing.T) {
	actorID := uuid.NewString()
	userID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake), WithDBPool(fake))

	inv, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	fake.workspace.Name = "Demo Capital"
	fake.workspace.Slug = "demo-capital"
	fake.lookupUserEmail = "guest@example.test"

	result, err := svc.AcceptInvitation(context.Background(), inv.Token, userID)
	if err != nil {
		t.Fatalf("accept invitation: %v", err)
	}
	if result.Role != RoleGuest {
		t.Fatalf("expected role guest, got %s", result.Role)
	}
	if result.WorkspaceSlug != "demo-capital" {
		t.Fatalf("expected workspace slug demo-capital, got %s", result.WorkspaceSlug)
	}
}

func TestPreviewInvitationPending(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	inv, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	fake.workspace.Name = "Demo Capital"
	fake.workspace.Slug = "demo-capital"

	preview, err := svc.PreviewInvitation(context.Background(), inv.Token)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Status != InvitationStatusPending {
		t.Fatalf("expected pending, got %q", preview.Status)
	}
	if preview.Email != "guest@example.test" || preview.WorkspaceSlug != "demo-capital" || preview.WorkspaceName != "Demo Capital" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
}

func TestPreviewInvitationExpiredAndUsed(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake))

	inv, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	fake.workspace.Name = "Demo Capital"
	fake.workspace.Slug = "demo-capital"
	fake.invitation.ExpiresAt = pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true}

	preview, err := svc.PreviewInvitation(context.Background(), inv.Token)
	if err != nil {
		t.Fatalf("preview expired: %v", err)
	}
	if preview.Status != InvitationStatusExpired {
		t.Fatalf("expected expired, got %q", preview.Status)
	}

	fake.invitation.ExpiresAt = pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour), Valid: true}
	fake.invitation.UsedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	preview, err = svc.PreviewInvitation(context.Background(), inv.Token)
	if err != nil {
		t.Fatalf("preview used: %v", err)
	}
	if preview.Status != InvitationStatusUsed {
		t.Fatalf("expected used, got %q", preview.Status)
	}
}

func TestPreviewInvitationNotFound(t *testing.T) {
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: uuid.NewString()}
	svc := NewService(db.New(fake))
	_, err := svc.PreviewInvitation(context.Background(), uuid.NewString())
	if !errors.Is(err, ErrInvitationNotFound) {
		t.Fatalf("expected ErrInvitationNotFound, got %v", err)
	}
}

func TestAcceptInvitationEmailMismatch(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake), WithDBPool(fake))

	inv, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	fake.workspace.Slug = "demo-capital"
	fake.lookupUserEmail = "other@example.test"

	_, err = svc.AcceptInvitation(context.Background(), inv.Token, uuid.NewString())
	if !errors.Is(err, ErrInvitationEmailMismatch) {
		t.Fatalf("expected ErrInvitationEmailMismatch, got %v", err)
	}
}

func TestAcceptInvitationExpired(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake), WithDBPool(fake))

	inv, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	fake.invitation.ExpiresAt = pgtype.Timestamptz{Time: time.Now().UTC().Add(-24 * time.Hour), Valid: true}

	_, err = svc.AcceptInvitation(context.Background(), inv.Token, uuid.NewString())
	if !errors.Is(err, ErrInvitationExpired) {
		t.Fatalf("expected ErrInvitationExpired, got %v", err)
	}
}

func TestAcceptInvitationAlreadyUsed(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake), WithDBPool(fake))

	inv, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	fake.invitation.UsedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	fake.workspace.Slug = "demo-capital"
	fake.lookupUserEmail = "guest@example.test"

	_, err = svc.AcceptInvitation(context.Background(), inv.Token, uuid.NewString())
	if !errors.Is(err, ErrInvitationUsed) {
		t.Fatalf("expected ErrInvitationUsed, got %v", err)
	}
}

func TestAcceptInvitationIdempotentWhenAlreadyMember(t *testing.T) {
	actorID := uuid.NewString()
	userID := uuid.NewString()
	fake := &fakeDB{t: t, memberRole: RoleOwner, actorUserID: actorID}
	svc := NewService(db.New(fake), WithDBPool(fake))

	inv, err := svc.CreateInvitation(context.Background(), actorID, uuid.NewString(), "", "guest@example.test", RoleGuest, 7)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	fake.workspace.Name = "Demo Capital"
	fake.workspace.Slug = "demo-capital"
	fake.lookupUserEmail = "guest@example.test"

	first, err := svc.AcceptInvitation(context.Background(), inv.Token, userID)
	if err != nil {
		t.Fatalf("first accept: %v", err)
	}
	fake.targetUserID = userID
	fake.targetRole = RoleGuest
	fake.lookupUserEmail = "guest@example.test"

	second, err := svc.AcceptInvitation(context.Background(), inv.Token, userID)
	if err != nil {
		t.Fatalf("second accept should be idempotent: %v", err)
	}
	if second.WorkspaceSlug != first.WorkspaceSlug || second.Role != RoleGuest {
		t.Fatalf("unexpected second result: %+v", second)
	}
}

func TestGetBillingUsesRealStorageUsage(t *testing.T) {
	actorID := uuid.NewString()
	fake := &fakeDB{
		t:            t,
		memberRole:   RoleOwner,
		actorUserID:  actorID,
		storageUsage: 5 * 1024 * 1024,
		linksCount:   3,
		roomsCount:   2,
		seatsCount:   1,
		billing:      activeTrialBilling(),
	}
	svc := NewService(db.New(fake))

	billing, err := svc.GetBilling(context.Background(), uuid.NewString())
	if err != nil {
		t.Fatalf("get billing: %v", err)
	}
	if billing.Plan != "trial" || billing.Period != "monthly" {
		t.Fatalf("unexpected plan %q %q", billing.Plan, billing.Period)
	}
	if billing.TrialExpired {
		t.Fatal("active trial must not report trial_expired")
	}
	if billing.StorageUsed != fake.storageUsage {
		t.Fatalf("expected storage used %d, got %d", fake.storageUsage, billing.StorageUsed)
	}
	trialLimits := plan.Lookup(plan.CodeTrial)
	if billing.StorageLimit != trialLimits.StorageBytes || billing.LinksLimit != trialLimits.Links ||
		billing.RoomsLimit != trialLimits.Rooms || billing.SeatsLimit != trialLimits.InternalSeats {
		t.Fatalf("trial caps mismatch, got %+v want %+v", billing, trialLimits)
	}
	if !billing.CustomDomainEnabled {
		t.Fatal("trial must enable custom domain")
	}
	if !billing.WatermarkEnabled {
		t.Fatal("trial must enable watermark")
	}
	if !billing.NDAEnabled {
		t.Fatal("trial must enable NDA")
	}
	if !billing.VisitorAskAIEnabled {
		t.Fatal("trial must enable visitor ask AI")
	}
	if !billing.BrandingEnabled || !billing.AccessControlsEnabled {
		t.Fatal("trial must enable branding and access controls")
	}
	if !billing.KnowledgeDeskEnabled || !billing.WebhooksEnabled || !billing.HubSpotEnabled ||
		!billing.DailyDigestEnabled || !billing.SlackAlertsEnabled || !billing.RoomAnalyticsEnabled ||
		!billing.RoomInsightsEnabled || !billing.FormalAskEnabled {
		t.Fatalf("trial must include business+formal entitlements: %+v", billing)
	}
	if billing.KnowledgeAnswersLimit != trialLimits.KnowledgeAnswersMonthly {
		t.Fatalf("trial knowledge cap=%d want %d", billing.KnowledgeAnswersLimit, trialLimits.KnowledgeAnswersMonthly)
	}
	if billing.DocumentsLimit != trialLimits.Documents || billing.AskAILimit != trialLimits.VisitorAskAIMonthly {
		t.Fatalf("trial document/ask caps %+v want docs=%d ask=%d", billing, trialLimits.Documents, trialLimits.VisitorAskAIMonthly)
	}
	if billing.LinksUsed != 3 || billing.RoomsUsed != 2 || billing.SeatsUsed != 1 {
		t.Fatalf("unexpected usage %+v", billing)
	}
}

func TestGetBillingUsesPersistedFreePlan(t *testing.T) {
	fake := &fakeDB{
		t:            t,
		storageUsage: 1024,
		linksCount:   4,
		roomsCount:   1,
		seatsCount:   1,
		billing: db.WorkspaceBilling{
			WorkspaceID: newPGUUID(),
			PlanCode:    "free",
			Period:      "monthly",
		},
	}
	svc := NewService(db.New(fake))

	billing, err := svc.GetBilling(context.Background(), uuid.NewString())
	if err != nil {
		t.Fatalf("get billing: %v", err)
	}
	if billing.Plan != "free" {
		t.Fatalf("expected free, got %q", billing.Plan)
	}
	if billing.StorageLimit != 2<<30 || billing.LinksLimit != 20 || billing.RoomsLimit != 1 || billing.SeatsLimit != 1 ||
		billing.DocumentsLimit != 50 {
		t.Fatalf("unexpected free limits %+v", billing)
	}
	if billing.BrandingEnabled || billing.AccessControlsEnabled {
		t.Fatal("free must disable branding and access controls")
	}
	if billing.AskAILimit != 0 {
		t.Fatalf("free ask limit must be 0 (feature off), got %d", billing.AskAILimit)
	}
	if billing.CustomDomainEnabled {
		t.Fatal("free must disable custom domain")
	}
	if billing.WatermarkEnabled {
		t.Fatal("free must disable watermark")
	}
	if billing.NDAEnabled {
		t.Fatal("free must disable NDA")
	}
	if billing.VisitorAskAIEnabled {
		t.Fatal("free must disable visitor ask AI")
	}
	if billing.KnowledgeDeskEnabled || billing.WebhooksEnabled || billing.HubSpotEnabled ||
		billing.DailyDigestEnabled || billing.SlackAlertsEnabled || billing.RoomAnalyticsEnabled ||
		billing.RoomInsightsEnabled || billing.FormalAskEnabled {
		t.Fatalf("free must disable paid entitlements: %+v", billing)
	}
	if billing.StorageUsed != 1024 || billing.LinksUsed != 4 || billing.RoomsUsed != 1 || billing.SeatsUsed != 1 {
		t.Fatalf("unexpected usage %+v", billing)
	}
}
