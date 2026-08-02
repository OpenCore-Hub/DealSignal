package assistant

import "testing"

func TestDetectAbsenceSlot(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"有没有竞业限制", true},
		{"是否有竞业限制", true},
		{"文档中有没有清算优先权", true},
		{"是否存在关联交易", true},
		{"Is there a non-compete clause?", true},
		{"Are there any drag-along rights?", true},
		{"Do we have an option pool?", true},
		{"是否可转让", false},
		{"能否不经同意转让股份", false},
		{"有哪些财务指标", false},
		{"财务数据", false},
		{"how is liquidation preference calculated", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := detectAbsenceSlot(tc.msg); got != tc.want {
			t.Fatalf("detectAbsenceSlot(%q)=%v want %v", tc.msg, got, tc.want)
		}
	}
}

func TestPeelAbsenceQuery(t *testing.T) {
	cases := []struct {
		msg     string
		want    string
		wantOK  bool
	}{
		{"有没有竞业限制", "竞业限制", true},
		{"是否有清算优先权？", "清算优先权", true},
		{"文档中有没有关联交易", "关联交易", true},
		{"Is there a non-compete clause?", "non-compete clause", true},
		{"Are there any drag-along rights in the documents?", "drag-along rights", true},
		{"Do we have an option pool?", "option pool", true},
		{"竞业限制", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := peelAbsenceQuery(tc.msg)
		if ok != tc.wantOK {
			t.Fatalf("peelAbsenceQuery(%q) ok=%v want %v (got %q)", tc.msg, ok, tc.wantOK, got)
		}
		if ok && got != tc.want {
			t.Fatalf("peelAbsenceQuery(%q)=%q want %q", tc.msg, got, tc.want)
		}
	}
}

func TestApplyAbsenceSlotOnlyOnQA(t *testing.T) {
	qa := decisionFromIntent(DocIntentQA, "rule", false)
	qa = applyAbsenceSlot(qa, "有没有竞业限制")
	if !qa.Absence {
		t.Fatal("expected Absence on qa")
	}

	list := decisionFromIntent(DocIntentList, "rule", false)
	list = applyAbsenceSlot(list, "有没有竞业限制")
	if list.Absence {
		t.Fatal("absence must not attach to list")
	}

	topic := decisionFromIntent(DocIntentTopic, "rule", false)
	topic = applyAbsenceSlot(topic, "有没有竞业限制")
	if topic.Absence {
		t.Fatal("absence must not attach to topic")
	}
}
