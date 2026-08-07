package link

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAccessCodeNeedsRemediation(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		status    string
		createdAt time.Time
		want      bool
	}{
		{name: "failed always", status: "failed", createdAt: now, want: true},
		{name: "sent never", status: "sent", createdAt: now.Add(-time.Hour), want: false},
		{name: "fresh pending waits", status: "pending", createdAt: now.Add(-time.Minute), want: false},
		{name: "stale pending remediates", status: "pending", createdAt: now.Add(-3 * time.Minute), want: true},
		{name: "zero created pending", status: "pending", createdAt: time.Time{}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := accessCodeNeedsRemediation(tt.status, tt.createdAt, now)
			if got != tt.want {
				t.Fatalf("accessCodeNeedsRemediation(%q)=%v want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestWrapAccessCodeSendErr(t *testing.T) {
	root := fmt.Errorf("smtp close data: 550 \"Queueing failed\"")
	wrapped := wrapAccessCodeSendErr(root)
	if !errors.Is(wrapped, ErrAccessCodeSendFailed) {
		t.Fatalf("expected ErrAccessCodeSendFailed, got %v", wrapped)
	}
}

func TestAccessCodeSendClientDetail(t *testing.T) {
	if got := accessCodeSendClientDetail(wrapAccessCodeSendErr(errors.New(`550 "Queueing failed"`))); got != "verification code could not be sent" {
		t.Fatalf("detail=%q", got)
	}
	if got := accessCodeSendClientDetail(ErrEmailCodeRateLimited); got != "rate limited" {
		t.Fatalf("detail=%q", got)
	}
	if got := accessCodeSendClientDetail(errors.New("db exploded")); got != "send failed" {
		t.Fatalf("detail=%q", got)
	}
}

func TestMapAccessCodeContactRowsOmitsSendError(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	rows := []db.ListLinkAccessCodeContactsByLinkRow{
		{
			ContactEmail:   "bad@example.com",
			CodeSendStatus: "failed",
			CodeSendError:  `smtp close data: 550 "Queueing failed"`,
			CreatedAt:      pgtype.Timestamptz{Time: now.Add(-3 * time.Minute), Valid: true},
		},
	}
	contacts := mapAccessCodeContactRows(rows, now)
	if len(contacts) != 1 {
		t.Fatalf("len=%d", len(contacts))
	}
	if contacts[0].SendStatus != "failed" || !contacts[0].CanResend {
		t.Fatalf("contact=%+v", contacts[0])
	}
	payload, err := json.Marshal(contacts[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "send_error") || strings.Contains(string(payload), "Queueing failed") {
		t.Fatalf("payload leaked send error: %s", payload)
	}
}
