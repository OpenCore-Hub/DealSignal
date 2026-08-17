package analytics

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestComputeDocumentHeat_DoesNotCopyLinkForwardOrDownload(t *testing.T) {
	res := computeDocumentHeat(documentHeatInputs{
		uniqueVisitors:     2,
		visitorDays:        3,
		bounceCount:        0,
		avgDurationSeconds: 120,
		lastViewedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		keyPageViews:       1,
	}, heat.CircleFounder)
	if res.Breakdown["forwardSignals"] != 0 || res.Breakdown["downloads"] != 0 {
		t.Fatalf("document heat must not include link-scoped forward/download: %+v", res.Breakdown)
	}
	if res.Score <= 0 {
		t.Fatalf("expected positive document score, got %d", res.Score)
	}
}

func TestComputeDocumentHeat_RevisitsFromVisitorDays(t *testing.T) {
	oneDay := computeDocumentHeat(documentHeatInputs{uniqueVisitors: 2, visitorDays: 2}, heat.CircleFounder)
	multiDay := computeDocumentHeat(documentHeatInputs{uniqueVisitors: 2, visitorDays: 5}, heat.CircleFounder)
	if multiDay.Breakdown["revisits"] <= oneDay.Breakdown["revisits"] {
		t.Fatalf("extra visit-days must increase revisits: one=%v multi=%v", oneDay.Breakdown["revisits"], multiDay.Breakdown["revisits"])
	}
}

func TestComputeDocumentHeat_BundleMembersCanDiverge(t *testing.T) {
	longRead := computeDocumentHeat(documentHeatInputs{
		uniqueVisitors:     1,
		visitorDays:        1,
		avgDurationSeconds: 600,
		lastViewedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}, heat.CircleFounder)
	skim := computeDocumentHeat(documentHeatInputs{
		uniqueVisitors:     1,
		visitorDays:        1,
		avgDurationSeconds: 5,
		bounceCount:        1,
		lastViewedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}, heat.CircleFounder)
	if longRead.Score <= skim.Score {
		t.Fatalf("document dwell must separate bundle members: long=%d skim=%d", longRead.Score, skim.Score)
	}
}

