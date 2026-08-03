package knowledge

import (
	"errors"
	"net"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/docling"
)

func TestGetCorpusDisabledWhenRAGNotConfigured(t *testing.T) {
	svc := NewService(nil, config.DoclingRAGConfig{}, nil, nil, "test-secret")
	status, err := svc.GetCorpus(t.Context(), "room", "ws", "user")
	if err != nil {
		t.Fatalf("GetCorpus: %v", err)
	}
	if status.Enabled {
		t.Fatal("expected enabled=false")
	}
	if status.Status != "none" {
		t.Fatalf("status = %q, want none", status.Status)
	}
	if status.Documents == nil {
		t.Fatal("documents should be empty slice, not nil")
	}
}

func TestEnqueueRoomSyncUnavailable(t *testing.T) {
	svc := NewService(nil, config.DoclingRAGConfig{}, nil, nil, "test-secret")
	err := svc.EnqueueRoomSync(t.Context(), "room", "ws", "user")
	if err != ErrUnavailable {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestMapUpstreamConnectionRefused(t *testing.T) {
	err := mapUpstream(&net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestMapUpstreamAPIError(t *testing.T) {
	err := mapUpstream(&docling.APIError{Status: 503, Code: "INDEX_NOT_READY"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestReconcileCorpusStatusHealsStuckProvisioning(t *testing.T) {
	got := reconcileCorpusStatus("provisioning", SyncProgress{
		Total: 2, Pending: 0, Syncing: 0, Synced: 2, Failed: 0,
	})
	if got != "ready" {
		t.Fatalf("got %q, want ready", got)
	}
	got = reconcileCorpusStatus("syncing", SyncProgress{
		Total: 2, Pending: 0, Syncing: 0, Synced: 1, Failed: 1,
	})
	if got != "degraded" {
		t.Fatalf("got %q, want degraded", got)
	}
	got = reconcileCorpusStatus("provisioning", SyncProgress{
		Total: 2, Pending: 1, Syncing: 0, Synced: 1, Failed: 0,
	})
	if got != "provisioning" {
		t.Fatalf("got %q, want provisioning while pending remains", got)
	}
}
