//go:build integration

package link

import (
	"context"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type stubVisitorAskKnowledge struct {
	enabled     bool
	corpusReady *bool
}

func (s stubVisitorAskKnowledge) Enabled() bool { return s.enabled }

func (s stubVisitorAskKnowledge) QueryLinkScoped(
	_ context.Context,
	_, _ string,
	_ []uuid.UUID,
	_ knowledge.LinkScopedRequest,
) (knowledge.QueryResponse, error) {
	return knowledge.QueryResponse{}, nil
}

func (s stubVisitorAskKnowledge) RoomCorpusAskReady(_ context.Context, _, _ pgtype.UUID) bool {
	if s.corpusReady != nil {
		return *s.corpusReady
	}
	return true
}
