package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type stubSessionRunner struct {
	fn func(ctx context.Context, roomID, workspaceID, userID string, req SessionQueryRequest, transport string) (SessionQueryResponse, error)
}

func (s stubSessionRunner) runSessionQuery(
	ctx context.Context,
	roomID, workspaceID, userID string,
	req SessionQueryRequest,
	transport string,
) (SessionQueryResponse, error) {
	return s.fn(ctx, roomID, workspaceID, userID, req, transport)
}

type stubAnswersQuota struct {
	err error
}

func (s stubAnswersQuota) enforceAnswersQuota(context.Context, string) error {
	return s.err
}

type stubCorpusReady struct {
	err error
}

func (s stubCorpusReady) enforceCorpusReady(context.Context, string, string, string) error {
	return s.err
}

func testKnowledgeRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", "user-1")
		c.Set("workspaceID", "ws-1")
		c.Next()
	})
	api := r.Group("/api/workspaces/:workspaceSlug")
	h.RegisterWorkspaceRoutes(api)
	return r
}

func TestQuerySessionStreamSSEDone(t *testing.T) {
	h := &Handler{
		admission: newMemoryAskAdmission(0),
		runner: stubSessionRunner{fn: func(
			_ context.Context, _, _, _ string, req SessionQueryRequest, transport string,
		) (SessionQueryResponse, error) {
			if transport != "stream" {
				t.Fatalf("transport=%q", transport)
			}
			return SessionQueryResponse{
				SessionID: "sess-1",
				Query:     req.Query,
				Mode:      "hybrid",
				Answer:    "Grounded answer for: " + req.Query,
				Results:   []QueryHit{{ChunkID: "c1", Text: "clause", Score: 0.9}},
				Turn: QATurn{
					ID:             "turn-1",
					SessionID:      "sess-1",
					Sequence:       1,
					Question:       req.Query,
					Answer:         "Grounded answer for: " + req.Query,
					ResultStatus:   "answered",
					Hits:           []QueryHit{{ChunkID: "c1", Text: "clause", Score: 0.9}},
					RetrieveQuery:  "Acme SAFE valuation cap",
					RewriteApplied: true,
				},
			}, nil
		}},
	}
	r := testKnowledgeRouter(h)
	body := `{"query":"他们免费吗？","answer":true,"top_k":8}`
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type=%q", ct)
	}
	out := w.Body.String()
	for _, want := range []string{
		"event: phase",
		`"phase":"retrieving"`,
		`"phase":"generating"`,
		`"rewriteApplied":true`,
		`"retrieveQuery":"Acme SAFE valuation cap"`,
		"event: sources",
		"event: token",
		"event: done",
		`"resultStatus":"answered"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
}

func TestQuerySessionStreamSSEUnavailable(t *testing.T) {
	h := &Handler{
		admission: newMemoryAskAdmission(0),
		runner: stubSessionRunner{fn: func(
			context.Context, string, string, string, SessionQueryRequest, string,
		) (SessionQueryResponse, error) {
			return SessionQueryResponse{}, ErrUnavailable
		}},
	}
	r := testKnowledgeRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query/stream", strings.NewReader(`{"query":"q"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SSE still opens with 200, got %d", w.Code)
	}
	out := w.Body.String()
	if !strings.Contains(out, "event: error") || !strings.Contains(out, "knowledge_unavailable") {
		t.Fatalf("expected SSE error frame, got:\n%s", out)
	}
}

func TestQuerySessionStreamRejectsBusyBeforeSSE(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	h := &Handler{
		admission: newMemoryAskAdmission(0),
		runner: stubSessionRunner{fn: func(
			context.Context, string, string, string, SessionQueryRequest, string,
		) (SessionQueryResponse, error) {
			close(started)
			<-release
			return SessionQueryResponse{
				SessionID: "s",
				Turn:      QATurn{ID: "t", ResultStatus: "answered"},
			}, nil
		}},
	}
	r := testKnowledgeRouter(h)

	go func() {
		req := httptest.NewRequest(http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query/stream", strings.NewReader(`{"query":"first"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first request did not start")
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query/stream", strings.NewReader(`{"query":"second"}`))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", w2.Code, w2.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["code"] != "knowledge_query_busy" {
		t.Fatalf("payload=%v", payload)
	}
	if ct := w2.Header().Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
		t.Fatal("busy reject must not open SSE")
	}
	close(release)
}

