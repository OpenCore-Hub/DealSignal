package link

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestResolveAskRouteReason(t *testing.T) {
	base := AskPolicy{Mode: AskModeSupervised, AIEnabled: false}

	if got := resolveAskRouteReason(base, true); got != routeReasonUserEscalate {
		t.Fatalf("escalate = %q", got)
	}
	if got := resolveAskRouteReason(base, false); got != routeReasonAINotEnabled {
		t.Fatalf("ai disabled = %q", got)
	}

	aiOn := AskPolicy{Mode: AskModeSupervised, AIEnabled: true}
	if got := resolveAskRouteReason(aiOn, false); got != routeReasonAILanePending {
		t.Fatalf("ai enabled = %q", got)
	}

	formal := AskPolicy{Mode: AskModeFormal, AIEnabled: true}
	if got := resolveAskRouteReason(formal, false); got != routeReasonPolicyFormal {
		t.Fatalf("formal = %q", got)
	}
}

func TestValidateAskMode(t *testing.T) {
	for _, mode := range []string{AskModeSelfServe, AskModeSupervised, AskModeFormal} {
		if err := validateAskMode(mode); err != nil {
			t.Fatalf("mode %q: %v", mode, err)
		}
	}
	if err := validateAskMode("invalid"); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestLoadAskPolicyDefaults(t *testing.T) {
	p := loadAskPolicy(db.Link{AskMode: AskModeSupervised})
	if p.Mode != AskModeSupervised || p.AIEnabled {
		t.Fatalf("unexpected default policy: %+v", p)
	}
	q := int32(100)
	p = loadAskPolicy(db.Link{
		AskMode:            AskModeSelfServe,
		AskAiEnabled:       true,
		AskAiMonthlyQuota:  pgtype.Int4{Int32: q, Valid: true},
	})
	if !p.AIEnabled || p.AIMonthlyQuota == nil || *p.AIMonthlyQuota != 100 {
		t.Fatalf("unexpected loaded policy: %+v", p)
	}
}

func TestEffectiveAskAIQuota(t *testing.T) {
	link := db.Link{}
	if got := effectiveAskAIQuota(link, 500); got != 500 {
		t.Fatalf("default = %d", got)
	}
	link.AskAiMonthlyQuota = pgtype.Int4{Int32: 42, Valid: true}
	if got := effectiveAskAIQuota(link, 500); got != 42 {
		t.Fatalf("override = %d", got)
	}
}
