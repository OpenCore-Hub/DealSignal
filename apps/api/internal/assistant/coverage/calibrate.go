package coverage

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// CalibrateLabel identifies a human-judged false-supported row (D16).
type CalibrateLabel struct {
	RoomID string `json:"room_id"`
	ItemID string `json:"item_id"`
	Note   string `json:"note,omitempty"`
}

// CalibrateRoomSnapshot is one room's coverage snapshot export for offline calibration.
type CalibrateRoomSnapshot struct {
	RoomID        string            `json:"room_id"`
	PackID        string            `json:"pack_id,omitempty"`
	Lang          string            `json:"lang,omitempty"`
	Rows          []CoverageRow     `json:"rows"`
	QueryByItemID map[string]string `json:"query_by_item_id"`
}

// CalibrateInput is the offline D16 calibration payload (≥3 rooms recommended).
type CalibrateInput struct {
	Rooms          []CalibrateRoomSnapshot `json:"rooms"`
	FalseSupported []CalibrateLabel        `json:"false_supported"`
}

// CalibrateSample is one supported row's relative score and Jaccard vs Pack query.
type CalibrateSample struct {
	RoomID         string  `json:"room_id"`
	ItemID         string  `json:"item_id"`
	RelScore       float64 `json:"rel_score"`
	Jaccard        float64 `json:"jaccard"`
	WouldBoundary  bool    `json:"would_boundary"`
	FalseSupported bool    `json:"false_supported"`
}

// CalibrateReport summarizes distributions for D16 threshold review.
type CalibrateReport struct {
	RoomCount             int               `json:"room_count"`
	SupportedCount        int               `json:"supported_count"`
	FalseSupportedCount   int               `json:"false_supported_count"`
	Defaults              Options           `json:"defaults"`
	AllRelScore           Distribution      `json:"all_rel_score"`
	AllJaccard            Distribution      `json:"all_jaccard"`
	FalseRelScore         Distribution      `json:"false_rel_score"`
	FalseJaccard          Distribution      `json:"false_jaccard"`
	FalseCaughtByDefaults int               `json:"false_caught_by_defaults"`
	Suggestion            string            `json:"suggestion"`
	Samples               []CalibrateSample `json:"samples,omitempty"`
}

// Distribution holds simple percentile stats.
type Distribution struct {
	N    int     `json:"n"`
	Min  float64 `json:"min"`
	P50  float64 `json:"p50"`
	P90  float64 `json:"p90"`
	Max  float64 `json:"max"`
	Mean float64 `json:"mean"`
}

// RunCalibration computes D16 offline stats. Mechanism unchanged; only suggests §7.2 default tweaks.
func RunCalibration(in CalibrateInput, opts Options) (CalibrateReport, error) {
	if len(in.Rooms) == 0 {
		return CalibrateReport{}, fmt.Errorf("calibrate: rooms required")
	}
	if opts.BoundaryScoreLow <= 0 {
		opts.BoundaryScoreLow = defaultBoundaryScoreLow
	}
	if opts.BoundaryScoreHigh <= 0 {
		opts.BoundaryScoreHigh = defaultBoundaryScoreHigh
	}
	if opts.BoundaryMinJaccard <= 0 {
		opts.BoundaryMinJaccard = defaultBoundaryMinJaccard
	}

	falseSet := make(map[string]struct{}, len(in.FalseSupported))
	for _, f := range in.FalseSupported {
		falseSet[calibrateKey(f.RoomID, f.ItemID)] = struct{}{}
	}

	var samples []CalibrateSample
	var allRel, allJac, falseRel, falseJac []float64
	caught := 0

	for _, room := range in.Rooms {
		globalMax := scanGlobalMaxScore(room.Rows)
		for _, row := range room.Rows {
			if row.Status != StatusSupported || len(row.Clues) == 0 {
				continue
			}
			query := ""
			if room.QueryByItemID != nil {
				query = room.QueryByItemID[row.ItemID]
			}
			if strings.TrimSpace(query) == "" {
				return CalibrateReport{}, fmt.Errorf("calibrate: room %q item %q missing query_by_item_id", room.RoomID, row.ItemID)
			}
			top := topClueScore(row)
			rel := 0.0
			if globalMax > 0 {
				rel = top / globalMax
			}
			quote := topClueQuote(row, top)
			jac := tokenJaccard(query, quote)
			boundary := isBoundaryRow(row, query, globalMax, opts)
			fs := false
			if _, ok := falseSet[calibrateKey(room.RoomID, row.ItemID)]; ok {
				fs = true
			}
			s := CalibrateSample{
				RoomID:         room.RoomID,
				ItemID:         row.ItemID,
				RelScore:       rel,
				Jaccard:        jac,
				WouldBoundary:  boundary,
				FalseSupported: fs,
			}
			samples = append(samples, s)
			allRel = append(allRel, rel)
			allJac = append(allJac, jac)
			if fs {
				falseRel = append(falseRel, rel)
				falseJac = append(falseJac, jac)
				if boundary {
					caught++
				}
			}
		}
	}

	rep := CalibrateReport{
		RoomCount:             len(in.Rooms),
		SupportedCount:        len(samples),
		FalseSupportedCount:   len(falseRel),
		Defaults:              opts,
		AllRelScore:           summarizeDist(allRel),
		AllJaccard:            summarizeDist(allJac),
		FalseRelScore:         summarizeDist(falseRel),
		FalseJaccard:          summarizeDist(falseJac),
		FalseCaughtByDefaults: caught,
		Samples:               samples,
	}
	rep.Suggestion = calibrateSuggestion(rep)
	return rep, nil
}

