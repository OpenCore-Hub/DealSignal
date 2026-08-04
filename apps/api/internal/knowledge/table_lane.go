package knowledge

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	tableLaneDefaultLimit = 8
	tableLaneMaxLimit     = 16
	tableLaneModeSuffix   = "+table"
)

var (
	tableLaneDigitRe  = regexp.MustCompile(`\d`)
	tableLaneKeywords = []string{
		"table", "spreadsheet", "xlsx", "csv", "row", "column", "sheet",
		"revenue", "ebitda", "ebit", "income", "balance", "cash flow",
		"valuation", "cap", "interest", "rate", "coupon", "multiple",
		"金额", "营收", "收入", "利润", "估值", "利率", "表格", "工作表", "行", "列",
		"损益", "资产负债表", "现金流量",
	}
)

// tableRowBBox is the subset of ingestion.TableRowMeta stored in chunk.bbox.
type tableRowBBox struct {
	Kind  string `json:"kind"`
	Sheet string `json:"sheet"`
	Row   int    `json:"row"`
}

// wantsTableLane reports whether the retrieve query should open the local table_row lane.
func wantsTableLane(query string) bool {
	q := strings.TrimSpace(query)
	if q == "" {
		return false
	}
	if tableLaneDigitRe.MatchString(q) {
		return true
	}
	lower := strings.ToLower(q)
	for _, kw := range tableLaneKeywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	for _, r := range q {
		if r == '%' || r == '￥' || r == '¥' || r == '$' || r == '€' {
			return true
		}
	}
	return strings.Contains(q, "万") || strings.Contains(q, "亿")
}

func escapeILIKEPattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// tableLaneSearchTerms picks distinctive tokens for ILIKE (longest first, capped).
func tableLaneSearchTerms(query string) []string {
	toks := distinctiveEvidenceTokens(strings.ToLower(query))
	if len(toks) == 0 {
		// Fall back to compact query without spaces for short numeric asks.
		compact := strings.Join(strings.Fields(query), "")
		if compact != "" {
			return []string{compact}
		}
		return nil
	}
	// Prefer longer tokens for precision.
	for i := 0; i < len(toks); i++ {
		for j := i + 1; j < len(toks); j++ {
			if len(toks[j]) > len(toks[i]) {
				toks[i], toks[j] = toks[j], toks[i]
			}
		}
	}
	if len(toks) > 4 {
		toks = toks[:4]
	}
	return toks
}

func scoreTableRowText(text string, terms []string) float64 {
	if len(terms) == 0 {
		return 0.1
	}
	lower := strings.ToLower(text)
	hits := 0
	for _, t := range terms {
		if t != "" && strings.Contains(lower, strings.ToLower(t)) {
			hits++
		}
	}
	if hits == 0 {
		return 0
	}
	return 0.35 + 0.15*float64(hits)
}

func parseTableRowSheet(bbox []byte) string {
	if len(bbox) == 0 {
		return ""
	}
	var meta tableRowBBox
	if err := json.Unmarshal(bbox, &meta); err != nil {
		return ""
	}
	return strings.TrimSpace(meta.Sheet)
}

func tableRowHitsFromChunks(rows []db.SearchTableRowsByDocumentsRow, terms []string) []QueryHit {
	out := make([]QueryHit, 0, len(rows))
	for _, row := range rows {
		text := strings.TrimSpace(row.Text)
		if text == "" {
			continue
		}
		score := scoreTableRowText(text, terms)
		if score <= 0 {
			continue
		}
		docID := ""
		if row.DocumentID.Valid {
			docID = uuid.UUID(row.DocumentID.Bytes).String()
		}
		hit := QueryHit{
			ChunkID:    uuid.UUID(row.ID.Bytes).String(),
			DocumentID: docID,
			Text:       text,
			Score:      score,
			Sheet:      parseTableRowSheet(row.Bbox),
		}
		out = append(out, hit)
	}
	return out
}

