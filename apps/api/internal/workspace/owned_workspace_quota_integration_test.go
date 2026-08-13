//go:build integration

package workspace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func createOwnedWorkspace(t *testing.T, svc *workspace.Service, userID, name string) workspace.Workspace {
	t.Helper()
	ws, err := svc.Create(context.Background(), userID, name, slugFor(name), "")
	if err != nil {
		t.Fatalf("Create %s: %v", name, err)
	}
	return ws
}

func slugFor(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "-")) + "-" + uuid.NewString()[:8]
}

func userIDString(user db.User) string {
	return uuid.UUID(user.ID.Bytes).String()
}

func parseUUID(t *testing.T, id string) pgtype.UUID {
	t.Helper()
	parsed, err := uuid.Parse(id)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", id, err)
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

func billingFor(t *testing.T, q *db.Queries, wsID string) db.WorkspaceBilling {
	t.Helper()
	row, err := q.GetWorkspaceBilling(context.Background(), parseUUID(t, wsID))
	if err != nil {
		t.Fatalf("GetWorkspaceBilling: %v", err)
	}
	return row
}

func postCreateWorkspace(router *gin.Engine, name, slug string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"name": name, "slug": slug})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/workspaces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

func TestBillingOwnedWorkspaceCapAndTrialGrant_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	svc := workspace.NewService(q, workspace.WithDBPool(tx))

	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("owned-cap-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ownerID := userIDString(owner)

	first := createOwnedWorkspace(t, svc, ownerID, "Owned First")
	row := billingFor(t, q, first.ID)
	if row.PlanCode != plan.CodeTrial || !row.TrialEndsAt.Valid {
		t.Fatalf("first owned must seed trial, got %+v", row)
	}
	granted, err := q.GetUserByID(ctx, owner.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if !granted.TrialGrantedAt.Valid {
		t.Fatal("first owned workspace must stamp trial_granted_at")
	}

	if _, err := svc.Create(ctx, ownerID, "Owned Second", slugFor("Owned Second"), ""); !errors.Is(err, plan.ErrLimitWorkspaces) {
		t.Fatalf("second owned on trial must hit cap, got %v", err)
	}

	reader, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("owned-reader-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create reader: %v", err)
	}
	if _, err := q.AddWorkspaceMember(ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: parseUUID(t, first.ID),
		UserID:      reader.ID,
		Role:        workspace.RoleGuest,
	}); err != nil {
		t.Fatalf("add guest: %v", err)
	}
	readerFirst := createOwnedWorkspace(t, svc, userIDString(reader), "Reader Owned")
	if billingFor(t, q, readerFirst.ID).PlanCode != plan.CodeTrial {
		t.Fatal("guest (read-only) must not consume owned cap; first owned still gets trial")
	}

	writer, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("owned-writer-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create writer: %v", err)
	}
	if _, err := q.AddWorkspaceMember(ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: parseUUID(t, first.ID),
		UserID:      writer.ID,
		Role:        workspace.RoleMember,
	}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	writerOwn := createOwnedWorkspace(t, svc, userIDString(writer), "Writer Own")
	if billingFor(t, q, writerOwn.ID).PlanCode != plan.CodeTrial {
		t.Fatal("member elsewhere must still receive a personal trial on first owned workspace")
	}

	adminUser, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("owned-admin-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if _, err := q.AddWorkspaceMember(ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: parseUUID(t, first.ID),
		UserID:      adminUser.ID,
		Role:        workspace.RoleAdmin,
	}); err != nil {
		t.Fatalf("add admin: %v", err)
	}
	adminOwn := createOwnedWorkspace(t, svc, userIDString(adminUser), "Admin Own")
	if billingFor(t, q, adminOwn.ID).PlanCode != plan.CodeTrial {
		t.Fatal("admin elsewhere must still receive a personal trial on first owned workspace")
	}

	if err := q.DeleteWorkspaceMember(ctx, db.DeleteWorkspaceMemberParams{
		WorkspaceID: parseUUID(t, first.ID),
		UserID:      owner.ID,
	}); err != nil {
		t.Fatalf("leave owner membership: %v", err)
	}
	remint := createOwnedWorkspace(t, svc, ownerID, "Owned Remint")
	remintBilling := billingFor(t, q, remint.ID)
	if remintBilling.PlanCode != plan.CodeFree {
		t.Fatalf("create after leave must be free, got %+v", remintBilling)
	}
	if remintBilling.TrialEndsAt.Valid {
		t.Fatal("remint after leave must not receive a trial clock")
	}

	if err := q.VerifyUserEmail(ctx, owner.ID); err != nil {
		t.Fatalf("verify email: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: parseUUID(t, remint.ID),
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}
	second := createOwnedWorkspace(t, svc, ownerID, "Owned Pro Extra")
	if billingFor(t, q, second.ID).PlanCode != plan.CodeFree {
		t.Fatalf("pro extra owned workspace must be free, got %+v", billingFor(t, q, second.ID))
	}
	third := createOwnedWorkspace(t, svc, ownerID, "Owned Pro Third")
	if billingFor(t, q, third.ID).PlanCode != plan.CodeFree {
		t.Fatalf("pro third owned workspace must be free, got %+v", billingFor(t, q, third.ID))
	}
	if _, err := svc.Create(ctx, ownerID, "Owned Pro Fourth", slugFor("Owned Pro Fourth"), ""); !errors.Is(err, plan.ErrLimitWorkspaces) {
		t.Fatalf("fourth owned on pro must hit cap, got %v", err)
	}
}

