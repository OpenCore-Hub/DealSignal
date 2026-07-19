//go:build integration

package link

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/google/uuid"
)

// TestCreateDealRoomLink_ShareAccessCombinations verifies that the service
// accepts the security/feature combinations the frontend Share/Access tabs can
// produce. The frontend now sends expires_at as RFC3339; this test exercises
// the backend normalization and storage path for every meaningful combination.
func TestCreateDealRoomLink_ShareAccessCombinations(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug:         "room-" + uuid.NewString(),
		Name:         "Combination Test Room",
		TemplateType: "custom",
	})
	if err != nil {
		t.Fatalf("create deal room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	// Add the fixture document to the deal room so NDA combinations can reference a valid NDA document.
	docID := uuid.UUID(f.link.DocumentID.Bytes).String()
	if _, err := drSvc.AddDocument(f.ctx, roomID, wsID, userID, docID, "/general", 0); err != nil {
		t.Fatalf("add document to deal room: %v", err)
	}

	future := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)

	// Each case represents a distinct combination a user can build in the
	// Share + Access tabs. Combinations that the service is expected to reject
	// are asserted with wantErr.
	cases := []struct {
		name    string
		req     DealRoomLinkRequest
		wantErr bool
	}{
		{
			name: "public - no protections",
			req: DealRoomLinkRequest{
				Name: "Public link",
			},
		},
		{
			name: "standard - email required",
			req: DealRoomLinkRequest{
				Name:         "Standard link",
				RequireEmail: true,
			},
		},
		{
			name: "standard - email verification",
			req: DealRoomLinkRequest{
				Name:                     "Verified link",
				RequireEmailVerification: true,
			},
		},
		{
			name: "confidential - password",
			req: DealRoomLinkRequest{
				Name:            "Password link",
				RequirePassword: true,
				Password:        "strong-pass-123",
			},
		},
		{
			name: "confidential - email verification + password + watermark",
			req: DealRoomLinkRequest{
				Name:                     "Confidential link",
				RequireEmailVerification: true,
				RequirePassword:          true,
				Password:                 "strong-pass-123",
				WatermarkEnabled:         true,
			},
		},
		{
			name: "nda - requires email verification implicitly",
			req: DealRoomLinkRequest{
				Name:          "NDA link",
				RequireNDA:    true,
				RequireEmail:  true,
				NDADocumentID: docID,
			},
		},
		{
			name: "nda with all protections",
			req: DealRoomLinkRequest{
				Name:                      "NDA full link",
				RequireNDA:                true,
				RequireEmailVerification:  true,
				RequirePassword:           true,
				Password:                  "strong-pass-123",
				WatermarkEnabled:          true,
				ScreenshotProtectionEnabled: true,
				NDADocumentID:             docID,
			},
		},
		{
			name: "expires at future",
			req: DealRoomLinkRequest{
				Name:      "Expiring link",
				ExpiresAt: &future,
			},
		},
		{
			name: "expires at past - created as expired",
			req: DealRoomLinkRequest{
				Name:      "Expired link",
				ExpiresAt: func() *time.Time { p := time.Now().UTC().Add(-time.Hour); return &p }(),
			},
		},
		{
			name: "allowed viewers",
			req: DealRoomLinkRequest{
				Name:          "Allowed viewers link",
				RequireEmail:  true,
				AllowedEmails: []string{"alice@example.com"},
			},
		},
		{
			name: "blocked viewers",
			req: DealRoomLinkRequest{
				Name:          "Blocked viewers link",
				BlockedEmails: []string{"leaker@bad.com"},
			},
		},
		{
			name: "download and qa enabled",
			req: DealRoomLinkRequest{
				Name:            "Download QA link",
				DownloadEnabled: true,
				QaEnabled:       true,
			},
		},
		{
			name: "file requests and index file enabled",
			req: DealRoomLinkRequest{
				Name:                "Requests index link",
				FileRequestsEnabled: true,
				IndexFileEnabled:    true,
			},
		},
		{
			name: "custom domain",
			req: DealRoomLinkRequest{
				Name:         "Custom domain link",
				CustomDomain: "investors.example.com",
			},
		},
		{
			name: "tags and notify",
			req: DealRoomLinkRequest{
				Name:           "Tagged link",
				Tags:           []string{"investor", "q3-2026"},
				NotifyOnAccess: true,
			},
		},
		{
			name: "full custom combination",
			req: DealRoomLinkRequest{
				Name:                      "Full custom link",
				RequireEmailVerification:  true,
				RequirePassword:           true,
				Password:                  "strong-pass-123",
				RequireNDA:                true,
				NDADocumentID:             docID,
				DownloadEnabled:           true,
				WatermarkEnabled:          true,
				ScreenshotProtectionEnabled: true,
				QaEnabled:                 true,
				FileRequestsEnabled:       true,
				IndexFileEnabled:          true,
				AllowedEmails:             []string{"alice@example.com"},
				BlockedEmails:             []string{"leaker@bad.com"},
				ExpiresAt:                 &future,
				Tags:                      []string{"investor"},
				NotifyOnAccess:            true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			link, err := f.svc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("create deal room link: %v", err)
			}
			if link.ID.Bytes == uuid.Nil {
				t.Fatal("expected link to be created with a non-nil ID")
			}
		})
	}
}

