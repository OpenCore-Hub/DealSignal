package coverage

import (
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
)

func TestRunCalibration_FlagsFalseSupported(t *testing.T) {
	in := CalibrateInput{
		Rooms: []CalibrateRoomSnapshot{
			{
				RoomID: "r1",
				QueryByItemID: map[string]string{
					"cap_table":   "capitalization table cap table share ownership",
					"option_pool": "option pool ESOP",
					"litigation":  "litigation dispute",
				},
				Rows: []CoverageRow{
					{
						ItemID: "cap_table",
						Status: StatusSupported,
						Clues:  []search.Evidence{{Quote: "capitalization table and share ownership", Score: 1.0}},
					},
					{
						ItemID: "option_pool",
						Status: StatusSupported,
						Clues:  []search.Evidence{{Quote: "unrelated weather report", Score: 0.4}},
					},
					{
						ItemID: "litigation",
						Status: StatusAbsentInScope,
					},
				},
			},
			{RoomID: "r2", QueryByItemID: map[string]string{"x": "x"}, Rows: []CoverageRow{
				{ItemID: "x", Status: StatusSupported, Clues: []search.Evidence{{Quote: "x", Score: 0.9}}},
			}},
			{RoomID: "r3", QueryByItemID: map[string]string{"y": "y"}, Rows: []CoverageRow{
				{ItemID: "y", Status: StatusSupported, Clues: []search.Evidence{{Quote: "y", Score: 0.8}}},
			}},
		},
		FalseSupported: []CalibrateLabel{{RoomID: "r1", ItemID: "option_pool", Note: "noise hit"}},
	}
	opts := Options{
		BoundaryScoreLow:   0.35,
		BoundaryScoreHigh:  0.75,
		BoundaryMinJaccard: 0.5,
	}
	rep, err := RunCalibration(in, opts)
	if err != nil {
		t.Fatal(err)
	}
	if rep.RoomCount != 3 {
		t.Fatalf("rooms=%d", rep.RoomCount)
	}
	if rep.FalseSupportedCount != 1 {
		t.Fatalf("false=%d", rep.FalseSupportedCount)
	}
	if rep.FalseCaughtByDefaults != 1 {
		t.Fatalf("caught=%d want 1 (weak jaccard)", rep.FalseCaughtByDefaults)
	}
	md := FormatCalibrateReportMarkdown(rep)
	if !strings.Contains(md, "D16") || !strings.Contains(md, "Suggestion") {
		t.Fatalf("markdown missing sections: %s", md)
	}
}

func TestRunCalibration_RequiresQuery(t *testing.T) {
	_, err := RunCalibration(CalibrateInput{
		Rooms: []CalibrateRoomSnapshot{{
			RoomID: "r1",
			Rows: []CoverageRow{{
				ItemID: "cap_table",
				Status: StatusSupported,
				Clues:  []search.Evidence{{Quote: "x", Score: 1}},
			}},
		}},
	}, Options{})
	if err == nil {
		t.Fatal("expected missing query error")
	}
}
