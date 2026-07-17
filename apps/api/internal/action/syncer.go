// Package action synchronizes operational events into dashboard action items.
package action

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// SourceType identifies the operational source of an action item.
const (
	SourceTypeLinkAccessRequest = "link_access_request"
	SourceTypeRoomAccessRequest = "room_access_request"
	SourceTypeRoomNDA           = "room_nda"
	SourceTypeLinkQuestion      = "link_question"
	SourceTypeUploadedFile      = "uploaded_file"
	SourceTypeExpiringLink      = "expiring_link"
	SourceTypeExpiringRoom      = "expiring_room"
)

// Syncer converts pending operational events into action items.
type Syncer struct {
	queries *db.Queries
}

// NewSyncer creates a syncer backed by the given queries.
func NewSyncer(q *db.Queries) *Syncer {
	return &Syncer{queries: q}
}

// SyncWorkspace creates or refreshes action items for all pending operational
// events in a workspace. It is idempotent: existing items are touched only to
// update their updated_at, so concurrent calls are safe.
func (s *Syncer) SyncWorkspace(ctx context.Context, workspaceID string) error {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	ws, err := s.queries.GetWorkspaceByID(ctx, wsUUID)
	if err != nil {
		return fmt.Errorf("get workspace: %w", err)
	}

	if err := s.syncLinkAccessRequests(ctx, ws.TenantID, wsUUID); err != nil {
		return err
	}
	if err := s.syncRoomAccessRequests(ctx, ws.TenantID, wsUUID); err != nil {
		return err
	}
	if err := s.syncRoomNDAs(ctx, ws.TenantID, wsUUID); err != nil {
		return err
	}
	if err := s.syncLinkQuestions(ctx, ws.TenantID, wsUUID); err != nil {
		return err
	}
	if err := s.syncUploadedFiles(ctx, ws.TenantID, wsUUID); err != nil {
		return err
	}
	if err := s.syncExpiringLinks(ctx, ws.TenantID, wsUUID); err != nil {
		return err
	}
	return s.syncExpiringRooms(ctx, ws.TenantID, wsUUID)
}

// ResolveBySource marks an operational action item as done when the underlying
// event is resolved (approved/rejected/answered/signed/renewed/verified). It is
// best-effort and does not return an error if the item does not exist.
func (s *Syncer) ResolveBySource(ctx context.Context, workspaceID, sourceType, sourceID string) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return
	}
	item, err := s.queries.GetActionItemBySource(ctx, db.GetActionItemBySourceParams{
		WorkspaceID: wsUUID,
		SourceType:  pgtype.Text{String: sourceType, Valid: true},
		SourceID:    pgtype.Text{String: sourceID, Valid: true},
	})
	if err != nil {
		return
	}
	if item.Status != "done" {
		_, _ = s.queries.UpdateActionItemStatus(ctx, db.UpdateActionItemStatusParams{
			Status:      "done",
			ID:          item.ID,
			WorkspaceID: wsUUID,
		})
	}
}

func (s *Syncer) syncLinkAccessRequests(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	rows, err := s.queries.ListPendingLinkAccessRequestsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list pending link access requests: %w", err)
	}
	for _, r := range rows {
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeLinkAccessRequest, r.ID, pgtype.Text{String: r.Email, Valid: true}, r.LinkName, "approve"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Syncer) syncRoomAccessRequests(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	rows, err := s.queries.ListPendingRoomAccessRequestsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list pending room access requests: %w", err)
	}
	for _, r := range rows {
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeRoomAccessRequest, r.ID, pgtype.Text{String: r.Email, Valid: true}, pgtype.Text{String: r.RoomName, Valid: true}, "approve"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Syncer) syncRoomNDAs(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	rows, err := s.queries.ListPendingRoomNDAsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list pending room ndas: %w", err)
	}
	for _, r := range rows {
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeRoomNDA, r.ID, pgtype.Text{String: r.Email, Valid: true}, pgtype.Text{String: r.RoomName, Valid: true}, "sign"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Syncer) syncLinkQuestions(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	rows, err := s.queries.ListPendingLinkQuestionsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list pending link questions: %w", err)
	}
	for _, r := range rows {
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeLinkQuestion, r.ID, r.VisitorEmail, r.LinkName, "answer"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Syncer) syncUploadedFiles(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	rows, err := s.queries.ListPendingUploadedFilesByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list pending uploaded files: %w", err)
	}
	for _, r := range rows {
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeUploadedFile, r.ID, pgtype.Text{String: r.OriginalFilename, Valid: true}, r.LinkName, "verify"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Syncer) syncExpiringLinks(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	rows, err := s.queries.ListExpiringLinksByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list expiring links: %w", err)
	}
	current := make(map[string]bool, len(rows))
	for _, r := range rows {
		id := uuid.UUID(r.ID.Bytes).String()
		current[id] = true
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeExpiringLink, r.ID, pgtype.Text{}, r.Name, "renew"); err != nil {
			return err
		}
	}
	return s.closeStaleActions(ctx, workspaceID, SourceTypeExpiringLink, current)
}

