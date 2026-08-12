package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOutboundWebhookURL(t *testing.T) {
	assert.NoError(t, ValidateOutboundWebhookURL("https://hooks.zapier.com/hooks/catch/123/abc"))
	assert.NoError(t, ValidateOutboundWebhookURL("http://localhost:3000/hook"))
	assert.NoError(t, ValidateOutboundWebhookURL("http://127.0.0.1:9999/hook"))
	assert.NoError(t, ValidateOutboundWebhookURL("https://127.0.0.1:8443/hook"))
	assert.Error(t, ValidateOutboundWebhookURL(""))
	assert.Error(t, ValidateOutboundWebhookURL("http://example.com/hook"))
	assert.Error(t, ValidateOutboundWebhookURL("ftp://example.com/hook"))
	assert.Error(t, ValidateOutboundWebhookURL("https://user:pass@example.com/hook"))
	assert.ErrorIs(t, ValidateOutboundWebhookURL("https://10.0.0.8/hook"), ErrInvalidWebhookURL)
	assert.ErrorIs(t, ValidateOutboundWebhookURL("https://192.168.1.1/hook"), ErrInvalidWebhookURL)
	assert.ErrorIs(t, ValidateOutboundWebhookURL("https://169.254.169.254/latest/meta-data"), ErrInvalidWebhookURL)
	assert.ErrorIs(t, ValidateOutboundWebhookURL("https://metadata.google.internal/computeMetadata/v1/"), ErrInvalidWebhookURL)
	assert.True(t, IsOutboundWebhookURLError(ValidateOutboundWebhookURL("http://example.com/hook")))
	assert.False(t, IsOutboundWebhookURLError(nil))
}

func TestNormalizeOutboundEventTypes(t *testing.T) {
	assert.Equal(t, []string{"key_page", "repeat_key_page"}, NormalizeOutboundEventTypes(nil))
	assert.Equal(t, []string{"key_page", "first_open"}, NormalizeOutboundEventTypes([]string{"key_page", "bogus", "first_open", "key_page"}))
}

func TestSignOutboundWebhook(t *testing.T) {
	body := []byte(`{"event":"key_page"}`)
	got := SignOutboundWebhook("secret", body)
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(body)
	assert.Equal(t, hex.EncodeToString(mac.Sum(nil)), got)
}

func TestEnqueueOutboundWebhookSkipsWhenDisabled(t *testing.T) {
	ws := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	var enqueued int
	engine := NewRuleEngine(nil, func(ctx context.Context, workspaceID, userID, channel, subject, body string, opts ...EnqueueOption) error {
		enqueued++
		return nil
	})
	// queries nil → no-op
	engine.enqueueOutboundWebhook(context.Background(), Event{
		WorkspaceID: ws.String(),
		EventType:   "key_page",
	})
	assert.Equal(t, 0, enqueued)
}

type webhookQuerier struct {
	mockNotificationQuerier
	row db.WorkspaceOutboundWebhook
	err error
}

func (q *webhookQuerier) GetWorkspaceOutboundWebhook(_ context.Context, _ pgtype.UUID) (db.WorkspaceOutboundWebhook, error) {
	if q.err != nil {
		return db.WorkspaceOutboundWebhook{}, q.err
	}
	return q.row, nil
}

func TestSendWebhookPostsSignedJSON(t *testing.T) {
	var gotSig, gotEvent, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotSig = r.Header.Get("X-DealSignal-Signature")
		gotEvent = r.Header.Get("X-DealSignal-Event")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	ws := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	secret := "0123456789abcdef0123456789abcdef"
	q := &webhookQuerier{
		row: db.WorkspaceOutboundWebhook{
			WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
			Url:         srv.URL,
			Secret:      secret,
			Enabled:     true,
			EventTypes:  []string{"key_page"},
		},
	}
	svc := NewService(nil, q, nil, &config.Config{})
	body := `{"event":"key_page","workspace_id":"` + ws.String() + `"}`
	err := svc.sendWebhook(context.Background(), db.Notification{
		WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		Channel:     "webhook",
		Subject:     "key_page",
		Body:        body,
	})
	require.NoError(t, err)
	assert.Equal(t, "key_page", gotEvent)
	assert.Equal(t, "sha256="+SignOutboundWebhook(secret, []byte(body)), gotSig)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(gotBody), &parsed))
	assert.Equal(t, "key_page", parsed["event"])
}

func TestSendWebhookDisabled(t *testing.T) {
	ws := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	q := &webhookQuerier{
		row: db.WorkspaceOutboundWebhook{
			WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
			Url:         "https://example.com/hook",
			Secret:      "0123456789abcdef0123456789abcdef",
			Enabled:     false,
		},
	}
	svc := NewService(nil, q, nil, &config.Config{})
	err := svc.sendWebhook(context.Background(), db.Notification{
		WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		Body:        `{}`,
		Subject:     "key_page",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestSendWebhookMissingConfig(t *testing.T) {
	ws := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	q := &webhookQuerier{err: pgx.ErrNoRows}
	svc := NewService(nil, q, nil, &config.Config{})
	err := svc.sendWebhook(context.Background(), db.Notification{
		WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		Body:        `{}`,
	})
	require.Error(t, err)
}
