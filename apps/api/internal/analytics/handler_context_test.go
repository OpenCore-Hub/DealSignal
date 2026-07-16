package analytics

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

func TestSignalFeedListIncludesContext(t *testing.T) {
	s := makeSignal("ctx", "hot_signal")
	s.Context = []byte(`{"opens":5,"keyPageCount":2,"keyPageTitles":["Financials","Team"]}`)
	feed := signalFeedList([]db.Signal{s})
	if len(feed) != 1 {
		t.Fatalf("expected 1 feed item, got %d", len(feed))
	}
	ctx, ok := feed[0]["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context map, got %T", feed[0]["context"])
	}
	if ctx["opens"] != float64(5) {
		t.Errorf("expected opens 5, got %v", ctx["opens"])
	}
}

func TestSignalFeedListIgnoresEmptyContext(t *testing.T) {
	s := makeSignal("empty", "hot_signal")
	s.Context = []byte("{}")
	feed := signalFeedList([]db.Signal{s})
	if _, ok := feed[0]["context"]; ok {
		t.Error("expected empty context to be omitted")
	}
}

func TestRiskAlertListIncludesMetadata(t *testing.T) {
	s := makeRiskSignal("meta", "forward")
	s.Metadata = []byte(`{"distinct_ips":"5"}`)
	alerts := riskAlertList([]db.Signal{s})
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	md, ok := alerts[0]["metadata"].(map[string]string)
	if !ok {
		t.Fatalf("expected metadata map, got %T", alerts[0]["metadata"])
	}
	if md["distinct_ips"] != "5" {
		t.Errorf("expected distinct_ips 5, got %v", md["distinct_ips"])
	}
}
