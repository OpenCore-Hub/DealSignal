// Package signal aggregates workspace signals and tracks action-item statuses.
package signal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrActionNotFound = errors.New("action not found")

// Service manages signals and action items.
type Service struct {
	queries *db.Queries
}

// NewService creates a signal service.
func NewService(q *db.Queries) *Service {
	return &Service{queries: q}
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

	if err := s.syncFromSuggestions(ctx, wsUUID); err != nil {
		return Feed{}, fmt.Errorf("sync signals: %w", err)
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
	return action, nil
}

// CreateFromSuggestion ensures a signal and action item exist for a suggestion.
func (s *Service) CreateFromSuggestion(ctx context.Context, suggestion db.Suggestion, lang string) (db.Signal, db.ActionItem, error) {
	existing, err := s.queries.GetSignalBySuggestion(ctx, db.GetSignalBySuggestionParams{
		SuggestionID: suggestion.ID,
		WorkspaceID:  suggestion.WorkspaceID,
	})
	if err == nil {
		actions, err := s.queries.ListActionItemsBySignal(ctx, existing.ID)
		if err != nil || len(actions) == 0 {
			action, err := s.createActionForSignal(ctx, existing)
			if err != nil {
				return db.Signal{}, db.ActionItem{}, err
			}
			return existing, action, nil
		}
		return existing, actions[0], nil
	}

	sig, err := s.queries.CreateSignal(ctx, db.CreateSignalParams{
		TenantID:    suggestion.TenantID,
		WorkspaceID: suggestion.WorkspaceID,
		SuggestionID: pgtype.UUID{
			Bytes: uuid.UUID(suggestion.ID.Bytes),
			Valid: true,
		},
		Type:        suggestion.Type,
		Title:       titleForType(suggestion.Type, lang),
		Description: suggestion.Reason,
		Explanation: suggestion.Reason,
		Suggestion:  suggestion.Action,
		DocumentID:  nullableUUID(suggestion.DocumentID),
		ContactID:   nullableUUID(suggestion.ContactID),
		LinkID:      nullableUUID(suggestion.LinkID),
		Priority:    priorityForType(suggestion.Type),
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

func (s *Service) syncFromSuggestions(ctx context.Context, workspaceID pgtype.UUID) error {
	suggestions, err := s.queries.ListSuggestionsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list suggestions: %w", err)
	}
	for _, sug := range suggestions {
		if _, _, err := s.CreateFromSuggestion(ctx, sug, "en"); err != nil {
			return err
		}
	}
	return nil
}

func titleForType(typ, lang string) string {
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
