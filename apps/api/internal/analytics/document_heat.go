package analytics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrDocumentHeatNotFound is returned when the library row is missing, archived, or an agreement.
var ErrDocumentHeatNotFound = errors.New("document not found")

const (
	documentHeatOverlayDepthWeight   = 8.0
	documentHeatOverlayQAWeight      = 3.0
	documentHeatOverlayQACap         = 5
	documentHeatOverlayDomainStep    = 0.08
	documentHeatOverlayDomainCap     = 1.25
	documentHeatOverlayScrollGate    = 0.1
	documentHeatOverlayShortDwellSec = 15.0
	documentHeatOverlayShallowFactor = 0.25

	documentHeatFactorReadingDepth = "readingDepth"
	documentHeatFactorQACitations  = "qaCitations"
	documentHeatFactorCrossDomain  = "crossDomain"

	documentHeatShareKindRoom     = "room"
	documentHeatShareKindBundle   = "bundle"
	documentHeatShareKindDocument = "document"

	documentHeatKeyPageMinSeconds = 3
)

// DocumentHeatScore is the document-native heat.Compute result plus contributing shares.
type DocumentHeatScore struct {
	DocumentID        string
	Title             string
	Score             int
	Level             string
	Trend             string
	Circle            string
	Breakdown         map[string]float64
	Overlay           DocumentHeatOverlay
	Views             int64
	ContributingLinks []DocumentHeatLink
	KeyPages          DocumentHeatKeyPages
}

// DocumentHeatKeyPages is explain evidence. Ranking uses Engaged only.
type DocumentHeatKeyPages struct {
	Engaged    int64
	Total      int64
	MinSeconds int
	Pages      []DocumentHeatKeyPage
}

// DocumentHeatKeyPage is one title-matched page on this file.
type DocumentHeatKeyPage struct {
	PageNumber   int32
	Title        string
	EngagedViews int64
	TotalViews   int64
}

// DocumentHeatOverlay is the document-only increment on top of heat.Compute.
// Link ranking does not use this.
type DocumentHeatOverlay struct {
	ReadingDepth float64
	QACitations  float64
	CrossDomain  float64
}

// DocumentHeatLink is a share that contributed page views to a document.
type DocumentHeatLink struct {
	ID               string
	Name             string
	PageViews        int64
	ShareKind        string
	DealRoomID       string
	HasDocumentScope bool
}

type documentHeatInputs struct {
	uniqueVisitors     int64
	visitorDays        int64
	bounceCount        int64
	avgDurationSeconds float64
	lastViewedAt       pgtype.Timestamptz
	keyPageViews       int
	sessionDepth       float64
	sessionCount       int64
	avgScrollDepth     float64
	scrollSamples      int64
	avgSessionDuration float64
	qaTurns            int
	emailDomains       int
}

type documentHeatExtras struct {
	sessionDepth       float64
	sessionCount       int64
	avgScrollDepth     float64
	scrollSamples      int64
	avgSessionDuration float64
	qaTurns            int
	emailDomains       int
}

// computeDocumentHeat scores a document from attributed page_views, then applies
// a document-only overlay. heat.Compute body and weights stay untouched.
// Forward/download stay 0 — those events are link-scoped and must not leak
// onto every bundle member.
func computeDocumentHeat(in documentHeatInputs, circle heat.Circle) heat.Result {
	revisits := int(in.visitorDays) - int(in.uniqueVisitors)
	if revisits < 0 {
		revisits = 0
	}
	decayDays := 0.0
	if in.lastViewedAt.Valid {
		decayDays = time.Since(in.lastViewedAt.Time).Hours() / 24
	}
	base := heat.Compute(circle, heat.Input{
		Opens:              int(in.uniqueVisitors),
		Revisits:           revisits,
		AvgDurationMinutes: in.avgDurationSeconds / 60.0,
		KeyPageViews:       in.keyPageViews,
		ForwardSignals:     0,
		Downloads:          0,
		BouncePenalty:      int(in.bounceCount),
		DecayDays:          decayDays,
	})
	return applyDocumentHeatOverlay(base, circle, in, decayDays)
}

