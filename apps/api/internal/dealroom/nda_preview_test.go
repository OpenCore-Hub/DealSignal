package dealroom

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestSignMemberNDAResourceHMAC(t *testing.T) {
	secret := "test-url-signing-secret"
	storageKey := "tenants/t1/nda/page-1.png"
	token := "room-nda:" + uuid.NewString()
	vid := uuid.NewString()
	signed := signMemberNDAResource(secret, storageKey, token, vid, "http://localhost:8080", 15*time.Minute)
	if signed == "" {
		t.Fatal("expected signed URL")
	}
	u, err := url.Parse(signed)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Path != "/api/v1/public/files/signed" {
		t.Fatalf("path: %s", u.Path)
	}
	q := u.Query()
	keyBytes, err := base64.URLEncoding.DecodeString(q.Get("key"))
	if err != nil || string(keyBytes) != storageKey {
		t.Fatalf("key: %v %q", err, q.Get("key"))
	}
	expires, err := strconv.ParseInt(q.Get("expires"), 10, 64)
	if err != nil {
		t.Fatalf("expires: %v", err)
	}
	payload := fmt.Sprintf("%s|%s|%s|%d", storageKey, token, vid, expires)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(q.Get("sig"))) {
		t.Fatalf("sig mismatch")
	}
}

func TestPreviewMemberNDAPendingInvitee(t *testing.T) {
	fake := newFakeDB(t)
	cfg := testCfg()
	cfg.AppBaseURL = "http://localhost:8080"
	cfg.URLSigningSecret = "test-url-signing-secret"
	svc := NewService(db.New(fake), nil, cfg)
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
		{WorkspaceID: wsUUID, UserID: pgUUID(inviteeID), Role: "guest", JoinedAt: nowTs()},
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:        "nda-preview-room",
		Name:        "NDA Preview Room",
		RequiresNDA: true,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	tplID := uuid.New()
	docID := uuid.New()
	contentSHA := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID.String()),
		TenantID:    fake.workspace.TenantID,
		WorkspaceID: wsUUID,
		Title:       "Series A NDA",
		SourceType:  "upload",
		Status:      "ready",
		StorageKey:  "tenants/t1/nda/original.pdf",
		PageCount:   pgtype.Int4{Int32: 2, Valid: true},
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	})
	fake.pages = append(fake.pages,
		db.ListPagesByDocumentRow{
			ID:             newPGUUID(),
			TenantID:       fake.workspace.TenantID,
			WorkspaceID:    wsUUID,
			DocumentID:     pgUUID(docID.String()),
			PageNumber:     1,
			ImageObjectKey: pgtype.Text{String: "tenants/t1/nda/page-1.png", Valid: true},
			CreatedAt:      nowTs(),
		},
		db.ListPagesByDocumentRow{
			ID:             newPGUUID(),
			TenantID:       fake.workspace.TenantID,
			WorkspaceID:    wsUUID,
			DocumentID:     pgUUID(docID.String()),
			PageNumber:     2,
			ImageObjectKey: pgtype.Text{String: "tenants/t1/nda/page-2.png", Valid: true},
			CreatedAt:      nowTs(),
		},
	)
	fake.ndaTemplates = append(fake.ndaTemplates, db.NdaTemplate{
		ID:               pgUUID(tplID.String()),
		TenantID:         fake.workspace.TenantID,
		WorkspaceID:      wsUUID,
		Name:             "Standard NDA",
		SourceDocumentID: pgUUID(docID.String()),
		ContentSha256:    contentSHA,
		Status:           "active",
		CreatedAt:        nowTs(),
		UpdatedAt:        nowTs(),
	})
	if _, err := svc.SetMemberNDAAgreement(context.Background(), roomID, wsID, ownerID, tplID.String(), ""); err != nil {
		t.Fatalf("set nda: %v", err)
	}
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "invitee@example.com", "member"); err != nil {
		t.Fatalf("add member: %v", err)
	}

	if _, err := svc.PreviewMemberNDA(context.Background(), roomID, wsID, ownerID); !errors.Is(err, ErrNDANotRequired) {
		t.Fatalf("owner preview: %v", err)
	}

	preview, err := svc.PreviewMemberNDA(context.Background(), roomID, wsID, inviteeID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.NDATemplate.Name != "Standard NDA" || preview.NDATemplate.ContentSHA256 != contentSHA {
		t.Fatalf("template: %+v", preview.NDATemplate)
	}
	if preview.Document.Title != "Series A NDA" || preview.SignerEmail != "invitee@example.com" {
		t.Fatalf("document/signer: %+v", preview)
	}
	if len(preview.PreviewPageURLs) != 2 {
		t.Fatalf("pages: %v", preview.PreviewPageURLs)
	}
	for _, pageURL := range preview.PreviewPageURLs {
		if !strings.Contains(pageURL, "/api/v1/public/files/signed") || !strings.Contains(pageURL, "room-nda") {
			t.Fatalf("page url: %s", pageURL)
		}
		if strings.Contains(strings.ToLower(pageURL), "invitee") {
			t.Fatalf("url must not embed email: %s", pageURL)
		}
	}
	if preview.DocumentURL == "" || !strings.Contains(preview.DocumentURL, "disposition=inline") {
		t.Fatalf("document url: %s", preview.DocumentURL)
	}

	detail, err := svc.GetRoomDetail(context.Background(), roomID, wsID, inviteeID)
	if err != nil {
		t.Fatalf("pending detail: %v", err)
	}
	if detail.Room.NdaTemplateID.Valid || detail.Room.NdaDocumentID.Valid {
		t.Fatalf("GetRoomDetail must still strip NDA ids: %+v", detail.Room)
	}

	if _, err := svc.SignMemberNDA(context.Background(), roomID, wsID, inviteeID, true, "deadbeef"); !errors.Is(err, ErrNDAContentMismatch) {
		t.Fatalf("mismatch: %v", err)
	}
	if _, err := svc.SignMemberNDA(context.Background(), roomID, wsID, inviteeID, true, ""); !errors.Is(err, ErrNDAContentMismatch) {
		t.Fatalf("empty hash: %v", err)
	}
	signed, err := svc.SignMemberNDA(context.Background(), roomID, wsID, inviteeID, true, contentSHA)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if fake.lastNDAContentSHA != contentSHA {
		t.Fatalf("stamped hash %q, want %s", fake.lastNDAContentSHA, contentSHA)
	}
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if signed.NdaRequired {
		t.Fatalf("after sign still gated: %+v", signed)
	}
	if _, err := svc.PreviewMemberNDA(context.Background(), roomID, wsID, inviteeID); !errors.Is(err, ErrNDANotRequired) {
		t.Fatalf("signed preview: %v", err)
	}
}

