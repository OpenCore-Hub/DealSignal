package assistant

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func signTestSession(s link.LinkSession, secret string) string {
	s.ExpiresAt = time.Now().Add(15 * time.Minute).Unix()
	payload, _ := json.Marshal(s)
	enc := base64.URLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(enc))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	return sig + "." + enc
}

type mockLinkResolver struct {
	link db.Link
	err  error
}

func (m *mockLinkResolver) ResolvePublicLink(_ context.Context, _ string) (db.Link, error) {
	return m.link, m.err
}

func TestPublicHandlerMissingSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), &mockLinkResolver{}, &config.Config{})
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/assistant/chat", bytes.NewReader([]byte(`{}`)))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestPublicHandlerInvalidSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), &mockLinkResolver{}, &config.Config{LinkSessionSecret: "secret"})
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/assistant/chat", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Link-Session", "bad-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestPublicHandlerLinkDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	resolver := &mockLinkResolver{err: link.ErrLinkDisabled}
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), resolver, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/assistant/chat", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestPublicHandlerAICopilotDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	resolver := &mockLinkResolver{link: db.Link{AiCopilotEnabled: false}}
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), resolver, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/assistant/chat", bytes.NewReader([]byte(`{"message":"hi"}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestPublicHandlerChatSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	docID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	resolver := &mockLinkResolver{link: db.Link{AiCopilotEnabled: true, DocumentID: pgtype.UUID{Bytes: docID, Valid: true}}}

	sessionID := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	q := &mockQuerier{sessionID: sessionID}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{{ChunkID: "chunk-1", PageNumber: 1, Quote: "quote"}}}
	l := &mockLLM{answer: "answer"}
	svc := NewService(q, s, evidence.NewFormatter(), l)

	h := NewPublicHandler(svc, resolver, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1", VisitorID: "v1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/assistant/chat", bytes.NewReader([]byte(`{"message":"hello"}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Answer != "answer" {
		t.Fatalf("expected answer answer, got %q", resp.Answer)
	}
}

func TestPublicHandlerLinkExpired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	resolver := &mockLinkResolver{err: link.ErrLinkExpired}
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), resolver, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/assistant/chat", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", w.Code)
	}
}

func TestPublicHandlerMessageRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	docID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	resolver := &mockLinkResolver{link: db.Link{AiCopilotEnabled: true, DocumentID: pgtype.UUID{Bytes: docID, Valid: true}}}
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), resolver, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/assistant/chat", bytes.NewReader([]byte(`{"message":"   "}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestMapPublicLinkErrorDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	mapPublicLinkError(c, errors.New("boom"))
	if c.Writer.Status() != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", c.Writer.Status())
	}
}
