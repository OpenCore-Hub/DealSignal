package knowledge

// Judgment kinds for answered turns (ceiling Phase K / L2).
// Refusal paths keep using RefusalInfo — judgment is only for result_status=answered.
const (
	JudgmentKindGrounded = "grounded" // ≥1 citation-grounded claim, no unresolved gaps
	JudgmentKindPartial  = "partial"  // weak-only, unresolved gaps, or mixed soft bind

	JudgmentReasonWeakOnly      = "weak_only"
	JudgmentReasonHasUnresolved = "has_unresolved"
	JudgmentReasonMixed         = "mixed" // weak binds + unresolved, no grounded cite
)

// JudgmentInfo is the auditable L2 stamp quality envelope (no new DB column).
type JudgmentInfo struct {
	Kind            string `json:"kind"`                      // grounded | partial
	Reason          string `json:"reason,omitempty"`          // weak_only | has_unresolved | mixed
	GroundedClaims  int    `json:"groundedClaims,omitempty"`  // confidence=grounded
	WeakClaims      int    `json:"weakClaims,omitempty"`      // confidence=weak
	UnresolvedCount int    `json:"unresolvedCount,omitempty"` // unbound factual sentences
}

// classifyJudgment grades an answered bound envelope.
// Returns nil for non-answered turns (refusal/error/no_hits use RefusalInfo).
func classifyJudgment(status string, bound BoundAnswer) *JudgmentInfo {
	if status != "answered" {
		return nil
	}
	grounded, weak := 0, 0
	for _, c := range bound.Claims {
		switch c.Confidence {
		case claimConfidenceGrounded:
			grounded++
		case claimConfidenceWeak:
			weak++
		}
	}
	unresolved := len(bound.Unresolved)
	info := &JudgmentInfo{
		GroundedClaims:  grounded,
		WeakClaims:      weak,
		UnresolvedCount: unresolved,
	}
	switch {
	case grounded > 0 && unresolved == 0:
		info.Kind = JudgmentKindGrounded
	case grounded > 0 && unresolved > 0:
		info.Kind = JudgmentKindPartial
		info.Reason = JudgmentReasonHasUnresolved
	case weak > 0 && unresolved > 0:
		info.Kind = JudgmentKindPartial
		info.Reason = JudgmentReasonMixed
	case weak > 0:
		info.Kind = JudgmentKindPartial
		info.Reason = JudgmentReasonWeakOnly
	case unresolved > 0:
		info.Kind = JudgmentKindPartial
		info.Reason = JudgmentReasonHasUnresolved
	default:
		// Connective / narrative-only answers with no factual gaps.
		info.Kind = JudgmentKindGrounded
	}
	return info
}
