package dealroom

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestPreviewAndAcceptRoomInviteUnbound(t *testing.T) {
	fake := newFakeDB(t)
	cfg := testCfg()
	cfg.FrontendURL = "https://app.example.com"
	svc := NewService(db.New(fake), nil, cfg, WithMailer(&captureMailer{}))
	ownerID := uuid.NewString()
	inviteeID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Acme",
		Slug:     "acme",
	}
	fake.users = []db.User{
		{ID: pgUUID(ownerID), Email: "owner@example.com", CreatedAt: nowTs()},
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "invite-room",
		Name: "Invite Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "guest@example.com", "guest"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	token, err := mintRoomInviteToken(cfg.InviteTokenHashKey, roomID, "guest@example.com")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	preview, err := svc.PreviewRoomInvite(context.Background(), token)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Email != "guest@example.com" || preview.Status != RoomInviteStatusPending || preview.RoomName != "Invite Room" || preview.WorkspaceSlug != "acme" {
		t.Fatalf("preview: %+v", preview)
	}

	fake.users = append(fake.users, db.User{ID: pgUUID(inviteeID), Email: "guest@example.com", CreatedAt: nowTs()})
	got, err := svc.AcceptRoomInvite(context.Background(), token, inviteeID)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if got.RoomID != roomID || got.UserID != inviteeID || got.MemberStatus == "" {
		t.Fatalf("accept result: %+v", got)
	}
	again, err := svc.AcceptRoomInvite(context.Background(), token, inviteeID)
	if err != nil {
		t.Fatalf("idempotent accept: %v", err)
	}
	if again.UserID != inviteeID {
		t.Fatalf("idempotent: %+v", again)
	}
	previewUsed, err := svc.PreviewRoomInvite(context.Background(), token)
	if err != nil {
		t.Fatalf("preview used: %v", err)
	}
	if previewUsed.Status != RoomInviteStatusUsed {
		t.Fatalf("expected used, got %+v", previewUsed)
	}
}

func TestAcceptRoomInviteEmailMismatchAndWrongUser(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	inviteeID := uuid.NewString()
	strangerID := uuid.NewString()
	otherBoundID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Acme",
		Slug:     "acme",
	}
	fake.users = []db.User{
		{ID: pgUUID(ownerID), Email: "owner@example.com", CreatedAt: nowTs()},
		{ID: pgUUID(inviteeID), Email: "guest@example.com", CreatedAt: nowTs()},
		{ID: pgUUID(strangerID), Email: "other@example.com", CreatedAt: nowTs()},
		{ID: pgUUID(otherBoundID), Email: "taken@example.com", CreatedAt: nowTs()},
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
		{WorkspaceID: wsUUID, UserID: pgUUID(inviteeID), Role: "guest", JoinedAt: nowTs()},
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{Slug: "mismatch-room", Name: "Mismatch"})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "guest@example.com", "guest"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	token, err := mintRoomInviteToken(testCfg().InviteTokenHashKey, roomID, "guest@example.com")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if _, err := svc.AcceptRoomInvite(context.Background(), token, strangerID); !errors.Is(err, ErrRoomInviteEmailMismatch) {
		t.Fatalf("mismatch: %v", err)
	}
	if _, err := svc.PreviewRoomInvite(context.Background(), "dsr1.aaaa.bbbb"); !errors.Is(err, ErrRoomInviteNotFound) {
		t.Fatalf("invalid token: %v", err)
	}

	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "taken@example.com", "guest"); err != nil {
		t.Fatalf("add bound member: %v", err)
	}
	takenToken, err := mintRoomInviteToken(testCfg().InviteTokenHashKey, roomID, "taken@example.com")
	if err != nil {
		t.Fatalf("mint taken: %v", err)
	}
	// Bound to taken@ account; invitee has a different mailbox so mismatch wins over used.
	if _, err := svc.AcceptRoomInvite(context.Background(), takenToken, inviteeID); !errors.Is(err, ErrRoomInviteEmailMismatch) {
		t.Fatalf("bound other mailbox: %v", err)
	}
}

func TestAcceptRoomInviteUsedByOtherUser(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	firstID := uuid.NewString()
	secondID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Acme",
		Slug:     "acme",
	}
	fake.users = []db.User{
		{ID: pgUUID(ownerID), Email: "owner@example.com", CreatedAt: nowTs()},
		{ID: pgUUID(firstID), Email: "guest@example.com", CreatedAt: nowTs()},
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{Slug: "used-room", Name: "Used"})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "guest@example.com", "guest"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	token, err := mintRoomInviteToken(testCfg().InviteTokenHashKey, roomID, "guest@example.com")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if _, err := svc.AcceptRoomInvite(context.Background(), token, firstID); err != nil {
		t.Fatalf("first accept: %v", err)
	}
	fake.users = append(fake.users, db.User{ID: pgUUID(secondID), Email: "guest@example.com", CreatedAt: nowTs()})
	if _, err := svc.AcceptRoomInvite(context.Background(), token, secondID); !errors.Is(err, ErrRoomInviteUsed) {
		t.Fatalf("second account: %v", err)
	}
}

