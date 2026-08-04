package knowledge

import (
	"strings"
	"testing"
)

func TestEstimateCostUnits(t *testing.T) {
	t.Parallel()
	if got := estimateCostUnits("", nil); got != 0 {
		t.Fatalf("empty=%d", got)
	}
	if got := estimateCostUnits("hi", nil); got != 1 {
		t.Fatalf("short answer=%d", got)
	}
	long := strings.Repeat("x", 2500)
	if got := estimateCostUnits(long, nil); got != 3 {
		t.Fatalf("long=%d", got)
	}
	if got := estimateCostUnits("a", []QueryHit{{Text: strings.Repeat("b", 999)}}); got != 1 {
		t.Fatalf("under 1k=%d", got)
	}
}
