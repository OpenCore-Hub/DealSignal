package link

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestFolderPathInDealRoomScope(t *testing.T) {
	dealRoom := pgtype.UUID{Valid: true}
	cases := []struct {
		name       string
		mode       string
		paths      []string
		folderPath string
		want       bool
	}{
		{name: "full empty paths allows", mode: FolderScopeModeFull, paths: nil, folderPath: "/legal", want: true},
		{name: "allowlist empty denies", mode: FolderScopeModeAllowlist, paths: nil, folderPath: "/legal", want: false},
		{name: "allowlist empty slice denies", mode: FolderScopeModeAllowlist, paths: []string{}, folderPath: "/legal", want: false},
		{name: "allowlist exact match", mode: FolderScopeModeAllowlist, paths: []string{"/legal"}, folderPath: "/legal", want: true},
		{name: "allowlist descendant match", mode: FolderScopeModeAllowlist, paths: []string{"/legal"}, folderPath: "/legal/contracts", want: true},
		{name: "allowlist sibling denied", mode: FolderScopeModeAllowlist, paths: []string{"/legal"}, folderPath: "/finance", want: false},
		{name: "allowlist prefix false friend denied", mode: FolderScopeModeAllowlist, paths: []string{"/legal"}, folderPath: "/legalism", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			link := db.Link{
				DealRoomID:       dealRoom,
				FolderScopeMode:  tc.mode,
				FolderScopePaths: tc.paths,
			}
			if got := folderPathInDealRoomScope(link, tc.folderPath); got != tc.want {
				t.Fatalf("folderPathInDealRoomScope() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDealRoomUsesFolderAllowlist_NonDealRoom(t *testing.T) {
	link := db.Link{
		FolderScopeMode:  FolderScopeModeAllowlist,
		FolderScopePaths: []string{"/legal"},
	}
	if dealRoomUsesFolderAllowlist(link) {
		t.Fatal("document links must not use folder allowlist")
	}
}

func TestLinkSessionInvalidatingChange(t *testing.T) {
	base := db.Link{
		RequireEmail:             true,
		RequireEmailVerification: false,
		RequireNda:               false,
		RequirePassword:          false,
		PermissionType:           "email_required",
		PasswordHash:             pgtype.Text{},
		NdaDocumentID:            pgtype.UUID{},
		ExpiresAt:                pgtype.Timestamptz{},
		MaxAccessCount:           pgtype.Int4{},
		FolderScopePaths:         []string{"/legal"},
		FolderScopeMode:          FolderScopeModeAllowlist,
	}

	t.Run("folder scope alone does not invalidate", func(t *testing.T) {
		if linkSessionInvalidatingChange(
			base,
			base.RequireEmail,
			base.RequireEmailVerification,
			base.RequireNda,
			base.RequirePassword,
			base.PasswordHash,
			base.PermissionType,
			base.ExpiresAt,
			base.MaxAccessCount,
			base.NdaDocumentID,
			base.NdaTemplateID,
		) {
			t.Fatal("unchanged gates must not invalidate sessions")
		}
	})

	t.Run("password requirement invalidates", func(t *testing.T) {
		if !linkSessionInvalidatingChange(
			base,
			base.RequireEmail,
			base.RequireEmailVerification,
			base.RequireNda,
			true,
			pgtype.Text{String: "hash", Valid: true},
			base.PermissionType,
			base.ExpiresAt,
			base.MaxAccessCount,
			base.NdaDocumentID,
			base.NdaTemplateID,
		) {
			t.Fatal("password gate change must invalidate sessions")
		}
	})
}
