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
	res         knowledge.QueryResponse
	err         error
}

func (s stubVisitorAskKnowledge) Enabled() bool { return s.enabled }

func (s stubVisitorAskKnowledge) RoomCorpusAskReady(_ context.Context, _, _ pgtype.UUID) bool {
	if !s.enabled {
		return false
	}
	if s.corpusReady != nil {
		return *s.corpusReady
	}
	return true
}

func (s stubVisitorAskKnowledge) QueryLinkScoped(
	_ context.Context,
	_, _ string,
	_ []uuid.UUID,
	_ knowledge.LinkScopedRequest,
) (knowledge.QueryResponse, error) {
	if s.err != nil {
		return knowledge.QueryResponse{}, s.err
	}
	return s.res, nil
}