func TestDocumentHeatSQLUsesAttributionNotLinkAccessCount(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "db", "queries.sql"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read queries.sql: %v", err)
	}
	sql := string(raw)
	block := queryNamedSQL(sql, "ListDocumentHeatMetricsByWorkspace")
	if block == "" {
		t.Fatal("missing ListDocumentHeatMetricsByWorkspace")
	}
	if !strings.Contains(block, "COALESCE(pv.document_id, l.document_id)") {
		t.Fatal("document heat must attribute page_views via COALESCE")
	}
	if strings.Contains(block, "l.access_count") {
		t.Fatal("document heat must not sum link.access_count")
	}
	if !strings.Contains(block, "status IS DISTINCT FROM 'archived'") {
		t.Fatal("document heat must exclude archived library rows")
	}
	if !strings.Contains(block, "category IS DISTINCT FROM 'agreement'") {
		t.Fatal("document heat must exclude agreement documents")
	}
	explain := queryNamedSQL(sql, "GetDocumentHeatMetrics")
	if explain == "" {
		t.Fatal("missing GetDocumentHeatMetrics")
	}
	if !strings.Contains(explain, "category IS DISTINCT FROM 'agreement'") {
		t.Fatal("document heat explain must exclude agreement documents")
	}
	if !strings.Contains(block, "workspace_members") {
		t.Fatal("document heat must exclude workspace-member traffic")
	}
	kp := queryNamedSQL(sql, "GetDocumentKeyPageViewMetricsBatch")
	if kp == "" {
		t.Fatal("key pages must be document-attributed")
	}
	if !strings.Contains(kp, "duration_seconds >= 3") {
		t.Fatal("document key-page ranking must require the same 3s engage gate as Insights KPIs")
	}
	if !strings.Contains(kp, "engaged_key_page_views") {
		t.Fatal("document key pages must expose engaged vs total")
	}
	details := queryNamedSQL(sql, "GetDocumentKeyPageViewDetails")
	if details == "" {
		t.Fatal("missing GetDocumentKeyPageViewDetails")
	}
	if !strings.Contains(details, "duration_seconds >= 3") {
		t.Fatal("key-page explain must use the same engage gate")
	}
	extras := queryNamedSQL(sql, "GetDocumentHeatExtrasBatch")
	if extras == "" {
		t.Fatal("missing GetDocumentHeatExtrasBatch")
	}
	if !strings.Contains(extras, "category IS DISTINCT FROM 'agreement'") {
		t.Fatal("extras must skip agreement documents")
	}
	if !strings.Contains(extras, "reading_sessions") {
		t.Fatal("extras must read session depth")
	}
	if !strings.Contains(extras, "hit->>'documentId'") {
		t.Fatal("Q&A must cite the document, not the whole share")
	}
	if !strings.Contains(extras, "jsonb_typeof") {
		t.Fatal("malformed Ask payloads must be skipped, not fail the batch")
	}
	if !strings.Contains(extras, "jsonb_array_length") {
		t.Fatal("empty Ask hit arrays must be skipped before expand")
	}
	if !strings.Contains(extras, "viewers AS") {
		t.Fatal("email domains must start from document viewers, not the whole workspace")
	}
	if !strings.Contains(extras, "FROM pages p") || !strings.Contains(extras, "NULLIF(d.page_count, 0)") {
		t.Fatal("missing page_count must fall back to pages rows, not invent depth")
	}
	if !strings.Contains(extras, "CROSS JOIN LATERAL jsonb_array_elements") {
		t.Fatal("Ask hits must expand with LATERAL, not a cartesian join")
	}
	if strings.Contains(extras, "knowledge_qa_turns") {
		t.Fatal("owner knowledge desk must not boost document heat")
	}
	if strings.Contains(extras, "contacts") || strings.Contains(extras, "organization") {
		t.Fatal("must not invent company names from domains")
	}
	if !strings.Contains(extras, "workspace_members") {
		t.Fatal("extras must exclude workspace-member traffic")
	}
	if !strings.Contains(extras, "COALESCE(rs.document_id, l.document_id)") {
		t.Fatal("session depth must use the same document attribution")
	}
	if !strings.Contains(extras, "WITH docs AS") {
		t.Fatal("extras must be set-based, not per-document LATERAL")
	}
	if strings.Contains(extras, "LEFT JOIN LATERAL") {
		t.Fatal("per-document LATERAL would timeout Insights")
	}
	if !strings.Contains(extras, "MAX(") {
		t.Fatal("session depth must keep the best read, not average skims")
	}
	if !strings.Contains(extras, "session_count") {
		t.Fatal("extras must expose session_count so the scroll gate cannot punish mixed traffic")
	}
	contrib := queryNamedSQL(sql, "ListDocumentHeatContributingLinks")
	if contrib == "" {
		t.Fatal("missing ListDocumentHeatContributingLinks")
	}
	if !strings.Contains(contrib, "COALESCE(pv.document_id, l.document_id)") {
		t.Fatal("contributing links must use the same page_view attribution")
	}
}

func TestGetDocumentHeatScoreAgreementNotFound(t *testing.T) {
	q := &mockAnalyticsQuerier{
		document: db.GetDocumentByIDRow{Status: "ready", Category: "agreement", Title: "NDA"},
	}
	svc := NewService(q, nil, testCfg())
	founder := heat.CircleFounder
	_, err := svc.GetDocumentHeatScore(context.Background(), pgtype.UUID{Valid: true}, pgtype.UUID{Valid: true}, &founder)
	if !errors.Is(err, ErrDocumentHeatNotFound) {
		t.Fatalf("expected ErrDocumentHeatNotFound, got %v", err)
	}
}

func TestLinkHeatKeyPageCountUsesEngaged(t *testing.T) {
	if got := linkHeatKeyPageCount(0); got != 0 {
		t.Fatalf("zero engaged: %d", got)
	}
	if got := linkHeatKeyPageCount(2); got != 2 {
		t.Fatalf("engaged: %d", got)
	}
}

