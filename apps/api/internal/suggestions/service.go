package suggestions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrLinkNotFound       = errors.New("link not found")
	ErrSuggestionNotFound = errors.New("suggestion not found")
)

// Suggestion is the public view of a generated suggestion.
type Suggestion struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	WorkspaceID string `json:"workspace_id"`
	ContactID  string `json:"contact_id,omitempty"`
	LinkID     string `json:"link_id"`
	DocumentID string `json:"document_id,omitempty"`
	Type       string `json:"type"`
	Priority   string `json:"priority"`
	Title      string `json:"title"`
	Reason     string `json:"reason"`
	Action     string `json:"action"`
	Dismissed  bool   `json:"dismissed"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// Notifier enqueues notifications for high-intent signals.
type Notifier interface {
	Enqueue(ctx context.Context, workspaceID, userID, channel, subject, body string) error
}

// Service generates follow-up suggestions from link analytics.
type Service struct {
	queries   *db.Queries
	notifier  Notifier
}

// NewService creates a suggestion service.
func NewService(q *db.Queries, n Notifier) *Service {
	return &Service{queries: q, notifier: n}
}

// Generate creates suggestions for a link based on recent access events.
func (s *Service) Generate(ctx context.Context, workspaceID, linkID string) ([]Suggestion, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	linkUUID, err := pgUUID(linkID)
	if err != nil {
		return nil, err
	}

	link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          linkUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		return nil, ErrLinkNotFound
	}

	metrics, err := s.metrics(ctx, linkUUID)
	if err != nil {
		return nil, err
	}

	result := heat.Compute(heat.CircleDefault, metrics.heatInput())
	candidates := buildCandidates(result, metrics)

	out := make([]Suggestion, 0, len(candidates))
	for _, c := range candidates {
		exists, err := s.recentExists(ctx, link.WorkspaceID, linkUUID, c.Type)
		if err != nil {
			return nil, fmt.Errorf("check recent suggestion: %w", err)
		}
		if exists {
			continue
		}
		row, err := s.queries.CreateSuggestion(ctx, db.CreateSuggestionParams{
			TenantID:    link.TenantID,
			WorkspaceID: link.WorkspaceID,
			ContactID:   pgtype.UUID{},
			LinkID:      pgtype.UUID{Bytes: linkUUID.Bytes, Valid: true},
			DocumentID:  link.DocumentID,
			Type:        c.Type,
			Reason:      c.Reason,
			Action:      c.Action,
		})
		if err != nil {
			return nil, fmt.Errorf("create suggestion: %w", err)
		}
		out = append(out, suggestionFromRow(row))

		if c.Type == "hot_signal" && s.notifier != nil {
			userID := ""
			if link.CreatedBy.Valid {
				userID = uuid.UUID(link.CreatedBy.Bytes).String()
			}
			_ = s.notifier.Enqueue(ctx, workspaceID, userID, "email", titleForType(c.Type), c.Reason+"\n"+c.Action)
		}
	}
	return out, nil
}

// List returns active suggestions for a link.
func (s *Service) List(ctx context.Context, workspaceID, linkID string) ([]Suggestion, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	linkUUID, err := pgUUID(linkID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListSuggestionsByLink(ctx, db.ListSuggestionsByLinkParams{
		LinkID:      linkUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Suggestion, len(rows))
	for i, r := range rows {
		out[i] = suggestionFromRow(r)
	}
	return out, nil
}

// WorkspaceSuggestion is the camelCase view used by the workspace insights list.
type WorkspaceSuggestion struct {
	ID             string `json:"id"`
	ContactID      string `json:"contactId"`
	ContactEmail   string `json:"contactEmail"`
	DocumentTitle  string `json:"documentTitle"`
	LinkID         string `json:"linkId"`
	HeatLevel      string `json:"heatLevel"`
	Score          int    `json:"score"`
	Reason         string `json:"reason"`
	Action         string `json:"action"`
	LastActivityAt string `json:"lastActivityAt"`
}

// ListWorkspace returns active suggestions across the workspace enriched for display.
func (s *Service) ListWorkspace(ctx context.Context, workspaceID string) ([]WorkspaceSuggestion, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListSuggestionsByWorkspace(ctx, wsUUID)
	if err != nil {
		return nil, err
	}

	contacts, err := s.queries.ListContactsByWorkspace(ctx, wsUUID)
	if err != nil {
		return nil, err
	}
	contactEmailByID := make(map[string]string, len(contacts))
	for _, c := range contacts {
		contactEmailByID[uuidToString(c.ID)] = c.Email.String
	}

	out := make([]WorkspaceSuggestion, 0, len(rows))
	for _, r := range rows {
		su := WorkspaceSuggestion{
			ID:             uuidToString(r.ID),
			ContactID:      uuidToString(r.ContactID),
			LinkID:         uuidToString(r.LinkID),
			Reason:         r.Reason,
			Action:         r.Action,
			LastActivityAt: r.UpdatedAt.Time.Format(time.RFC3339),
		}
		if su.ContactID != "" {
			su.ContactEmail = contactEmailByID[su.ContactID]
		}

		if r.DocumentID.Valid {
			doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
				ID:          r.DocumentID,
				WorkspaceID: wsUUID,
			})
			if err == nil {
				su.DocumentTitle = doc.Title
			}
		}

		if r.LinkID.Valid {
			res := s.linkHeatResult(ctx, r.LinkID)
			su.Score = res.Score
			su.HeatLevel = res.Level
			if su.HeatLevel == "" {
				su.HeatLevel = "cold"
			}
		}

		out = append(out, su)
	}
	return out, nil
}

func (s *Service) linkHeatResult(ctx context.Context, linkID pgtype.UUID) heat.Result {
	access, err := s.queries.GetLinkAccessMetrics(ctx, linkID)
	if err != nil {
		return heat.Result{Level: "cold"}
	}
	pageViews, err := s.queries.GetLinkPageViewMetrics(ctx, linkID)
	if err != nil {
		return heat.Result{Level: "cold"}
	}
	bounce, err := s.queries.GetLinkBounceCount(ctx, linkID)
	if err != nil {
		bounce = 0
	}
	revisits := int(access.Opens) - int(access.UniqueVisitors)
	if revisits < 0 {
		revisits = 0
	}
	return heat.Compute(heat.CircleDefault, heat.Input{
		Opens:              int(access.Opens),
		Revisits:           revisits,
		AvgDurationMinutes: pageViews.AvgDurationSeconds / 60.0,
		KeyPageViews:       int(pageViews.KeyPageViews),
		ForwardSignals:     int(access.UniqueVisitors),
		Downloads:          int(access.Downloads),
		BouncePenalty:      int(bounce),
	})
}

// Dismiss marks a suggestion as dismissed.
func (s *Service) Dismiss(ctx context.Context, workspaceID, suggestionID string) error {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	id, err := pgUUID(suggestionID)
	if err != nil {
		return err
	}
	_, err = s.queries.GetSuggestionByID(ctx, db.GetSuggestionByIDParams{ID: id, WorkspaceID: wsUUID})
	if err != nil {
		return ErrSuggestionNotFound
	}
	return s.queries.DismissSuggestion(ctx, db.DismissSuggestionParams{ID: id, WorkspaceID: wsUUID})
}

func (s *Service) recentExists(ctx context.Context, workspaceID, linkID pgtype.UUID, typ string) (bool, error) {
	count, err := s.queries.CountRecentSuggestionsByLinkAndType(ctx, db.CountRecentSuggestionsByLinkAndTypeParams{
		LinkID:      linkID,
		WorkspaceID: workspaceID,
		Type:        typ,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Service) metrics(ctx context.Context, linkID pgtype.UUID) (suggestionMetrics, error) {
	var m suggestionMetrics
	access, err := s.queries.GetLinkAccessMetrics(ctx, linkID)
	if err != nil {
		return m, err
	}
	m.opens = int(access.Opens)
	m.uniqueVisitors = int(access.UniqueVisitors)
	m.downloads = int(access.Downloads)

	pv, err := s.queries.GetLinkPageViewMetrics(ctx, linkID)
	if err != nil {
		return m, err
	}
	m.avgDurationMinutes = pv.AvgDurationSeconds / 60.0
	m.keyPageViews = int(pv.KeyPageViews)
	m.totalPageViews = int(pv.TotalPageViews)

	bounceCount, err := s.queries.GetLinkBounceCount(ctx, linkID)
	if err != nil {
		return m, err
	}
	m.bounces = int(bounceCount)
	m.revisits = m.opens - m.uniqueVisitors
	if m.revisits < 0 {
		m.revisits = 0
	}
	return m, nil
}

type suggestionMetrics struct {
	opens              int
	uniqueVisitors     int
	revisits           int
	avgDurationMinutes float64
	keyPageViews       int
	totalPageViews     int
	downloads          int
	bounces            int
}

func (m suggestionMetrics) heatInput() heat.Input {
	return heat.Input{
		Opens:              m.opens,
		Revisits:           m.revisits,
		AvgDurationMinutes: m.avgDurationMinutes,
		KeyPageViews:       m.keyPageViews,
		ForwardSignals:     m.uniqueVisitors,
		Downloads:          m.downloads,
		BouncePenalty:      m.bounces,
	}
}

type candidate struct {
	Type   string
	Reason string
	Action string
}

func buildCandidates(result heat.Result, m suggestionMetrics) []candidate {
	var out []candidate
	if result.Level == "hot" && m.opens >= 2 {
		out = append(out, candidate{
			Type:   "hot_signal",
			Reason: fmt.Sprintf("热度评分达到 %d（%s），该联系人在 %d 次打开中查看了 %d 个关键页面", result.Score, result.Level, m.opens, m.keyPageViews),
			Action: "立即发送 follow-up 邮件并提供深度资料",
		})
	}
	if m.downloads > 0 {
		out = append(out, candidate{
			Type:   "follow_up",
			Reason: fmt.Sprintf("联系人在最近 %d 次访问中尝试了下载", m.opens),
			Action: "确认对方是否收到文件并询问反馈",
		})
	}
	if m.revisits > 0 {
		out = append(out, candidate{
			Type:   "follow_up",
			Reason: fmt.Sprintf("联系人重复访问了 %d 次，表现出持续兴趣", m.revisits),
			Action: "发送针对性的内容或安排一次通话",
		})
	}
	if m.bounces > 0 && m.avgDurationMinutes < 0.5 {
		out = append(out, candidate{
			Type:   "risk_alert",
			Reason: fmt.Sprintf("%d 次访问后快速离开，平均停留 %.1f 分钟", m.bounces, m.avgDurationMinutes),
			Action: "优化材料首屏或换一种触达方式",
		})
	}
	return out
}

func suggestionFromRow(r db.Suggestion) Suggestion {
	s := Suggestion{
		ID:          uuidToString(r.ID),
		TenantID:    uuidToString(r.TenantID),
		WorkspaceID: uuidToString(r.WorkspaceID),
		LinkID:      uuidToString(r.LinkID),
		DocumentID:  uuidToString(r.DocumentID),
		Type:        r.Type,
		Reason:      r.Reason,
		Action:      r.Action,
		Dismissed:   r.Dismissed,
		CreatedAt:   r.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt:   r.UpdatedAt.Time.Format(time.RFC3339),
	}
	if r.ContactID.Valid {
		s.ContactID = uuidToString(r.ContactID)
	}
	s.Priority = priorityForType(r.Type)
	s.Title = titleForType(r.Type)
	return s
}

func priorityForType(typ string) string {
	switch typ {
	case "hot_signal":
		return "high"
	case "risk_alert":
		return "medium"
	default:
		return "low"
	}
}

func titleForType(typ string) string {
	switch typ {
	case "hot_signal":
		return "高意向信号"
	case "risk_alert":
		return "风险预警"
	default:
		return "跟进建议"
	}
}

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

func pgUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}
