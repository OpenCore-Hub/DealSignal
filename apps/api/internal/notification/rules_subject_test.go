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
	got = notificationSubject(Event{EventType: "repeat_key_page"})
	if got != "[repeat_key_page] Sensitive page revisited" {
		t.Fatalf("got %q", got)
	}
}
