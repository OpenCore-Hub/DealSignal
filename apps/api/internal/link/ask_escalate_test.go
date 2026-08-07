package link

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/visitorask"
)

type mockAskSecurityRecorder struct {
	eventTypes []string
	reasons    []string
}

func (m *mockAskSecurityRecorder) RecordSecurityEvent(
	_ context.Context,
	_ db.Link,
	eventType, _, _, _, _, reason string,
) error {
	m.eventTypes = append(m.eventTypes, eventType)
	m.reasons = append(m.reasons, reason)
	return nil
}

func TestRecordAskEscalated(t *testing.T) {
	rec := &mockAskSecurityRecorder{}
	svc := &Service{askSecurity: rec}
	svc.recordAskEscalated(context.Background(), db.Link{}, "visitor-1", "a@example.com", routeReasonUserEscalate)
	if len(rec.eventTypes) != 1 || rec.eventTypes[0] != visitorask.EventTypeAskEscalated {
		t.Fatalf("event types = %v", rec.eventTypes)
	}
	if len(rec.reasons) != 1 || rec.reasons[0] != routeReasonUserEscalate {
		t.Fatalf("reasons = %v", rec.reasons)
	}
}

func TestRecordAskEscalated_NoRecorder(t *testing.T) {
	svc := &Service{}
	svc.recordAskEscalated(context.Background(), db.Link{}, "visitor-1", "", routeReasonLowConfidence)
}

func TestMaybeAutoEscalateSupervisedRefuse_SkipsSelfServe(t *testing.T) {
	rec := &mockAskSecurityRecorder{}
	svc := &Service{askSecurity: rec}
	link := db.Link{AskMode: AskModeSelfServe}
	turn := db.LinkAskTurn{Status: askStatusAIRefused}
	svc.maybeAutoEscalateSupervisedRefuse(context.Background(), link, "visitor-1", turn)
	if len(rec.eventTypes) != 0 {
		t.Fatalf("expected no security events, got %v", rec.eventTypes)
	}
}

func TestMaybeAutoEscalateSupervisedRefuse_SkipsAnsweredTurn(t *testing.T) {
	rec := &mockAskSecurityRecorder{}
	svc := &Service{askSecurity: rec}
	link := db.Link{AskMode: AskModeSupervised}
	turn := db.LinkAskTurn{Status: askStatusAIAnswered}
	svc.maybeAutoEscalateSupervisedRefuse(context.Background(), link, "visitor-1", turn)
	if len(rec.eventTypes) != 0 {
		t.Fatalf("expected no security events, got %v", rec.eventTypes)
	}
}
