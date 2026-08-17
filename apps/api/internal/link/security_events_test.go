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
		{
			name: "email required prompt",
			err:  ErrRequiresEmail,
		},
		{
			name: "email code required prompt",
			err:  ErrRequiresEmailCode,
		},
		{
			name: "nda required prompt",
			err:  ErrRequiresNDA,
		},
		{
			name: "password required prompt",
			err:  ErrRequiresPassword,
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

func TestRecordSecurityEventFromAccessError_SkipsGatePrompts(t *testing.T) {
	prompts := []error{ErrRequiresEmail, ErrRequiresEmailCode, ErrRequiresNDA, ErrRequiresPassword}
	for _, err := range prompts {
		sink := &askHostSecuritySink{}
		h := &Handler{analytics: nil}
		h.SetSecuritySink(sink)
		h.recordSecurityEventFromAccessError(
			context.Background(),
			db.Link{ID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}},
			err,
			"visitor-1",
			"guest@example.com",
			"203.0.113.1",
			"test-agent",
		)
		if len(sink.events) != 0 {
			t.Fatalf("%v must not write an audit event, got %+v", err, sink.events)
		}
	}
}

func TestAuditPublicEmailCheck_RecordsAllowlistDenialsOnly(t *testing.T) {
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{9}, Valid: true}}
	cases := []struct {
		name     string
		err      error
		wantType string
	}{
		{name: "not allowed", err: ErrNotAllowedEmail, wantType: "not_in_allow_list"},
		{name: "blocked", err: ErrBlockedEmail, wantType: "blocked_email"},
		{name: "expired is not a check-email denial", err: ErrLinkExpired},
		{name: "invalid email prompt", err: ErrRequiresEmail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sink := &askHostSecuritySink{}
			h := &Handler{analytics: nil}
			h.SetSecuritySink(sink)
			h.auditPublicEmailCheck(context.Background(), nil, link, "wrong@example.com", tc.err)
			if tc.wantType == "" {
				if len(sink.events) != 0 {
					t.Fatalf("expected no event, got %+v", sink.events)
				}
				return
			}
			if len(sink.events) != 1 {
				t.Fatalf("expected 1 event, got %+v", sink.events)
			}
			if sink.events[0].eventType != tc.wantType {
				t.Fatalf("eventType=%q want %q", sink.events[0].eventType, tc.wantType)
			}
			if sink.events[0].email != "wrong@example.com" {
				t.Fatalf("email=%q", sink.events[0].email)
			}
		})
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