func applyDocumentHeatOverlay(base heat.Result, circle heat.Circle, in documentHeatInputs, decayDays float64) heat.Result {
	depth := in.sessionDepth
	if depth < 0 {
		depth = 0
	}
	if depth > 1 {
		depth = 1
	}
	// Only gate a single session. Averaged scroll/dwell across mixed
	// traffic would punish a real deep read sitting next to skims.
	if in.sessionCount == 1 && in.scrollSamples > 0 && in.avgScrollDepth < documentHeatOverlayScrollGate && in.avgSessionDuration < documentHeatOverlayShortDwellSec {
		depth *= documentHeatOverlayShallowFactor
	}

	qa := in.qaTurns
	if qa < 0 {
		qa = 0
	}
	if qa > documentHeatOverlayQACap {
		qa = documentHeatOverlayQACap
	}
	domains := in.emailDomains
	if domains < 0 {
		domains = 0
	}

	decay := documentHeatDecay(decayDays)
	bonus := depth*documentHeatOverlayDepthWeight + float64(qa)*documentHeatOverlayQAWeight
	mult := documentHeatDomainMultiplier(domains)
	depthPts := depth * documentHeatOverlayDepthWeight * decay
	qaPts := float64(qa) * documentHeatOverlayQAWeight * decay
	crossPts := (depthPts + qaPts) * (mult - 1)
	overlay := bonus * decay * mult

	score := int(math.Max(0, math.Min(100, math.Round(float64(base.Score)+overlay))))
	bd := make(map[string]float64, len(base.Breakdown)+3)
	for k, v := range base.Breakdown {
		bd[k] = v
	}
	bd[documentHeatFactorReadingDepth] = depthPts
	bd[documentHeatFactorQACitations] = qaPts
	bd[documentHeatFactorCrossDomain] = crossPts

	return heat.Result{
		Score:     score,
		Level:     documentHeatLevel(circle, score),
		Trend:     base.Trend,
		Breakdown: bd,
	}
}

func documentHeatDecay(days float64) float64 {
	if days <= 0 || heat.DecayHalfLifeDays <= 0 {
		return 1
	}
	return math.Pow(2, -days/heat.DecayHalfLifeDays)
}

func documentHeatShareKind(isRoom, hasDocumentScope bool) string {
	if isRoom {
		return documentHeatShareKindRoom
	}
	if hasDocumentScope {
		return documentHeatShareKindBundle
	}
	return documentHeatShareKindDocument
}

func documentHeatDomainMultiplier(domains int) float64 {
	if domains <= 1 {
		return 1
	}
	mult := 1 + documentHeatOverlayDomainStep*float64(domains-1)
	if mult > documentHeatOverlayDomainCap {
		return documentHeatOverlayDomainCap
	}
	return mult
}

func documentHeatLevel(circle heat.Circle, score int) string {
	hot, warm := 75, 40
	switch circle {
	case heat.CircleInvestor:
		hot, warm = 70, 35
	case heat.CircleSales:
		hot, warm = 72, 38
	}
	if score >= hot {
		return "hot"
	}
	if score >= warm {
		return "warm"
	}
	return "cold"
}

func overlayFromBreakdown(bd map[string]float64) DocumentHeatOverlay {
	if bd == nil {
		return DocumentHeatOverlay{}
	}
	return DocumentHeatOverlay{
		ReadingDepth: bd[documentHeatFactorReadingDepth],
		QACitations:  bd[documentHeatFactorQACitations],
		CrossDomain:  bd[documentHeatFactorCrossDomain],
	}
}

func extrasFromRow(row db.GetDocumentHeatExtrasBatchRow) documentHeatExtras {
	return documentHeatExtras{
		sessionDepth:       row.SessionDepth,
		sessionCount:       row.SessionCount,
		avgScrollDepth:     row.AvgScrollDepth,
		scrollSamples:      row.ScrollSamples,
		avgSessionDuration: row.AvgSessionDuration,
		qaTurns:            int(row.QaTurns),
		emailDomains:       int(row.EmailDomains),
	}
}

func applyExtras(in documentHeatInputs, extra documentHeatExtras) documentHeatInputs {
	in.sessionDepth = extra.sessionDepth
	in.sessionCount = extra.sessionCount
	in.avgScrollDepth = extra.avgScrollDepth
	in.scrollSamples = extra.scrollSamples
	in.avgSessionDuration = extra.avgSessionDuration
	in.qaTurns = extra.qaTurns
	in.emailDomains = extra.emailDomains
	return in
}