// mergeTableLaneHits prepends table-lane hits, deduping by chunkId and doc+text.
// Hybrid hits keep their relative order after table hits.
func mergeTableLaneHits(hybrid, table []QueryHit, topK int) []QueryHit {
	if topK <= 0 {
		topK = tableLaneDefaultLimit
	}
	if len(table) == 0 {
		if len(hybrid) > topK {
			return hybrid[:topK]
		}
		return hybrid
	}
	seenChunk := map[string]struct{}{}
	seenText := map[string]struct{}{}
	out := make([]QueryHit, 0, topK)
	add := func(h QueryHit) bool {
		if len(out) >= topK {
			return false
		}
		if id := strings.TrimSpace(h.ChunkID); id != "" {
			if _, ok := seenChunk[id]; ok {
				return true
			}
			seenChunk[id] = struct{}{}
		}
		key := strings.ToLower(h.DocumentID) + "|" + strings.ToLower(strings.TrimSpace(h.Text))
		if _, ok := seenText[key]; ok {
			return true
		}
		seenText[key] = struct{}{}
		out = append(out, h)
		return true
	}
	for _, h := range table {
		if !add(h) {
			return out
		}
	}
	for _, h := range hybrid {
		if !add(h) {
			return out
		}
	}
	return out
}

func (s *Service) unlockedRoomDocumentIDs(ctx context.Context, room db.DealRoom, lockedIDs map[string]bool) ([]pgtype.UUID, error) {
	rows, err := s.queries.ListDealRoomDocuments(ctx, room.ID)
	if err != nil {
		return nil, err
	}
	out := make([]pgtype.UUID, 0, len(rows))
	for _, row := range rows {
		id := uuid.UUID(row.DocumentID.Bytes).String()
		if lockedIDs[id] {
			continue
		}
		out = append(out, row.DocumentID)
	}
	return out, nil
}

// retrieveTableLaneHits runs the local table_row lane for unlocked room documents.
func (s *Service) retrieveTableLaneHits(
	ctx context.Context,
	room db.DealRoom,
	lockedIDs map[string]bool,
	query string,
	limit int,
) ([]QueryHit, error) {
	if s == nil || s.queries == nil || !s.tableLaneEnabled {
		return nil, nil
	}
	if !wantsTableLane(query) {
		return nil, nil
	}
	terms := tableLaneSearchTerms(query)
	if len(terms) == 0 {
		return nil, nil
	}
	docIDs, err := s.unlockedRoomDocumentIDs(ctx, room, lockedIDs)
	if err != nil || len(docIDs) == 0 {
		return nil, err
	}
	if limit <= 0 {
		limit = tableLaneDefaultLimit
	}
	if limit > tableLaneMaxLimit {
		limit = tableLaneMaxLimit
	}

	// Try terms until we collect enough hits (longest/most distinctive first).
	seen := map[string]QueryHit{}
	order := []string{}
	for _, term := range terms {
		pattern := escapeILIKEPattern(term)
		if pattern == "" {
			continue
		}
		rows, err := s.queries.SearchTableRowsByDocuments(ctx, db.SearchTableRowsByDocumentsParams{
			WorkspaceID: room.WorkspaceID,
			DocumentIds: docIDs,
			Pattern:     pgtype.Text{String: pattern, Valid: true},
			RowLimit:    int32(limit),
		})
		if err != nil {
			return nil, err
		}
		for _, h := range tableRowHitsFromChunks(rows, terms) {
			id := strings.TrimSpace(h.ChunkID)
			if id == "" {
				continue
			}
			if prev, ok := seen[id]; ok {
				if h.Score > prev.Score {
					seen[id] = h
				}
				continue
			}
			seen[id] = h
			order = append(order, id)
		}
		if len(seen) >= limit {
			break
		}
	}
	out := make([]QueryHit, 0, len(order))
	for _, id := range order {
		out = append(out, seen[id])
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// applyTableLane merges local table hits into a hybrid QueryResponse.
func applyTableLane(out *QueryResponse, tableHits []QueryHit, topK int) int {
	if out == nil || len(tableHits) == 0 {
		return 0
	}
	before := len(out.Results)
	out.Results = mergeTableLaneHits(out.Results, tableHits, topK)
	merged := 0
	for _, h := range out.Results {
		for _, t := range tableHits {
			if h.ChunkID != "" && h.ChunkID == t.ChunkID {
				merged++
				break
			}
		}
	}
	if merged > 0 && !strings.Contains(out.Mode, "table") {
		if out.Mode == "" {
			out.Mode = "table"
		} else {
			out.Mode = out.Mode + tableLaneModeSuffix
		}
	}
	_ = before
	return merged
}
