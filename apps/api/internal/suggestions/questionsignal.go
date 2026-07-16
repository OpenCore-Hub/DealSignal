package suggestions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateQuestionSignalInput describes a high-intent assistant question to convert into a signal.
type CreateQuestionSignalInput struct {
	WorkspaceID  string
	LinkID       string
	DocumentID   string
	SessionID    string
	VisitorID    string
	VisitorEmail string
	UserID       string
	UserEmail    string
	Question     string
	Intent       string
	Lang         string
}

// CreateQuestionSignal turns a high-intent assistant question into a follow-up/hot signal.
// It is safe to call from an async goroutine after the assistant answer has been sent.
func (s *Service) CreateQuestionSignal(ctx context.Context, input CreateQuestionSignalInput) error {
	if !questionIntentCreatesSignal(input.Intent) {
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
		// No active link to attach the signal to; skip creation.
		return nil
	}

	actor := input.VisitorEmail
	if actor == "" {
		actor = input.UserEmail
	}
	if actor == "" {
		actor = input.VisitorID
	}
	if actor == "" {
		actor = input.UserID
	}

	var contactID pgtype.UUID
	if actor != "" {
		contact, cerr := s.queries.GetContactByEmailAndWorkspace(ctx, db.GetContactByEmailAndWorkspaceParams{
			Email:       pgText(actor),
			WorkspaceID: wsUUID,
		})
		if cerr == nil {
			contactID = contact.ID
		}
	}

	sessionID := input.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	exists, err := s.recentQuestionExists(ctx, wsUUID, sessionID)
	if err != nil {
		return fmt.Errorf("check recent question signal: %w", err)
	}
	if exists {
		return nil
	}

	docTitle := ""
	if docUUID.Valid {
		doc, derr := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{ID: docUUID, WorkspaceID: wsUUID})
		if derr == nil {
			docTitle = doc.Title
		}
	}

	sigType := questionSignalType(input.Intent)
	ls := newLocalizedStrings(input.Lang)
	ctxSnapshot := Context{
		Question:      input.Question,
		Intent:        input.Intent,
		Actor:         actor,
		DocumentTitle: docTitle,
	}

	metadata := map[string]string{
		"session_id": sessionID,
		"intent":     input.Intent,
		"actor":      actor,
	}
	if input.VisitorID != "" {
		metadata["visitor_id"] = input.VisitorID
	}
	if input.UserID != "" {
		metadata["user_id"] = input.UserID
	}

	_, err = s.queries.CreateSuggestion(ctx, db.CreateSuggestionParams{
		TenantID:    tenantID,
		WorkspaceID: wsUUID,
		ContactID:   contactID,
		LinkID:      linkUUID,
		DocumentID:  docUUID,
		Type:        sigType,
		Subtype:     pgText(SubtypeQuestion),
		Reason:      fmt.Sprintf(ls.questionReasonTmpl, actor, input.Question),
		Action:      ls.questionAction,
		Metadata:    metadataToBytes(metadata),
		Context:     ctxSnapshot.ToJSONB(),
	})
	if err != nil {
		return fmt.Errorf("create question suggestion: %w", err)
	}

	if sigType == "hot_signal" && s.notifier != nil {
		userID := ""
		if linkUUID.Valid {
			link, lerr := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{ID: linkUUID, WorkspaceID: wsUUID})
			if lerr == nil && link.CreatedBy.Valid {
				userID = uuid.UUID(link.CreatedBy.Bytes).String()
			}
		}
		_ = s.notifier.Enqueue(ctx, input.WorkspaceID, userID, "email",
			titleForSubtype(SubtypeQuestion, sigType, input.Lang),
			fmt.Sprintf(ls.questionReasonTmpl, actor, input.Question)+"\n"+ls.questionAction)
	}

	return nil
}

func (s *Service) resolveLinkAndDocument(ctx context.Context, tenantID, wsUUID pgtype.UUID, linkID, documentID string) (pgtype.UUID, pgtype.UUID, pgtype.UUID, error) {
	var linkUUID pgtype.UUID
	var docUUID pgtype.UUID

	if linkID != "" {
		parsed, err := pgUUID(linkID)
		if err != nil {
			return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("invalid link id: %w", err)
		}
		linkUUID = parsed
		link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{ID: linkUUID, WorkspaceID: wsUUID})
		if err != nil {
			return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("link not found: %w", err)
		}
		if link.DocumentID.Valid {
			docUUID = link.DocumentID
		}
		return linkUUID, docUUID, link.TenantID, nil
	}

	if documentID != "" {
		parsed, err := pgUUID(documentID)
		if err != nil {
			return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("invalid document id: %w", err)
		}
		docUUID = parsed
		doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{ID: docUUID, WorkspaceID: wsUUID})
		if err != nil {
			return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("document not found: %w", err)
		}
		links, err := s.queries.ListLinksByDocument(ctx, db.ListLinksByDocumentParams{
			WorkspaceID: wsUUID,
			DocumentID:  docUUID,
		})
		if err != nil {
			return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("list links by document: %w", err)
		}
		now := time.Now()
		for _, l := range links {
			if l.Status != "active" {
				continue
			}
			if l.ExpiresAt.Valid && l.ExpiresAt.Time.Before(now) {
				continue
			}
			linkUUID = l.ID
			break
		}
		return linkUUID, docUUID, doc.TenantID, nil
	}

	return pgtype.UUID{}, pgtype.UUID{}, tenantID, nil
}

func (s *Service) recentQuestionExists(ctx context.Context, wsUUID pgtype.UUID, sessionID string) (bool, error) {
	md := map[string]string{"session_id": sessionID}
	b, _ := json.Marshal(md)
	count, err := s.queries.CountRecentQuestionSuggestionsBySession(ctx, db.CountRecentQuestionSuggestionsBySessionParams{
		WorkspaceID:     wsUUID,
		SessionMetadata: b,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func questionIntentCreatesSignal(intent string) bool {
	switch intent {
	case "pricing", "objection", "timeline", "implementation", "feature_request":
		return true
	}
	return false
}

func questionSignalType(intent string) string {
	switch intent {
	case "pricing", "objection", "timeline":
		return "hot_signal"
	}
	return "follow_up"
}