func (s *Service) loadDocumentHeatExtras(ctx context.Context, workspaceID pgtype.UUID, docIDs []pgtype.UUID) (map[string]documentHeatExtras, error) {
	out := map[string]documentHeatExtras{}
	if len(docIDs) == 0 {
		return out, nil
	}
	rows, err := s.queries.GetDocumentHeatExtrasBatch(ctx, db.GetDocumentHeatExtrasBatchParams{
		WorkspaceID: workspaceID,
		DocumentIds: docIDs,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if !row.DocumentID.Valid {
			continue
		}
		out[uuid.UUID(row.DocumentID.Bytes).String()] = extrasFromRow(row)
	}
	return out, nil
}

// LinkKeyPageEvidence is explain-only. GetScore return type stays heat.Result.
func (s *Service) LinkKeyPageEvidence(ctx context.Context, linkID, workspaceID pgtype.UUID, circleOverride *heat.Circle) DocumentHeatKeyPages {
	out := DocumentHeatKeyPages{MinSeconds: documentHeatKeyPageMinSeconds}
	link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          linkID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return out
	}
	rs := s.documentHeatRuleSet(ctx, uuid.UUID(workspaceID.Bytes).String(), circleOverride)
	rs = s.enrichRuleSetForLink(ctx, uuid.UUID(workspaceID.Bytes).String(), link, rs)
	patterns := rs.Patterns()
	if len(patterns) == 0 {
		return out
	}
	metrics, mErr := s.queries.GetLinkKeyPageViewMetrics(ctx, db.GetLinkKeyPageViewMetricsParams{
		LinkID:   linkID,
		Patterns: patterns,
	})
	if mErr != nil {
		slog.Error("link key page metrics unavailable",
			"workspace_id", uuid.UUID(workspaceID.Bytes).String(),
			"link_id", uuid.UUID(linkID.Bytes).String(),
			"error", mErr)
	} else {
		out.Engaged = metrics.EngagedKeyPageViews
		out.Total = metrics.TotalKeyPageViews
	}
	rows, dErr := s.queries.GetLinkKeyPageViewDetails(ctx, db.GetLinkKeyPageViewDetailsParams{
		LinkID:   linkID,
		Patterns: patterns,
	})
	if dErr != nil {
		slog.Error("link key page details unavailable",
			"workspace_id", uuid.UUID(workspaceID.Bytes).String(),
			"link_id", uuid.UUID(linkID.Bytes).String(),
			"error", dErr)
		return out
	}
	out.Pages = make([]DocumentHeatKeyPage, 0, len(rows))
	for _, row := range rows {
		out.Pages = append(out.Pages, DocumentHeatKeyPage{
			PageNumber:   row.PageNumber,
			Title:        row.Title,
			EngagedViews: row.EngagedViews,
			TotalViews:   row.Views,
		})
	}
	return out
}

func (s *Service) documentHeatRuleSet(ctx context.Context, workspaceID string, circleOverride *heat.Circle) heat.RuleSet {
	rs, err := s.loadWorkspaceRuleSet(ctx, workspaceID, circleOverride)
	if err != nil {
		slog.Error("document heat ruleset unavailable; using founder defaults",
			"workspace_id", workspaceID,
			"error", err)
		return heat.NewRuleSet(heat.CircleDefault, nil)
	}
	return rs
}

func (s *Service) documentKeyPagesOrZero(ctx context.Context, workspaceID pgtype.UUID, docIDs []pgtype.UUID, patterns []string) map[string]int64 {
	out := map[string]int64{}
	if len(docIDs) == 0 || len(patterns) == 0 {
		return out
	}
	kpRows, err := s.queries.GetDocumentKeyPageViewMetricsBatch(ctx, db.GetDocumentKeyPageViewMetricsBatchParams{
		WorkspaceID: workspaceID,
		DocumentIds: docIDs,
		Patterns:    patterns,
	})
	if err != nil {
		slog.Error("document key pages unavailable; ranking without key-page boost",
			"workspace_id", uuid.UUID(workspaceID.Bytes).String(),
			"document_count", len(docIDs),
			"error", err)
		return out
	}
	for _, kp := range kpRows {
		if kp.DocumentID.Valid {
			out[uuid.UUID(kp.DocumentID.Bytes).String()] = kp.EngagedKeyPageViews
		}
	}
	return out
}

