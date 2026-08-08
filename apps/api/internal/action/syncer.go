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
//
// Share-link access requests are split by product surface so Document Library
// and Deal Room inboxes never share a dashboard deep-link:
//   - link_access_request            → document library share (source_id = link id)
//   - deal_room_link_access_request  → deal-room share (source_id = link id, target_id = room id)
// Room-level items use source_id = room id for both resolve and navigation.
const (
	SourceTypeLinkAccessRequest         = "link_access_request"
	SourceTypeDealRoomLinkAccessRequest = "deal_room_link_access_request"
	SourceTypeRoomAccessRequest         = "room_access_request"
	SourceTypeRoomNDA                   = "room_nda"
	SourceTypeLinkQuestion              = "link_question"
	SourceTypeDealRoomLinkQuestion      = "deal_room_link_question"
	SourceTypeUploadedFile              = "uploaded_file"
	SourceTypeExpiringLink              = "expiring_link"
	SourceTypeExpiringRoom              = "expiring_room"
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

	if err := s.syncDocumentLinkAccessRequests(ctx, ws.TenantID, wsUUID); err != nil {
		return err
	}
	if err := s.syncDealRoomLinkAccessRequests(ctx, ws.TenantID, wsUUID); err != nil {
		return err
	}
	if err := s.syncRoomAccessRequests(ctx, ws.TenantID, wsUUID); err != nil {
		return err
	}
	if err := s.syncRoomNDAs(ctx, ws.TenantID, wsUUID); err != nil {
		return err
	}
	if err := s.syncPendingAskTurns(ctx, ws.TenantID, wsUUID); err != nil {
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

func (s *Syncer) syncDocumentLinkAccessRequests(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	rows, err := s.queries.ListPendingDocumentLinkAccessRequestsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list pending document link access requests: %w", err)
	}
	current := make(map[string]bool, len(rows))
	for _, r := range rows {
		linkID := uuid.UUID(r.LinkID.Bytes).String()
		current[linkID] = true
		// source_id = link id; no target_id (Document Library surface).
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeLinkAccessRequest, r.LinkID, pgtype.UUID{}, pgtype.Text{String: r.Email, Valid: true}, r.LinkName, "approve"); err != nil {
			return err
		}
	}
	return s.closeStaleActions(ctx, workspaceID, SourceTypeLinkAccessRequest, current)
}

func (s *Syncer) syncDealRoomLinkAccessRequests(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	rows, err := s.queries.ListPendingDealRoomLinkAccessRequestsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list pending deal-room link access requests: %w", err)
	}
	current := make(map[string]bool, len(rows))
	for _, r := range rows {
		linkID := uuid.UUID(r.LinkID.Bytes).String()
		current[linkID] = true
		// source_id = link id (resolve key); target_id = room id (navigation).
		targetName := pgtype.Text{String: r.RoomName, Valid: r.RoomName != ""}
		if r.LinkName.Valid && r.LinkName.String != "" {
			targetName = r.LinkName
		}
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeDealRoomLinkAccessRequest, r.LinkID, r.DealRoomID, pgtype.Text{String: r.Email, Valid: true}, targetName, "approve"); err != nil {
			return err
		}
	}
	return s.closeStaleActions(ctx, workspaceID, SourceTypeDealRoomLinkAccessRequest, current)
}

func (s *Syncer) syncRoomAccessRequests(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	rows, err := s.queries.ListPendingRoomAccessRequestsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list pending room access requests: %w", err)
	}
	current := make(map[string]bool, len(rows))
	for _, r := range rows {
		roomID := uuid.UUID(r.RoomID.Bytes).String()
		current[roomID] = true
		// source_id = room id so dashboard navigates to the deal room access tab.
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeRoomAccessRequest, r.RoomID, pgtype.UUID{}, pgtype.Text{String: r.Email, Valid: true}, pgtype.Text{String: r.RoomName, Valid: true}, "approve"); err != nil {
			return err
		}
	}
	return s.closeStaleActions(ctx, workspaceID, SourceTypeRoomAccessRequest, current)
}

