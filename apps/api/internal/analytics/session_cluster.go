package analytics

import (
	"sort"
	"time"
)

// PageViewClusterEvent is one page_view fact used to rebuild idle-gap sessions.
// Events with the same LinkID+VisitorID must be ordered by CreatedAt then Seq.
type PageViewClusterEvent struct {
	LinkID          string
	VisitorID       string
	CreatedAt       time.Time
	PageNumber      int32
	DurationSeconds int32
	// Seq breaks ties when CreatedAt is equal (e.g. page_view id order).
	Seq int64
}

// ReadingSessionCluster is one idle-gap session aggregate (historical rebuild grain).
type ReadingSessionCluster struct {
	LinkID               string
	VisitorID            string
	StartedAt            time.Time
	LastActivityAt       time.Time
	MaxPage              int32
	DistinctPageCount    int32
	TotalDurationSeconds int32
	// PageDurations maps page_number → summed duration_seconds in this session.
	PageDurations map[int32]int32
}

// ClusterPageViewsIntoSessions groups page views into idle-gap reading sessions.
// A new session starts when the gap from the previous view (same link+visitor)
// exceeds idle (live path uses readingSessionIdle = 30m).
func ClusterPageViewsIntoSessions(events []PageViewClusterEvent, idle time.Duration) []ReadingSessionCluster {
	if idle <= 0 {
		idle = readingSessionIdle
	}
	if len(events) == 0 {
		return nil
	}

	sorted := append([]PageViewClusterEvent(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.LinkID != b.LinkID {
			return a.LinkID < b.LinkID
		}
		if a.VisitorID != b.VisitorID {
			return a.VisitorID < b.VisitorID
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.Seq < b.Seq
	})

	out := make([]ReadingSessionCluster, 0)
	var cur *ReadingSessionCluster
	var curLink, curVisitor string
	var lastAt time.Time

	flush := func() {
		if cur == nil {
			return
		}
		cur.DistinctPageCount = int32(len(cur.PageDurations))
		out = append(out, *cur)
		cur = nil
	}

	for _, ev := range sorted {
		if ev.LinkID == "" || ev.VisitorID == "" || ev.PageNumber <= 0 {
			continue
		}
		at := ev.CreatedAt.UTC()
		dur := ev.DurationSeconds
		if dur < 0 {
			dur = 0
		}

		sameActor := cur != nil && curLink == ev.LinkID && curVisitor == ev.VisitorID
		if !sameActor || at.Sub(lastAt) > idle {
			flush()
			cur = &ReadingSessionCluster{
				LinkID:         ev.LinkID,
				VisitorID:      ev.VisitorID,
				StartedAt:      at,
				LastActivityAt: at,
				MaxPage:        ev.PageNumber,
				PageDurations:  map[int32]int32{ev.PageNumber: dur},
			}
			cur.TotalDurationSeconds = dur
			curLink, curVisitor = ev.LinkID, ev.VisitorID
			lastAt = at
			continue
		}

		cur.LastActivityAt = at
		if ev.PageNumber > cur.MaxPage {
			cur.MaxPage = ev.PageNumber
		}
		cur.PageDurations[ev.PageNumber] += dur
		cur.TotalDurationSeconds += dur
		lastAt = at
	}
	flush()
	return out
}
