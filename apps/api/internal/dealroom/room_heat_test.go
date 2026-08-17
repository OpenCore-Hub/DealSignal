package dealroom

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestRoomHeatFeedsEngagedKeyPages(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "room_heat.go"))
	if err != nil {
		t.Fatalf("read room_heat.go: %v", err)
	}
	src := string(raw)
	if !strings.Contains(src, "r.EngagedKeyPageViews") {
		t.Fatal("room-card heat must score engaged key pages")
	}
	if strings.Contains(src, "r.TotalKeyPageViews") {
		t.Fatal("room-card heat must not score skim key-page hits")
	}
}

func TestMaxRoomHeatUsesHottestShare(t *testing.T) {
	roomA := pgUUID(uuid.NewString())
	roomB := pgUUID(uuid.NewString())
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	hot := roomLinkHeatMetrics{
		DealRoomID:         roomA,
		LinkID:             pgUUID(uuid.NewString()),
		Opens:              8,
		UniqueVisitors:     3,
		AvgDurationSeconds: 180,
		LastAccessAt:       now,
	}
	cold := roomLinkHeatMetrics{
		DealRoomID:     roomA,
		LinkID:         pgUUID(uuid.NewString()),
		Opens:          1,
		UniqueVisitors: 1,
		LastAccessAt:   now,
	}
	other := roomLinkHeatMetrics{
		DealRoomID:     roomB,
		LinkID:         pgUUID(uuid.NewString()),
		Opens:          2,
		UniqueVisitors: 1,
		LastAccessAt:   now,
	}

	got := maxRoomHeatByRoom([]roomLinkHeatMetrics{cold, hot, other}, nil)
	if got[uuid.UUID(roomA.Bytes).String()] <= got[uuid.UUID(roomB.Bytes).String()] {
		t.Fatalf("room A should take the hotter share: %#v", got)
	}
	if got[uuid.UUID(roomA.Bytes).String()] != int32(computeRoomLinkHeat(hot, 0).Score) {
		t.Fatalf("room A heat = %d, want hottest share %d", got[uuid.UUID(roomA.Bytes).String()], computeRoomLinkHeat(hot, 0).Score)
	}
}

func TestComputeRoomLinkHeatUsesKeyPagesAndDecay(t *testing.T) {
	fresh := roomLinkHeatMetrics{
		Opens:              4,
		UniqueVisitors:     2,
		AvgDurationSeconds: 120,
		LastAccessAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	aged := fresh
	aged.LastAccessAt = pgtype.Timestamptz{Time: time.Now().Add(-14 * 24 * time.Hour), Valid: true}

	freshScore := computeRoomLinkHeat(fresh, 3).Score
	agedScore := computeRoomLinkHeat(aged, 3).Score
	noKeyScore := computeRoomLinkHeat(fresh, 0).Score
	if agedScore >= freshScore {
		t.Fatalf("decay should lower score: fresh=%d aged=%d", freshScore, agedScore)
	}
	if noKeyScore >= freshScore {
		t.Fatalf("key pages should raise score: with=%d without=%d", freshScore, noKeyScore)
	}
}
