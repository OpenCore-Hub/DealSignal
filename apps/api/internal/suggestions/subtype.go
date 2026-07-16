package suggestions

// Signal subtypes stored in suggestions.subtype and signals.subtype.
const (
	SubtypeHot             = "hot"
	SubtypeRevisit         = "revisit"
	SubtypeDownload        = "download"
	SubtypeQuestion        = "question"
	SubtypeBounce          = "bounce"
	SubtypeExpired         = "expired"
	SubtypeAccessExhausted = "access_exhausted"
	SubtypeAccessRevoked   = "access_revoked"
	SubtypeBlockedAttempt  = "blocked_attempt"
	SubtypeAnomaly         = "anomaly"
	SubtypeForward         = "forward"
)

// IsRiskSubtype reports whether a subtype belongs to a risk_alert.
func IsRiskSubtype(subtype string) bool {
	switch subtype {
	case SubtypeBounce, SubtypeDownload, SubtypeExpired,
		SubtypeAccessExhausted, SubtypeAccessRevoked,
		SubtypeBlockedAttempt, SubtypeAnomaly, SubtypeForward:
		return true
	}
	return false
}
