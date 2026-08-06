package link

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestSecurityEventFromError_AccessRuleFailures(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantType    string
		wantReason  string
		wantGate    bool
	}{
		{
			name:       "blocked email",
			err:        ErrBlockedEmail,
			wantType:   "blocked_email",
			wantGate:   true,
		},
		{
			name:       "not in allow list",
			err:        ErrNotAllowedEmail,
			wantType:   "not_in_allow_list",
			wantGate:   true,
		},
		{
			name:       "invalid password",
			err:        ErrInvalidPassword,
			wantType:   "security_gate_failed",
			wantReason: "password",
			wantGate:   true,
		},
		{
			name:       "delivery email mismatch",
			err:        ErrDeliveryEmailMismatch,
			wantType:   "security_gate_failed",
			wantReason: "delivery_email_mismatch",
			wantGate:   true,
		},
		{
			name:       "invite already used",
			err:        ErrInviteAlreadyUsed,
			wantType:   "invite_token_already_used",
		},
		{
			name:       "invite token invalid",
			err:        &InviteTokenError{Reason: "invalid_or_unknown_token"},
			wantType:   "invite_token_failed",
			wantReason: "invalid_or_unknown_token",
		},
		{
			name:       "invite wrong link",
			err:        &InviteTokenError{Reason: "invitation does not belong to link"},
			wantType:   "invite_token_failed",
			wantReason: "invitation does not belong to link",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eventType, reason, gateFailure := securityEventFromError(tc.err)
			if eventType != tc.wantType || reason != tc.wantReason || gateFailure != tc.wantGate {
				t.Fatalf("securityEventFromError(%v) = (%q, %q, %v), want (%q, %q, %v)",
					tc.err, eventType, reason, gateFailure, tc.wantType, tc.wantReason, tc.wantGate)
			}
		})
	}
}

func TestSecurityEventFromError_Unmapped(t *testing.T) {
	eventType, reason, gateFailure := securityEventFromError(fmt.Errorf("db down"))
	if eventType != "" || reason != "" || gateFailure {
		t.Fatalf("expected empty mapping for unmapped error, got (%q, %q, %v)", eventType, reason, gateFailure)
	}
}

func TestIsCredentialGateAccessError(t *testing.T) {
	if !isCredentialGateAccessError(ErrBlockedEmail) {
		t.Fatal("expected blocked email to be credential gate error")
	}
	if !isCredentialGateAccessError(&InviteTokenError{Reason: "invalid_or_unknown_token"}) {
		t.Fatal("expected invite token failure to be credential gate error")
	}
	if isCredentialGateAccessError(fmt.Errorf("db down")) {
		t.Fatal("expected unmapped error to not be credential gate error")
	}
}

func TestRecordSecurityEventFromAccessError_SingleBlockedEmailEvent(t *testing.T) {
	sink := &askHostSecuritySink{}
	h := &Handler{analytics: nil}
	h.SetSecuritySink(sink)

	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}}
	h.recordSecurityEventFromAccessError(
		context.Background(),
		link,
		ErrBlockedEmail,
		"visitor-1",
		"blocked@example.com",
		"203.0.113.1",
		"test-agent",
	)

	if len(sink.events) != 1 {
		t.Fatalf("expected exactly one security event, got %+v", sink.events)
	}
	if sink.events[0].eventType != "blocked_email" {
		t.Fatalf("expected blocked_email event, got %+v", sink.events[0])
	}
}

func TestInviteTokenError_Wraps(t *testing.T) {
	err := fmt.Errorf("resolve invite: %w", &InviteTokenError{Reason: "invalid_or_unknown_token"})
	var inviteFail *InviteTokenError
	if !errors.As(err, &inviteFail) {
		t.Fatal("expected InviteTokenError in chain")
	}
	eventType, reason, _ := securityEventFromError(err)
	if eventType != "invite_token_failed" || reason != "invalid_or_unknown_token" {
		t.Fatalf("got (%q, %q)", eventType, reason)
	}
}