func (s *Service) documentKeyPageDetailsOrEmpty(ctx context.Context, workspaceID, documentID pgtype.UUID, patterns []string) DocumentHeatKeyPages {
	out := DocumentHeatKeyPages{MinSeconds: documentHeatKeyPageMinSeconds}
	if len(patterns) == 0 {
		return out
	}
	rows, err := s.queries.GetDocumentKeyPageViewDetails(ctx, db.GetDocumentKeyPageViewDetailsParams{
		WorkspaceID: workspaceID,
		DocumentID:  documentID,
		Patterns:    patterns,
	})
	if err != nil {
		slog.Error("document key page details unavailable",
			"workspace_id", uuid.UUID(workspaceID.Bytes).String(),
			"document_id", uuid.UUID(documentID.Bytes).String(),
			"error", err)
		return out
	}
	out.Pages = make([]DocumentHeatKeyPage, 0, len(rows))
	for _, row := range rows {
		out.Engaged += row.EngagedViews
		out.Total += row.TotalViews
		title := ""
		if row.Title.Valid {
			title = row.Title.String
		}
		out.Pages = append(out.Pages, DocumentHeatKeyPage{
			PageNumber:   row.PageNumber,
			Title:        title,
			EngagedViews: row.EngagedViews,
			TotalViews:   row.TotalViews,
		})
	}
	return out
}

func (s *Service) documentHeatExtrasOrZero(ctx context.Context, workspaceID pgtype.UUID, docIDs []pgtype.UUID) map[string]documentHeatExtras {
	extras, err := s.loadDocumentHeatExtras(ctx, workspaceID, docIDs)
	if err != nil {
		slog.Error("document heat extras unavailable; using Compute-only scores",
			"workspace_id", uuid.UUID(workspaceID.Bytes).String(),
			"document_count", len(docIDs),
			"error", err)
		return map[string]documentHeatExtras{}
	}
	return extras
}

func (s *Service) rankTopDocuments(ctx context.Context, workspaceID pgtype.UUID, rs heat.RuleSet, topN int) ([]DocumentScore, error) {
	rows, err := s.queries.ListDocumentHeatMetricsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("document heat metrics: %w", err)
	}
	if len(rows) == 0 {
		return []DocumentScore{}, nil
	}

	docIDs := make([]pgtype.UUID, 0, len(rows))
	for _, row := range rows {
		docIDs = append(docIDs, row.ID)
	}

	keyPages := s.documentKeyPagesOrZero(ctx, workspaceID, docIDs, rs.Patterns())

	extras := s.documentHeatExtrasOrZero(ctx, workspaceID, docIDs)

	out := make([]DocumentScore, 0, len(rows))
	for _, row := range rows {
		docID := uuid.UUID(row.ID.Bytes).String()
		res := computeDocumentHeat(applyExtras(documentHeatInputs{
			uniqueVisitors:     row.UniqueVisitors,
			visitorDays:        row.VisitorDays,
			bounceCount:        row.BounceCount,
			avgDurationSeconds: row.AvgDurationSeconds,
			lastViewedAt:       row.LastViewedAt,
			keyPageViews:       int(keyPages[docID]),
		}, extras[docID]), rs.Circle)
		out = append(out, DocumentScore{
			ID:            row.ID,
			Title:         row.Title,
			Views:         row.TotalPageViews, // display grain; Compute still uses UniqueVisitors as opens
			Score:         res.Score,
			Level:         res.Level,
			PrimaryLinkID: row.PrimaryLinkID,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Views != out[j].Views {
			return out[i].Views > out[j].Views
		}
		return uuid.UUID(out[i].ID.Bytes).String() < uuid.UUID(out[j].ID.Bytes).String()
	})
	if topN > 0 && len(out) > topN {
		out = out[:topN]
	}
	return out, nil
}

// ListDocumentHeatScores returns every attributed file's document-native heat.
// topN=0 keeps the full set; Insights overview still truncates via rankTopDocuments.
func (s *Service) ListDocumentHeatScores(ctx context.Context, workspaceID pgtype.UUID, circleOverride *heat.Circle) ([]DocumentScore, error) {
	rs := s.documentHeatRuleSet(ctx, uuid.UUID(workspaceID.Bytes).String(), circleOverride)
	return s.rankTopDocuments(ctx, workspaceID, rs, 0)
}

