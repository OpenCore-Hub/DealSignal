package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestValidateKeyPageSettingsUpdate(t *testing.T) {
	circle, extras, err := validateKeyPageSettingsUpdate(KeyPageSettingsUpdate{
		DefaultCircle: "sales",
		ExtraKeywords: map[string][]string{
			"custom":     {"watermark", "保密"},
			"financials": {"cap table"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if circle != heat.CircleSales {
		t.Fatalf("circle=%s", circle)
	}
	if len(extras["custom"]) != 2 || extras["financials"][0] != "cap table" {
		t.Fatalf("extras=%v", extras)
	}

	_, _, err = validateKeyPageSettingsUpdate(KeyPageSettingsUpdate{
		DefaultCircle: "nope",
	})
	if !errors.Is(err, errKeyPageSettingsInvalid) {
		t.Fatalf("want invalid, got %v", err)
	}

	_, _, err = validateKeyPageSettingsUpdate(KeyPageSettingsUpdate{
		DefaultCircle: "founder",
		ExtraKeywords: map[string][]string{"custom": {"100%"}},
	})
	if !errors.Is(err, errKeyPageSettingsInvalid) {
		t.Fatalf("wildcard should reject, got %v", err)
	}
}

func TestGetKeyPageSettingsSeparatesBuiltinAndExtras(t *testing.T) {
	ws := uuid.New()
	user := uuid.New()
	raw, _ := json.Marshal(map[string][]string{
		"financials": {"cap table"},
		"custom":     {"watermark"},
	})
	q := &mockAnalyticsQuerier{
		keyPageSettings: db.WorkspaceKeyPageSetting{
			WorkspaceID:   pgtype.UUID{Bytes: ws, Valid: true},
			DefaultCircle: "founder",
			ExtraKeywords: raw,
		},
	}
	svc := NewService(q, nil, testCfg())
	got, err := svc.GetKeyPageSettings(context.Background(), ws.String(), user.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.BuiltinRules) == 0 {
		t.Fatal("expected builtin rules")
	}
	for _, r := range got.BuiltinRules {
		for _, kw := range r.Keywords {
			if kw == "cap table" || kw == "watermark" {
				t.Fatalf("builtin must not include extras: %v", r)
			}
		}
	}
	merged := false
	for _, r := range got.MatchRules {
		if r.Category == "financials" {
			for _, kw := range r.Keywords {
				if kw == "cap table" {
					merged = true
				}
			}
		}
	}
	if !merged {
		t.Fatal("matchRules should merge financials extras")
	}
	if got.ExtraKeywords["custom"][0] != "watermark" {
		t.Fatalf("extras=%v", got.ExtraKeywords)
	}
}

func TestLoadWorkspaceRuleSetMergesExtras(t *testing.T) {
	ws := uuid.New()
	raw, _ := json.Marshal(map[string][]string{"custom": {"watermark"}})
	q := &mockAnalyticsQuerier{
		keyPageSettings: db.WorkspaceKeyPageSetting{
			WorkspaceID:   pgtype.UUID{Bytes: ws, Valid: true},
			DefaultCircle: "founder",
			ExtraKeywords: raw,
		},
	}
	svc := NewService(q, nil, testCfg())
	rs, err := svc.loadWorkspaceRuleSet(context.Background(), ws.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !rs.IsKeyPage("Document Watermark Guide") {
		t.Fatal("expected custom extra to match")
	}
	if !rs.IsKeyPage("Financial Projections") {
		t.Fatal("defaults must remain")
	}
}
