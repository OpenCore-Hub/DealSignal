package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heatkw"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	maxExtraKeywordsTotal   = 50
	maxExtraKeywordRunes    = 64
	maxExtraCategories      = 12
)

var (
	errKeyPageSettingsInvalid = errors.New("invalid key page settings")
	errKeyPageSettingsForbidden = errors.New("forbidden")
)

// KeyPageSettings is the workspace-configurable key-page / heat keyword config.
type KeyPageSettings struct {
	DefaultCircle string
	ExtraKeywords map[string][]string
	// BuiltinRules are circle defaults only (no workspace extras) for editor disclosure.
	BuiltinRules []KeyPageComplianceMatchRule
	// MatchRules are the effective merged set (builtins + extras).
	MatchRules []KeyPageComplianceMatchRule
	UpdatedAt  time.Time
	CanEdit    bool
}

// KeyPageSettingsUpdate is the PUT body for workspace key-page settings.
type KeyPageSettingsUpdate struct {
	DefaultCircle string
	ExtraKeywords map[string][]string
}

// loadWorkspaceRuleSet returns circle defaults merged with workspace extras.
// circleOverride nil → workspace default_circle (or founder). Non-nil forces that circle
// while still merging the same workspace extras (additive keywords stay shared).
func (s *Service) loadWorkspaceRuleSet(ctx context.Context, workspaceID string, circleOverride *heat.Circle) (heat.RuleSet, error) {
	return heatkw.Load(ctx, s.queries, workspaceID, circleOverride)
}

func workspaceIDFromLink(link db.Link) string {
	if !link.WorkspaceID.Valid {
		return ""
	}
	return uuid.UUID(link.WorkspaceID.Bytes).String()
}

func decodeExtraKeywords(raw []byte) (map[string][]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out map[string][]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%w: extra_keywords json", errKeyPageSettingsInvalid)
	}
	return out, nil
}

func validateKeyPageSettingsUpdate(req KeyPageSettingsUpdate) (heat.Circle, map[string][]string, error) {
	circle := normalizeHeatCircle(heat.Circle(req.DefaultCircle))
	if req.DefaultCircle != "" && circle != heat.Circle(req.DefaultCircle) {
		// normalizeHeatCircle maps unknown → default; reject explicit junk.
		switch heat.Circle(req.DefaultCircle) {
		case heat.CircleFounder, heat.CircleInvestor, heat.CircleSales:
		default:
			return "", nil, fmt.Errorf("%w: default_circle", errKeyPageSettingsInvalid)
		}
	}
	extras := heat.NewRuleSet(circle, req.ExtraKeywords).Extra
	if extras == nil {
		extras = map[string][]string{}
	}
	if len(extras) > maxExtraCategories {
		return "", nil, fmt.Errorf("%w: too many categories", errKeyPageSettingsInvalid)
	}
	total := 0
	for cat, kws := range extras {
		if cat != "custom" {
			// Allow built-in category keys plus custom; reject garbage keys.
			allowed := false
			for _, c := range []string{
				"financials", "team", "traction", "market",
				"performance", "distribution", "strategy", "portfolio",
				"pricing", "security", "case_studies", "implementation",
				"custom",
			} {
				if cat == c {
					allowed = true
					break
				}
			}
			if !allowed {
				return "", nil, fmt.Errorf("%w: category %q", errKeyPageSettingsInvalid, cat)
			}
		}
		for _, kw := range kws {
			total++
			if utf8.RuneCountInString(kw) > maxExtraKeywordRunes {
				return "", nil, fmt.Errorf("%w: keyword too long", errKeyPageSettingsInvalid)
			}
			if strings.ContainsAny(kw, "%_") {
				return "", nil, fmt.Errorf("%w: keyword SQL wildcards not allowed", errKeyPageSettingsInvalid)
			}
		}
	}
	if total > maxExtraKeywordsTotal {
		return "", nil, fmt.Errorf("%w: too many keywords", errKeyPageSettingsInvalid)
	}
	return circle, extras, nil
}

// GetKeyPageSettings returns workspace defaults + extras + effective match rules.
func (s *Service) GetKeyPageSettings(ctx context.Context, workspaceID, userID string) (KeyPageSettings, error) {
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return KeyPageSettings{}, err
	}
	out := KeyPageSettings{
		DefaultCircle: string(heat.CircleDefault),
		ExtraKeywords: map[string][]string{},
		BuiltinRules:  []KeyPageComplianceMatchRule{},
		MatchRules:    []KeyPageComplianceMatchRule{},
		CanEdit:       s.isWorkspaceManager(ctx, workspaceID, userID),
	}
	row, err := s.queries.GetWorkspaceKeyPageSettings(ctx, wsUUID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return KeyPageSettings{}, err
	}
	if err == nil {
		out.DefaultCircle = string(normalizeHeatCircle(heat.Circle(row.DefaultCircle)))
		extras, derr := decodeExtraKeywords(row.ExtraKeywords)
		if derr != nil {
			return KeyPageSettings{}, derr
		}
		if extras != nil {
			out.ExtraKeywords = extras
		}
		if row.UpdatedAt.Valid {
			out.UpdatedAt = row.UpdatedAt.Time.UTC()
		}
	}
	circle := heat.Circle(out.DefaultCircle)
	lang := heat.KeywordLangFromLocale(locale.FromContext(ctx))
	for _, r := range heat.NewRuleSet(circle, nil).WithLang(lang).Rules() {
		out.BuiltinRules = append(out.BuiltinRules, KeyPageComplianceMatchRule{
			Category: r.Category,
			Keywords: append([]string(nil), r.Keywords...),
		})
	}
	rs := heat.NewRuleSet(circle, out.ExtraKeywords).WithLang(lang)
	for _, r := range rs.Rules() {
		out.MatchRules = append(out.MatchRules, KeyPageComplianceMatchRule{
			Category: r.Category,
			Keywords: append([]string(nil), r.Keywords...),
		})
	}
	return out, nil
}

// SaveKeyPageSettings upserts workspace key-page settings (owner/admin only).
func (s *Service) SaveKeyPageSettings(ctx context.Context, workspaceID, userID string, req KeyPageSettingsUpdate) (KeyPageSettings, error) {
	if !s.isWorkspaceManager(ctx, workspaceID, userID) {
		return KeyPageSettings{}, errKeyPageSettingsForbidden
	}
	circle, extras, err := validateKeyPageSettingsUpdate(req)
	if err != nil {
		return KeyPageSettings{}, err
	}
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return KeyPageSettings{}, err
	}
	ws, err := s.queries.GetWorkspaceByID(ctx, wsUUID)
	if err != nil {
		return KeyPageSettings{}, fmt.Errorf("workspace: %w", err)
	}
	raw, err := json.Marshal(extras)
	if err != nil {
		return KeyPageSettings{}, err
	}
	_, err = s.queries.UpsertWorkspaceKeyPageSettings(ctx, db.UpsertWorkspaceKeyPageSettingsParams{
		WorkspaceID:   wsUUID,
		TenantID:      ws.TenantID,
		DefaultCircle: string(circle),
		ExtraKeywords: raw,
	})
	if err != nil {
		return KeyPageSettings{}, err
	}
	return s.GetKeyPageSettings(ctx, workspaceID, userID)
}

func (s *Service) isWorkspaceManager(ctx context.Context, workspaceID, userID string) bool {
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return false
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return false
	}
	m, err := s.queries.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: wsUUID,
		UserID:      pgtype.UUID{Bytes: userUUID, Valid: true},
	})
	if err != nil {
		return false
	}
	return m.Role == "owner" || m.Role == "admin"
}
