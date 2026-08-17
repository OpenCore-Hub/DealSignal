// Package turnstile verifies Cloudflare Turnstile tokens server-side.
package turnstile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
)

const (
	// DefaultVerifyURL is Cloudflare's siteverify endpoint.
	DefaultVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	// ActionRegister is the widget action bound to POST /api/auth/register.
	ActionRegister = "register"
	maxTokenBytes  = 2048
	maxBodyBytes   = 1 << 20
)

var (
	ErrMissing     = errors.New("captcha token is required")
	ErrFailed      = errors.New("captcha verification failed")
	ErrUnavailable = errors.New("captcha verification unavailable")
)

// Verifier checks a Turnstile token. Disabled implementations are a no-op.
type Verifier interface {
	Enabled() bool
	SiteKey() string
	Verify(ctx context.Context, token, remoteIP string) error
}

// Client verifies tokens against siteverify.
type Client struct {
	secret          string
	siteKey         string
	action          string
	expectedHost    string
	verifyURL       string
	httpClient      *http.Client
	skipConstraints bool
}

// Option configures Client.
type Option func(*Client)

// WithHTTPClient overrides the HTTP client (tests).
func WithHTTPClient(c *http.Client) Option {
	return func(v *Client) { v.httpClient = c }
}

// WithVerifyURL overrides the siteverify URL (tests).
func WithVerifyURL(raw string) Option {
	return func(v *Client) { v.verifyURL = raw }
}

// WithAction expects siteverify to echo this widget action when present.
func WithAction(action string) Option {
	return func(v *Client) { v.action = action }
}

// WithExpectedHost rejects tokens issued for a different hostname.
func WithExpectedHost(host string) Option {
	return func(v *Client) { v.expectedHost = normalizeHost(host) }
}

// New returns a verifier. Empty secret disables checks (local / e2e).
func New(secret, siteKey string, opts ...Option) Verifier {
	secret = strings.TrimSpace(secret)
	siteKey = strings.TrimSpace(siteKey)
	if secret == "" {
		return disabled{}
	}
	v := &Client{
		secret:          secret,
		siteKey:         siteKey,
		action:          ActionRegister,
		verifyURL:       DefaultVerifyURL,
		skipConstraints: isDummySecret(secret),
		httpClient:      nil,
	}
	for _, opt := range opts {
		opt(v)
	}
	if v.httpClient == nil {
		v.httpClient = &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	if v.verifyURL == "" {
		v.verifyURL = DefaultVerifyURL
	}
	return v
}

// WithTimeout sets the siteverify HTTP timeout.
func WithTimeout(d time.Duration) Option {
	return func(v *Client) {
		if d <= 0 {
			return
		}
		if v.httpClient == nil {
			v.httpClient = &http.Client{
				Timeout: d,
				CheckRedirect: func(*http.Request, []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			return
		}
		cloned := *v.httpClient
		cloned.Timeout = d
		v.httpClient = &cloned
	}
}

type disabled struct{}

func (disabled) Enabled() bool                                { return false }
func (disabled) SiteKey() string                              { return "" }
func (disabled) Verify(context.Context, string, string) error { return nil }

func (v *Client) Enabled() bool { return v != nil && v.secret != "" }
func (v *Client) SiteKey() string {
	if v == nil {
		return ""
	}
	return v.siteKey
}

type siteverifyResponse struct {
	Success    bool     `json:"success"`
	Hostname   string   `json:"hostname"`
	Action     string   `json:"action"`
	ErrorCodes []string `json:"error-codes"`
}

func (v *Client) Verify(ctx context.Context, token, remoteIP string) error {
	if v == nil || v.secret == "" {
		return nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrMissing
	}
	if len(token) > maxTokenBytes {
		return ErrFailed
	}

	form := url.Values{}
	form.Set("secret", v.secret)
	form.Set("response", token)
	if ip := strings.TrimSpace(remoteIP); ip != "" {
		form.Set("remoteip", ip)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return ErrUnavailable
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "DealSignal-API")

	res, err := v.httpClient.Do(req)
	if err != nil {
		logger.ErrorCtx(ctx, "turnstile: siteverify transport failed", err)
		return ErrUnavailable
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBodyBytes))
	if err != nil {
		logger.ErrorCtx(ctx, "turnstile: siteverify read failed", err)
		return ErrUnavailable
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		logger.ErrorCtx(ctx, "turnstile: siteverify http status", fmt.Errorf("status %d", res.StatusCode),
			slog.Int("status", res.StatusCode),
		)
		return ErrUnavailable
	}

	var parsed siteverifyResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		logger.ErrorCtx(ctx, "turnstile: siteverify json", err)
		return ErrUnavailable
	}
	if !parsed.Success {
		logger.InfoCtx(ctx, "turnstile: verification rejected",
			slog.String("error_codes", strings.Join(parsed.ErrorCodes, ",")),
		)
		return ErrFailed
	}
	if v.skipConstraints {
		return nil
	}
	if v.action != "" && parsed.Action != "" && !strings.EqualFold(parsed.Action, v.action) {
		logger.InfoCtx(ctx, "turnstile: action mismatch", slog.String("action", parsed.Action))
		return ErrFailed
	}
	if host := v.expectedHost; host != "" && !isLocalHost(host) && parsed.Hostname != "" {
		if !strings.EqualFold(normalizeHost(parsed.Hostname), host) {
			logger.InfoCtx(ctx, "turnstile: hostname mismatch", slog.String("hostname", parsed.Hostname))
			return ErrFailed
		}
	}
	return nil
}

func isDummySecret(secret string) bool {
	return strings.HasPrefix(secret, "1x000000") ||
		strings.HasPrefix(secret, "2x000000") ||
		strings.HasPrefix(secret, "3x000000")
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	return host
}

func isLocalHost(host string) bool {
	switch normalizeHost(host) {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

// HostFromURL returns the hostname of a frontend/base URL.
func HostFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return normalizeHost(u.Hostname())
}