func (s *Syncer) syncRoomNDAs(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	rows, err := s.queries.ListPendingRoomNDAsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list pending room ndas: %w", err)
	}
	current := make(map[string]bool, len(rows))
	for _, r := range rows {
		roomID := uuid.UUID(r.RoomID.Bytes).String()
		current[roomID] = true
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeRoomNDA, r.RoomID, pgtype.UUID{}, pgtype.Text{String: r.Email, Valid: true}, pgtype.Text{String: r.RoomName, Valid: true}, "sign"); err != nil {
			return err
		}
	}
	return s.closeStaleActions(ctx, workspaceID, SourceTypeRoomNDA, current)
}

const (
	operationalActionTypeAnswer = "answer"
	operationalActionTypeReview = "review"
)

func (s *Syncer) syncPendingAskTurns(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	hostRows, err := s.queries.ListPendingAskTurnsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list pending ask turns: %w", err)
	}
	formalRows, err := s.queries.ListPendingFormalAskTurnsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list pending formal ask turns: %w", err)
	}
	n := len(hostRows) + len(formalRows)
	currentDealRoom := make(map[string]bool, n)
	currentLibrary := make(map[string]bool, n)
	for _, r := range hostRows {
		if err := s.upsertPendingAskTurnAction(ctx, tenantID, workspaceID, r.ID, r.DealRoomID, r.LinkID, r.VisitorEmail, r.LinkName, operationalActionTypeAnswer, currentDealRoom, currentLibrary); err != nil {
			return err
		}
	}
	for _, r := range formalRows {
		if err := s.upsertPendingAskTurnAction(ctx, tenantID, workspaceID, r.ID, r.DealRoomID, r.LinkID, r.VisitorEmail, r.LinkName, operationalActionTypeReview, currentDealRoom, currentLibrary); err != nil {
			return err
		}
	}
	if err := s.closeStaleActionsBySourceID(ctx, workspaceID, SourceTypeDealRoomLinkQuestion, currentDealRoom); err != nil {
		return err
	}
	return s.closeStaleActionsBySourceID(ctx, workspaceID, SourceTypeLinkQuestion, currentLibrary)
}

func (s *Syncer) upsertPendingAskTurnAction(
	ctx context.Context,
	tenantID, workspaceID pgtype.UUID,
	turnID, dealRoomID, linkID pgtype.UUID,
	visitorEmail, linkName pgtype.Text,
	actionType string,
	currentDealRoom, currentLibrary map[string]bool,
) error {
	if !turnID.Valid {
		return nil
	}
	turnKey := uuid.UUID(turnID.Bytes).String()
	if dealRoomID.Valid {
		currentDealRoom[turnKey] = true
		target := dealRoomAskTargetID(dealRoomID, linkID)
		return s.upsertOperationalTextTarget(ctx, tenantID, workspaceID, SourceTypeDealRoomLinkQuestion, turnID, target, visitorEmail, linkName, actionType)
	}
	if !linkID.Valid {
		return nil
	}
	// Document-library share links: source_id = turn, target_id = link for Ask inbox deep-link.
	currentLibrary[turnKey] = true
	return s.upsertOperationalTextTarget(ctx, tenantID, workspaceID, SourceTypeLinkQuestion, turnID, uuid.UUID(linkID.Bytes).String(), visitorEmail, linkName, actionType)
}

func dealRoomAskTargetID(roomID, linkID pgtype.UUID) string {
	room := uuid.UUID(roomID.Bytes).String()
	if !linkID.Valid {
		return room
	}
	return room + "/" + uuid.UUID(linkID.Bytes).String()
}

