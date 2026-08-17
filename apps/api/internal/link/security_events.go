package link

import "errors"

// InviteTokenError marks invite-token validation failures audited once at the
// HTTP handler layer via recordSecurityEventFromAccessError.
type InviteTokenError struct {
	Reason string
}

func (e *InviteTokenError) Error() string {
	if e != nil && e.Reason != "" {
		return "invite token failed: " + e.Reason
	}
	return "invite token failed"
}

// securityEventFromError maps an access error to a security event type and reason.
// The third return value indicates whether the event represents a security gate
// failure that should contribute to abnormal-access-pattern detection.
func securityEventFromError(err error) (eventType, reason string, gateFailure bool) {
	var inviteFail *InviteTokenError
	switch {
	case errors.As(err, &inviteFail):
		detail := inviteFail.Reason
		if detail == "" {
			detail = "invalid_or_unknown_token"
		}
		return "invite_token_failed", detail, false
	case errors.Is(err, ErrLinkExpired):
		return "expired_link_accessed", "", false
	case errors.Is(err, ErrLinkRevoked), errors.Is(err, ErrLinkDisabled):
		return "revoked_link_accessed", "", false
	case errors.Is(err, ErrLinkArchived):
		return "revoked_link_accessed", "archived", false
	case errors.Is(err, ErrLinkMaxAccessReached):
		return "max_access_reached", "", false
	case errors.Is(err, ErrInvalidEmailCode):
		return "security_gate_failed", "invalid_email_code", true
	case errors.Is(err, ErrRequiresEmail), errors.Is(err, ErrRequiresEmailCode),
		errors.Is(err, ErrRequiresNDA), errors.Is(err, ErrRequiresPassword):
		// Empty-form gate prompts ("fill this in") are not audit denials.
		return "", "", false
	case errors.Is(err, ErrInvalidPassword):
		return "security_gate_failed", "password", true
	case errors.Is(err, ErrBlockedEmail):
		return "blocked_email", "", true
	case errors.Is(err, ErrNotAllowedEmail):
		return "not_in_allow_list", "", true
	case errors.Is(err, ErrDeliveryEmailMismatch):
		return "security_gate_failed", "delivery_email_mismatch", true
	case errors.Is(err, ErrInviteExpired):
		return "invite_token_expired", "", false
	case errors.Is(err, ErrInviteRevoked):
		return "invite_token_revoked", "", false
	case errors.Is(err, ErrInviteAlreadyUsed):
		return "invite_token_already_used", "", false
	default:
		return "", "", false
	}
}

// isCredentialGateAccessError reports errors that should return structured gate
// payloads from Handler.Access (security flags + i18n code).
func isCredentialGateAccessError(err error) bool {
	var inviteFail *InviteTokenError
	return errors.Is(err, ErrRequiresEmail) ||
		errors.Is(err, ErrRequiresEmailCode) ||
		errors.Is(err, ErrInvalidEmailCode) ||
		errors.Is(err, ErrRequiresNDA) ||
		errors.Is(err, ErrInvalidSignerName) ||
		errors.Is(err, ErrRequiresPassword) ||
		errors.Is(err, ErrInvalidPassword) ||
		errors.Is(err, ErrBlockedEmail) ||
		errors.Is(err, ErrNotAllowedEmail) ||
		errors.Is(err, ErrDeliveryEmailMismatch) ||
		errors.Is(err, ErrInviteExpired) ||
		errors.Is(err, ErrInviteRevoked) ||
		errors.Is(err, ErrInviteAlreadyUsed) ||
		errors.As(err, &inviteFail)
}
