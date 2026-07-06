package mailer

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/net/html"
)

// TrackingToken is the signed payload embedded in open/click URLs.
type TrackingToken struct {
	LogID string `json:"log_id"`
	Type  string `json:"type"` // "open" or "click"
	URL   string `json:"url,omitempty"`
	Exp   int64  `json:"exp"`
}

// TrackingRedis is the minimal Redis interface needed for replay protection.
type TrackingRedis interface {
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
}

// TrackerOption configures an optional Tracker behavior.
type TrackerOption func(*Tracker)

// WithRedis enables single-use tracking token replay protection.
func WithRedis(r TrackingRedis) TrackerOption {
	return func(t *Tracker) {
		t.redisClient = r
	}
}

// Tracker generates signed tracking URLs and records open/click events.
type Tracker struct {
	queries     *db.Queries
	redisClient TrackingRedis
	baseURL     string
	secret      []byte
	ttl         time.Duration
}

// NewTracker creates a tracker. If secret is empty, tracking tokens are not
// signed and verification always fails; this prevents accidental exposure.
func NewTracker(queries *db.Queries, baseURL, secret string, ttl time.Duration, opts ...TrackerOption) *Tracker {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	t := &Tracker{
		queries: queries,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		secret:  []byte(secret),
		ttl:     ttl,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Enabled reports whether tracking URLs can be generated (a non-empty secret is set).
func (t *Tracker) Enabled() bool {
	return len(t.secret) > 0
}

// OpenPixelURL returns a signed URL for the open-tracking 1x1 pixel.
func (t *Tracker) OpenPixelURL(logID string) string {
	token := TrackingToken{LogID: logID, Type: "open", Exp: time.Now().Add(t.ttl).Unix()}
	return fmt.Sprintf("%s/api/v1/public/emails/track/open.png?token=%s", t.baseURL, t.sign(token))
}

// ClickURL returns a signed URL that records a click and redirects to targetURL.
func (t *Tracker) ClickURL(logID, targetURL string) string {
	token := TrackingToken{LogID: logID, Type: "click", URL: targetURL, Exp: time.Now().Add(t.ttl).Unix()}
	return fmt.Sprintf("%s/api/v1/public/emails/track/click?token=%s", t.baseURL, t.sign(token))
}

func (t *Tracker) sign(token TrackingToken) string {
	b, _ := json.Marshal(token)
	mac := hmac.New(sha256.New, t.secret)
	mac.Write(b)
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	data := base64.URLEncoding.EncodeToString(b)
	return data + "." + sig
}

func (t *Tracker) verify(s string) (TrackingToken, error) {
	var zero TrackingToken
	if len(t.secret) == 0 {
		return zero, errors.New("tracking secret not configured")
	}
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return zero, errors.New("invalid token format")
	}
	data, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return zero, err
	}
	var token TrackingToken
	if err := json.Unmarshal(data, &token); err != nil {
		return zero, err
	}
	if time.Now().Unix() > token.Exp {
		return zero, errors.New("tracking token expired")
	}
	mac := hmac.New(sha256.New, t.secret)
	mac.Write(data)
	expected := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(expected)) {
		return zero, errors.New("invalid tracking signature")
	}
	return token, nil
}

// RecordEvent writes an event to email_events.
func (t *Tracker) RecordEvent(ctx context.Context, token TrackingToken, ua, ip string) error {
	if t.queries == nil {
		return errors.New("tracking queries not configured")
	}
	if !t.Enabled() {
		return errors.New("tracking is not enabled")
	}
	id, err := uuid.Parse(token.LogID)
	if err != nil {
		return err
	}
	return t.queries.CreateEmailEvent(ctx, db.CreateEmailEventParams{
		EmailLogID: pgtype.UUID{Bytes: id, Valid: true},
		EventType:  token.Type,
		UserAgent:  pgtype.Text{String: ua, Valid: ua != ""},
		IpAddress:  pgtype.Text{String: ip, Valid: ip != ""},
		LinkUrl:    pgtype.Text{String: token.URL, Valid: token.URL != ""},
	})
}

// RegisterRoutes adds public tracking endpoints to the router group.
func (t *Tracker) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/emails/track/open.png", t.handleOpen)
	rg.GET("/emails/track/click", t.handleClick)
}

