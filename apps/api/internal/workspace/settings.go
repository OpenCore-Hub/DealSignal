package workspace

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

// MemberDetail is the public view of a workspace member with user profile.
type MemberDetail struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	JoinedAt  string `json:"joined_at"`
	Status    string `json:"status"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// Settings is the public view of workspace general settings.
type Settings struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	BrandColor   string `json:"brand_color"`
	ViewerDomain string `json:"viewer_domain"`
	LogoURL      string `json:"logo_url,omitempty"`
}

// SecuritySettings is the public view of workspace security settings.
type SecuritySettings struct {
	ForceEmailVerification bool `json:"force_email_verification"`
	WatermarkDownloads     bool `json:"watermark_downloads"`
	TwoFactorEnabled       bool `json:"two_factor_enabled"`
}

// Billing is the public view of workspace billing usage.
type Billing struct {
	Plan         string `json:"plan"`
	Period       string `json:"period"`
	StorageUsed  int64  `json:"storage_used"`
	StorageLimit int64  `json:"storage_limit"`
	LinksUsed    int64  `json:"links_used"`
	LinksLimit   int64  `json:"links_limit"`
	RoomsUsed    int64  `json:"rooms_used"`
	RoomsLimit   int64  `json:"rooms_limit"`
}

// ListMembers returns workspace members with basic profile info.
func (s *Service) ListMembers(ctx context.Context, workspaceID string) ([]MemberDetail, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListWorkspaceMembers(ctx, wsUUID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberDetail, 0, len(rows))
	for _, r := range rows {
		out = append(out, MemberDetail{
			ID:       uuidToString(r.UserID),
			UserID:   uuidToString(r.UserID),
			Email:    r.Email,
			Name:     "",
			Role:     r.Role,
			JoinedAt: r.JoinedAt.Time.Format(time.RFC3339),
			Status:   "active",
		})
	}
	return out, nil
}

// GetSettings returns workspace general settings.
func (s *Service) GetSettings(ctx context.Context, workspaceID string) (Settings, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Settings{}, err
	}
	ws, err := s.queries.GetWorkspaceByID(ctx, wsUUID)
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		Name:         ws.Name,
		Slug:         ws.Slug,
		BrandColor:   ws.BrandColor.String,
		ViewerDomain: "",
		LogoURL:      "",
	}, nil
}

// UpdateSettings updates workspace general settings.
func (s *Service) UpdateSettings(ctx context.Context, workspaceID, name, brandColor string) (Settings, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Settings{}, err
	}
	ws, err := s.queries.UpdateWorkspace(ctx, db.UpdateWorkspaceParams{
		ID:         wsUUID,
		Name:       name,
		BrandColor: pgtype.Text{String: brandColor, Valid: brandColor != ""},
	})
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		Name:         ws.Name,
		Slug:         ws.Slug,
		BrandColor:   ws.BrandColor.String,
		ViewerDomain: "",
		LogoURL:      "",
	}, nil
}

// GetSecurity returns workspace security settings.
func (s *Service) GetSecurity(ctx context.Context, workspaceID string) (SecuritySettings, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return SecuritySettings{}, err
	}
	ws, err := s.queries.GetWorkspaceByID(ctx, wsUUID)
	if err != nil {
		return SecuritySettings{}, err
	}
	return SecuritySettings{
		ForceEmailVerification: ws.ForceEmailVerification,
		WatermarkDownloads:     ws.WatermarkDownloads,
		TwoFactorEnabled:       ws.TwoFactorEnabled,
	}, nil
}

// UpdateSecurity updates workspace security settings.
func (s *Service) UpdateSecurity(ctx context.Context, workspaceID string, req SecuritySettings) (SecuritySettings, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return SecuritySettings{}, err
	}
	ws, err := s.queries.UpdateWorkspaceSecurity(ctx, db.UpdateWorkspaceSecurityParams{
		ForceEmailVerification: req.ForceEmailVerification,
		WatermarkDownloads:     req.WatermarkDownloads,
		TwoFactorEnabled:       req.TwoFactorEnabled,
		ID:                     wsUUID,
	})
	if err != nil {
		return SecuritySettings{}, err
	}
	return SecuritySettings{
		ForceEmailVerification: ws.ForceEmailVerification,
		WatermarkDownloads:     ws.WatermarkDownloads,
		TwoFactorEnabled:       ws.TwoFactorEnabled,
	}, nil
}

// GetBilling returns workspace billing usage (currently defaults with zero usage).
func (s *Service) GetBilling(ctx context.Context, workspaceID string) (Billing, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Billing{}, err
	}
	// Count links and deal rooms for the workspace.
	links, err := s.queries.ListLinksByWorkspace(ctx, wsUUID)
	if err != nil {
		return Billing{}, fmt.Errorf("count links: %w", err)
	}
	rooms, err := s.queries.ListDealRoomsByWorkspace(ctx, wsUUID)
	if err != nil {
		return Billing{}, fmt.Errorf("count deal rooms: %w", err)
	}
	storageUsage, err := s.queries.GetWorkspaceStorageUsage(ctx, wsUUID)
	if err != nil {
		return Billing{}, fmt.Errorf("get storage usage: %w", err)
	}

	return Billing{
		Plan:         "free",
		Period:       "monthly",
		StorageUsed:  storageUsage,
		StorageLimit: 1073741824, // 1 GB
		LinksUsed:    int64(len(links)),
		LinksLimit:   100,
		RoomsUsed:    int64(len(rooms)),
		RoomsLimit:   10,
	}, nil
}