func (s *Syncer) upsertOperationalTextTarget(
	ctx context.Context,
	tenantID, workspaceID pgtype.UUID,
	sourceType string,
	sourceID pgtype.UUID,
	targetID string,
	actor, target pgtype.Text,
	actionType string,
) error {
	_, err := s.queries.CreateOperationalActionItem(ctx, db.CreateOperationalActionItemParams{
		TenantID:    tenantID,
		WorkspaceID: workspaceID,
		SourceType:  pgtype.Text{String: sourceType, Valid: true},
		SourceID:    pgtype.Text{String: uuid.UUID(sourceID.Bytes).String(), Valid: sourceID.Valid},
		TargetID:    pgtype.Text{String: targetID, Valid: targetID != ""},
		Title:       titleForAction(sourceType, actionType, actor.String, target.String),
		Impact:      impactFor(sourceType),
		DueAt:       pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		Status:      "pending",
		ActionType:  actionType,
	})
	return err
}

func (s *Syncer) closeStaleActionsBySourceID(ctx context.Context, workspaceID pgtype.UUID, sourceType string, current map[string]bool) error {
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

func (s *Syncer) syncUploadedFiles(ctx context.Context, tenantID, workspaceID pgtype.UUID) error {
	rows, err := s.queries.ListPendingUploadedFilesByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list pending uploaded files: %w", err)
	}
	for _, r := range rows {
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeUploadedFile, r.ID, pgtype.UUID{}, pgtype.Text{String: r.OriginalFilename, Valid: true}, r.LinkName, "verify"); err != nil {
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
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeExpiringLink, r.ID, pgtype.UUID{}, pgtype.Text{}, r.Name, "renew"); err != nil {
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
		if err := s.upsertOperational(ctx, tenantID, workspaceID, SourceTypeExpiringRoom, r.ID, pgtype.UUID{}, pgtype.Text{}, pgtype.Text{String: r.Name, Valid: true}, "renew"); err != nil {
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

func (s *Syncer) upsertOperational(
	ctx context.Context,
	tenantID, workspaceID pgtype.UUID,
	sourceType string,
	sourceID, targetID pgtype.UUID,
	actor, target pgtype.Text,
	actionType string,
) error {
	_, err := s.queries.CreateOperationalActionItem(ctx, db.CreateOperationalActionItemParams{
		TenantID:    tenantID,
		WorkspaceID: workspaceID,
		SourceType:  pgtype.Text{String: sourceType, Valid: true},
		SourceID:    pgtype.Text{String: uuid.UUID(sourceID.Bytes).String(), Valid: sourceID.Valid},
		TargetID:    pgtype.Text{String: uuid.UUID(targetID.Bytes).String(), Valid: targetID.Valid},
		Title:       titleFor(sourceType, actor.String, target.String),
		Impact:      impactFor(sourceType),
		DueAt:       pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		Status:      "pending",
		ActionType:  actionType,
	})
	return err
}

func titleForAction(sourceType, actionType, actor, target string) string {
	if actionType == operationalActionTypeReview &&
		(sourceType == SourceTypeDealRoomLinkQuestion || sourceType == SourceTypeLinkQuestion) {
		if target != "" {
			return fmt.Sprintf("Review formal Q&A from %s on %s", actor, target)
		}
		return fmt.Sprintf("Review formal Q&A from %s", actor)
	}
	return titleFor(sourceType, actor, target)
}

func titleFor(sourceType, actor, target string) string {
	switch sourceType {
	case SourceTypeLinkAccessRequest:
		if target != "" {
			return fmt.Sprintf("Approve access request from %s for %s", actor, target)
		}
		return fmt.Sprintf("Approve access request from %s", actor)
	case SourceTypeDealRoomLinkAccessRequest:
		if target != "" {
			return fmt.Sprintf("Approve deal room share access from %s for %s", actor, target)
		}
		return fmt.Sprintf("Approve deal room share access from %s", actor)
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
	case SourceTypeDealRoomLinkQuestion:
		if target != "" {
			return fmt.Sprintf("Answer visitor Ask from %s on %s", actor, target)
		}
		return fmt.Sprintf("Answer visitor Ask from %s", actor)
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
	case SourceTypeLinkAccessRequest, SourceTypeDealRoomLinkAccessRequest, SourceTypeRoomAccessRequest, SourceTypeRoomNDA, SourceTypeExpiringLink, SourceTypeExpiringRoom:
		return "high"
	default:
		return "medium"
	}
}

func pgUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid uuid %q: %w", id, err)
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}
