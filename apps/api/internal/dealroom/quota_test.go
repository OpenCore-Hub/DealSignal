package dealroom

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
)

type stubPlanChecker struct {
	plan.Unrestricted
	roomErr error
	ndaErr  error
}

func (s stubPlanChecker) AssertCanCreateRoom(context.Context, string) error {
	return s.roomErr
}
func (s stubPlanChecker) AssertCanUseNDA(context.Context, string) error { return s.ndaErr }
func (s stubPlanChecker) WithCreateRoomQuota(ctx context.Context, workspaceID string, fn func(context.Context) error) error {
	if err := s.AssertCanCreateRoom(ctx, workspaceID); err != nil {
		return err
	}
	return fn(ctx)
}

func TestCreateRoomPlanLimit(t *testing.T) {
	svc := NewService(nil, nil, testCfg(), WithPlanChecker(stubPlanChecker{roomErr: plan.ErrLimitRooms}))
	_, err := svc.CreateRoom(context.Background(), uuid.NewString(), uuid.NewString(), CreateRoomRequest{
		Slug: "second-room",
		Name: "Second Room",
	})
	if !errors.Is(err, plan.ErrLimitRooms) {
		t.Fatalf("expected ErrLimitRooms, got %v", err)
	}
}

func TestCreateRoomNDAPlanGate(t *testing.T) {
	svc := NewService(nil, nil, testCfg(), WithPlanChecker(stubPlanChecker{ndaErr: plan.ErrFeatureNDA}))
	_, err := svc.CreateRoom(context.Background(), uuid.NewString(), uuid.NewString(), CreateRoomRequest{
		Slug:        "nda-room",
		Name:        "NDA Room",
		RequiresNDA: true,
	})
	if !errors.Is(err, plan.ErrFeatureNDA) {
		t.Fatalf("expected ErrFeatureNDA, got %v", err)
	}
}
