package analytics

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func makeSignal(id, typ string) db.Signal {
	return db.Signal{
		ID:          pgtype.UUID{Bytes: uuidBytes(id), Valid: true},
		Type:        typ,
		Title:       "title " + id,
		Description: "desc " + id,
		Explanation: "explanation " + id,
		Suggestion:  "suggestion " + id,
		Priority:    "medium",
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

func makeRiskSignal(id, subtype string) db.Signal {
	s := makeSignal(id, "risk_alert")
	s.Subtype = pgtype.Text{String: subtype, Valid: true}
	return s
}

func uuidBytes(s string) [16]byte {
	var b [16]byte
	copy(b[:], s)
	return b
}

func TestHeatAlertList(t *testing.T) {
	signals := []db.Signal{
		makeSignal("a", "hot_signal"),
		makeSignal("b", "risk_alert"),
		makeSignal("c", "follow_up"),
	}
	alerts := heatAlertList(signals)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 heat alert, got %d", len(alerts))
	}
	if alerts[0]["heatLevel"] != "hot_signal" {
		t.Errorf("expected heatLevel hot_signal, got %v", alerts[0]["heatLevel"])
	}
}

func TestRiskAlertList(t *testing.T) {
	signals := []db.Signal{
		makeSignal("a", "hot_signal"),
		makeSignal("b", "risk_alert"),
		makeSignal("c", "follow_up"),
	}
	alerts := riskAlertList(signals)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 risk alert, got %d", len(alerts))
	}
	if alerts[0]["type"] != "forward" {
		t.Errorf("expected default risk type forward, got %v", alerts[0]["type"])
	}

	download := makeRiskSignal("d", "download")
	expired := makeRiskSignal("e", "expired")
	alerts = riskAlertList([]db.Signal{download, expired})
	if len(alerts) != 2 {
		t.Fatalf("expected 2 risk alerts, got %d", len(alerts))
	}
	if alerts[0]["type"] != "download" {
		t.Errorf("expected download type, got %v", alerts[0]["type"])
	}
	if alerts[1]["type"] != "expired" {
		t.Errorf("expected expired type, got %v", alerts[1]["type"])
	}

	anomaly := makeRiskSignal("f", "anomaly")
	alerts = riskAlertList([]db.Signal{anomaly})
	if len(alerts) != 1 || alerts[0]["type"] != "anomaly" {
		t.Errorf("expected anomaly type, got %v", alerts[0]["type"])
	}
}

func TestPublicURLOmitsLocalhost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/insights/overview", nil)
	c.Request.Header.Set("Origin", "http://localhost:5173")

	got := publicURL(c, &config.Config{}, "tok123")
	if got != "/l/tok123" {
		t.Fatalf("expected relative /l/tok123 for localhost origin, got %q", got)
	}

	got = publicURL(c, &config.Config{ViewerBaseURL: "https://view.dealsignal.app"}, "tok123")
	if got != "https://view.dealsignal.app/l/tok123" {
		t.Fatalf("expected production viewer URL, got %q", got)
	}
}