func TestLinkHeatSQLScoringUsesEngagedNotTotal(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	svcPath := filepath.Clean(filepath.Join(filepath.Dir(file), "service.go"))
	raw, err := os.ReadFile(svcPath)
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	src := string(raw)
	if !strings.Contains(src, "linkHeatKeyPageCount(keyMetrics.EngagedKeyPageViews)") {
		t.Fatal("getScoreForLink must score engaged key pages")
	}
	if strings.Count(src, "r.TotalKeyPageViews") != 0 {
		t.Fatal("dashboard/insights must not feed TotalKeyPageViews into Compute")
	}
	sqlPath := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "db", "queries.sql"))
	sqlRaw, err := os.ReadFile(sqlPath)
	if err != nil {
		t.Fatalf("read queries.sql: %v", err)
	}
	details := queryNamedSQL(string(sqlRaw), "GetLinkKeyPageViewDetails")
	if !strings.Contains(details, "duration_seconds >= 3") {
		t.Fatal("link key-page explain must expose the 3s engage gate")
	}
	if !strings.Contains(details, "AS engaged_views") {
		t.Fatal("link key-page explain must expose engaged_views")
	}
}

func TestDocumentHeatShareKind(t *testing.T) {
	if got := documentHeatShareKind(true, true); got != documentHeatShareKindRoom {
		t.Fatalf("room wins over bundle: %s", got)
	}
	if got := documentHeatShareKind(false, true); got != documentHeatShareKindBundle {
		t.Fatalf("bundle: %s", got)
	}
	if got := documentHeatShareKind(false, false); got != documentHeatShareKindDocument {
		t.Fatalf("document: %s", got)
	}
}

func TestGetDocumentHeatScoreArchivedNotFound(t *testing.T) {
	q := &mockAnalyticsQuerier{
		document: db.GetDocumentByIDRow{Status: "archived", Title: "Old deck"},
	}
	svc := NewService(q, nil, testCfg())
	founder := heat.CircleFounder
	_, err := svc.GetDocumentHeatScore(context.Background(), pgtype.UUID{Valid: true}, pgtype.UUID{Valid: true}, &founder)
	if !errors.Is(err, ErrDocumentHeatNotFound) {
		t.Fatalf("expected ErrDocumentHeatNotFound, got %v", err)
	}
}

func TestComputeDocumentHeat_ZeroOverlayMatchesCompute(t *testing.T) {
	in := documentHeatInputs{
		uniqueVisitors:     2,
		visitorDays:        3,
		avgDurationSeconds: 120,
		keyPageViews:       1,
	}
	withOverlay := computeDocumentHeat(in, heat.CircleFounder)
	baseOnly := heat.Compute(heat.CircleFounder, heat.Input{
		Opens:              2,
		Revisits:           1,
		AvgDurationMinutes: 2,
		KeyPageViews:       1,
		DecayDays:          0,
	})
	if withOverlay.Score != baseOnly.Score {
		t.Fatalf("zero extras must not change Compute score: overlay=%d base=%d", withOverlay.Score, baseOnly.Score)
	}
	if withOverlay.Breakdown["forwardSignals"] != 0 || withOverlay.Breakdown["downloads"] != 0 {
		t.Fatalf("link-scoped factors leaked: %+v", withOverlay.Breakdown)
	}
}

func TestComputeDocumentHeat_DeepReadPlusQABeatsSkim(t *testing.T) {
	now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	base := documentHeatInputs{
		uniqueVisitors:     1,
		visitorDays:        1,
		avgDurationSeconds: 90,
		lastViewedAt:       now,
	}
	skim := computeDocumentHeat(base, heat.CircleFounder)
	deep := computeDocumentHeat(documentHeatInputs{
		uniqueVisitors:     1,
		visitorDays:        1,
		avgDurationSeconds: 90,
		lastViewedAt:       now,
		sessionDepth:       1,
		qaTurns:            3,
		emailDomains:       2,
	}, heat.CircleFounder)
	if deep.Score <= skim.Score {
		t.Fatalf("deep read + cited Q&A must outrank a skim: deep=%d skim=%d", deep.Score, skim.Score)
	}
	if deep.Breakdown[documentHeatFactorReadingDepth] <= 0 {
		t.Fatalf("expected reading-depth overlay, got %+v", deep.Breakdown)
	}
	if deep.Breakdown[documentHeatFactorQACitations] <= 0 {
		t.Fatalf("expected Q&A overlay, got %+v", deep.Breakdown)
	}
	if deep.Breakdown[documentHeatFactorCrossDomain] <= 0 {
		t.Fatalf("expected cross-domain overlay, got %+v", deep.Breakdown)
	}
}

