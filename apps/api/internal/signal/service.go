// Package signal aggregates workspace signals and tracks action-item statuses.
package signal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrActionNotFound = errors.New("action not found")

// Service manages signals and action items.
type Service struct {
	queries      *db.Queries
	actionSyncer ActionSyncer
}

// ActionSyncer synchronizes operational events into action items.
type ActionSyncer interface {
	SyncWorkspace(ctx context.Context, workspaceID string) error
}

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// WithActionSyncer wires an operational action syncer into the service.
func WithActionSyncer(a ActionSyncer) ServiceOption {
	return func(s *Service) { s.actionSyncer = a }
}

// NewService creates a signal service.
func NewService(q *db.Queries, opts ...ServiceOption) *Service {
	s := &Service{queries: q}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Feed is the workspace signal feed returned to clients.
type Feed struct {
	Signals []db.Signal
	Actions []db.ActionItem
}

// GetFeed returns the signal feed for a workspace, syncing from suggestions first.
func (s *Service) GetFeed(ctx context.Context, workspaceID string) (Feed, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Feed{}, err
	}

	if err := s.SyncWorkspace(ctx, workspaceID); err != nil {
		return Feed{}, fmt.Errorf("sync signals: %w", err)
	}
	if s.actionSyncer != nil {
		if err := s.actionSyncer.SyncWorkspace(ctx, workspaceID); err != nil {
			return Feed{}, fmt.Errorf("sync operational actions: %w", err)
		}
	}

	signals, err := s.queries.ListSignalsByWorkspace(ctx, wsUUID)
	if err != nil {
		return Feed{}, fmt.Errorf("list signals: %w", err)
	}

	actions, err := s.queries.ListActionItemsByWorkspace(ctx, wsUUID)
	if err != nil {
		return Feed{}, fmt.Errorf("list actions: %w", err)
	}

	return Feed{Signals: signals, Actions: actions}, nil
}

// UpdateActionStatus updates the status of an action item.
func (s *Service) UpdateActionStatus(ctx context.Context, workspaceID, actionID, status string) (db.ActionItem, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return db.ActionItem{}, err
	}
	actionUUID, err := pgUUID(actionID)
	if err != nil {
		return db.ActionItem{}, err
	}

	action, err := s.queries.UpdateActionItemStatus(ctx, db.UpdateActionItemStatusParams{
		Status:      status,
		ID:          actionUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		return db.ActionItem{}, fmt.Errorf("update action status: %w", err)
	}

	if status == "done" && action.SignalID.Valid {
		sig, serr := s.queries.GetSignalByID(ctx, db.GetSignalByIDParams{
			ID:          action.SignalID,
			WorkspaceID: wsUUID,
		})
		if serr == nil && sig.SuggestionID.Valid {
			_, _ = s.queries.CreateSuggestionFeedback(ctx, db.CreateSuggestionFeedbackParams{
				TenantID:     sig.TenantID,
				WorkspaceID:  sig.WorkspaceID,
				SuggestionID: sig.SuggestionID,
				FeedbackType: "acted",
			})
		}
	}

	return action, nil
}

// CreateFromSuggestion ensures a signal and action item exist for a suggestion.
// Uses upserts so concurrent callers do not create duplicates.
func (s *Service) CreateFromSuggestion(ctx context.Context, suggestion db.Suggestion, lang string) (db.Signal, db.ActionItem, error) {
	sig, action, err := s.createSignalAndActionFromSuggestion(ctx, suggestion, lang)
	if err != nil {
		return db.Signal{}, db.ActionItem{}, err
	}

	if err := s.markSynced(ctx, []pgtype.UUID{suggestion.ID}); err != nil {
		return sig, action, err
	}
	return sig, action, nil
}

