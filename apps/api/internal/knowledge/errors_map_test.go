package knowledge

import (
	"context"
	"net/http"
	"testing"
)

func TestMapKnowledgeErrorClientCancelled(t *testing.T) {
	t.Parallel()
	for _, err := range []error{errStreamClientCancelled, context.Canceled} {
		body := mapKnowledgeError(err)
		if body.Code != "client_cancelled" {
			t.Fatalf("err=%v code=%q", err, body.Code)
		}
		if body.Status != http.StatusOK {
			t.Fatalf("err=%v status=%d", err, body.Status)
		}
	}
}

func TestMapKnowledgeErrorQueryBusy(t *testing.T) {
	t.Parallel()
	body := mapKnowledgeError(ErrQueryBusy)
	if body.Code != "knowledge_query_busy" || body.Status != http.StatusTooManyRequests {
		t.Fatalf("%+v", body)
	}
}