func TestComputeDocumentHeat_ScrollGateCapsShallowBounce(t *testing.T) {
	now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	full := computeDocumentHeat(documentHeatInputs{
		uniqueVisitors:     1,
		visitorDays:        1,
		lastViewedAt:       now,
		sessionDepth:       1,
		sessionCount:       1,
		avgScrollDepth:     0.8,
		scrollSamples:      4,
		avgSessionDuration: 120,
	}, heat.CircleFounder)
	gated := computeDocumentHeat(documentHeatInputs{
		uniqueVisitors:     1,
		visitorDays:        1,
		lastViewedAt:       now,
		sessionDepth:       1,
		sessionCount:       1,
		avgScrollDepth:     0.05,
		scrollSamples:      4,
		avgSessionDuration: 5,
	}, heat.CircleFounder)
	if gated.Breakdown[documentHeatFactorReadingDepth] >= full.Breakdown[documentHeatFactorReadingDepth] {
		t.Fatalf("low scroll + short dwell must cap depth: gated=%v full=%v", gated.Breakdown[documentHeatFactorReadingDepth], full.Breakdown[documentHeatFactorReadingDepth])
	}
}

func TestComputeDocumentHeat_MixedSessionsDoNotGateBestRead(t *testing.T) {
	res := computeDocumentHeat(documentHeatInputs{
		uniqueVisitors:     1,
		visitorDays:        1,
		sessionDepth:       1,
		sessionCount:       4,
		avgScrollDepth:     0.05,
		scrollSamples:      12,
		avgSessionDuration: 5,
	}, heat.CircleFounder)
	if res.Breakdown[documentHeatFactorReadingDepth] != documentHeatOverlayDepthWeight {
		t.Fatalf("mixed traffic must keep the best-read depth: %+v", res.Breakdown)
	}
}

func TestComputeDocumentHeat_MissingScrollDoesNotGate(t *testing.T) {
	res := computeDocumentHeat(documentHeatInputs{
		uniqueVisitors: 1,
		visitorDays:    1,
		sessionDepth:   1,
	}, heat.CircleFounder)
	if res.Breakdown[documentHeatFactorReadingDepth] != documentHeatOverlayDepthWeight {
		t.Fatalf("missing scroll must not punish depth: %+v", res.Breakdown)
	}
}

func TestComputeDocumentHeat_QACapAndSingleDomainNoMult(t *testing.T) {
	capped := computeDocumentHeat(documentHeatInputs{
		uniqueVisitors: 1,
		visitorDays:    1,
		qaTurns:        20,
		emailDomains:   1,
	}, heat.CircleFounder)
	five := computeDocumentHeat(documentHeatInputs{
		uniqueVisitors: 1,
		visitorDays:    1,
		qaTurns:        5,
		emailDomains:   1,
	}, heat.CircleFounder)
	if capped.Breakdown[documentHeatFactorQACitations] != five.Breakdown[documentHeatFactorQACitations] {
		t.Fatalf("Q&A must cap at 5: capped=%v five=%v", capped.Breakdown[documentHeatFactorQACitations], five.Breakdown[documentHeatFactorQACitations])
	}
	if capped.Breakdown[documentHeatFactorCrossDomain] != 0 {
		t.Fatalf("one email domain must not apply a multiplier: %+v", capped.Breakdown)
	}
}

