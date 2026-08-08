package link

import (
	"context"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
)

// FormalAskInsightsAdapter bridges link Formal Ask events into the suggestions service.
type FormalAskInsightsAdapter struct {
	Suggestions *suggestions.Service
}

func (a FormalAskInsightsAdapter) OnSubmitted(
	ctx context.Context,
	workspaceID, linkID, documentID, turnID, sessionID, visitorID, visitorEmail, question, lang string,
) error {
	if a.Suggestions == nil {
		return nil
	}
	return a.Suggestions.CreateFormalAskSuggestion(ctx, suggestions.CreateFormalAskSuggestionInput{
		WorkspaceID:  workspaceID,
		LinkID:       linkID,
		DocumentID:   documentID,
		TurnID:       turnID,
		SessionID:    sessionID,
		VisitorID:    visitorID,
		VisitorEmail: visitorEmail,
		Question:     question,
		Lang:         lang,
	})
}

func (a FormalAskInsightsAdapter) OnResolved(ctx context.Context, workspaceID, turnID string) error {
	if a.Suggestions == nil {
		return nil
	}
	return a.Suggestions.ResolveFormalAskSuggestion(ctx, workspaceID, turnID)
}
