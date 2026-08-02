package coverage

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
	"github.com/google/uuid"
)

func TestValidateForkItems(t *testing.T) {
	err := ValidateForkItems(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	items := []jobs.PackItem{{
		ID: "custom_pool", LabelEN: "Custom pool", QueryEN: "option pool", ValueType: "percent",
	}}
	if err := ValidateForkItems(items); err != nil {
		t.Fatal(err)
	}
	bad := []jobs.PackItem{{
		ID: "Bad-ID", LabelEN: "x", QueryEN: "y",
	}}
	if err := ValidateForkItems(bad); err == nil {
		t.Fatal("expected invalid id")
	}
}

func TestPutPack_AndScanUsesFork(t *testing.T) {
	f := newFakeCoverageDB()
	q := &memQueue{}
	svc := NewService(f, &stubSearcher{}, jobs.MustLoadBuiltinPacks(), q, Options{Enabled: true})
	userID := uuid.NewString()
	ws := uuid.UUID(f.room.WorkspaceID.Bytes).String()
	room := uuid.UUID(f.room.ID.Bytes).String()

	view, err := svc.PutPack(context.Background(), ws, room, userID, PutPackRequest{
		Items: []jobs.PackItem{{
			ID: "custom_metric", LabelEN: "Custom metric", QueryEN: "custom metric ARR", ValueType: "money",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !view.Forked || view.PackVersion != "1.f1" || len(view.Items) != 1 {
		t.Fatalf("%+v", view)
	}

	run, err := svc.StartScan(context.Background(), ws, room, userID, StartScanRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if run.PackVersion != "1.f1" {
		t.Fatalf("run version=%s", run.PackVersion)
	}
	if len(q.jobs) != 1 || len(q.jobs[0].PackItems) != 1 || q.jobs[0].PackItems[0].ID != "custom_metric" {
		t.Fatalf("job=%+v", q.jobs[0])
	}

	reset, err := svc.ResetPack(context.Background(), ws, room, userID)
	if err != nil {
		t.Fatal(err)
	}
	if reset.Forked || len(reset.Items) != 20 {
		t.Fatalf("%+v", reset)
	}
}
