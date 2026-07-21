package link

import "github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"

// sessionSecurityGatesUnsatisfied reports whether a reused LinkSession no
// longer proves the link's current NDA / email-verification requirements.
// Access POST and resolvePublicAccess MUST use the same expression.
func sessionSecurityGatesUnsatisfied(link db.Link, session LinkSession) bool {
	if link.RequireNda && !session.NDAAgreed {
		return true
	}
	// Require a verified email identity when email verification is enabled.
	// Empty email or EmailVerified=false means the session never completed verification
	// (covers legacy sessions issued before verification was turned on).
	if link.RequireEmailVerification && (session.Email == "" || !session.EmailVerified) {
		return true
	}
	return false
}

// sessionSecurityConfigChanged reports whether a reused session must be rejected
// because the link's security_version no longer matches. Legacy sessions with
// SecurityVersion=0 are rejected once the link has a positive security_version
// (config was versioned / bumped after the session was issued).
func sessionSecurityConfigChanged(link db.Link, session LinkSession) bool {
	if session.SecurityVersion == 0 {
		return link.SecurityVersion > 0
	}
	return link.SecurityVersion != session.SecurityVersion
}
