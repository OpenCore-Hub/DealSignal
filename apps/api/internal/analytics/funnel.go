package analytics

import "sort"

// visitorReach is one visitor's reading depth on a document.
type visitorReach struct {
	MaxPage              int32
	DistinctPages        int64
	TotalDurationSeconds int64
}

// FunnelStep is one page in the reach drop-off funnel.
type FunnelStep struct {
	PageNumber      int32   `json:"pageNumber"`
	VisitorsReached int64   `json:"visitorsReached"`
	DropOffFromPrev float64 `json:"dropOffFromPrev"`
}

// DocumentReadingFunnel summarizes reading sessions and page reach for a document.
type DocumentReadingFunnel struct {
	DocumentID         string       `json:"documentId"`
	PageCount          int32        `json:"pageCount"`
	SessionCount       int          `json:"sessionCount"`
	CompletedSessions  int          `json:"completedSessions"`
	CompletionRate     float64      `json:"completionRate"`
	MedianMaxPage      float64      `json:"medianMaxPage"`
	AvgPagesPerSession float64      `json:"avgPagesPerSession"`
	AvgDurationSeconds float64      `json:"avgDurationSeconds"`
	BiggestDropOffPage int32        `json:"biggestDropOffPage"`
	Steps              []FunnelStep `json:"steps"`
	// SessionModel identifies the session grain: "reading_session" (idle-gap table).
	SessionModel string `json:"sessionModel,omitempty"`
	RangeDays    int    `json:"rangeDays,omitempty"`
	RangeFrom    string `json:"rangeFrom,omitempty"`
	RangeTo      string `json:"rangeTo,omitempty"`
	RangeCustom  bool   `json:"rangeCustom,omitempty"`
	Lifetime bool   `json:"lifetime,omitempty"`
}

// buildReadingFunnel computes session completion + reach funnel from session depth rows.
// Completion = max page ≥ pageCount (falls back to the deepest observed page when
// pageCount is unknown).
func buildReadingFunnel(documentID string, pageCount int32, sessions []visitorReach) DocumentReadingFunnel {
	out := DocumentReadingFunnel{
		DocumentID:   documentID,
		PageCount:    pageCount,
		Steps:        []FunnelStep{},
		SessionModel: "reading_session",
	}
	if len(sessions) == 0 {
		return out
	}

	out.SessionCount = len(sessions)

	depth := pageCount
	if depth <= 0 {
		for _, s := range sessions {
			if s.MaxPage > depth {
				depth = s.MaxPage
			}
		}
		out.PageCount = depth
	}
	if depth <= 0 {
		return out
	}

	maxPages := make([]int32, 0, len(sessions))
	var sumDistinct int64
	var sumDuration int64
	for _, s := range sessions {
		maxPages = append(maxPages, s.MaxPage)
		sumDistinct += s.DistinctPages
		sumDuration += s.TotalDurationSeconds
		if s.MaxPage >= depth {
			out.CompletedSessions++
		}
	}

	out.CompletionRate = float64(out.CompletedSessions) / float64(out.SessionCount)
	out.AvgPagesPerSession = float64(sumDistinct) / float64(out.SessionCount)
	out.AvgDurationSeconds = float64(sumDuration) / float64(out.SessionCount)
	out.MedianMaxPage = medianInt32(maxPages)

	reached := make([]int64, depth)
	for _, s := range sessions {
		limit := s.MaxPage
		if limit > depth {
			limit = depth
		}
		for p := int32(1); p <= limit; p++ {
			reached[p-1]++
		}
	}

	out.Steps = make([]FunnelStep, depth)
	var biggestDrop float64 = -1
	var biggestPage int32
	for i := int32(0); i < depth; i++ {
		page := i + 1
		step := FunnelStep{
			PageNumber:      page,
			VisitorsReached: reached[i],
		}
		if i > 0 {
			prev := reached[i-1]
			if prev > 0 {
				step.DropOffFromPrev = 1 - float64(reached[i])/float64(prev)
			}
			dropAbs := float64(prev - reached[i])
			if dropAbs > biggestDrop {
				biggestDrop = dropAbs
				biggestPage = page
			}
		}
		out.Steps[i] = step
	}
	out.BiggestDropOffPage = biggestPage
	return out
}

func medianInt32(values []int32) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int32(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return float64(sorted[mid])
	}
	return float64(sorted[mid-1]+sorted[mid]) / 2
}
