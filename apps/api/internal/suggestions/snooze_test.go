package suggestions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnoozeRejectsInvalidDuration(t *testing.T) {
	svc := NewService(nil, nil, nil)
	_, err := svc.Snooze(context.Background(), "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", 12)
	require.ErrorIs(t, err, ErrInvalidSnoozeDuration)
}

func TestAllowedSnoozeHours(t *testing.T) {
	for _, h := range []int{24, 72, 168} {
		_, ok := AllowedSnoozeHours[h]
		assert.True(t, ok, "hours=%d", h)
	}
	_, ok := AllowedSnoozeHours[48]
	assert.False(t, ok)
}

func TestMetadataStringTurnID(t *testing.T) {
	assert.Equal(t, "turn-1", metadataString([]byte(`{"turn_id":"turn-1","link_id":"l1"}`), "turn_id"))
	assert.Equal(t, "turn-1", metadataString([]byte(`{"turn_id":"  turn-1  "}`), "turn_id"))
	assert.Equal(t, "", metadataString([]byte(`{"link_id":"l1"}`), "turn_id"))
	assert.Equal(t, "", metadataString(nil, "turn_id"))
	assert.Equal(t, "", metadataString([]byte(`not-json`), "turn_id"))
}
