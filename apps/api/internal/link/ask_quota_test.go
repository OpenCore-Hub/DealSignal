package link

import "testing"

func TestAskAIQuotaExceededView(t *testing.T) {
	if askAIQuotaExceededView(AskAIQuotaView{Used: 0, Limit: 500}) {
		t.Fatal("expected not exceeded at 0/500")
	}
	if !askAIQuotaExceededView(AskAIQuotaView{Used: 500, Limit: 500}) {
		t.Fatal("expected exceeded at 500/500")
	}
	if askAIQuotaExceededView(AskAIQuotaView{Used: 10, Limit: 0}) {
		t.Fatal("zero limit should not mark exceeded")
	}
}