func TestPreviewMemberNDANotRequired(t *testing.T) {
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
		{ID: pgUUID(inviteeID), Email: "invitee@example.com", CreatedAt: nowTs()},
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
		{WorkspaceID: wsUUID, UserID: pgUUID(inviteeID), Role: "guest", JoinedAt: nowTs()},
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "no-nda-preview",
		Name: "No NDA",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.AddMember(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID, ownerID, "invitee@example.com", "member"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := svc.PreviewMemberNDA(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID, inviteeID); !errors.Is(err, ErrNDANotRequired) {
		t.Fatalf("expected ErrNDANotRequired, got %v", err)
	}
}

func TestPreviewMemberNDARequiresReadableArtifact(t *testing.T) {
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
		{WorkspaceID: wsUUID, UserID: pgUUID(inviteeID), Role: "guest", JoinedAt: nowTs()},
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:        "nda-preview-unsigned",
		Name:        "NDA Preview Unsigned",
		RequiresNDA: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	tplID := uuid.New()
	docID := uuid.New()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID.String()),
		TenantID:    fake.workspace.TenantID,
		WorkspaceID: wsUUID,
		Title:       "Series A NDA",
		SourceType:  "upload",
		Status:      "ready",
		StorageKey:  "tenants/t1/nda/original.pdf",
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	})
	fake.ndaTemplates = append(fake.ndaTemplates, db.NdaTemplate{
		ID:               pgUUID(tplID.String()),
		TenantID:         fake.workspace.TenantID,
		WorkspaceID:      wsUUID,
		Name:             "Standard NDA",
		SourceDocumentID: pgUUID(docID.String()),
		Status:           "active",
		CreatedAt:        nowTs(),
		UpdatedAt:        nowTs(),
	})
	if _, err := svc.SetMemberNDAAgreement(context.Background(), roomID, wsID, ownerID, tplID.String(), ""); err != nil {
		t.Fatalf("set nda: %v", err)
	}
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "invitee@example.com", "member"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := svc.PreviewMemberNDA(context.Background(), roomID, wsID, inviteeID); !errors.Is(err, ErrNDAPreviewUnavailable) {
		t.Fatalf("expected ErrNDAPreviewUnavailable, got %v", err)
	}
}