func (t *Tracker) handleOpen(c *gin.Context) {
	tokenStr := c.Query("token")
	token, err := t.verify(tokenStr)
	if err != nil || token.Type != "open" {
		c.Status(http.StatusNotFound)
		return
	}
	if !t.consumeToken(c.Request.Context(), tokenStr, token) {
		c.Data(http.StatusOK, "image/png", transparentPNG())
		return
	}
	_ = t.RecordEvent(c.Request.Context(), token, c.Request.UserAgent(), clientIP(c.Request))
	c.Data(http.StatusOK, "image/png", transparentPNG())
}

func (t *Tracker) handleClick(c *gin.Context) {
	tokenStr := c.Query("token")
	token, err := t.verify(tokenStr)
	if err != nil || token.Type != "click" || token.URL == "" {
		c.Status(http.StatusNotFound)
		return
	}
	if !t.consumeToken(c.Request.Context(), tokenStr, token) {
		c.Redirect(http.StatusFound, token.URL)
		return
	}
	_ = t.RecordEvent(c.Request.Context(), token, c.Request.UserAgent(), clientIP(c.Request))
	c.Redirect(http.StatusFound, token.URL)
}

// consumeToken marks a tracking token as consumed. It returns true the first
// time a token is seen and false on replay (or when replay protection is
// unavailable). The key TTL matches the token's remaining validity window.
func (t *Tracker) consumeToken(ctx context.Context, tokenStr string, token TrackingToken) bool {
	if t.redisClient == nil {
		return true
	}
	hash := sha256.Sum256([]byte(tokenStr))
	key := "email:track:consumed:" + hex.EncodeToString(hash[:])
	ttl := time.Until(time.Unix(token.Exp, 0))
	if ttl <= 0 {
		return false
	}
	ok, err := t.redisClient.SetNX(ctx, key, "1", ttl)
	if err != nil {
		// Redis failure should not drop legitimate events; treat as consumed.
		logger.L().LogAttrs(ctx, slog.LevelWarn,
			"tracking token consume failed, allowing event",
			logger.Attr("error", err.Error()),
			logger.Attr("email_log_id", token.LogID),
			logger.Attr("event_type", token.Type),
		)
		return true
	}
	return ok
}

// clientIP returns the client IP, preferring X-Forwarded-For when present.
func clientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		if idx := strings.Index(ip, ","); idx > 0 {
			ip = strings.TrimSpace(ip[:idx])
		}
		return ip
	}
	if ip = r.Header.Get("X-Real-Ip"); ip != "" {
		return ip
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// transparentPNG is a 1x1 transparent PNG.
func transparentPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
		0x0d, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0x60, 0x60, 0x60, 0x60,
		0x00, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00,
		0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

// rewriteClickLinks parses htmlSrc and rewrites every <a href="..."> link to
// route through the tracking endpoint. Non-HTTP(S) links and anchors are left
// untouched.
func rewriteClickLinks(htmlSrc, logID string, tracker *Tracker) string {
	if tracker == nil || !tracker.Enabled() || logID == "" {
		return htmlSrc
	}
	doc, err := html.Parse(strings.NewReader(htmlSrc))
	if err != nil {
		return htmlSrc
	}
	var rewrite func(*html.Node)
	rewrite = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for i, attr := range n.Attr {
				if attr.Key == "href" {
					u := strings.TrimSpace(attr.Val)
					if shouldTrackURL(u) {
						n.Attr[i].Val = tracker.ClickURL(logID, u)
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			rewrite(c)
		}
	}
	rewrite(doc)
	var buf strings.Builder
	if err := html.Render(&buf, doc); err != nil {
		return htmlSrc
	}
	return buf.String()
}

func shouldTrackURL(u string) bool {
	u = strings.ToLower(u)
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

// injectOpenPixel appends a 1x1 tracking pixel to the HTML body.
func injectOpenPixel(htmlSrc, logID string, tracker *Tracker) string {
	if tracker == nil || !tracker.Enabled() || logID == "" {
		return htmlSrc
	}
	pixel := fmt.Sprintf(`<img src="%s" width="1" height="1" alt="" style="display:block;" />`, tracker.OpenPixelURL(logID))
	if strings.Contains(strings.ToLower(htmlSrc), "</body>") {
		return strings.Replace(htmlSrc, "</body>", pixel+"</body>", 1)
	}
	return htmlSrc + pixel
}
