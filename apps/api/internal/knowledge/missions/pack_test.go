package missions

import "testing"

func TestBuiltinPacksLoad(t *testing.T) {
	t.Parallel()
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(catalogOrder) {
		t.Fatalf("want %d packs, got %d", len(catalogOrder), len(list))
	}
	for i, id := range catalogOrder {
		if list[i].ID != id {
			t.Fatalf("catalog order[%d]=%s want %s", i, list[i].ID, id)
		}
		p, ok := Get(id)
		if !ok || len(p.Items) < 3 {
			t.Fatalf("pack %s %#v", id, p)
		}
		if p.Title.For("en") == "" || p.Title.For("zh-CN") == "" {
			t.Fatalf("pack %s missing title locale", id)
		}
		for _, item := range p.Items {
			if item.Prompts.For("en") == "" || item.Prompts.For("zh-CN") == "" {
				t.Fatalf("pack %s item %s missing prompts", id, item.ID)
			}
			if len(item.Keywords) == 0 {
				t.Fatalf("pack %s item %s missing keywords", id, item.ID)
			}
		}
	}
}

func TestDefaultForRoomTemplate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"tmpl_ma_acquisition", MARedFlagV1},
		{"ma-acquisition", MARedFlagV1},
		{"startup-fundraising", FinancingDDV1},
		{"tmpl_startup_fundraising", FinancingDDV1},
		{"tmpl_raising_first_fund", FirstFundV1},
		{"raising-first-fund", FirstFundV1},
		{"tmpl_series_a_plus", SeriesAPlusV1},
		{"series-a-plus", SeriesAPlusV1},
		{"tmpl_real_estate_transaction", RealEstateV1},
		{"real-estate-transaction", RealEstateV1},
		{"tmpl_fund_management", FundMgmtV1},
		{"fund-management", FundMgmtV1},
		{"tmpl_portfolio_management", PortfolioMgmtV1},
		{"portfolio-management", PortfolioMgmtV1},
		{"tmpl_project_management", ProjectMgmtV1},
		{"project-management", ProjectMgmtV1},
		{"tmpl_sales_dataroom", SalesDataroomV1},
		{"sales-dataroom", SalesDataroomV1},
		{"", FinancingDDV1},
	}
	for _, tc := range cases {
		if got := DefaultForRoomTemplate(tc.in); got != tc.want {
			t.Fatalf("%q → %s want %s", tc.in, got, tc.want)
		}
	}
}