func (s *Service) createSignalAndActionFromSuggestion(ctx context.Context, suggestion db.Suggestion, lang string) (db.Signal, db.ActionItem, error) {
	sig, err := s.queries.CreateSignal(ctx, db.CreateSignalParams{
		TenantID:    suggestion.TenantID,
		WorkspaceID: suggestion.WorkspaceID,
		SuggestionID: pgtype.UUID{
			Bytes: uuid.UUID(suggestion.ID.Bytes),
			Valid: true,
		},
		Type:        suggestion.Type,
		Subtype:     suggestion.Subtype,
		Title:       titleForSubtype(suggestion.Subtype.String, suggestion.Type, lang),
		Description: suggestion.Reason,
		Explanation: suggestion.Reason,
		Suggestion:  suggestion.Action,
		DocumentID:  nullableUUID(suggestion.DocumentID),
		ContactID:   nullableUUID(suggestion.ContactID),
		LinkID:      nullableUUID(suggestion.LinkID),
		Priority:    priorityForType(suggestion.Type),
		Metadata:    suggestion.Metadata,
		Context:     suggestion.Context,
	})
	if err != nil {
		return db.Signal{}, db.ActionItem{}, fmt.Errorf("create signal: %w", err)
	}

	action, err := s.createActionForSignal(ctx, sig)
	if err != nil {
		return db.Signal{}, db.ActionItem{}, err
	}

	return sig, action, nil
}

func (s *Service) createActionForSignal(ctx context.Context, sig db.Signal) (db.ActionItem, error) {
	return s.queries.CreateActionItem(ctx, db.CreateActionItemParams{
		TenantID:    sig.TenantID,
		WorkspaceID: sig.WorkspaceID,
		SignalID:    sig.ID,
		Title:       sig.Suggestion,
		Impact:      sig.Priority,
		DueAt:       pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		Status:      "pending",
		ActionType:  actionTypeForSignalType(sig.Type),
	})
}

func (s *Service) markSynced(ctx context.Context, ids []pgtype.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	if err := s.queries.MarkSuggestionsSynced(ctx, ids); err != nil {
		return fmt.Errorf("mark suggestions synced: %w", err)
	}
	return nil
}

// SyncWorkspace synchronizes all unsynced suggestions for a workspace into signals.
// It is safe to call concurrently and is used by both HTTP polling and the event-driven consumer.
func (s *Service) SyncWorkspace(ctx context.Context, workspaceID string) error {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	start := time.Now()
	if err := s.syncFromSuggestions(ctx, wsUUID); err != nil {
		return err
	}
	observeSignalSyncDuration(workspaceID, start)
	return nil
}

func (s *Service) syncFromSuggestions(ctx context.Context, workspaceID pgtype.UUID) error {
	suggestions, err := s.queries.ListUnsyncedSuggestionsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list unsynced suggestions: %w", err)
	}
	if len(suggestions) == 0 {
		return nil
	}

	lang := locale.FromContext(ctx)
	if lang == "" {
		lang = "en"
	}

	syncedIDs := make([]pgtype.UUID, 0, len(suggestions))
	for _, sug := range suggestions {
		if _, _, err := s.createSignalAndActionFromSuggestion(ctx, sug, lang); err != nil {
			return err
		}
		syncedIDs = append(syncedIDs, sug.ID)
	}

	recordSignalsSynced(uuid.UUID(workspaceID.Bytes).String(), len(syncedIDs))
	return s.markSynced(ctx, syncedIDs)
}

func titleForSubtype(subtype, typ, lang string) string {
	return suggestions.TitleForSubtype(subtype, typ, lang)
}

// TitleForType returns the localized title for a signal type.
func TitleForType(typ, lang string) string {
	return suggestions.TitleForType(typ, lang)
}

func priorityForType(typ string) string {
	switch typ {
	case "hot_signal":
		return "high"
	case "risk_alert":
		return "medium"
	default:
		return "low"
	}
}

func actionTypeForSignalType(typ string) string {
	switch typ {
	case "hot_signal":
		return "call"
	case "risk_alert":
		return "review"
	default:
		return "email"
	}
}

func pgUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func nullableUUID(u pgtype.UUID) pgtype.UUID {
	if u.Valid {
		return u
	}
	return pgtype.UUID{Valid: false}
}
