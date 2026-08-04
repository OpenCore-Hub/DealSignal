package knowledge

import (
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge/missions"
)

func TestBuildMissionFollowUpsFromOpenQuestionsAndPack(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing pack")
	}
	items := buildMissionFollowUps(
		SessionState{
			OpenQuestions: []SessionOpenQuestion{
				{Text: "What is the valuation cap?", SourceTurnID: "t1"},
			},
		},
		QATurn{Question: "cap?", ResultStatus: "no_hits"},
		&pack,
		"en",
	)
	if len(items) < 2 {
		t.Fatalf("want open + pack chips, got %#v", items)
	}
	if items[0].Text != "What is the valuation cap?" {
		t.Fatalf("open question first: %#v", items[0])
	}
	// valuation_cap item should be skipped (keywords already in open question / corpus).
	for _, it := range items {
		if it.ID == "mission-valuation_cap" {
			t.Fatalf("covered pack item should be skipped: %#v", it)
		}
	}
}

func TestBuildMissionFollowUpsUnresolved(t *testing.T) {
	t.Parallel()
	// Industry trivia must never become a mission chip.
	if items := buildMissionFollowUps(
		SessionState{},
		QATurn{Unresolved: []string{"EBITDA multiple is typically 12x in this sector."}},
		nil,
		"en",
	); len(items) != 0 {
		t.Fatalf("out-of-room trivia must not become chips: %#v", items)
	}
	items := buildMissionFollowUps(
		SessionState{},
		QATurn{
			Unresolved: []string{"The exclusivity period under the letter of intent is ninety days."},
		},
		nil,
		"en",
	)
	if len(items) != 1 || items[0].ID != "mission-unresolved-1" {
		t.Fatalf("got %#v", items)
	}
	if !strings.Contains(items[0].Text, "Verify in this room") {
		t.Fatalf("expected verify wrapper, got %q", items[0].Text)
	}
}

func TestBuildMissionFollowUpsDropsBrokenOpenQuestions(t *testing.T) {
	t.Parallel()
	items := buildMissionFollowUps(
		SessionState{
			OpenQuestions: []SessionOpenQuestion{
				{Text: `docx"的上下文），需注意的内容及对应风险如下：### 一、需注意的内容 1.`, SourceTurnID: "t1"},
				{Text: "该模式仅约束接收方保守披露方的信息，对接收方明显不利。", SourceTurnID: "t1"},
			},
		},
		QATurn{ResultStatus: "answered"},
		nil,
		"zh-CN",
	)
	if len(items) != 1 {
		t.Fatalf("want only the complete gap, got %#v", items)
	}
	if !strings.Contains(items[0].Text, "接收方明显不利") {
		t.Fatalf("got %#v", items[0])
	}
}

func TestBuildMissionFollowUpsDropsSoftRefusalMeta(t *testing.T) {
	t.Parallel()
	items := buildMissionFollowUps(
		SessionState{
			OpenQuestions: []SessionOpenQuestion{
				{Text: "因此，无法根据现有上下文回答该问题。", SourceTurnID: "t1"},
			},
		},
		QATurn{
			ResultStatus: "answered",
			Unresolved:   []string{"因此，无法根据现有上下文回答该问题。"},
		},
		nil,
		"zh-CN",
	)
	if len(items) != 0 {
		t.Fatalf("meta refusal must not become chips: %#v", items)
	}
}

func TestMissionItemCovered(t *testing.T) {
	t.Parallel()
	item := missions.Item{Keywords: []string{"valuation", "cap"}}
	if !missionItemCovered(item, "what is the valuation cap in safe") {
		t.Fatal("should cover")
	}
	if missionItemCovered(item, "board observer rights only") {
		t.Fatal("should not cover")
	}
	zh := missions.Item{
		Keywords: []string{"anti-dilution", "antidilution", "反稀释"},
		Prompts: missions.LocalizedString{
			EN:   "Which anti-dilution protection is stated in this room’s financing docs?",
			ZhCN: "本室融资文件约定了哪种反稀释保护？",
		},
	}
	if !missionItemCovered(zh, "本室融资文件约定了哪种反稀释保护？加权平均。") {
		t.Fatal("zh checklist prompt ask should cover")
	}
}
