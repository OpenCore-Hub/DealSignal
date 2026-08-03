package docling

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to a docling-rag Platform v2 HTTP API.
type Client struct {
	baseURL    string
	adminKey   string
	httpClient *http.Client
}

// NewClient builds a docling-rag client. baseURL must be non-empty for Enabled().
func NewClient(baseURL, adminKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 180 * time.Second
	}
	return &Client{
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		adminKey: strings.TrimSpace(adminKey),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Enabled reports whether a base URL is configured.
func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != ""
}

// APIError is a non-2xx response from docling-rag.
type APIError struct {
	Status  int
	Code    string
	Message string
	Body    string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("docling-rag: status=%d code=%s message=%s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("docling-rag: status=%d message=%s", e.Status, e.Message)
}

// CreateTenantRequest provisions a platform tenant.
type CreateTenantRequest struct {
	Name         string `json:"name"`
	Slug         string `json:"slug,omitempty"`
	ExternalRef  string `json:"external_ref,omitempty"`
	IssueAPIKey  bool   `json:"issue_api_key"`
}

// IssuedAPIKeyBrief is the plaintext key returned once on create.
type IssuedAPIKeyBrief struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// CreateTenantResponse is returned by POST /api/v2/tenants.
type CreateTenantResponse struct {
	TenantID       string             `json:"tenant_id"`
	TenantSlug     string             `json:"tenant_slug"`
	DefaultKBID    string             `json:"default_kb_id"`
	DefaultKBSlug  string             `json:"default_kb_slug"`
	APIKey         *IssuedAPIKeyBrief `json:"api_key"`
}

// KnowledgeBase is a KB summary.
type KnowledgeBase struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// CreateKBRequest creates a knowledge base under a tenant.
type CreateKBRequest struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	IndexMode string `json:"index_mode,omitempty"`
}

// IngestResult is returned by document upload.
type IngestResult struct {
	Outcome string `json:"outcome"`
	Name    string `json:"name"`
	Chunks  int    `json:"chunks"`
}

// DocumentSummary is one item from list documents.
type DocumentSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SearchRequest is POST /search body.
type SearchRequest struct {
	Query  string `json:"query"`
	Mode   string `json:"mode,omitempty"`
	TopK   int    `json:"top_k,omitempty"`
	Answer bool   `json:"answer,omitempty"`
	Extend bool   `json:"extend,omitempty"`
}

