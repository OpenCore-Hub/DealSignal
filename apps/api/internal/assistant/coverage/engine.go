package coverage

import (
	"context"
	"fmt"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Row status values (P2 ClaimPack).
const (
	StatusSupported     = "supported"
	StatusAbsentInScope = "absent_in_scope"
	StatusInsufficient  = "insufficient"
)

// Default row retrieval Top-K (design §7.1 P2 pseudocode).
const defaultRowTopK = 8

// CoverageRow is one checklist item result in a ClaimPack snapshot.
type CoverageRow struct {
	ItemID         string            `json:"item_id"`
	Label          string            `json:"label"`
	Status         string            `json:"status"`
	Clues          []search.Evidence `json:"clues"`
	Error          string            `json:"error,omitempty"`
	ValueType      string            `json:"value_type,omitempty"`
	ExtractedValue string            `json:"extracted_value,omitempty"`
}

// Searcher is the hybrid search surface used by the row engine.
type Searcher interface {
	SearchInDocuments(ctx context.Context, workspaceID pgtype.UUID, documentIDs []uuid.UUID, query string, topK int, opts ...search.SearchOptions) ([]search.Evidence, error)
}

// ScanPack runs the P2 row engine over every pack item.
// Empty documentIDs → all rows absent_in_scope (fail-closed; no workspace-wide search).
// Per-row search errors → insufficient for that row; other rows continue.
func ScanPack(ctx context.Context, searcher Searcher, workspaceID pgtype.UUID, documentIDs []uuid.UUID, pack jobs.Pack, lang string) ([]CoverageRow, error) {
	if searcher == nil {
		return nil, fmt.Errorf("coverage: searcher required")
	}
	rows := make([]CoverageRow, 0, len(pack.Items))
	for _, item := range pack.Items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows = append(rows, scanItem(ctx, searcher, workspaceID, documentIDs, item, lang))
	}
	return rows, nil
}

func scanItem(ctx context.Context, searcher Searcher, workspaceID pgtype.UUID, documentIDs []uuid.UUID, item jobs.PackItem, lang string) CoverageRow {
	row := CoverageRow{
		ItemID: item.ID,
		Label:  item.LabelFor(lang),
		Clues:  []search.Evidence{},
	}
	if len(documentIDs) == 0 {
		row.Status = StatusAbsentInScope
		return row
	}
	query := item.QueryFor(lang)
	if query == "" {
		row.Status = StatusInsufficient
		row.Error = "empty query template"
		return row
	}
	hits, err := searcher.SearchInDocuments(ctx, workspaceID, documentIDs, query, defaultRowTopK)
	if err != nil {
		row.Status = StatusInsufficient
		row.Error = err.Error()
		return row
	}
	if len(hits) == 0 {
		row.Status = StatusAbsentInScope
		return row
	}
	row.Status = StatusSupported
	row.Clues = hits
	return row
}