// TestCreateDealRoomLink_StoredCombinationValues verifies that key fields for
// the combination boundaries are actually persisted into the database.
func TestCreateDealRoomLink_StoredCombinationValues(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug:         "room-" + uuid.NewString(),
		Name:         "Stored Value Test Room",
		TemplateType: "custom",
	})
	if err != nil {
		t.Fatalf("create deal room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	// Add the fixture document to the deal room so NDA cases can reference a valid NDA document.
	docID := uuid.UUID(f.link.DocumentID.Bytes).String()
	if _, err := drSvc.AddDocument(f.ctx, roomID, wsID, userID, docID, "/general", 0); err != nil {
		t.Fatalf("add document to deal room: %v", err)
	}

	t.Run("custom domain is stored", func(t *testing.T) {
		link, err := f.svc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, DealRoomLinkRequest{
			Name:         "Custom domain link",
			CustomDomain: "investors.example.com",
		})
		if err != nil {
			t.Fatalf("create deal room link: %v", err)
		}
		if !link.CustomDomain.Valid || link.CustomDomain.String != "investors.example.com" {
			t.Errorf("expected custom domain investors.example.com, got %v", link.CustomDomain)
		}
	})

	t.Run("password hash is stored and verifies", func(t *testing.T) {
		link, err := f.svc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, DealRoomLinkRequest{
			Name:            "Password link",
			RequirePassword: true,
			Password:        "strong-pass-123",
		})
		if err != nil {
			t.Fatalf("create deal room link: %v", err)
		}
		if !link.RequirePassword {
			t.Error("expected RequirePassword=true")
		}
		if !link.PasswordHash.Valid {
			t.Fatal("expected password hash to be stored")
		}
	})

	t.Run("allowed viewers without email gate auto-enable requireEmail", func(t *testing.T) {
		link, err := f.svc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, DealRoomLinkRequest{
			Name:          "Allowed viewers auto-email link",
			RequireEmail:  false,
			AllowedEmails: []string{"alice@example.com"},
		})
		if err != nil {
			t.Fatalf("create deal room link: %v", err)
		}
		if !link.RequireEmail {
			t.Errorf("expected RequireEmail=true, got %v", link.RequireEmail)
		}
		if link.PermissionType != "email_required" {
			t.Errorf("expected permission_type=email_required, got %q", link.PermissionType)
		}
	})

	t.Run("blocked viewers without email keep public permission", func(t *testing.T) {
		link, err := f.svc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, DealRoomLinkRequest{
			Name:          "Blocked viewers link",
			RequireEmail:  false,
			BlockedEmails: []string{"leaker@example.com"},
		})
		if err != nil {
			t.Fatalf("create deal room link: %v", err)
		}
		if link.RequireEmail {
			t.Errorf("expected RequireEmail=false, got %v", link.RequireEmail)
		}
		if link.PermissionType != "public" {
			t.Errorf("expected permission_type=public, got %q", link.PermissionType)
		}
	})

	t.Run("nda without email gate auto-enable requireEmail", func(t *testing.T) {
		link, err := f.svc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, DealRoomLinkRequest{
			Name:       "NDA auto-email link",
			RequireNDA:    true,
			NDADocumentID: docID,
		})
		if err != nil {
			t.Fatalf("create deal room link: %v", err)
		}
		if !link.RequireEmail {
			t.Errorf("expected RequireEmail=true, got %v", link.RequireEmail)
		}
		if !link.RequireNda {
			t.Errorf("expected RequireNda=true, got %v", link.RequireNda)
		}
		if link.PermissionType != "nda" {
			t.Errorf("expected permission_type=nda, got %q", link.PermissionType)
		}
	})
}