// SearchChunk is a retrieved passage.
type SearchChunk struct {
	ID       string         `json:"id"`
	DocID    string         `json:"doc_id"`
	Ordinal  int            `json:"ordinal"`
	Text     string         `json:"text"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ScoredHit is one search result.
type ScoredHit struct {
	Chunk SearchChunk `json:"chunk"`
	Score float64     `json:"score"`
}

// SearchResponse is returned by /search.
type SearchResponse struct {
	Query   string      `json:"query"`
	Mode    string      `json:"mode"`
	Answer  string      `json:"answer,omitempty"`
	Results []ScoredHit `json:"results"`
}

// StatsResponse is KB stats.
type StatsResponse struct {
	Documents int `json:"documents"`
	Chunks    int `json:"chunks"`
}

// EntitlementsLimits are quota fields nested under GET entitlements.
type EntitlementsLimits struct {
	MaxKBs          uint32 `json:"max_kbs"`
	MaxDocs         uint32 `json:"max_docs"`
	MaxIngestBytes  uint64 `json:"max_ingest_bytes"`
	MonthlySearches uint32 `json:"monthly_searches"`
	DailyAnswers    uint32 `json:"daily_answers"`
}

// EntitlementsResponse is GET /api/v2/tenants/{tenant}/entitlements.
type EntitlementsResponse struct {
	Version      uint64             `json:"version"`
	PlanCode     string             `json:"plan_code"`
	Entitlements EntitlementsLimits `json:"entitlements"`
}

// PutEntitlementsRequest is PUT /api/v2/tenants/{tenant}/entitlements.
type PutEntitlementsRequest struct {
	PlanCode        string `json:"plan_code"`
	MaxKBs          uint32 `json:"max_kbs"`
	MaxDocs         uint32 `json:"max_docs"`
	MaxIngestBytes  uint64 `json:"max_ingest_bytes"`
	MonthlySearches uint32 `json:"monthly_searches"`
	DailyAnswers    uint32 `json:"daily_answers"`
	Version         uint64 `json:"version"`
}

// IsMaxKBs reports whether err is a MAX_KBS entitlement denial.
func IsMaxKBs(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Status == http.StatusForbidden && strings.EqualFold(apiErr.Code, "MAX_KBS")
}

// CreateTenant provisions a tenant (admin key).
func (c *Client) CreateTenant(ctx context.Context, req CreateTenantRequest) (CreateTenantResponse, error) {
	var out CreateTenantResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v2/tenants", c.adminKey, req, &out)
	return out, err
}

// APIKeyGrant is a docling-rag API key scope grant.
type APIKeyGrant struct {
	KB      string   `json:"kb"`
	Actions []string `json:"actions"`
}

// CreateAPIKey issues a tenant API key (platform admin).
func (c *Client) CreateAPIKey(ctx context.Context, tenantSlug, name string, scopes []APIKeyGrant) (IssuedAPIKeyBrief, error) {
	body := map[string]any{
		"name":   name,
		"scopes": scopes,
	}
	var out IssuedAPIKeyBrief
	path := fmt.Sprintf("/api/v2/tenants/%s/api-keys", url.PathEscape(tenantSlug))
	err := c.doJSON(ctx, http.MethodPost, path, c.adminKey, body, &out)
	return out, err
}

// ListKnowledgeBases lists KBs for a tenant.
func (c *Client) ListKnowledgeBases(ctx context.Context, tenantSlug, apiKey string) ([]KnowledgeBase, error) {
	var out struct {
		KnowledgeBases []KnowledgeBase `json:"knowledge_bases"`
	}
	path := fmt.Sprintf("/api/v2/tenants/%s/knowledge-bases", url.PathEscape(tenantSlug))
	if err := c.doJSON(ctx, http.MethodGet, path, apiKey, nil, &out); err != nil {
		return nil, err
	}
	return out.KnowledgeBases, nil
}

// CreateKnowledgeBase creates a KB (tenant admin/ingest key with admin action).
func (c *Client) CreateKnowledgeBase(ctx context.Context, tenantSlug, apiKey string, req CreateKBRequest) (KnowledgeBase, error) {
	var out KnowledgeBase
	path := fmt.Sprintf("/api/v2/tenants/%s/knowledge-bases", url.PathEscape(tenantSlug))
	err := c.doJSON(ctx, http.MethodPost, path, apiKey, req, &out)
	return out, err
}

// GetEntitlements fetches the tenant entitlements snapshot (platform admin).
func (c *Client) GetEntitlements(ctx context.Context, tenantSlug string) (EntitlementsResponse, error) {
	var out EntitlementsResponse
	path := fmt.Sprintf("/api/v2/tenants/%s/entitlements", url.PathEscape(tenantSlug))
	err := c.doJSON(ctx, http.MethodGet, path, c.adminKey, nil, &out)
	return out, err
}

// PutEntitlements replaces tenant entitlements (platform admin, optimistic version).
func (c *Client) PutEntitlements(ctx context.Context, tenantSlug string, req PutEntitlementsRequest) (EntitlementsResponse, error) {
	var out EntitlementsResponse
	path := fmt.Sprintf("/api/v2/tenants/%s/entitlements", url.PathEscape(tenantSlug))
	err := c.doJSON(ctx, http.MethodPut, path, c.adminKey, req, &out)
	return out, err
}

// EnsureMinEntitlements raises trial quotas so DealSignal can create per-room KBs.
// CreateTenant always provisions a "default" KB, so trial max_kbs=1 blocks room KBs.
func (c *Client) EnsureMinEntitlements(ctx context.Context, tenantSlug string, min PutEntitlementsRequest) error {
	if c.adminKey == "" {
		return fmt.Errorf("docling-rag platform admin key is required to adjust entitlements")
	}
	cur, err := c.GetEntitlements(ctx, tenantSlug)
	if err != nil {
		return err
	}
	limits := cur.Entitlements
	need := limits.MaxKBs < min.MaxKBs ||
		limits.MaxDocs < min.MaxDocs ||
		limits.MaxIngestBytes < min.MaxIngestBytes ||
		limits.MonthlySearches < min.MonthlySearches ||
		limits.DailyAnswers < min.DailyAnswers
	if !need {
		return nil
	}
	plan := strings.TrimSpace(cur.PlanCode)
	if plan == "" {
		plan = min.PlanCode
	}
	if plan == "" {
		plan = "trial"
	}
	_, err = c.PutEntitlements(ctx, tenantSlug, PutEntitlementsRequest{
		PlanCode:        plan,
		MaxKBs:          maxU32(limits.MaxKBs, min.MaxKBs),
		MaxDocs:         maxU32(limits.MaxDocs, min.MaxDocs),
		MaxIngestBytes:  maxU64(limits.MaxIngestBytes, min.MaxIngestBytes),
		MonthlySearches: maxU32(limits.MonthlySearches, min.MonthlySearches),
		DailyAnswers:    maxU32(limits.DailyAnswers, min.DailyAnswers),
		Version:         cur.Version,
	})
	return err
}

func maxU32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func maxU64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

// EnsureKnowledgeBase returns an existing KB by slug or creates it.
// Prefer the platform admin key for this control-plane call; tenant keys are for data plane.
func (c *Client) EnsureKnowledgeBase(ctx context.Context, tenantSlug, apiKey, slug, name string) (KnowledgeBase, error) {
	controlKey := strings.TrimSpace(apiKey)
	if c.adminKey != "" {
		controlKey = c.adminKey
	}
	kbs, err := c.ListKnowledgeBases(ctx, tenantSlug, controlKey)
	if err != nil {
		return KnowledgeBase{}, err
	}
	for _, kb := range kbs {
		if kb.Slug == slug {
			return kb, nil
		}
	}
	created, cerr := c.CreateKnowledgeBase(ctx, tenantSlug, controlKey, CreateKBRequest{
		Slug:      slug,
		Name:      name,
		IndexMode: "inherit",
	})
	if cerr == nil || !IsMaxKBs(cerr) {
		return created, cerr
	}
	// Trial tenants start with max_kbs=1 (the auto "default" KB). Raise and retry once.
	if bumpErr := c.EnsureMinEntitlements(ctx, tenantSlug, DefaultPartnerEntitlements()); bumpErr != nil {
		return KnowledgeBase{}, fmt.Errorf("%w (also failed to raise max_kbs: %v)", cerr, bumpErr)
	}
	return c.CreateKnowledgeBase(ctx, tenantSlug, controlKey, CreateKBRequest{
		Slug:      slug,
		Name:      name,
		IndexMode: "inherit",
	})
}

// DefaultPartnerEntitlements are DealSignal-side minimums for a workspace tenant.
func DefaultPartnerEntitlements() PutEntitlementsRequest {
	return PutEntitlementsRequest{
		PlanCode:        "trial",
		MaxKBs:          100,
		MaxDocs:         5000,
		MaxIngestBytes:  5 << 30, // 5 GiB
		MonthlySearches: 100_000,
		DailyAnswers:    10_000,
	}
}

// IngestBytes uploads raw document bytes.
func (c *Client) IngestBytes(ctx context.Context, tenantSlug, kbSlug, apiKey, name, contentType string, body []byte) (IngestResult, error) {
	q := url.Values{}
	q.Set("name", name)
	path := fmt.Sprintf("/api/v2/tenants/%s/kbs/%s/documents?%s",
		url.PathEscape(tenantSlug), url.PathEscape(kbSlug), q.Encode())
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	var out IngestResult
	err := c.doRaw(ctx, http.MethodPost, path, apiKey, contentType, bytes.NewReader(body), &out)
	return out, err
}

// ListDocuments lists documents in a KB.
func (c *Client) ListDocuments(ctx context.Context, tenantSlug, kbSlug, apiKey string) ([]DocumentSummary, error) {
	var out struct {
		Documents []DocumentSummary `json:"documents"`
	}
	path := fmt.Sprintf("/api/v2/tenants/%s/kbs/%s/documents",
		url.PathEscape(tenantSlug), url.PathEscape(kbSlug))
	if err := c.doJSON(ctx, http.MethodGet, path, apiKey, nil, &out); err != nil {
		return nil, err
	}
	return out.Documents, nil
}

// DeleteDocument deletes a document by external id.
func (c *Client) DeleteDocument(ctx context.Context, tenantSlug, kbSlug, apiKey, documentID string) error {
	path := fmt.Sprintf("/api/v2/tenants/%s/kbs/%s/documents/%s",
		url.PathEscape(tenantSlug), url.PathEscape(kbSlug), url.PathEscape(documentID))
	return c.doJSON(ctx, http.MethodDelete, path, apiKey, nil, &struct{}{})
}

// Search runs retrieval (and optional grounded answer).
func (c *Client) Search(ctx context.Context, tenantSlug, kbSlug, apiKey string, req SearchRequest) (SearchResponse, error) {
	var out SearchResponse
	path := fmt.Sprintf("/api/v2/tenants/%s/kbs/%s/search",
		url.PathEscape(tenantSlug), url.PathEscape(kbSlug))
	err := c.doJSON(ctx, http.MethodPost, path, apiKey, req, &out)
	return out, err
}

// Stats returns KB document/chunk counts.
func (c *Client) Stats(ctx context.Context, tenantSlug, kbSlug, apiKey string) (StatsResponse, error) {
	var out StatsResponse
	path := fmt.Sprintf("/api/v2/tenants/%s/kbs/%s/stats",
		url.PathEscape(tenantSlug), url.PathEscape(kbSlug))
	err := c.doJSON(ctx, http.MethodGet, path, apiKey, nil, &out)
	return out, err
}

func (c *Client) doJSON(ctx context.Context, method, path, apiKey string, body any, out any) error {
	var rdr io.Reader
	contentType := ""
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
		contentType = "application/json"
	}
	return c.doRaw(ctx, method, path, apiKey, contentType, rdr, out)
}

func (c *Client) doRaw(ctx context.Context, method, path, apiKey, contentType string, body io.Reader, out any) error {
	if !c.Enabled() {
		return fmt.Errorf("docling-rag client is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var eb struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		_ = json.Unmarshal(raw, &eb)
		msg := eb.Error
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		if msg == "" {
			msg = resp.Status
		}
		return &APIError{Status: resp.StatusCode, Code: eb.Code, Message: msg, Body: string(raw)}
	}
	if out == nil || len(raw) == 0 || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode docling-rag response: %w", err)
	}
	return nil
}
