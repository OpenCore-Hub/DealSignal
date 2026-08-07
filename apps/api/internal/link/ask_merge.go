package link

import (
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
)

func coveredHostQuestionIDs(hostQuestionIDs []string) map[string]struct{} {
	covered := make(map[string]struct{}, len(hostQuestionIDs))
	for _, id := range hostQuestionIDs {
		if id == "" {
			continue
		}
		covered[id] = struct{}{}
	}
	return covered
}

func filterLegacyQuestionsNotInTurns(
	legacy []db.LinkVisitorQuestion,
	covered map[string]struct{},
) []db.LinkVisitorQuestion {
	out := make([]db.LinkVisitorQuestion, 0, len(legacy))
	for _, q := range legacy {
		qid := uuid.UUID(q.ID.Bytes).String()
		if _, ok := covered[qid]; ok {
			continue
		}
		out = append(out, q)
	}
	return out
}
