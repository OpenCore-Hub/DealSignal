package knowledge

import (
	"context"
	"math"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

const opsDefaultWindowHours = 24

// OpsAnswersQuota is workspace answer entitlement snapshot for the SLO board.
type OpsAnswersQuota struct {
	Used        int  `json:"used"`
	Limit       int  `json:"limit"`
	Included    bool `json:"included"`
	WindowHours int  `json:"windowHours"`
}

// OpsSummary is the workspace-scoped knowledge Q&A SLO / cost board.
type OpsSummary struct {
	Scope           string           `json:"scope"` // workspace
	WindowHours     int              `json:"windowHours"`
	TurnsTotal      int64            `json:"turnsTotal"`
	TurnsByStatus   map[string]int64 `json:"turnsByStatus"`
	AvgDurationMs   float64          `json:"avgDurationMs"`
	P95DurationMs   float64          `json:"p95DurationMs"`
	CostUnitsTotal  int64            `json:"costUnitsTotal"`
	RefusalsByKind  map[string]int64 `json:"refusalsByKind"`
	JudgmentsByKind map[string]int64 `json:"judgmentsByKind"`
	// EvalCandidatesByStatus is workspace gold-review queue depth (ceiling Phase O).
	EvalCandidatesByStatus map[string]int64 `json:"evalCandidatesByStatus"`
	PendingEvalCandidates  int64            `json:"pendingEvalCandidates"`
	AnswersQuota           OpsAnswersQuota  `json:"answersQuota"`
	RetentionDays          int              `json:"retentionDays"`
	ColdArchiveCount       int64            `json:"coldArchiveCount"`
	RoomCorpusFingerprint  string           `json:"roomCorpusFingerprint,omitempty"`
	PrometheusHints        []string         `json:"prometheusHints"`
}

// WithRetentionDays records hot retention for ops summaries (optional).
func (s *Service) WithRetentionDays(days int) *Service {
	if s != nil {
		s.retentionDays = days
	}
	return s
}

// GetOpsSummary returns workspace ask volume, latency, cost proxy, and cold-archive counts.
func (s *Service) GetOpsSummary(
	ctx context.Context,
	roomID, workspaceID, userID string,
	windowHours int,
) (OpsSummary, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return OpsSummary{}, err
	}
	if windowHours <= 0 {
		windowHours = opsDefaultWindowHours
	}
	if windowHours > 24*90 {
		windowHours = 24 * 90
	}
	since := time.Now().UTC().Add(-time.Duration(windowHours) * time.Hour)
	ws := pgUUID(workspaceID)
	sinceTS := pgtype.Timestamptz{Time: since, Valid: true}

	out := OpsSummary{
		Scope:                  "workspace",
		WindowHours:            windowHours,
		TurnsByStatus:          map[string]int64{},
		RefusalsByKind:         map[string]int64{},
		JudgmentsByKind:        map[string]int64{},
		EvalCandidatesByStatus: map[string]int64{},
		RetentionDays:          s.retentionDays,
		PrometheusHints: []string{
			"dealsignal_knowledge_qa_turn_duration_seconds",
			"dealsignal_knowledge_qa_turns_total",
			"dealsignal_knowledge_qa_refusal_total",
			"dealsignal_knowledge_qa_judgment_total",
			"dealsignal_knowledge_qa_eval_candidates_total",
			"dealsignal_knowledge_qa_rewrite_total",
			"dealsignal_knowledge_qa_archive_success_total",
		},
	}

	statusRows, err := s.queries.CountKnowledgeQATurnsForWorkspaceByStatusSince(ctx, db.CountKnowledgeQATurnsForWorkspaceByStatusSinceParams{
		WorkspaceID: ws,
		Since:       sinceTS,
	})
	if err != nil {
		return OpsSummary{}, err
	}
	var total int64
	for _, row := range statusRows {
		out.TurnsByStatus[row.ResultStatus] = row.Count
		total += row.Count
	}
	out.TurnsTotal = total

	avgRow, err := s.queries.AvgKnowledgeQATurnDurationMsForWorkspaceSince(ctx, db.AvgKnowledgeQATurnDurationMsForWorkspaceSinceParams{
		WorkspaceID: ws,
		Since:       sinceTS,
	})
	if err != nil {
		return OpsSummary{}, err
	}
	out.AvgDurationMs = avgRow.AvgMs

	p95Row, err := s.queries.P95KnowledgeQATurnDurationMsForWorkspaceSince(ctx, db.P95KnowledgeQATurnDurationMsForWorkspaceSinceParams{
		WorkspaceID: ws,
		Since:       sinceTS,
	})
	if err != nil {
		return OpsSummary{}, err
	}
	out.P95DurationMs = p95Row.P95Ms

	costUnits, err := s.queries.SumKnowledgeQACostUnitsForWorkspaceSince(ctx, db.SumKnowledgeQACostUnitsForWorkspaceSinceParams{
		WorkspaceID: ws,
		Since:       sinceTS,
	})
	if err != nil {
		return OpsSummary{}, err
	}
	out.CostUnitsTotal = costUnits

	refusalRows, err := s.queries.CountKnowledgeQARefusalsByKindForWorkspaceSince(ctx, db.CountKnowledgeQARefusalsByKindForWorkspaceSinceParams{
		WorkspaceID: ws,
		Since:       sinceTS,
	})
	if err != nil {
		return OpsSummary{}, err
	}
	for _, row := range refusalRows {
		if row.Kind == "" {
			continue
		}
		out.RefusalsByKind[row.Kind] = row.Count
	}

	judgmentRows, err := s.queries.CountKnowledgeQAJudgmentsByKindForWorkspaceSince(ctx, db.CountKnowledgeQAJudgmentsByKindForWorkspaceSinceParams{
		WorkspaceID: ws,
		Since:       sinceTS,
	})
	if err != nil {
		return OpsSummary{}, err
	}
	for _, row := range judgmentRows {
		if row.Kind == "" {
			continue
		}
		out.JudgmentsByKind[row.Kind] = row.Count
	}

	quota := s.answersQuotaSnapshot(ctx, ws)
	windowH := int(math.Round(quota.Window.Hours()))
	if windowH <= 0 {
		windowH = 24
	}
	out.AnswersQuota = OpsAnswersQuota{
		Used:        quota.Used,
		Limit:       quota.Limit,
		Included:    quota.Included,
		WindowHours: windowH,
	}

	if n, cerr := s.queries.CountKnowledgeQASessionArchivesForWorkspace(ctx, ws); cerr == nil {
		out.ColdArchiveCount = n
	}
	if evalRows, eerr := s.queries.CountKnowledgeQAEvalCandidatesByStatusForWorkspace(ctx, ws); eerr == nil {
		for _, row := range evalRows {
			if row.ReviewStatus == "" {
				continue
			}
			out.EvalCandidatesByStatus[row.ReviewStatus] = row.Count
			if row.ReviewStatus == EvalReviewPending {
				out.PendingEvalCandidates = row.Count
			}
		}
	}
	if fp, ferr := s.roomCorpusFingerprint(ctx, roomID); ferr == nil {
		out.RoomCorpusFingerprint = fp
	}
	return out, nil
}
