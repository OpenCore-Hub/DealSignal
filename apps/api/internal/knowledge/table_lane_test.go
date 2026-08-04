package knowledge

import (
	"encoding/json"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestWantsTableLane(t *testing.T) {
	t.Parallel()
	cases := []struct {
		q    string
		want bool
	}{
		{"What is the valuation cap?", true},
		{"revenue in 2024", true},
		{"show the EBITDA row", true},
		{"营收是多少", true},
		{"interest rate 5%", true},
		{"who is the buyer", false},
		{"define change of control", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := wantsTableLane(tc.q); got != tc.want {
			t.Fatalf("wantsTableLane(%q)=%v want %v", tc.q, got, tc.want)
		}
	}
}

func TestEscapeILIKEPattern(t *testing.T) {
	t.Parallel()
	got := escapeILIKEPattern(`100%_raw\x`)
	if got != `100\%\_raw\\x` {
		t.Fatalf("got %q", got)
	}
}

func TestMergeTableLaneHitsDedupAndCap(t *testing.T) {
	t.Parallel()
	hybrid := []QueryHit{
		{ChunkID: "h1", DocumentID: "d1", Text: "hybrid text", Score: 0.9},
		{ChunkID: "h2", DocumentID: "d2", Text: "other", Score: 0.5},
	}
	table := []QueryHit{
		{ChunkID: "t1", DocumentID: "d1", Text: "Revenue 2024 12.5", Score: 0.8, Sheet: "P&L"},
		{ChunkID: "h1", DocumentID: "d1", Text: "hybrid text", Score: 0.7}, // dup chunk
	}
	got := mergeTableLaneHits(hybrid, table, 3)
	if len(got) != 3 {
		t.Fatalf("len=%d %#v", len(got), got)
	}
	if got[0].ChunkID != "t1" {
		t.Fatalf("table hit should lead: %#v", got[0])
	}
	if got[1].ChunkID != "h1" {
		t.Fatalf("hybrid after table: %#v", got)
	}
}

func TestTableRowHitsFromChunksParsesSheet(t *testing.T) {
	t.Parallel()
	docID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	chunkID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	bbox, _ := json.Marshal(tableRowBBox{Kind: "table_row", Sheet: "损益表", Row: 3})
	rows := []db.SearchTableRowsByDocumentsRow{{
		ID:         pgtype.UUID{Bytes: chunkID, Valid: true},
		DocumentID: pgtype.UUID{Bytes: docID, Valid: true},
		ChunkIndex: pgtype.Int4{Int32: 1_000_001, Valid: true},
		Text:       "Revenue | 2024 | 12,500,000",
		Bbox:       bbox,
	}}
	hits := tableRowHitsFromChunks(rows, []string{"revenue", "2024"})
	if len(hits) != 1 {
		t.Fatalf("hits=%d", len(hits))
	}
	if hits[0].Sheet != "损益表" || hits[0].DocumentID != docID.String() {
		t.Fatalf("hit=%#v", hits[0])
	}
	if hits[0].Score <= 0 {
		t.Fatalf("score=%v", hits[0].Score)
	}
}

func TestApplyTableLaneModesAndCount(t *testing.T) {
	t.Parallel()
	out := QueryResponse{
		Mode: "hybrid",
		Results: []QueryHit{
			{ChunkID: "h1", Text: "a", Score: 0.5},
		},
	}
	table := []QueryHit{
		{ChunkID: "t1", Text: "Revenue 10", Score: 0.9, Sheet: "IS"},
	}
	n := applyTableLane(&out, table, 8)
	if n != 1 {
		t.Fatalf("merged=%d", n)
	}
	if out.Mode != "hybrid+table" {
		t.Fatalf("mode=%s", out.Mode)
	}
	if out.Results[0].ChunkID != "t1" {
		t.Fatalf("order %#v", out.Results)
	}
}