// GetDocumentHeatScore returns document-native heat for the explain dialog.
func (s *Service) GetDocumentHeatScore(ctx context.Context, documentID, workspaceID pgtype.UUID, circleOverride *heat.Circle) (DocumentHeatScore, error) {
	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          documentID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DocumentHeatScore{}, ErrDocumentHeatNotFound
		}
		return DocumentHeatScore{}, err
	}
	if doc.Status == "archived" || strings.EqualFold(doc.Category, "agreement") {
		return DocumentHeatScore{}, ErrDocumentHeatNotFound
	}

	rs := s.documentHeatRuleSet(ctx, uuid.UUID(workspaceID.Bytes).String(), circleOverride)

	row, err := s.queries.GetDocumentHeatMetrics(ctx, db.GetDocumentHeatMetricsParams{
		WorkspaceID: workspaceID,
		ID:          documentID,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return DocumentHeatScore{}, fmt.Errorf("document heat metrics: %w", err)
	}

	keyPageViews := 0
	if err == nil {
		keyPages := s.documentKeyPagesOrZero(ctx, workspaceID, []pgtype.UUID{documentID}, rs.Patterns())
		keyPageViews = int(keyPages[uuid.UUID(documentID.Bytes).String()])
	}

	var extra documentHeatExtras
	if err == nil {
		extras := s.documentHeatExtrasOrZero(ctx, workspaceID, []pgtype.UUID{documentID})
		extra = extras[uuid.UUID(documentID.Bytes).String()]
	}

	var res heat.Result
	views := int64(0)
	title := doc.Title
	if err == nil {
		title = row.Title
		views = row.TotalPageViews // display grain; Compute still uses UniqueVisitors as opens
		res = computeDocumentHeat(applyExtras(documentHeatInputs{
			uniqueVisitors:     row.UniqueVisitors,
			visitorDays:        row.VisitorDays,
			bounceCount:        row.BounceCount,
			avgDurationSeconds: row.AvgDurationSeconds,
			lastViewedAt:       row.LastViewedAt,
			keyPageViews:       keyPageViews,
		}, extra), rs.Circle)
	} else {
		res = computeDocumentHeat(documentHeatInputs{}, rs.Circle)
	}

	contrib, cErr := s.queries.ListDocumentHeatContributingLinks(ctx, db.ListDocumentHeatContributingLinksParams{
		WorkspaceID: workspaceID,
		DocumentID:  documentID,
	})
	if cErr != nil {
		slog.Error("document heat contributing links unavailable",
			"workspace_id", uuid.UUID(workspaceID.Bytes).String(),
			"document_id", uuid.UUID(documentID.Bytes).String(),
			"error", cErr)
		contrib = nil
	}
	links := make([]DocumentHeatLink, 0, len(contrib))
	for _, c := range contrib {
		name := ""
		if c.Name.Valid {
			name = c.Name.String
		}
		dealRoomID := ""
		if c.DealRoomID.Valid {
			dealRoomID = uuid.UUID(c.DealRoomID.Bytes).String()
		}
		links = append(links, DocumentHeatLink{
			ID:               uuid.UUID(c.LinkID.Bytes).String(),
			Name:             name,
			PageViews:        c.PageViews,
			ShareKind:        documentHeatShareKind(c.DealRoomID.Valid, c.HasDocumentScope),
			DealRoomID:       dealRoomID,
			HasDocumentScope: c.HasDocumentScope,
		})
	}

	keyEvidence := s.documentKeyPageDetailsOrEmpty(ctx, workspaceID, documentID, rs.Patterns())
	if keyEvidence.Engaged == 0 && keyPageViews > 0 {
		keyEvidence.Engaged = int64(keyPageViews)
	}

	return DocumentHeatScore{
		DocumentID:        uuid.UUID(documentID.Bytes).String(),
		Title:             title,
		Score:             res.Score,
		Level:             res.Level,
		Trend:             res.Trend,
		Circle:            string(rs.Circle),
		Breakdown:         res.Breakdown,
		Overlay:           overlayFromBreakdown(res.Breakdown),
		Views:             views,
		ContributingLinks: links,
		KeyPages:          keyEvidence,
	}, nil
}
