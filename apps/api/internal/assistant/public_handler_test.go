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
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

type mockRateLimiter struct {
	allow     bool
	calls     int
	lastKey   string
	lastLimit int
}

func (m *mockRateLimiter) RateLimitAllow(_ context.Context, key string, limit int, _ time.Duration) (bool, int, error) {
	m.calls++
	m.lastKey = key
	m.lastLimit = limit
	if m.allow {
		return true, limit, nil
	}
	return false, 0, nil
}

type securityEventCall struct {
	eventType string
	visitorID string
	email     string
	reason    string
}

type mockSecurityEvents struct {
	events []securityEventCall
}

func (m *mockSecurityEvents) RecordSecurityEvent(_ context.Context, _ db.Link, eventType, visitorID, email, _, _, reason string) error {
	m.events = append(m.events, securityEventCall{
		eventType: eventType,
		visitorID: visitorID,
		email:     email,
		reason:    reason,
	})
	return nil
}

func TestPublicAskDocsMissingSessionDoesNotWriteSecurityEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	events := &mockSecurityEvents{}
	h := NewPublicHandler(NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{}), &mockAccessAuthorizer{}, &config.Config{})
	h.WithSecurityEvents(events)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{"message":"hi"}`)))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if len(events.events) != 0 {
		t.Fatalf("ordinary 401 must not write security events, got %+v", events.events)
	}
}

func TestPublicAskDocsScopeViolationWritesSecurityEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	inScope := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	outOfScope := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	auth := &mockAccessAuthorizer{result: link.AccessResult{
		Link: db.Link{
			AiCopilotEnabled: true,
			DocumentID:       pgtype.UUID{Bytes: inScope, Valid: true},
			WorkspaceID:      pgtype.UUID{Bytes: uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"), Valid: true},
			PublicToken:      "token-1",
		},
		VisitorID: "v1",
		Email:     "v@example.com",
	}}
	sessionID := pgtype.UUID{Bytes: [16]byte{13}, Valid: true}
	q := &mockQuerier{
		sessionID:   sessionID,
		legacyDocOK: true,
		legacyDoc:   db.GetDocumentByIDRow{ID: pgtype.UUID{Bytes: inScope, Valid: true}},
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "ok", DocumentID: inScope.String(), Quote: "in"},
		{ChunkID: "bad", DocumentID: outOfScope.String(), Quote: "out"},
	}}
	events := &mockSecurityEvents{}
	h := NewPublicHandler(NewService(q, s, evidence.NewFormatter(), &mockLLM{answer: "ans"}), auth, cfg)
	h.WithSecurityEvents(events)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1", VisitorID: "v1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{"message":"q"}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Evidence) != 1 {
		t.Fatalf("expected in-scope evidence only, got %+v", resp.Evidence)
	}
	if len(events.events) != 1 || events.events[0].eventType != "scope_violation" {
		t.Fatalf("expected scope_violation security event, got %+v", events.events)
	}
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

func TestPublicAskDocsRateLimitReturns429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	docID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	linkID := pgtype.UUID{Bytes: uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"), Valid: true}
	auth := &mockAccessAuthorizer{result: link.AccessResult{
		Link: db.Link{
			ID:               linkID,
			AiCopilotEnabled: true,
			DocumentID:       pgtype.UUID{Bytes: docID, Valid: true},
			PublicToken:      "token-1",
		},
		VisitorID: "v1",
		Email:     "v@example.com",
	}}

	sessionID := pgtype.UUID{Bytes: [16]byte{11}, Valid: true}
	q := &mockQuerier{
		sessionID:   sessionID,
		legacyDocOK: true,
		legacyDoc:   db.GetDocumentByIDRow{ID: pgtype.UUID{Bytes: docID, Valid: true}},
	}
	limiter := &mockRateLimiter{allow: false}
	events := &mockSecurityEvents{}
	h := NewPublicHandler(NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{answer: "ans"}), auth, cfg)
	h.WithRateLimiter(limiter).WithSecurityEvents(events)

	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1", VisitorID: "v1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{"message":"hello"}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "rate_limit_exceeded" {
		t.Fatalf("expected rate_limit_exceeded, got %v", body["code"])
	}
	if len(events.events) != 1 || events.events[0].eventType != "rate_limit_exceeded" {
		t.Fatalf("expected rate_limit_exceeded security event, got %+v", events.events)
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
	q := &mockQuerier{
		sessionID:   sessionID,
		legacyDocOK: true,
		legacyDoc:   db.GetDocumentByIDRow{ID: pgtype.UUID{Bytes: docID, Valid: true}},
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "chunk-1", DocumentID: docID.String(), PageNumber: 1, Quote: "quote"},
	}}
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

func TestPublicAskDocsRetrievalScopedToAccessAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	inScope := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	outOfScope := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	auth := &mockAccessAuthorizer{result: link.AccessResult{
		Link: db.Link{
			AiCopilotEnabled: true,
			DealRoomID:       pgtype.UUID{Bytes: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), Valid: true},
			FolderScopeMode:  link.FolderScopeModeAllowlist,
			FolderScopePaths: []string{"/general"},
			PublicToken:      "token-1",
		},
		VisitorID: "v1",
	}}

	sessionID := pgtype.UUID{Bytes: [16]byte{10}, Valid: true}
	q := &mockQuerier{
		sessionID: sessionID,
		roomDocs: []db.ListDealRoomDocumentsWithMetaRow{
			{DocumentID: pgtype.UUID{Bytes: inScope, Valid: true}, FolderPath: "/general"},
			{DocumentID: pgtype.UUID{Bytes: outOfScope, Valid: true}, FolderPath: "/legal"},
		},
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "ok", DocumentID: inScope.String(), Quote: "in"},
		{ChunkID: "bad", DocumentID: outOfScope.String(), Quote: "out"},
	}}
	h := NewPublicHandler(NewService(q, s, evidence.NewFormatter(), &mockLLM{answer: "ans"}), auth, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1", VisitorID: "v1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{"message":"scope?"}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !s.inDocumentsCalled {
		t.Fatal("expected SearchInDocuments at public Ask Docs seam")
	}
	if len(s.lastDocumentIDs) != 1 || s.lastDocumentIDs[0] != inScope {
		t.Fatalf("retrieval must match Access allowlist, got %v", s.lastDocumentIDs)
	}
	if s.searchCalled {
		t.Fatal("public Ask Docs must not use workspace-wide Search")
	}
	var resp ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Evidence) != 1 || resp.Evidence[0].DocumentID != inScope.String() {
		t.Fatalf("response must drop out-of-scope evidence, got %+v", resp.Evidence)
	}
}

func TestPublicAskDocsTruncatesEvidenceQuotes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{LinkSessionSecret: "secret"}
	docID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	auth := &mockAccessAuthorizer{result: link.AccessResult{
		Link: db.Link{
			AiCopilotEnabled: true,
			DocumentID:       pgtype.UUID{Bytes: docID, Valid: true},
			PublicToken:      "token-1",
		},
		VisitorID: "v1",
	}}
	longQuote := strings.Repeat("字", 400)
	sessionID := pgtype.UUID{Bytes: [16]byte{12}, Valid: true}
	q := &mockQuerier{
		sessionID:   sessionID,
		legacyDocOK: true,
		legacyDoc:   db.GetDocumentByIDRow{ID: pgtype.UUID{Bytes: docID, Valid: true}},
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "c1", DocumentID: docID.String(), PageNumber: 3, Quote: longQuote},
	}}
	h := NewPublicHandler(NewService(q, s, evidence.NewFormatter(), &mockLLM{answer: "ans"}), auth, cfg)
	r := gin.New()
	h.RegisterPublicRoutes(r.Group("/api/v1/public"))

	token := signTestSession(link.LinkSession{PublicToken: "token-1", VisitorID: "v1"}, cfg.LinkSessionSecret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/public/links/token-1/assistant/chat", bytes.NewReader([]byte(`{"message":"q"}`)))
	req.Header.Set("X-Link-Session", token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Evidence) != 1 {
		t.Fatalf("expected 1 evidence, got %d", len(resp.Evidence))
	}
	if got := utf8.RuneCountInString(resp.Evidence[0].Quote); got != 320 {
		t.Fatalf("quote rune length = %d, want 320", got)
	}
	if resp.Evidence[0].PageNumber != 3 {
		t.Fatalf("page jump must remain, got page %d", resp.Evidence[0].PageNumber)
	}
	if resp.Evidence[0].DocumentID != docID.String() {
		t.Fatalf("document_id must remain, got %q", resp.Evidence[0].DocumentID)
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
	q := &mockQuerier{
		sessionID:   sessionID,
		legacyDocOK: true,
		legacyDoc:   db.GetDocumentByIDRow{ID: pgtype.UUID{Bytes: docID, Valid: true}},
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "chunk-1", DocumentID: docID.String(), PageNumber: 1, Quote: "quote"},
	}}
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
