package integration

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveOutboundWebhookRejectsInsecureURL(t *testing.T) {
	s := NewService(nil, &config.Config{})
	_, err := s.SaveOutboundWebhook(context.Background(), "11111111-1111-1111-1111-111111111111", SaveOutboundWebhookRequest{
		URL:     "http://example.com/hooks/catch/1",
		Enabled: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "https")
}

func TestSecretHint(t *testing.T) {
	assert.Equal(t, "••••", secretHint("ab"))
	assert.Equal(t, "••••cdef", secretHint("0123456789abcdef"))
}