func TestBillingOwnedWorkspaceUnverifiedProHTTP_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	svc := workspace.NewService(q, workspace.WithDBPool(tx))
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("unverified-pro-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	userID := userIDString(user)
	first := createOwnedWorkspace(t, svc, userID, "Unverified First")
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: parseUUID(t, first.ID),
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, first.ID))
	router.POST("/workspaces", workspace.NewHandler(svc, nil).Create)

	w := postCreateWorkspace(router, "Unverified Second", slugFor("unverified-second"))
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), `"code":"email_unverified"`) {
		t.Fatalf("unverified pro second: status=%d body=%s", w.Code, w.Body.String())
	}

	if err := q.VerifyUserEmail(ctx, user.ID); err != nil {
		t.Fatalf("verify: %v", err)
	}
	w = postCreateWorkspace(router, "Verified Second", slugFor("verified-second"))
	if w.Code != http.StatusCreated {
		t.Fatalf("verified pro second: status=%d body=%s", w.Code, w.Body.String())
	}

	var created workspace.Workspace
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if billingFor(t, q, created.ID).PlanCode != plan.CodeFree {
		t.Fatalf("verified extra must be free, got %+v", billingFor(t, q, created.ID))
	}
}

func TestBillingOwnedWorkspaceHTTPCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	svc := workspace.NewService(q, workspace.WithDBPool(tx))
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("http-cap-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	userID := userIDString(user)
	first := createOwnedWorkspace(t, svc, userID, "HTTP Cap First")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, first.ID))
	router.POST("/workspaces", workspace.NewHandler(svc, nil).Create)

	before := plan.TestingDenialCount(plan.CodeLimitWorkspaces)
	w := postCreateWorkspace(router, "HTTP Cap Second", slugFor("http-cap-second"))
	assertPlanLimitHTTP(t, w, plan.CodeLimitWorkspaces)
	if plan.TestingDenialCount(plan.CodeLimitWorkspaces) < before+1 {
		t.Fatal("HTTP cap must record plan_limit_workspaces")
	}
}

func TestBillingOwnedWorkspaceExpiredTrialCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	svc := workspace.NewService(q, workspace.WithDBPool(tx))
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("expired-trial-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	userID := userIDString(user)
	first := createOwnedWorkspace(t, svc, userID, "Expired Trial First")
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: parseUUID(t, first.ID),
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("expire trial: %v", err)
	}
	if _, err := svc.Create(ctx, userID, "Expired Trial Second", slugFor("expired-trial-second"), ""); !errors.Is(err, plan.ErrLimitWorkspaces) {
		t.Fatalf("expired trial must use free owned cap, got %v", err)
	}
}

func TestBillingOwnedWorkspaceConcurrentCreate_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	svc := workspace.NewService(q, workspace.WithDBPool(testPool))
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("owned-race-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	userID := userIDString(user)

	type result struct {
		ws  workspace.Workspace
		err error
	}
	results := make([]result, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			defer wg.Done()
			ws, createErr := svc.Create(ctx, userID, fmt.Sprintf("Race %d", i), slugFor(fmt.Sprintf("race-%d", i)), "")
			results[i] = result{ws: ws, err: createErr}
		}()
	}
	wg.Wait()

	var trials, denied, other int
	for _, r := range results {
		if errors.Is(r.err, plan.ErrLimitWorkspaces) {
			denied++
			continue
		}
		if r.err != nil {
			other++
			t.Errorf("unexpected create error: %v", r.err)
			continue
		}
		if billingFor(t, q, r.ws.ID).PlanCode == plan.CodeTrial {
			trials++
		}
	}
	if trials != 1 || denied != 1 || other != 0 {
		t.Fatalf("concurrent create: trials=%d denied=%d other=%d want 1 trial and 1 cap deny", trials, denied, other)
	}
}