func TestQuerySessionStreamRejectsCorpusNotReady(t *testing.T) {
	h := &Handler{
		admission: newMemoryAskAdmission(0),
		corpus:    stubCorpusReady{err: ErrCorpusNotReady},
		runner: stubSessionRunner{fn: func(
			context.Context, string, string, string, SessionQueryRequest, string,
		) (SessionQueryResponse, error) {
			t.Fatal("runner must not run when corpus is not ready")
			return SessionQueryResponse{}, nil
		}},
	}
	r := testKnowledgeRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query/stream", strings.NewReader(`{"query":"q"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "knowledge_corpus_not_ready") {
		t.Fatalf("body=%s", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
		t.Fatal("corpus reject must not open SSE")
	}
}

func TestLegacyQueryRejectsAnswerTrue(t *testing.T) {
	h := &Handler{
		admission: newMemoryAskAdmission(0),
		runner: stubSessionRunner{fn: func(
			context.Context, string, string, string, SessionQueryRequest, string,
		) (SessionQueryResponse, error) {
			t.Fatal("session runner must not run for legacy query")
			return SessionQueryResponse{}, nil
		}},
	}
	r := testKnowledgeRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/knowledge/query", strings.NewReader(`{"query":"q","answer":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "answer_requires_session") {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func TestQuerySessionStreamRejectsQuotaExceeded(t *testing.T) {
	h := &Handler{
		admission: newMemoryAskAdmission(0),
		quota:     stubAnswersQuota{err: ErrQueryQuotaExceeded},
		runner: stubSessionRunner{fn: func(
			context.Context, string, string, string, SessionQueryRequest, string,
		) (SessionQueryResponse, error) {
			t.Fatal("runner must not run when quota exceeded")
			return SessionQueryResponse{}, nil
		}},
	}
	r := testKnowledgeRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query/stream", strings.NewReader(`{"query":"q"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "knowledge_query_quota_exceeded") {
		t.Fatalf("body=%s", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
		t.Fatal("quota reject must not open SSE")
	}
}

func TestQuerySessionStreamRejectsRateLimited(t *testing.T) {
	a := newMemoryAskAdmission(1)
	h := &Handler{
		admission: a,
		runner: stubSessionRunner{fn: func(
			context.Context, string, string, string, SessionQueryRequest, string,
		) (SessionQueryResponse, error) {
			return SessionQueryResponse{SessionID: "s", Turn: QATurn{ResultStatus: "answered"}}, nil
		}},
	}
	r := testKnowledgeRouter(h)
	req1 := httptest.NewRequest(http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query", strings.NewReader(`{"query":"a"}`))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first=%d %s", w1.Code, w1.Body.String())
	}
	req2 := httptest.NewRequest(http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query", strings.NewReader(`{"query":"b"}`))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "knowledge_query_rate_limited") {
		t.Fatalf("body=%s", w2.Body.String())
	}
}

func TestQuerySessionInvalidJSON(t *testing.T) {
	h := &Handler{admission: newMemoryAskAdmission(0), runner: stubSessionRunner{fn: func(
		context.Context, string, string, string, SessionQueryRequest, string,
	) (SessionQueryResponse, error) {
		t.Fatal("runner should not be called")
		return SessionQueryResponse{}, nil
	}}}
	r := testKnowledgeRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query", bytes.NewReader([]byte(`{`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestQuerySessionJSONBusy(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	h := &Handler{
		admission: newMemoryAskAdmission(0),
		runner: stubSessionRunner{fn: func(
			context.Context, string, string, string, SessionQueryRequest, string,
		) (SessionQueryResponse, error) {
			close(started)
			<-release
			return SessionQueryResponse{SessionID: "s", Turn: QATurn{ResultStatus: "answered"}}, nil
		}},
	}
	r := testKnowledgeRouter(h)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query", strings.NewReader(`{"query":"a"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(httptest.NewRecorder(), req)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first")
	}
	req2 := httptest.NewRequest(http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query", strings.NewReader(`{"query":"b"}`))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", w2.Code, w2.Body.String())
	}
	close(release)
}

func TestStreamErrorFromQueryBusy(t *testing.T) {
	t.Parallel()
	p := streamErrorFrom(ErrQueryBusy)
	if p.Code != "knowledge_query_busy" {
		t.Fatalf("%+v", p)
	}
}

func TestSuggestFollowUpsSoftFailsWhenBusy(t *testing.T) {
	t.Parallel()
	a := newMemoryMemberAdmission(followUpAdmissionScope, 0)
	if err := a.Admit(context.Background(), "room-1", "user-1"); err != nil {
		t.Fatal(err)
	}
	// service intentionally nil — gate must soft-fail before SuggestFollowUps.
	h := &Handler{followUpAdmission: a}
	r := testKnowledgeRouter(h)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/workspaces/acme/deal-rooms/room-1/knowledge/turns/turn-1/follow-ups",
		nil,
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var res FollowUpsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 0 {
		t.Fatalf("expected empty items so FE keeps templates, got %#v", res)
	}
}
