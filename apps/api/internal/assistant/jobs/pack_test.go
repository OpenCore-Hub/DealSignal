package jobs

import (
	"reflect"
	"testing"
)

func TestLoadBuiltinPacks_FinancingDDV1(t *testing.T) {
	reg, err := LoadBuiltinPacks()
	if err != nil {
		t.Fatal(err)
	}
	pack, ok := reg.Get(FinancingDDV1)
	if !ok {
		t.Fatal("missing financing_dd_v1")
	}
	if pack.PackVersion != "1" {
		t.Fatalf("version=%q", pack.PackVersion)
	}
	if len(pack.Items) != 20 {
		t.Fatalf("items=%d", len(pack.Items))
	}
	first := pack.Items[0]
	if first.ID != "cap_table" {
		t.Fatalf("first id=%q", first.ID)
	}
	if first.QueryFor("zh-CN") == "" || first.QueryFor("en") == "" {
		t.Fatal("queries required")
	}
	last := pack.Items[len(pack.Items)-1]
	if last.ID != "revenue_metrics" || last.ValueType != "money" {
		t.Fatalf("last=%+v", last)
	}
}

func TestLoadBuiltinPacks_MARedflagV1(t *testing.T) {
	reg, err := LoadBuiltinPacks()
	if err != nil {
		t.Fatal(err)
	}
	pack, ok := reg.Get(MARedflagV1)
	if !ok {
		t.Fatal("missing ma_redflag_v1")
	}
	if len(pack.Items) != 18 {
		t.Fatalf("items=%d", len(pack.Items))
	}
	if pack.Items[0].ID != "change_of_control" {
		t.Fatalf("first=%q", pack.Items[0].ID)
	}
	if pack.Items[17].ID != "disclosure_schedules" {
		t.Fatalf("last=%q", pack.Items[17].ID)
	}
	if ForkAllowed(MARedflagV1) {
		t.Fatal("ma_redflag must be read-only (no fork)")
	}
	if !ForkAllowed(FinancingDDV1) {
		t.Fatal("financing fork allowed")
	}
	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("list=%d", len(list))
	}
}

// Locked against design §15.1 / D10 / D15 / D18 (item_id + value_type stable).
func TestFinancingDDV1_StableIDsAndValueTypes(t *testing.T) {
	reg, err := LoadBuiltinPacks()
	if err != nil {
		t.Fatal(err)
	}
	pack, _ := reg.Get(FinancingDDV1)
	wantIDs := []string{
		"cap_table", "option_pool", "outstanding_equity", "preferred_rights", "liquidation_preference",
		"anti_dilution", "board_composition", "protective_provisions", "transfer_restrictions", "roa_co_sale",
		"drag_tag", "ip_ownership", "material_contracts", "litigation", "debt_liens",
		"related_party", "key_employees", "nda_confidentiality", "financial_statements", "revenue_metrics",
	}
	wantVT := map[string]string{
		"option_pool":        "percent",
		"outstanding_equity": "share",
		"revenue_metrics":    "money",
	}
	gotIDs := make([]string, len(pack.Items))
	for i, it := range pack.Items {
		gotIDs[i] = it.ID
		want := wantVT[it.ID]
		if it.ValueType != want {
			t.Fatalf("item %s value_type=%q want %q (D18)", it.ID, it.ValueType, want)
		}
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("financing item_id drift\ngot  %v\nwant %v", gotIDs, wantIDs)
	}
}

// Locked against design §15.2 / D13 / D17 / D18.
func TestMARedflagV1_StableIDs(t *testing.T) {
	reg, err := LoadBuiltinPacks()
	if err != nil {
		t.Fatal(err)
	}
	pack, _ := reg.Get(MARedflagV1)
	wantIDs := []string{
		"change_of_control", "mac_mae", "reps_warranty_survival", "indemnity_cap", "closing_conditions",
		"termination_fees", "non_compete", "key_customer_concentration", "key_supplier", "litigation_disputes",
		"regulatory_approvals", "ip_infringement", "data_privacy", "employment_claims", "environmental",
		"debt_change_terms", "related_party_ma", "disclosure_schedules",
	}
	gotIDs := make([]string, len(pack.Items))
	for i, it := range pack.Items {
		gotIDs[i] = it.ID
		if it.ValueType != "" {
			t.Fatalf("ma item %s must not set value_type (got %q)", it.ID, it.ValueType)
		}
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("ma item_id drift\ngot  %v\nwant %v", gotIDs, wantIDs)
	}
}

func TestBuiltinPacks_KeywordQueriesNotQuestions(t *testing.T) {
	reg, err := LoadBuiltinPacks()
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range reg.List() {
		for _, it := range pack.Items {
			for _, q := range []string{it.QueryEN, it.QueryZH} {
				if q == "" {
					continue
				}
				if containsQuestionMark(q) {
					t.Fatalf("%s/%s query looks like a question: %q", pack.PackID, it.ID, q)
				}
			}
		}
	}
}

func containsQuestionMark(s string) bool {
	for _, r := range s {
		if r == '?' || r == '？' {
			return true
		}
	}
	return false
}
