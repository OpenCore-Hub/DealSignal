package knowledge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSessionQueryHandleWaitDone(t *testing.T) {
	var ran bool
	handle := &sessionQueryHandle{
		outcomeCh: make(chan sessionQueryOutcome, 1),
		done:      make(chan struct{}),
	}
	go func() {
		defer close(handle.done)
		time.Sleep(20 * time.Millisecond)
		ran = true
		handle.outcomeCh <- sessionQueryOutcome{
			res: SessionQueryResponse{SessionID: "s1"},
		}
	}()

	done := make(chan struct{})
	go func() {
		handle.waitDone()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("waitDone did not return")
	}
	if !ran {
		t.Fatal("runner did not finish before waitDone returned")
	}
}

func TestWaitForSessionQueryPrefersOutcomeOverCancel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx, cancel := context.WithCancel(req.Context())
	c.Request = req.WithContext(ctx)

	stream := newSessionStream(c, w, 30*time.Second)
	handle := &sessionQueryHandle{
		outcomeCh: make(chan sessionQueryOutcome, 1),
		done:      make(chan struct{}),
	}
	close(handle.done)
	handle.outcomeCh <- sessionQueryOutcome{
		res: SessionQueryResponse{SessionID: "sess-ok", Turn: QATurn{ID: "t1"}},
	}

	cancel()
	res, err := stream.waitForSessionQuery(handle)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if res.SessionID != "sess-ok" {
		t.Fatalf("session=%q", res.SessionID)
	}
}

func TestWaitForSessionQueryReturnsCancelWhenDisconnected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx, cancel := context.WithCancel(req.Context())
	c.Request = req.WithContext(ctx)
	cancel()

	stream := newSessionStream(c, w, 30*time.Second)
	handle := &sessionQueryHandle{
		outcomeCh: make(chan sessionQueryOutcome, 1),
		done:      make(chan struct{}),
	}

	_, err := stream.waitForSessionQuery(handle)
	if err != errStreamClientCancelled {
		t.Fatalf("expected errStreamClientCancelled, got %v", err)
	}
}

func TestWriteAuditedResultEmitsDone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	stream := newSessionStream(c, w, 30*time.Second)
	ok := stream.writeAuditedResult(
		SessionQueryRequest{Query: "q", Answer: true},
		SessionQueryResponse{
			SessionID: "sess-1",
			Query:     "q",
			Answer:    "Hello world",
			Turn: QATurn{
				ID:           "t1",
				SessionID:    "sess-1",
				Question:     "q",
				Answer:       "Hello world",
				ResultStatus: "answered",
				Hits:         []QueryHit{{ChunkID: "c1"}},
			},
		},
	)
	if !ok {
		t.Fatal("writeAuditedResult returned false")
	}
	out := w.Body.String()
	for _, want := range []string{"event: phase", "event: sources", "event: token", "event: done"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
}
