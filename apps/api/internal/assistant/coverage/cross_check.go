package coverage

import (
	"context"
	"fmt"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Cross-check claim statuses (P2.2). Distinct from coverage row three-state.
const (
	ClaimAligned         = "aligned"
	ClaimConflict        = "conflict"
	ClaimAbsentInScope   = StatusAbsentInScope
	ClaimInsufficient    = StatusInsufficient
)

// Default token Jaccard below which two supported sides are treated as conflict.
const defaultCrossCheckConflictJaccard = 0.35

// CrossCheckClaim is one pack-item comparison across two documents.
type CrossCheckClaim struct {
	ItemID  string            `json:"item_id"`
	Label   string            `json:"label"`
	Status  string            `json:"status"`
	CluesA  []search.Evidence `json:"clues_a"`
	CluesB  []search.Evidence `json:"clues_b"`
	Error   string            `json:"error,omitempty"`
}

// CrossCheckPack runs limited Owner dual-document comparison (rel.cross_check).
// Not exposed to visitor chat. Failures on one item → insufficient for that claim.
func CrossCheckPack(
	ctx context.Context,
	searcher Searcher,
	workspaceID pgtype.UUID,
	docA, docB uuid.UUID,
	pack jobs.Pack,
	lang string,
) ([]CrossCheckClaim, error) {
	if searcher == nil {
		return nil, fmt.Errorf("coverage: searcher required")
	}
	if docA == docB {
		return nil, fmt.Errorf("%w: document ids must differ", ErrInvalidInput)
	}
	out := make([]CrossCheckClaim, 0, len(pack.Items))
	for _, item := range pack.Items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out = append(out, crossCheckItem(ctx, searcher, workspaceID, docA, docB, item, lang))
	}
	return out, nil
}

func crossCheckItem(
	ctx context.Context,
	searcher Searcher,
	workspaceID pgtype.UUID,
	docA, docB uuid.UUID,
	item jobs.PackItem,
	lang string,
) CrossCheckClaim {
	claim := CrossCheckClaim{
		ItemID: item.ID,
		Label:  item.LabelFor(lang),
		CluesA: []search.Evidence{},
		CluesB: []search.Evidence{},
	}
	query := item.QueryFor(lang)
	if query == "" {
		claim.Status = ClaimInsufficient
		claim.Error = "empty query template"
		return claim
	}
	hitsA, errA := searcher.SearchInDocuments(ctx, workspaceID, []uuid.UUID{docA}, query, defaultRowTopK)
	hitsB, errB := searcher.SearchInDocuments(ctx, workspaceID, []uuid.UUID{docB}, query, defaultRowTopK)
	if errA != nil || errB != nil {
		claim.Status = ClaimInsufficient
		if errA != nil {
			claim.Error = errA.Error()
		} else {
			claim.Error = errB.Error()
		}
		return claim
	}
	claim.CluesA = hitsA
	claim.CluesB = hitsB
	hasA := len(hitsA) > 0
	hasB := len(hitsB) > 0
	switch {
	case !hasA && !hasB:
		claim.Status = ClaimAbsentInScope
	case hasA != hasB:
		claim.Status = ClaimConflict
	default:
		// Both supported: conflict when top quotes diverge (weak token overlap).
		if tokenJaccard(hitsA[0].Quote, hitsB[0].Quote) < defaultCrossCheckConflictJaccard {
			claim.Status = ClaimConflict
		} else {
			claim.Status = ClaimAligned
		}
	}
	return claim
}
