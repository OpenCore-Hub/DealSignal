package workspace

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

const (
	maxLogoBytes  = 5 * 1024 * 1024
	logoURLExpiry = 7 * 24 * time.Hour
)

var logoContentTypes = map[string]string{
	"image/png":     ".png",
	"image/jpeg":    ".jpg",
	"image/jpg":     ".jpg",
	"image/webp":    ".webp",
	"image/gif":     ".gif",
	"image/svg+xml": ".svg",
	"image/svg":     ".svg",
}

func brandLogoKey(tenantID, workspaceID string) string {
	return fmt.Sprintf("tenants/%s/workspaces/%s/brand/logo", tenantID, workspaceID)
}

func normalizeLogoContentType(contentType, filename string) (string, error) {
	ctype := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if _, ok := logoContentTypes[ctype]; ok {
		if ctype == "image/svg" {
			return "image/svg+xml", nil
		}
		return ctype, nil
	}
	switch strings.ToLower(path.Ext(filename)) {
	case ".png":
		return "image/png", nil
	case ".jpg", ".jpeg":
		return "image/jpeg", nil
	case ".webp":
		return "image/webp", nil
	case ".gif":
		return "image/gif", nil
	case ".svg":
		return "image/svg+xml", nil
	default:
		return "", ErrInvalidLogoType
	}
}

func (s *Service) resolveLogoURL(ctx context.Context, workspaceID string) string {
	if s.storage == nil {
		return ""
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return ""
	}
	row, err := s.queries.GetWorkspaceLogo(ctx, wsUUID)
	if err != nil {
		return ""
	}
	url, err := s.storage.PresignedGetURL(ctx, row.StorageKey, logoURLExpiry)
	if err != nil {
		return ""
	}
	return url
}

// UploadLogo stores a workspace brand logo and returns updated settings.
func (s *Service) UploadLogo(ctx context.Context, workspaceID, tenantID string, file multipart.File, header *multipart.FileHeader) (Settings, error) {
	if s.storage == nil {
		return Settings{}, ErrLogoStorageUnavailable
	}
	if header == nil || header.Size <= 0 {
		return Settings{}, ErrInvalidLogoType
	}
	if header.Size > maxLogoBytes {
		return Settings{}, ErrLogoTooLarge
	}
	contentType, err := normalizeLogoContentType(header.Header.Get("Content-Type"), header.Filename)
	if err != nil {
		return Settings{}, err
	}

	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Settings{}, err
	}
	if _, err := s.queries.GetWorkspaceByID(ctx, wsUUID); err != nil {
		return Settings{}, err
	}
	if err := s.AssertCanUseBranding(ctx, workspaceID); err != nil {
		return Settings{}, err
	}

	_, _ = file.Seek(0, io.SeekStart)
	key := brandLogoKey(tenantID, workspaceID)
	if err := s.storage.PutObject(ctx, key, file, header.Size, contentType); err != nil {
		return Settings{}, fmt.Errorf("store logo: %w", err)
	}
	if _, err := s.queries.UpsertWorkspaceLogo(ctx, db.UpsertWorkspaceLogoParams{
		WorkspaceID: wsUUID,
		StorageKey:  key,
		ContentType: contentType,
	}); err != nil {
		return Settings{}, err
	}
	return s.GetSettings(ctx, workspaceID)
}
