package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type stubAnswersPlan struct {
	n        int32
	included bool
	err      error
}

func (s stubAnswersPlan) KnowledgeAnswersQuota(context.Context, string) (int32, bool, error) {
	return s.n, s.included, s.err
}

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
	limit, _ = resolveAnswersQuotaLimit(0, 0)
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
	// No limiter → Free fail-closed (desk off).
	if !errors.Is(s.enforceAnswersQuota(context.Background(), uuid.NewString()), ErrQueryQuotaExceeded) {
		t.Fatal("nil limiter must deny free desk")
	}
	if err := s.enforceAnswersQuota(context.Background(), ""); err != nil {
		t.Fatalf("empty workspace: %v", err)
	}
}

func TestEnforceAnswersQuotaPlanLimiter(t *testing.T) {
	t.Parallel()
	wsID := uuid.NewString()
	s := &Service{answersPlan: stubAnswersPlan{n: 100, included: true}}
	if err := s.enforceAnswersQuota(context.Background(), wsID); err != nil {
		t.Fatalf("under plan cap: %v", err)
	}

	s.answersPlan = stubAnswersPlan{n: 0, included: true}
	if err := s.enforceAnswersQuota(context.Background(), wsID); err != nil {
		t.Fatalf("unlimited: %v", err)
	}

	s.answersPlan = stubAnswersPlan{n: 0, included: false}
	if !errors.Is(s.enforceAnswersQuota(context.Background(), wsID), ErrQueryQuotaExceeded) {
		t.Fatal("feature-off 0 must not be unlimited")
	}

	s.answersPlan = stubAnswersPlan{err: errors.New("billing down")}
	if !errors.Is(s.enforceAnswersQuota(context.Background(), wsID), ErrQueryQuotaCheckFailed) {
		t.Fatal("limiter error must fail closed")
	}
}

func TestAnswersQuotaSnapshotUsesPlanNotPartnerDefault(t *testing.T) {
	t.Parallel()
	id := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	s := &Service{answersPlan: stubAnswersPlan{n: 100, included: true}}
	snap := s.answersQuotaSnapshot(context.Background(), id)
	if snap.Limit != 100 || !snap.Included {
		t.Fatalf("limit=%d included=%v want plan 100 included", snap.Limit, snap.Included)
	}
	now := time.Now().UTC()
	since := calendarMonthStartUTC(now)
	wantWindow := since.AddDate(0, 1, 0).Sub(since)
	if snap.Window != wantWindow {
		t.Fatalf("window=%s want calendar month %s", snap.Window, wantWindow)
	}
	s.answersPlan = nil
	snap = s.answersQuotaSnapshot(context.Background(), id)
	if snap.Included || snap.Limit != 0 {
		t.Fatalf("nil limiter must fail-closed to free off, got included=%v limit=%d", snap.Included, snap.Limit)
	}
}

func TestCalendarMonthStartUTC(t *testing.T) {
	t.Parallel()
	got := calendarMonthStartUTC(time.Date(2026, 8, 13, 15, 4, 5, 0, time.FixedZone("CST", 8*3600)))
	want := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
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