func TestGetDocumentHeatScore_ExtrasFailureFallsBackToCompute(t *testing.T) {
	q := &mockAnalyticsQuerier{
		document:       db.GetDocumentByIDRow{Status: "ready", Title: "Live deck"},
		heatMetricsSet: true,
		heatMetrics: db.GetDocumentHeatMetricsRow{
			Title:          "Live deck",
			UniqueVisitors: 2,
			VisitorDays:    2,
		},
		extrasErr: errors.New("extras down"),
	}
	svc := NewService(q, nil, testCfg())
	founder := heat.CircleFounder
	score, err := svc.GetDocumentHeatScore(context.Background(), pgtype.UUID{Valid: true}, pgtype.UUID{Valid: true}, &founder)
	if err != nil {
		t.Fatalf("extras failure must not 500 explain: %v", err)
	}
	base := computeDocumentHeat(documentHeatInputs{uniqueVisitors: 2, visitorDays: 2}, heat.CircleFounder)
	if score.Score != base.Score {
		t.Fatalf("fallback score: got %d want %d", score.Score, base.Score)
	}
}

func TestRankTopDocuments_KeyPagesFailureStillRanks(t *testing.T) {
	id := pgtype.UUID{Valid: true}
	q := &mockAnalyticsQuerier{
		listHeatMetrics: []db.ListDocumentHeatMetricsByWorkspaceRow{{
			ID:             id,
			Title:          "Deck",
			UniqueVisitors: 1,
			VisitorDays:    1,
		}},
		keyPageBatchErr: errors.New("key pages down"),
	}
	svc := NewService(q, nil, testCfg())
	out, err := svc.rankTopDocuments(context.Background(), id, heat.NewRuleSet(heat.CircleFounder, nil), 5)
	if err != nil {
		t.Fatalf("key-page failure must not 500 overview: %v", err)
	}
	if len(out) != 1 || out[0].Title != "Deck" {
		t.Fatalf("ranked: %+v", out)
	}
}

func TestRankTopDocuments_ExtrasFailureStillRanks(t *testing.T) {
	id := pgtype.UUID{Valid: true}
	q := &mockAnalyticsQuerier{
		listHeatMetrics: []db.ListDocumentHeatMetricsByWorkspaceRow{{
			ID:             id,
			Title:          "Deck",
			UniqueVisitors: 1,
			VisitorDays:    1,
		}},
		extrasErr: errors.New("extras down"),
	}
	svc := NewService(q, nil, testCfg())
	out, err := svc.rankTopDocuments(context.Background(), id, heat.NewRuleSet(heat.CircleFounder, nil), 5)
	if err != nil {
		t.Fatalf("extras failure must not 500 overview: %v", err)
	}
	if len(out) != 1 || out[0].Title != "Deck" {
		t.Fatalf("ranked: %+v", out)
	}
}

func TestRankTopDocuments_ViewsUsesAttributedPageViews(t *testing.T) {
	ws := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	deep := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	skim := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	q := &mockAnalyticsQuerier{
		listHeatMetrics: []db.ListDocumentHeatMetricsByWorkspaceRow{
			{
				ID:             skim,
				Title:          "Skim",
				TotalPageViews: 2,
				UniqueVisitors: 1,
				VisitorDays:    1,
			},
			{
				ID:             deep,
				Title:          "Deep",
				TotalPageViews: 12,
				UniqueVisitors: 1,
				VisitorDays:    1,
			},
		},
	}
	svc := NewService(q, nil, testCfg())
	out, err := svc.rankTopDocuments(context.Background(), ws, heat.NewRuleSet(heat.CircleFounder, nil), 5)
	if err != nil {
		t.Fatalf("rank: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("ranked: %+v", out)
	}
	if out[0].Title != "Deep" || out[0].Views != 12 {
		t.Fatalf("views must be attributed page_views, not unique visitors: %+v", out)
	}
	if out[1].Title != "Skim" || out[1].Views != 2 {
		t.Fatalf("tie-break must use page_views: %+v", out)
	}
	if out[0].Score != out[1].Score {
		t.Fatalf("same unique visitors must keep equal heat scores: %+v", out)
	}
}

func TestListDocumentHeatScores_UsesAttributedPageViews(t *testing.T) {
	ws := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	id := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	q := &mockAnalyticsQuerier{
		listHeatMetrics: []db.ListDocumentHeatMetricsByWorkspaceRow{{
			ID:             id,
			Title:          "Deck",
			TotalPageViews: 12,
			UniqueVisitors: 1,
			VisitorDays:    1,
		}},
	}
	svc := NewService(q, nil, testCfg())
	out, err := svc.ListDocumentHeatScores(context.Background(), ws, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out) != 1 || out[0].Title != "Deck" || out[0].Views != 12 {
		t.Fatalf("library overlay must use attributed page_views: %+v", out)
	}
}

func TestRegisterWorkspaceRoutesIncludesDocumentScores(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "handler.go"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read handler.go: %v", err)
	}
	src := string(raw)
	scoresAt := strings.Index(src, `g.GET("/documents/scores"`)
	itemAt := strings.Index(src, `g.GET("/documents/:documentId/score"`)
	if scoresAt < 0 || itemAt < 0 {
		t.Fatal("missing document heat score routes")
	}
	if scoresAt > itemAt {
		t.Fatal("static /documents/scores must register before /documents/:documentId/score")
	}
}

