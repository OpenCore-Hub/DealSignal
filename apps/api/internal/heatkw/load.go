// Package heatkw loads workspace key-page RuleSets shared by analytics,
// suggestions, and contact heat scoring.
package heatkw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Querier is the DB surface needed to resolve workspace key-page settings.
type Querier interface {
	GetWorkspaceKeyPageSettings(ctx context.Context, workspaceID pgtype.UUID) (db.WorkspaceKeyPageSetting, error)
}

// Load returns circle defaults merged with workspace extras.
// circleOverride nil → workspace default_circle (or founder).
func Load(ctx context.Context, q Querier, workspaceID string, circleOverride *heat.Circle) (heat.RuleSet, error) {
	circle := heat.CircleDefault
	var extras map[string][]string

	if workspaceID != "" {
		parsed, err := uuid.Parse(workspaceID)
		if err != nil {
			return heat.RuleSet{}, fmt.Errorf("workspace id: %w", err)
		}
		wsUUID := pgtype.UUID{Bytes: parsed, Valid: true}
		row, err := q.GetWorkspaceKeyPageSettings(ctx, wsUUID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return heat.RuleSet{}, fmt.Errorf("workspace key page settings: %w", err)
		}
		if err == nil {
			circle = normalizeCircle(heat.Circle(row.DefaultCircle))
			extras, err = decodeExtras(row.ExtraKeywords)
			if err != nil {
				return heat.RuleSet{}, err
			}
		}
	}
	if circleOverride != nil {
		circle = normalizeCircle(*circleOverride)
	}
	// Built-ins follow Settings → Language via Accept-Language; extras stay bilingual.
	return heat.NewRuleSet(circle, extras).WithLang(heat.KeywordLangFromLocale(locale.FromContext(ctx))), nil
}

// LoadForWorkspaceUUID is Load with a pgtype workspace id.
func LoadForWorkspaceUUID(ctx context.Context, q Querier, workspaceID pgtype.UUID, circleOverride *heat.Circle) (heat.RuleSet, error) {
	if !workspaceID.Valid {
		return heat.NewRuleSet(heat.CircleDefault, nil).WithLang(heat.KeywordLangFromLocale(locale.FromContext(ctx))), nil
	}
	return Load(ctx, q, uuid.UUID(workspaceID.Bytes).String(), circleOverride)
}

func normalizeCircle(circle heat.Circle) heat.Circle {
	switch circle {
	case heat.CircleFounder, heat.CircleInvestor, heat.CircleSales:
		return circle
	default:
		return heat.CircleDefault
	}
}

func decodeExtras(raw []byte) (map[string][]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out map[string][]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("extra_keywords json: %w", err)
	}
	return out, nil
}
