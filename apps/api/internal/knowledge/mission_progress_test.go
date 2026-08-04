package knowledge

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge/missions"
)

func TestBuildMissionProgressEmptyCorpus(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing pack")
	}
	out := buildMissionProgress(pack, "template_default", "en", SessionState{}, nil)
	if out.PackID != missions.FinancingDDV1 {
		t.Fatalf("packId=%q", out.PackID)
	}
	if out.Total != len(pack.Items) || out.Covered != 0 {
		t.Fatalf("want covered=0 total=%d, got covered=%d total=%d", len(pack.Items), out.Covered, out.Total)
	}
	for _, it := range out.Items {
		if it.Covered {
			t.Fatalf("empty corpus should not cover %#v", it)
		}
		if it.Prompt == "" {
			t.Fatalf("missing localized prompt for %s", it.ID)
		}
	}
}

func TestBuildMissionProgressPartialCoverage(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing pack")
	}
	out := buildMissionProgress(
		pack,
		"room",
		"en",
		SessionState{
			Entities: []SessionEntity{
				{Name: "valuation cap term", Type: "clause"},
			},
		},
		[]QATurn{{Question: "What is the valuation cap?", Answer: "The valuation cap is $10M."}},
	)
	if out.Covered < 1 {
		t.Fatalf("expected valuation_cap covered, got %#v", out)
	}
	var found bool
	for _, it := range out.Items {
		if it.ID == "valuation_cap" {
			found = true
			if !it.Covered {
				t.Fatalf("valuation_cap should be covered: %#v", it)
			}
		}
	}
	if !found {
		t.Fatal("missing valuation_cap item")
	}
	if out.Covered > out.Total {
		t.Fatalf("covered=%d > total=%d", out.Covered, out.Total)
	}
}

func TestBuildMissionProgressAccumulatesTurnsAndZhPrompts(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing pack")
	}
	var optionPrompt, antiPrompt string
	for _, it := range pack.Items {
		switch it.ID {
		case "option_pool":
			optionPrompt = it.Prompts.ZhCN
		case "anti_dilution":
			antiPrompt = it.Prompts.ZhCN
		}
	}
	if optionPrompt == "" || antiPrompt == "" {
		t.Fatal("missing zh prompts")
	}
	out := buildMissionProgress(
		pack,
		"template_default",
		"zh-CN",
		SessionState{},
		[]QATurn{
			{Question: optionPrompt, Answer: "期权池为 10%。"},
			{Question: antiPrompt, Answer: "加权平均反稀释。"},
		},
	)
	byID := map[string]bool{}
	for _, it := range out.Items {
		byID[it.ID] = it.Covered
	}
	if !byID["option_pool"] {
		t.Fatalf("option_pool should be covered after asking zh prompt: %#v", out.Items)
	}
	if !byID["anti_dilution"] {
		t.Fatalf("anti_dilution should be covered after asking zh prompt: %#v", out.Items)
	}
	// Earlier items must remain covered when later turns are asked.
	if out.Covered < 2 {
		t.Fatalf("want ≥2 covered across turns, got %d", out.Covered)
	}
}

func TestBuildMissionProgressZhCN(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing pack")
	}
	out := buildMissionProgress(pack, "template_default", "zh-CN", SessionState{}, nil)
	if out.Title == "" || out.Title == pack.Title.EN {
		// zh-CN title should prefer Chinese when present.
		if pack.Title.ZhCN != "" && out.Title != pack.Title.ZhCN {
			t.Fatalf("title=%q want %q", out.Title, pack.Title.ZhCN)
		}
	}
}

func TestMissionItemCoveredStrongCJK(t *testing.T) {
	t.Parallel()
	item := missions.Item{
		Keywords: []string{"option", "pool", "ESOP", "期权池"},
		Prompts:  missions.LocalizedString{EN: "How is the pool sized?", ZhCN: "期权池怎么约定？"},
	}
	if !missionItemCovered(item, "本室融资文件里的员工期权池规模如何约定？") {
		t.Fatal("single CJK keyword hit should cover")
	}
	if missionItemCovered(item, "only talking about pool size vaguely") {
		// "pool" alone is weak; without a second hit must not cover.
		t.Fatal("weak EN token alone should not cover")
	}
}
