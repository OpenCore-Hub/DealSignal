package knowledge

import (
	"context"
	"testing"
	"time"
)

func TestResolveAnswersQuotaLimit(t *testing.T) {
	t.Parallel()
	limit, window := resolveAnswersQuotaLimit(100, 1000)
	if limit != 100 || window != 24*time.Hour {
		t.Fatalf("daily: limit=%d window=%s", limit, window)
	}
	limit, window = resolveAnswersQuotaLimit(0, 500)
	if limit != 500 || window != 30*24*time.Hour {
		t.Fatalf("monthly: limit=%d window=%s", limit, window)
	}
	limit, window = resolveAnswersQuotaLimit(0, 0)
	if limit != 0 {
		t.Fatalf("unlimited: %d", limit)
	}
}

func TestStreamErrorFromQuotaExceeded(t *testing.T) {
	t.Parallel()
	p := streamErrorFrom(ErrQueryQuotaExceeded)
	if p.Code != "knowledge_query_quota_exceeded" {
		t.Fatalf("%+v", p)
	}
}

func TestEnforceAnswersQuotaNilService(t *testing.T) {
	t.Parallel()
	var s *Service
	if err := s.enforceAnswersQuota(context.Background(), "ws"); err != nil {
		t.Fatalf("nil service: %v", err)
	}
	s = &Service{}
	// No queries → Used stays 0; default DailyAnswers is high → allow.
	if err := s.enforceAnswersQuota(context.Background(), "ws-1"); err != nil {
		t.Fatalf("under limit: %v", err)
	}
	if err := s.enforceAnswersQuota(context.Background(), ""); err != nil {
		t.Fatalf("empty workspace: %v", err)
	}
}

func TestMapKnowledgeErrorQuotaCheckFailed(t *testing.T) {
	t.Parallel()
	body := mapKnowledgeError(ErrQueryQuotaCheckFailed)
	if body.Code != "knowledge_query_quota_unavailable" || body.Status != 503 {
		t.Fatalf("%+v", body)
	}
	body = mapKnowledgeError(ErrAnswerRequiresSession)
	if body.Code != "answer_requires_session" || body.Status != 400 {
		t.Fatalf("%+v", body)
	}
}