func TestBillingInviteJoinDoesNotConsumeOwnedCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	svc := workspace.NewService(q, workspace.WithDBPool(tx))

	ownerA, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("join-a-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	ownerB, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("join-b-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	aID := userIDString(ownerA)
	bID := userIDString(ownerB)
	wsA := createOwnedWorkspace(t, svc, aID, "Join A")
	wsB := createOwnedWorkspace(t, svc, bID, "Join B")

	inv, err := svc.CreateInvitation(ctx, bID, wsB.ID, "", ownerA.Email, workspace.RoleMember, 7)
	if err != nil {
		t.Fatalf("invite existing owner as member must succeed: %v", err)
	}
	if _, err := svc.AcceptInvitation(ctx, inv.Token, aID); err != nil {
		t.Fatalf("Accept member invite must succeed: %v", err)
	}
	if _, err := svc.Create(ctx, aID, "A Second", slugFor("A Second"), ""); !errors.Is(err, plan.ErrLimitWorkspaces) {
		t.Fatalf("joining another tenant must not raise owned cap; second owned on trial must deny, got %v", err)
	}

	if _, err := svc.AddMember(ctx, aID, wsA.ID, "", bID, workspace.RoleMember); err != nil {
		t.Fatalf("AddMember existing owner as member must succeed: %v", err)
	}

	stranger, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("join-guest-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create stranger: %v", err)
	}
	guestTok, err := svc.CreateInvitation(ctx, aID, wsA.ID, "", stranger.Email, workspace.RoleGuest, 7)
	if err != nil {
		t.Fatalf("guest invite: %v", err)
	}
	if _, err := svc.AcceptInvitation(ctx, guestTok.Token, userIDString(stranger)); err != nil {
		t.Fatalf("guest accept must succeed: %v", err)
	}
}

func TestBillingInviteRegisterThenCreatePersonalTrial_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	svc := workspace.NewService(q, workspace.WithDBPool(tx))

	host, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("host-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	hostID := userIDString(host)
	hostWS := createOwnedWorkspace(t, svc, hostID, "Host Room")

	invitee, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("invitee-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create invitee: %v", err)
	}
	inviteeID := userIDString(invitee)

	tok, err := svc.CreateInvitation(ctx, hostID, hostWS.ID, "", invitee.Email, workspace.RoleMember, 7)
	if err != nil {
		t.Fatalf("invite member: %v", err)
	}
	accepted, err := svc.AcceptInvitation(ctx, tok.Token, inviteeID)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if accepted.WorkspaceID != hostWS.ID {
		t.Fatalf("accept workspace=%s want %s", accepted.WorkspaceID, hostWS.ID)
	}

	listed, err := svc.List(ctx, inviteeID)
	if err != nil {
		t.Fatalf("list after accept: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != hostWS.ID {
		t.Fatalf("invitee switcher after accept: %+v want only host workspace", listed)
	}

	personal := createOwnedWorkspace(t, svc, inviteeID, "Personal")
	if billingFor(t, q, personal.ID).PlanCode != plan.CodeTrial {
		t.Fatalf("invitee first owned must be trial, got %+v", billingFor(t, q, personal.ID))
	}
	granted, err := q.GetUserByID(ctx, invitee.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if !granted.TrialGrantedAt.Valid {
		t.Fatal("invitee first owned must stamp trial_granted_at")
	}

	listed, err = svc.List(ctx, inviteeID)
	if err != nil {
		t.Fatalf("list after create: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("invitee switcher after create: %d workspaces want 2", len(listed))
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(inviteeID, hostWS.ID))
	router.POST("/workspaces", workspace.NewHandler(svc, nil).Create)
	w := postCreateWorkspace(router, "Second Personal", slugFor("second-personal"))
	assertPlanLimitHTTP(t, w, plan.CodeLimitWorkspaces)
}
