package assistant

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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

type mockAccessAuthorizer struct {
	result    link.AccessResult
	err       error
	calls     int
	lastToken string
}

func (m *mockAccessAuthorizer) AuthorizePublicAccess(_ *gin.Context, publicToken string) (link.AccessResult, error) {
	m.calls++
	m.lastToken = publicToken
	return m.result, m.err
}

// legacyAuthorizer adapts mockLinkResolver for older handler tests.
func legacyAuthorizer(lr *mockLinkResolver) PublicAccessAuthorizer {
	return publicAccessFunc(func(c *gin.Context, publicToken string) (link.AccessResult, error) {
		row, err := lr.ResolvePublicLink(c.Request.Context(), publicToken)
		if err != nil {
			return link.AccessResult{}, err
		}
		return link.AccessResult{Link: row}, nil
	})
}

type publicAccessFunc func(c *gin.Context, publicToken string) (link.AccessResult, error)

func (f publicAccessFunc) AuthorizePublicAccess(c *gin.Context, publicToken string) (link.AccessResult, error) {
	return f(c, publicToken)
}

func TestPublicAskDocsRejectsMissingSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), &mockAccessAuthorizer{}, &config.Config{})
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{"message":"hi"}`)))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestPublicAskDocsRejectsTokenMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	docID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	auth := &mockAccessAuthorizer{result: link.AccessResult{
		Link: db.Link{AiCopilotEnabled: true, DocumentID: pgtype.UUID{Bytes: docID, Valid: true}},
	}}
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), auth, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	// Session bound to token-1, but request path uses token-2.
	token := signTestSession(link.LinkSession{PublicToken: "token-1", VisitorID: "v1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-2/assistant/chat", bytes.NewReader([]byte(`{"message":"hello"}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if auth.calls != 0 {
		t.Fatalf("Access authorizer must not run on token mismatch, got %d calls", auth.calls)
	}
}

func TestPublicAskDocsAllowsAuthorizedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	docID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	auth := &mockAccessAuthorizer{result: link.AccessResult{
		Link:      db.Link{AiCopilotEnabled: true, DocumentID: pgtype.UUID{Bytes: docID, Valid: true}},
		VisitorID: "v1",
		Email:     "visitor@example.com",
	}}

	sessionID := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	q := &mockQuerier{sessionID: sessionID}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{{ChunkID: "chunk-1", PageNumber: 1, Quote: "quote"}}}
	l := &mockLLM{answer: "answer"}
	svc := NewService(q, s, evidence.NewFormatter(), l)

	h := NewPublicHandler(svc, auth, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1", VisitorID: "v1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{"message":"hello"}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if auth.calls != 1 {
		t.Fatalf("expected Access authorizer to be called once, got %d", auth.calls)
	}
	if auth.lastToken != "token-1" {
		t.Fatalf("expected authorizer token token-1, got %q", auth.lastToken)
	}
	var resp ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Answer != "answer" {
		t.Fatalf("expected answer answer, got %q", resp.Answer)
	}
}

func TestPublicHandlerInvalidSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), &mockAccessAuthorizer{}, &config.Config{LinkSessionSecret: "secret"})
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Link-Session", "bad-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestPublicHandlerLinkDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	auth := &mockAccessAuthorizer{err: link.ErrLinkDisabled}
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), auth, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	// Access-parity uses mapAccessError: disabled → 410 Gone.
	if w.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", w.Code)
	}
}

func TestPublicAskDocsRejectsBlockedEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	auth := &mockAccessAuthorizer{err: link.ErrBlockedEmail}
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), auth, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1", Email: "blocked@example.com"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{"message":"hi"}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "blocked_email" {
		t.Fatalf("expected blocked_email, got %v", body["code"])
	}
}

func TestPublicAskDocsRejectsWhenAIDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	auth := &mockAccessAuthorizer{result: link.AccessResult{Link: db.Link{AiCopilotEnabled: false}}}
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), auth, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{"message":"hi"}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "ai_copilot_disabled" {
		t.Fatalf("expected ai_copilot_disabled, got %v", body["code"])
	}
}

func TestPublicHandlerChatSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	docID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	auth := &mockAccessAuthorizer{result: link.AccessResult{
		Link:      db.Link{AiCopilotEnabled: true, DocumentID: pgtype.UUID{Bytes: docID, Valid: true}},
		VisitorID: "v1",
	}}

	sessionID := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	q := &mockQuerier{sessionID: sessionID}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{{ChunkID: "chunk-1", PageNumber: 1, Quote: "quote"}}}
	l := &mockLLM{answer: "answer"}
	svc := NewService(q, s, evidence.NewFormatter(), l)

	h := NewPublicHandler(svc, auth, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1", VisitorID: "v1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{"message":"hello"}`)))
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
	auth := &mockAccessAuthorizer{err: link.ErrLinkExpired}
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), auth, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{}`)))
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
	auth := legacyAuthorizer(&mockLinkResolver{link: db.Link{AiCopilotEnabled: true, DocumentID: pgtype.UUID{Bytes: docID, Valid: true}}})
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), auth, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{"message":"   "}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