func (s *Syncer) syncExpiringRooms(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	rows, err := s.queries.ListExpiringRoomsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list expiring rooms: %w", err)
	}
	current := make(map[string]bool, len(rows))
	for _, r := range rows {
		id := uuid.UUID(r.ID.Bytes).String()
		current[id] = true
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeExpiringRoom, r.ID, pgtype.Text{}, pgtype.Text{String: r.Name, Valid: true}, "renew"); err != nil {
			return err
		}
	}
	return s.closeStaleActions(ctx, workspaceID, SourceTypeExpiringRoom, current)
}

func (s *Syncer) closeStaleActions(ctx context.Context, workspaceID pgtype.UUID, sourceType string, current map[string]bool) error {
	items, err := s.queries.ListPendingActionItemsBySourceType(ctx, db.ListPendingActionItemsBySourceTypeParams{
		WorkspaceID: workspaceID,
		SourceType:  pgtype.Text{String: sourceType, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("list pending %s actions: %w", sourceType, err)
	}
	for _, item := range items {
		if item.SourceID.Valid && !current[item.SourceID.String] {
			_, _ = s.queries.UpdateActionItemStatus(ctx, db.UpdateActionItemStatusParams{
				Status:      "done",
				ID:          item.ID,
				WorkspaceID: workspaceID,
			})
		}
	}
	return nil
}

func (s *Syncer) upsertOperational(ctx context.Context, tenantID, workspaceID pgtype.UUID, sourceType string, sourceID pgtype.UUID, actor pgtype.Text, target pgtype.Text, actionType string) error {
	_, err := s.queries.CreateOperationalActionItem(ctx, db.CreateOperationalActionItemParams{
		TenantID:    tenantID,
		WorkspaceID: workspaceID,
		SourceType:  pgtype.Text{String: sourceType, Valid: true},
		SourceID:    pgtype.Text{String: uuid.UUID(sourceID.Bytes).String(), Valid: sourceID.Valid},
		Title:       titleFor(sourceType, actor.String, target.String),
		Impact:      impactFor(sourceType),
		DueAt:       pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		Status:      "pending",
		ActionType:  actionType,
	})
	return err
}

func titleFor(sourceType, actor, target string) string {
	switch sourceType {
	case SourceTypeLinkAccessRequest:
		if target != "" {
			return fmt.Sprintf("Approve access request from %s for %s", actor, target)
		}
		return fmt.Sprintf("Approve access request from %s", actor)
	case SourceTypeRoomAccessRequest:
		if target != "" {
			return fmt.Sprintf("Approve room access request from %s for %s", actor, target)
		}
		return fmt.Sprintf("Approve room access request from %s", actor)
	case SourceTypeRoomNDA:
		if target != "" {
			return fmt.Sprintf("NDA signature required from %s for %s", actor, target)
		}
		return fmt.Sprintf("NDA signature required from %s", actor)
	case SourceTypeLinkQuestion:
		if target != "" {
			return fmt.Sprintf("Answer question from %s on %s", actor, target)
		}
		return fmt.Sprintf("Answer question from %s", actor)
	case SourceTypeUploadedFile:
		if target != "" {
			return fmt.Sprintf("Review uploaded file %s on %s", actor, target)
		}
		return fmt.Sprintf("Review uploaded file %s", actor)
	case SourceTypeExpiringLink:
		if target != "" {
			return fmt.Sprintf("Link %s expires soon", target)
		}
		return "A share link expires soon"
	case SourceTypeExpiringRoom:
		if target != "" {
			return fmt.Sprintf("Deal room %s expires soon", target)
		}
		return "A deal room expires soon"
	default:
		return fmt.Sprintf("Review %s", sourceType)
	}
}

func impactFor(sourceType string) string {
	switch sourceType {
	case SourceTypeLinkAccessRequest, SourceTypeRoomAccessRequest, SourceTypeRoomNDA, SourceTypeExpiringLink, SourceTypeExpiringRoom:
		return "high"
	default:
		return "medium"
	}
}

func pgUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}
