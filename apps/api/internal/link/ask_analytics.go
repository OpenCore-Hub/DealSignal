package link

import "github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"

// LinkAskSummary aggregates unified Ask turn counts for owner analytics (Phase B).
type LinkAskSummary struct {
	TotalTurns      int64    `json:"total_turns"`
	AIAnswered      int64    `json:"ai_answered"`
	AIRefused       int64    `json:"ai_refused"`
	HostPending     int64    `json:"host_pending"`
	HostAnswered    int64    `json:"host_answered"`
	UserEscalated   int64    `json:"user_escalated"`
	AutoEscalated   int64    `json:"auto_escalated"`
	DeflectionRate  *float64 `json:"deflection_rate,omitempty"`
	RefuseRate      *float64 `json:"refuse_rate,omitempty"`
	EscalationRate  *float64 `json:"escalation_rate,omitempty"`
}

func linkAskDeflectionRate(aiAnswered, hostPending int64) *float64 {
	den := aiAnswered + hostPending
	if den <= 0 {
		return nil
	}
	rate := float64(aiAnswered) / float64(den)
	return &rate
}

func linkAskRefuseRate(aiAnswered, aiRefused int64) *float64 {
	den := aiAnswered + aiRefused
	if den <= 0 {
		return nil
	}
	rate := float64(aiRefused) / float64(den)
	return &rate
}

func linkAskEscalationRate(userEscalated, autoEscalated, aiAnswered, aiRefused int64) *float64 {
	den := aiAnswered + aiRefused
	escalated := userEscalated + autoEscalated
	if den <= 0 || escalated <= 0 {
		return nil
	}
	rate := float64(escalated) / float64(den)
	return &rate
}

func mapLinkAskSummary(row db.GetLinkAskTurnSummaryRow) LinkAskSummary {
	summary := LinkAskSummary{
		TotalTurns:    row.TotalTurns,
		AIAnswered:    row.AiAnswered,
		AIRefused:     row.AiRefused,
		HostPending:   row.HostPending,
		HostAnswered:  row.HostAnswered,
		UserEscalated: row.UserEscalated,
		AutoEscalated: row.AutoEscalated,
	}
	summary.DeflectionRate = linkAskDeflectionRate(summary.AIAnswered, summary.HostPending)
	summary.RefuseRate = linkAskRefuseRate(summary.AIAnswered, summary.AIRefused)
	summary.EscalationRate = linkAskEscalationRate(
		summary.UserEscalated,
		summary.AutoEscalated,
		summary.AIAnswered,
		summary.AIRefused,
	)
	return summary
}
