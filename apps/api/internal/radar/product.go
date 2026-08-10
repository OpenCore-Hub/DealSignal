// Package radar compiles workspace actions and signals into Deal Radar work items.
package radar

import (
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
)

// Product is a productized radar card — not a raw event type.
type Product string

const (
	ProductBuyingWindow  Product = "buying_window"
	ProductDiligenceGate Product = "diligence_gate"
	ProductCommitmentAsk Product = "commitment_ask"
	ProductLeakWatch     Product = "leak_watch"
	ProductAccessDecay   Product = "access_decay"
	ProductAbuseGuard    Product = "abuse_guard"
)

// Outcome is a closed-loop completion reason persisted on action_items.
type Outcome string

const (
	OutcomeActed         Outcome = "acted"
	OutcomeFalsePositive Outcome = "false_positive"
	OutcomeRenewed       Outcome = "renewed"
	OutcomeApproved      Outcome = "approved"
	OutcomeReplied       Outcome = "replied"
	OutcomeOther         Outcome = "other"
)

// ValidOutcome reports whether s is an allowed completion outcome.
func ValidOutcome(s string) bool {
	switch Outcome(s) {
	case OutcomeActed, OutcomeFalsePositive, OutcomeRenewed,
		OutcomeApproved, OutcomeReplied, OutcomeOther:
		return true
	default:
		return false
	}
}

// ParseCircle maps query/lens values onto heat circles (default founder).
func ParseCircle(raw string) heat.Circle {
	switch heat.Circle(raw) {
	case heat.CircleFounder, heat.CircleInvestor, heat.CircleSales:
		return heat.Circle(raw)
	default:
		return heat.CircleDefault
	}
}

// Verb is the single primary CTA for a work item.
type Verb string

const (
	VerbApprove Verb = "approve"
	VerbReply   Verb = "reply"
	VerbEmail   Verb = "email"
	VerbRenew   Verb = "renew"
	VerbReview  Verb = "review"
	VerbOpen    Verb = "open"
)

// Priority mirrors signal/action priority bands.
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// AllProducts is the stable filter order for the radar UI.
var AllProducts = []Product{
	ProductBuyingWindow,
	ProductDiligenceGate,
	ProductCommitmentAsk,
	ProductLeakWatch,
	ProductAccessDecay,
	ProductAbuseGuard,
}

func (p Product) Valid() bool {
	switch p {
	case ProductBuyingWindow, ProductDiligenceGate, ProductCommitmentAsk,
		ProductLeakWatch, ProductAccessDecay, ProductAbuseGuard:
		return true
	default:
		return false
	}
}

// productRank orders Next Up / strand sorting (lower = more urgent band).
// productRankForCircle adjusts product urgency by role lens.
func productRankForCircle(circle heat.Circle, p Product) int {
	switch circle {
	case heat.CircleInvestor:
		switch p {
		case ProductLeakWatch, ProductBuyingWindow:
			return 0
		case ProductDiligenceGate, ProductCommitmentAsk:
			return 1
		case ProductAccessDecay:
			return 2
		case ProductAbuseGuard:
			return 3
		default:
			return 9
		}
	case heat.CircleSales:
		switch p {
		case ProductBuyingWindow:
			return 0
		case ProductDiligenceGate, ProductAccessDecay:
			return 1
		case ProductCommitmentAsk, ProductLeakWatch:
			return 2
		case ProductAbuseGuard:
			return 3
		default:
			return 9
		}
	default: // founder
		switch p {
		case ProductDiligenceGate, ProductLeakWatch:
			return 0
		case ProductCommitmentAsk:
			return 1
		case ProductBuyingWindow:
			return 2
		case ProductAccessDecay, ProductAbuseGuard:
			return 3
		default:
			return 9
		}
	}
}

func priorityRank(p Priority) int {
	switch p {
	case PriorityHigh:
		return 3
	case PriorityMedium:
		return 2
	default:
		return 1
	}
}

// coalesceWindow is the max age span for merging same actor+deal+product cards.
const coalesceWindow = 24 * time.Hour

// defaultSLADuration returns the relative SLA for products other than Commitment Ask.
func defaultSLADuration(p Product) time.Duration {
	switch p {
	case ProductDiligenceGate:
		return 2 * time.Hour
	case ProductLeakWatch:
		return time.Hour
	case ProductBuyingWindow:
		return 4 * time.Hour
	case ProductAbuseGuard:
		return 4 * time.Hour
	case ProductAccessDecay:
		return 24 * time.Hour
	case ProductCommitmentAsk:
		// Fallback only — prefer slaDueAt() end-of-day.
		return 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// slaDueAt computes the work-item SLA deadline.
// Commitment Ask uses end of the current UTC day (spec: 当日), floored at created+2h.
func slaDueAt(p Product, created, now time.Time) time.Time {
	created = created.UTC()
	now = now.UTC()
	if p == ProductCommitmentAsk {
		endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)
		minDue := created.Add(2 * time.Hour)
		if endOfDay.Before(minDue) {
			return minDue
		}
		return endOfDay
	}
	return created.Add(defaultSLADuration(p))
}
