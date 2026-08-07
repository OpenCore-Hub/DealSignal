package link

import "testing"

func TestQaEnabledForLink(t *testing.T) {
	if QaEnabledForLink(false) {
		t.Fatal("document share links should not enable visitor Q&A")
	}
	if !QaEnabledForLink(true) {
		t.Fatal("deal-room share links should enable visitor Q&A")
	}
}
