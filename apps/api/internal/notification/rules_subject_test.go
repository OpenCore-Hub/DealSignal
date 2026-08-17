package notification

import "testing"

func TestNotificationSubjectKeyPage(t *testing.T) {
	got := notificationSubject(Event{
		EventType: "key_page",
		Metadata:  map[string]string{"page_title": "财务模型"},
	})
	if got != "[key_page] Sensitive page viewed: 财务模型" {
		t.Fatalf("got %q", got)
	}
	got = notificationSubject(Event{
		EventType: "key_page",
		Metadata: map[string]string{
			"page_title":     "财务模型",
			"document_title": "Memo.pdf",
		},
	})
	if got != "[key_page] Sensitive page viewed: 财务模型 · Memo.pdf" {
		t.Fatalf("bundle subject got %q", got)
	}
	got = notificationSubject(Event{EventType: "repeat_key_page"})
	if got != "[repeat_key_page] Sensitive page revisited" {
		t.Fatalf("got %q", got)
	}
	got = notificationSubject(Event{
		EventType: "key_page",
		Metadata:  map[string]string{"page_title": `{"parameters": {"window": 5}}`, "page_number": "27"},
	})
	if got != "[key_page] Sensitive page viewed" {
		t.Fatalf("json title must not enter subject, got %q", got)
	}
}