func TestAcceptRoomInviteLeavesNDAPending(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	inviteeID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Acme",
		Slug:     "acme",
	}
	fake.users = []db.User{
		{ID: pgUUID(ownerID), Email: "owner@example.com", CreatedAt: nowTs()},
		{ID: pgUUID(inviteeID), Email: "invitee@example.com", CreatedAt: nowTs()},
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:        "nda-invite-room",
		Name:        "NDA Invite",
		RequiresNDA: true,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	tplID := uuid.New()
	fake.ndaTemplates = append(fake.ndaTemplates, db.NdaTemplate{
		ID:          pgUUID(tplID.String()),
		TenantID:    fake.workspace.TenantID,
		WorkspaceID: wsUUID,
		Name:        "NDA",
		Status:      "active",
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	})
	if _, err := svc.SetMemberNDAAgreement(context.Background(), roomID, wsID, ownerID, tplID.String(), ""); err != nil {
		t.Fatalf("set nda: %v", err)
	}
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "invitee@example.com", "member"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	token, err := mintRoomInviteToken(testCfg().InviteTokenHashKey, roomID, "invitee@example.com")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	got, err := svc.AcceptRoomInvite(context.Background(), token, inviteeID)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if got.MemberStatus != "pending" {
		t.Fatalf("nda must stay pending: %+v", got)
	}
	detail, err := svc.GetRoomDetail(context.Background(), roomID, wsID, inviteeID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if !detail.NdaRequired || detail.MemberStatus != "pending" || len(detail.Documents) != 0 {
		t.Fatalf("nda shell: %+v", detail)
	}
}

func TestSendRoomInviteEmailFallsBackWithoutSecret(t *testing.T) {
	fake := newFakeDB(t)
	mail := &captureMailer{}
	cfg := testCfg()
	cfg.InviteTokenHashKey = ""
	cfg.JWTSecret = ""
	cfg.FrontendURL = "https://app.example.com"
	svc := NewService(db.New(fake), nil, cfg, WithMailer(mail))
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Acme",
		Slug:     "acme",
	}
	fake.users = []db.User{{ID: pgUUID(ownerID), Email: "owner@example.com", CreatedAt: nowTs()}}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: pgUUID(wsID), UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{Slug: "fallback-room", Name: "Fallback"})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "new@example.com", "guest"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if len(mail.jobs) != 1 || mail.jobs[0].EmailType != mailer.EmailTypeRoomInvite {
		t.Fatalf("mail: %+v", mail.jobs)
	}
	link := mail.jobs[0].TemplateVariables["InvitationLink"]
	if !strings.Contains(link, "/acme/deal-rooms/"+roomID) || strings.Contains(link, "dsr1.") {
		t.Fatalf("fallback link: %s", link)
	}
}

func TestRoomInviteHandlerPreviewAcceptsDottedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Acme",
		Slug:     "acme",
	}
	fake.users = []db.User{{ID: pgUUID(ownerID), Email: "owner@example.com", CreatedAt: nowTs()}}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: pgUUID(wsID), UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{Slug: "http-invite", Name: "HTTP Invite"})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "guest@example.com", "guest"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	token, err := mintRoomInviteToken(testCfg().InviteTokenHashKey, roomID, "guest@example.com")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	h := NewHandler(svc)
	router := gin.New()
	api := router.Group("/api")
	h.RegisterInviteRoutes(api, func(c *gin.Context) {
		c.Set("userID", ownerID)
		c.Next()
	})

	req := httptest.NewRequest(http.MethodGet, "/api/deal-room-invites/"+token, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status %d: %s", rec.Code, rec.Body.String())
	}
	var preview RoomInvitePreview
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if preview.Email != "guest@example.com" || preview.RoomID != roomID {
		t.Fatalf("preview body: %+v", preview)
	}

	mismatch := httptest.NewRequest(http.MethodPost, "/api/deal-room-invites/"+token+"/accept", nil)
	mismatchRec := httptest.NewRecorder()
	router.ServeHTTP(mismatchRec, mismatch)
	if mismatchRec.Code != http.StatusForbidden {
		t.Fatalf("owner accept must mismatch, got %d: %s", mismatchRec.Code, mismatchRec.Body.String())
	}
}