func TestGetDocumentHeatScore_ContributingLinksFailureStillExplains(t *testing.T) {
	q := &mockAnalyticsQuerier{
		document:       db.GetDocumentByIDRow{Status: "ready", Title: "Live deck"},
		heatMetricsSet: true,
		heatMetrics: db.GetDocumentHeatMetricsRow{
			Title:          "Live deck",
			TotalPageViews: 12,
			UniqueVisitors: 1,
			VisitorDays:    1,
		},
		contribLinksErr: errors.New("contrib down"),
	}
	svc := NewService(q, nil, testCfg())
	founder := heat.CircleFounder
	score, err := svc.GetDocumentHeatScore(context.Background(), pgtype.UUID{Valid: true}, pgtype.UUID{Valid: true}, &founder)
	if err != nil {
		t.Fatalf("contributing-link failure must not 500 explain: %v", err)
	}
	if score.Title != "Live deck" || score.Views != 12 {
		t.Fatalf("score: %+v", score)
	}
	if len(score.ContributingLinks) != 0 {
		t.Fatalf("expected empty contributing links, got %+v", score.ContributingLinks)
	}
}

func TestGetDocumentHeatScore_RuleSetFailureStillExplains(t *testing.T) {
	q := &mockAnalyticsQuerier{
		document:           db.GetDocumentByIDRow{Status: "ready", Title: "Live deck"},
		keyPageSettingsErr: errors.New("settings down"),
	}
	svc := NewService(q, nil, testCfg())
	score, err := svc.GetDocumentHeatScore(context.Background(), pgtype.UUID{Valid: true}, pgtype.UUID{Valid: true}, nil)
	if err != nil {
		t.Fatalf("ruleset failure must not 500 explain: %v", err)
	}
	if score.Circle != string(heat.CircleFounder) {
		t.Fatalf("fallback circle: %s", score.Circle)
	}
}

func TestGetDocumentHeatScoreZeroWithoutPageViews(t *testing.T) {
	q := &mockAnalyticsQuerier{
		document: db.GetDocumentByIDRow{Status: "ready", Title: "Live deck"},
	}
	svc := NewService(q, nil, testCfg())
	founder := heat.CircleFounder
	score, err := svc.GetDocumentHeatScore(context.Background(), pgtype.UUID{Valid: true}, pgtype.UUID{Valid: true}, &founder)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if score.Title != "Live deck" {
		t.Fatalf("title: %s", score.Title)
	}
	if score.Circle != string(heat.CircleFounder) {
		t.Fatalf("circle: %s", score.Circle)
	}
	if score.Views != 0 {
		t.Fatalf("views: %d", score.Views)
	}
	if score.Breakdown["forwardSignals"] != 0 || score.Breakdown["downloads"] != 0 {
		t.Fatalf("link-scoped factors leaked: %+v", score.Breakdown)
	}
}

func queryNamedSQL(sql, name string) string {
	marker := "-- name: " + name + " "
	start := strings.Index(sql, marker)
	if start < 0 {
		return ""
	}
	rest := sql[start:]
	next := strings.Index(rest[len(marker):], "\n-- name: ")
	if next < 0 {
		return rest
	}
	return rest[:len(marker)+next]
}
