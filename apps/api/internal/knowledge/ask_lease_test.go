package knowledge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAskLeaseHoldsUntilAuditCompletes(t *testing.T) {
	admission := newMemoryAskAdmission(0)
	var started atomic.Bool
	release := make(chan struct{})
	h := &Handler{
		admission: admission,
		runner: stubSessionRunner{fn: func(
			ctx context.Context, _, _, _ string, _ SessionQueryRequest, _ string,
		) (SessionQueryResponse, error) {
			started.Store(true)
			select {
			case <-ctx.Done():
				return SessionQueryResponse{}, ctx.Err()
			case <-release:
				return SessionQueryResponse{SessionID: "s", Turn: QATurn{ID: "t"}}, nil
			}
		}},
	}
	r := testKnowledgeRouter(h)

	reqCtx, cancelClient := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query/stream",
		strings.NewReader(`{"query":"first","clientRequestId":"cr-1"}`),
	)
	req = req.WithContext(reqCtx)
	req.Header.Set("Content-Type", "application/json")

	done := make(chan struct{})
	go func() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for !started.Load() {
		select {
		case <-deadline:
			t.Fatal("first request did not start")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	assertBusy := func(label string) {
		t.Helper()
		reqN := httptest.NewRequest(http.MethodPost,
			"/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query/stream",
			strings.NewReader(`{"query":"overlap","clientRequestId":"cr-overlap"}`),
		)
		reqN.Header.Set("Content-Type", "application/json")
		wN := httptest.NewRecorder()
		r.ServeHTTP(wN, reqN)
		if wN.Code != http.StatusTooManyRequests {
			t.Fatalf("%s: expected busy, got %d", label, wN.Code)
		}
	}

	assertBusy("while first in flight")

	cancelClient()
	assertBusy("after disconnect before audit completes")

	close(release)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("first handler did not exit after audit completed")
	}

	req3 := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query/stream",
		strings.NewReader(`{"query":"third","clientRequestId":"cr-3"}`),
	)
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code == http.StatusTooManyRequests {
		t.Fatalf("expected retry after audit release, body=%s", w3.Body.String())
	}
}

func TestQuerySessionJSONHoldUntilAuditCompletes(t *testing.T) {
	admission := newMemoryAskAdmission(0)
	block := make(chan struct{})
	h := &Handler{
		admission: admission,
		runner: stubSessionRunner{fn: func(
			ctx context.Context, _, _, _ string, _ SessionQueryRequest, transport string,
		) (SessionQueryResponse, error) {
			if transport != "json" {
				t.Fatalf("transport=%q", transport)
			}
			select {
			case <-ctx.Done():
				<-block
				return SessionQueryResponse{
					SessionID: "s",
					Turn:      QATurn{ID: "t", ResultStatus: "error", ErrorSummary: "client_cancelled"},
				}, nil
			case <-block:
				return SessionQueryResponse{
					SessionID: "s",
					Turn:      QATurn{ID: "t2", ResultStatus: "answered"},
				}, nil
			}
		}},
	}
	r := testKnowledgeRouter(h)

	reqCtx, cancelClient := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query",
		strings.NewReader(`{"query":"hold","clientRequestId":"cr-json"}`),
	)
	req = req.WithContext(reqCtx)
	req.Header.Set("Content-Type", "application/json")

	done := make(chan struct{})
	go func() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancelClient()

	reqOverlap := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query",
		strings.NewReader(`{"query":"overlap","clientRequestId":"cr-overlap"}`),
	)
	reqOverlap.Header.Set("Content-Type", "application/json")
	wOverlap := httptest.NewRecorder()
	r.ServeHTTP(wOverlap, reqOverlap)
	if wOverlap.Code != http.StatusTooManyRequests {
		t.Fatalf("expected busy before json audit completes, got %d", wOverlap.Code)
	}

	close(block)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("json handler did not exit after audit completed")
	}

	reqAfter := httptest.NewRequest(http.MethodPost,
		"/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query",
		strings.NewReader(`{"query":"after","clientRequestId":"cr-after"}`),
	)
	reqAfter.Header.Set("Content-Type", "application/json")
	wAfter := httptest.NewRecorder()
	r.ServeHTTP(wAfter, reqAfter)
	if wAfter.Code == http.StatusTooManyRequests {
		t.Fatalf("expected retry after json audit release, body=%s", wAfter.Body.String())
	}
}