// FormatCalibrateReportMarkdown renders a one-page D16 report.
func FormatCalibrateReportMarkdown(rep CalibrateReport) string {
	var b strings.Builder
	b.WriteString("# Ask Docs boundary calibration (D16)\n\n")
	fmt.Fprintf(&b, "- Rooms: **%d** (design recommends ≥3 financing rooms)\n", rep.RoomCount)
	fmt.Fprintf(&b, "- Supported rows: **%d**; human false-supported: **%d**\n", rep.SupportedCount, rep.FalseSupportedCount)
	fmt.Fprintf(&b, "- Defaults under test: score ∈ [%.2f, %.2f]×globalMax OR Jaccard < %.2f\n",
		rep.Defaults.BoundaryScoreLow, rep.Defaults.BoundaryScoreHigh, rep.Defaults.BoundaryMinJaccard)
	fmt.Fprintf(&b, "- False-supported caught by defaults: **%d** / %d\n\n", rep.FalseCaughtByDefaults, rep.FalseSupportedCount)
	b.WriteString("## Distributions\n\n")
	b.WriteString("| Cohort | Metric | n | min | p50 | p90 | max | mean |\n")
	b.WriteString("| ------ | ------ | - | --- | --- | --- | --- | ---- |\n")
	writeDistRow(&b, "all supported", "rel_score", rep.AllRelScore)
	writeDistRow(&b, "all supported", "jaccard", rep.AllJaccard)
	writeDistRow(&b, "false supported", "rel_score", rep.FalseRelScore)
	writeDistRow(&b, "false supported", "jaccard", rep.FalseJaccard)
	b.WriteString("\n## Suggestion\n\n")
	b.WriteString(rep.Suggestion)
	b.WriteString("\n")
	return b.String()
}

// ParseCalibrateInputJSON decodes a calibration payload.
func ParseCalibrateInputJSON(raw []byte) (CalibrateInput, error) {
	var in CalibrateInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return CalibrateInput{}, err
	}
	return in, nil
}

func calibrateKey(roomID, itemID string) string {
	return strings.TrimSpace(roomID) + "\x00" + strings.TrimSpace(itemID)
}

func topClueQuote(row CoverageRow, top float64) string {
	quote := ""
	for _, c := range row.Clues {
		if c.Score == top || quote == "" {
			quote = c.Quote
			if c.Score == top {
				break
			}
		}
	}
	return quote
}

func summarizeDist(vals []float64) Distribution {
	if len(vals) == 0 {
		return Distribution{}
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	return Distribution{
		N:    len(sorted),
		Min:  sorted[0],
		P50:  percentile(sorted, 0.50),
		P90:  percentile(sorted, 0.90),
		Max:  sorted[len(sorted)-1],
		Mean: sum / float64(len(sorted)),
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := p * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	w := idx - float64(lo)
	return sorted[lo]*(1-w) + sorted[hi]*w
}

func calibrateSuggestion(rep CalibrateReport) string {
	if rep.RoomCount < 3 {
		return "Collect snapshots from ≥3 real financing rooms before changing §7.2 defaults. Mechanism stays the same."
	}
	if rep.FalseSupportedCount == 0 {
		return "No false-supported labels provided. Tag mislabeled supported rows, re-run, then consider PR to §7.2 only if distributions clearly disagree with 0.35 / 0.75 / 0.5."
	}
	if rep.FalseCaughtByDefaults == rep.FalseSupportedCount {
		return "Current defaults already flag all labeled false-supported rows as boundary. Keep 0.35 / 0.75 / 0.5 unless precision (over-boundary) is too high on clean supported rows."
	}
	missed := rep.FalseSupportedCount - rep.FalseCaughtByDefaults
	return fmt.Sprintf(
		"%d false-supported row(s) would not enter the boundary band under current defaults. Inspect false_rel_score / false_jaccard; if a stable alternate band emerges, open a PR to update §7.2 defaults only (do not change the relative-score OR Jaccard mechanism).",
		missed,
	)
}

func writeDistRow(b *strings.Builder, cohort, metric string, d Distribution) {
	fmt.Fprintf(b, "| %s | %s | %d | %.3f | %.3f | %.3f | %.3f | %.3f |\n",
		cohort, metric, d.N, d.Min, d.P50, d.P90, d.Max, d.Mean)
}
