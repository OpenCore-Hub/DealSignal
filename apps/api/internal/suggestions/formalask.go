package suggestions

import (
	"context"
	"fmt"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateFormalAskSuggestionInput converts a Formal Q&A turn into an Insights suggestion.
type CreateFormalAskSuggestionInput struct {
	WorkspaceID  string
	LinkID       string
	DocumentID   string
	TurnID       string
	SessionID    string
	VisitorID    string
	VisitorEmail string
	Question     string
	Lang         string
}

// CreateFormalAskSuggestion records a high-intent Formal Ask as a workspace suggestion.
// Idempotent per turn_id (metadata). Safe to call after formal turn commit.
func (s *Service) CreateFormalAskSuggestion(ctx context.Context, input CreateFormalAskSuggestionInput) error {
	if s == nil || s.queries == nil {
		return nil
	}
	turnID := strings.TrimSpace(input.TurnID)
	question := strings.TrimSpace(input.Question)
	if turnID == "" || question == "" || strings.TrimSpace(input.LinkID) == "" {
		return nil
	}

	wsUUID, err := pgUUID(input.WorkspaceID)
	if err != nil {
		return fmt.Errorf("invalid workspace id: %w", err)
	}
	workspace, err := s.queries.GetWorkspaceByID(ctx, wsUUID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	linkUUID, docUUID, tenantID, err := s.resolveLinkAndDocument(ctx, workspace.TenantID, wsUUID, input.LinkID, input.DocumentID)
	if err != nil {
		return err
	}
	if !linkUUID.Valid {
		return nil
	}

	exists, err := s.recentFormalAskExists(ctx, wsUUID, turnID)
	if err != nil {
		return fmt.Errorf("check recent formal ask suggestion: %w", err)
	}
	if exists {
		return nil
	}

	actor := strings.TrimSpace(input.VisitorEmail)
	if actor == "" {
		actor = strings.TrimSpace(input.VisitorID)
	}
	if actor == "" {
		actor = "visitor"
	}

	var contactID pgtype.UUID
	if input.VisitorEmail != "" {
		contact, cerr := s.queries.GetContactByEmailAndWorkspace(ctx, db.GetContactByEmailAndWorkspaceParams{
			Email:       pgText(input.VisitorEmail),
			WorkspaceID: wsUUID,
		})
		if cerr == nil {
			contactID = contact.ID
		}
	}

	docTitle := ""
	if docUUID.Valid {
		doc, derr := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{ID: docUUID, WorkspaceID: wsUUID})
		if derr == nil {
			docTitle = doc.Title
		}
	}

	ls := newLocalizedStrings(input.Lang)
	metadata := map[string]string{
		"turn_id": turnID,
		"formal":  "true",
		"actor":   actor,
	}
	if input.SessionID != "" {
		metadata["session_id"] = input.SessionID
	}
	if input.VisitorID != "" {
		metadata["visitor_id"] = input.VisitorID
	}

	_, err = s.queries.CreateSuggestion(ctx, db.CreateSuggestionParams{
		TenantID:    tenantID,
		WorkspaceID: wsUUID,
		ContactID:   contactID,
		LinkID:      linkUUID,
		DocumentID:  docUUID,
		Type:        "hot_signal",
		Subtype:     pgText(SubtypeFormalAsk),
		Reason:      fmt.Sprintf(ls.formalAskReasonTmpl, actor, question),
		Action:      ls.formalAskAction,
		Metadata:    metadataToBytes(metadata),
		Context: Context{
			Question:      question,
			Intent:        "formal_ask",
			Actor:         actor,
			DocumentTitle: docTitle,
		}.ToJSONB(),
		RuleID: pgText("formal_ask"),
	})
	if err != nil {
		return fmt.Errorf("create formal ask suggestion: %w", err)
	}

	if s.notifier != nil {
		userID := ""
		link, lerr := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{ID: linkUUID, WorkspaceID: wsUUID})
		if lerr == nil && link.CreatedBy.Valid {
			userID = uuid.UUID(link.CreatedBy.Bytes).String()
		}
		_ = s.notifier.Enqueue(ctx, input.WorkspaceID, userID, "email",
			titleForSubtype(SubtypeFormalAsk, "hot_signal", input.Lang),
			fmt.Sprintf(ls.formalAskReasonTmpl, actor, question)+"\n"+ls.formalAskAction)
	}
	return nil
}

// ResolveFormalAskSuggestion dismisses Insights suggestions tied to a published/answered Formal turn.
func (s *Service) ResolveFormalAskSuggestion(ctx context.Context, workspaceID, turnID string) error {
	if s == nil || s.queries == nil {
		return nil
	}
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return nil
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	return s.queries.DismissSuggestionsByMetadata(ctx, db.DismissSuggestionsByMetadataParams{
		WorkspaceID:   wsUUID,
		MatchMetadata: metadataToBytes(map[string]string{"turn_id": turnID}),
	})
}

func (s *Service) recentFormalAskExists(ctx context.Context, workspaceID pgtype.UUID, turnID string) (bool, error) {
	count, err := s.queries.CountActiveSuggestionsByMetadata(ctx, db.CountActiveSuggestionsByMetadataParams{
		WorkspaceID:   workspaceID,
		MatchMetadata: metadataToBytes(map[string]string{"turn_id": turnID}),
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
