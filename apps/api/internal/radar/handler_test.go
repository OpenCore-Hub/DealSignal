package radar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/signal"
	"github.com/gin-gonic/gin"
)

func TestHandlerGetEvidenceMapsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{service: &Service{}}

	// Invalid UUID → ErrItemNotFound before DB.
	r := gin.New()
	r.GET("/workspaces/:workspaceSlug/radar/items/:id/evidence", func(c *gin.Context) {
		c.Set("workspaceID", "00000000-0000-0000-0000-000000000001")
		h.GetEvidence(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/workspaces/acme/radar/items/not-a-uuid/evidence", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "not_found" {
		t.Fatalf("code=%v", body["code"])
	}
}

func TestHandlerUpdateItemRejectsInvalidStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{service: &Service{}}

	r := gin.New()
	r.PATCH("/workspaces/:workspaceSlug/radar/items/:id", func(c *gin.Context) {
		c.Set("workspaceID", "00000000-0000-0000-0000-000000000001")
		h.UpdateItem(c)
	})

	body, _ := json.Marshal(map[string]any{"status": "archived"})
	req := httptest.NewRequest(http.MethodPatch, "/workspaces/acme/radar/items/00000000-0000-0000-0000-000000000099", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["code"] != "invalid_input" {
		t.Fatalf("code=%v", resp["code"])
	}
}

func TestHandlerUpdateItemMapsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Empty service signals → UpdateItem fails at Get path; use stub feed that returns ErrActionNotFound.
	h := &Handler{service: NewService(nil, notFoundFeed{})}

	r := gin.New()
	r.PATCH("/workspaces/:workspaceSlug/radar/items/:id", func(c *gin.Context) {
		c.Set("workspaceID", "00000000-0000-0000-0000-000000000001")
		h.UpdateItem(c)
	})

	body, _ := json.Marshal(map[string]any{"status": "done", "outcome": "acted"})
	req := httptest.NewRequest(http.MethodPatch, "/workspaces/acme/radar/items/00000000-0000-0000-0000-000000000099", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["code"] != "not_found" {
		t.Fatalf("code=%v", resp["code"])
	}
}

type notFoundFeed struct{}

func (notFoundFeed) GetFeed(ctx context.Context, workspaceID, userID string) (signal.Feed, error) {
	return signal.Feed{}, nil
}

func (notFoundFeed) UpdateActionStatus(ctx context.Context, workspaceID, actionID, status string, snoozeHours int, outcome string) (db.ActionItem, error) {
	return db.ActionItem{}, signal.ErrActionNotFound
}

func TestParseCircleLensValues(t *testing.T) {
	if got := ParseCircle("sales"); got != "sales" {
		t.Fatalf("sales → %s", got)
	}
	if got := ParseCircle("investor_ir"); got != "investor_ir" {
		t.Fatalf("investor_ir → %s", got)
	}
	if got := ParseCircle("founder"); got != "founder" {
		t.Fatalf("founder → %s", got)
	}
	if got := ParseCircle("nope"); got != "founder" {
		t.Fatalf("unknown → %s want founder", got)
	}
	if got := ParseCircle(""); got != "founder" {
		t.Fatalf("empty → %s want founder", got)
	}
}

func TestErrItemNotFoundIsSentinel(t *testing.T) {
	if !errors.Is(ErrItemNotFound, ErrItemNotFound) {
		t.Fatal("sentinel broken")
	}
}
