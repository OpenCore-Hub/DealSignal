// Package search implements hybrid retrieval over a workspace's document chunks.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

const (
	defaultTopK = 5
	maxTopK     = 20
	rrfK        = 60 // RRF constant for score fusion
)

// Embedder creates vector embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// BoundingBox is a normalized bounding box in PAGE_IMAGE_NORMALIZED coordinate space.
type BoundingBox struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
}

// Evidence is a single retrieved chunk with its source location and precise bbox.
type Evidence struct {
	ChunkID    string        `json:"chunk_id"`
	DocumentID string        `json:"document_id"`
	PageNumber int32         `json:"page_number"`
	Quote      string        `json:"quote"`
	Score      float64       `json:"score"`
	MatchType  string        `json:"match_type"`
	Boxes      []BoundingBox `json:"boxes,omitempty"`
}

// Service performs hybrid vector + full-text + trigram search with RRF fusion.
type Service struct {
	queries  *db.Queries
	embedder Embedder
}

// NewService creates a search service.
func NewService(q *db.Queries, e Embedder) *Service {
	return &Service{queries: q, embedder: e}
}

// Search retrieves the most relevant evidence for a query within a workspace.
// It performs three retrieval strategies (vector, full-text, trigram) and fuses
// results using Reciprocal Rank Fusion (RRF).
func (s *Service) Search(ctx context.Context, workspaceID pgtype.UUID, query string, topK int) ([]Evidence, error) {
	if topK <= 0 {
		topK = defaultTopK
	}
	if topK > maxTopK {
		topK = maxTopK
	}

	// Run all three search strategies in parallel-like fashion (sequential but independent)
	vectorResults, err := s.vectorSearch(ctx, workspaceID, query, topK)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	textResults, err := s.textSearch(ctx, workspaceID, query, topK)
	if err != nil {
		return nil, fmt.Errorf("full-text search: %w", err)
	}

	trigramResults, err := s.trigramSearch(ctx, workspaceID, query, topK)
	if err != nil {
		return nil, fmt.Errorf("trigram search: %w", err)
	}

	return rrfFuse(topK, vectorResults, textResults, trigramResults), nil
}

// SearchInDocuments retrieves evidence restricted to a specific set of documents.
// It is used by public AI Copilot so that anonymous users cannot access chunks
// outside the link they were invited to view.
func (s *Service) SearchInDocuments(ctx context.Context, workspaceID pgtype.UUID, documentIDs []uuid.UUID, query string, topK int) ([]Evidence, error) {
	if topK <= 0 {
		topK = defaultTopK
	}
	if topK > maxTopK {
		topK = maxTopK
	}

	pgDocIDs := make([]pgtype.UUID, 0, len(documentIDs))
	for _, id := range documentIDs {
		pgDocIDs = append(pgDocIDs, pgtype.UUID{Bytes: id, Valid: true})
	}

	vectorResults, err := s.vectorSearchInDocuments(ctx, workspaceID, pgDocIDs, query, topK)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	textResults, err := s.textSearchInDocuments(ctx, workspaceID, pgDocIDs, query, topK)
	if err != nil {
		return nil, fmt.Errorf("full-text search: %w", err)
	}

	trigramResults, err := s.trigramSearchInDocuments(ctx, workspaceID, pgDocIDs, query, topK)
	if err != nil {
		return nil, fmt.Errorf("trigram search: %w", err)
	}

	return rrfFuse(topK, vectorResults, textResults, trigramResults), nil
}

// rankedEvidence holds an evidence item and its rank within a single strategy.
type rankedEvidence struct {
	evidence Evidence
	rank     int // 1-indexed
}

func (s *Service) vectorSearch(ctx context.Context, workspaceID pgtype.UUID, query string, topK int) ([]rankedEvidence, error) {
	if s.embedder == nil {
		return nil, nil
	}

	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		log.Printf(`{"level":"warn","component":"search","message":"vector embed failed, skipping vector search: %s"}`, err.Error())
		return nil, nil
	}

	rows, err := s.queries.SearchChunksByVector(ctx, db.SearchChunksByVectorParams{
		WorkspaceID: workspaceID,
		Limit:       int32(topK),
		Embedding:   pgvector.NewVector(vec),
	})
	if err != nil {
		return nil, err
	}

	out := make([]rankedEvidence, 0, len(rows))
	for i, r := range rows {
		ev := rowToEvidence(r.ID, r.DocumentID, r.PageNumber, r.Text, r.Bbox)
		ev.MatchType = "vector"
		out = append(out, rankedEvidence{evidence: ev, rank: i + 1})
	}
	return out, nil
}

func (s *Service) vectorSearchInDocuments(ctx context.Context, workspaceID pgtype.UUID, documentIDs []pgtype.UUID, query string, topK int) ([]rankedEvidence, error) {
	if s.embedder == nil {
		return nil, nil
	}

	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		log.Printf(`{"level":"warn","component":"search","message":"vector embed failed, skipping vector search: %s"}`, err.Error())
		return nil, nil
	}

	rows, err := s.queries.SearchChunksByVectorInDocuments(ctx, db.SearchChunksByVectorInDocumentsParams{
		WorkspaceID: workspaceID,
		Limit:       int32(topK),
		Embedding:   pgvector.NewVector(vec),
		DocumentIds: documentIDs,
	})
	if err != nil {
		return nil, err
	}

	out := make([]rankedEvidence, 0, len(rows))
	for i, r := range rows {
		ev := rowToEvidence(r.ID, r.DocumentID, r.PageNumber, r.Text, r.Bbox)
		ev.MatchType = "vector"
		out = append(out, rankedEvidence{evidence: ev, rank: i + 1})
	}
	return out, nil
}

func (s *Service) textSearch(ctx context.Context, workspaceID pgtype.UUID, query string, topK int) ([]rankedEvidence, error) {
	rows, err := s.queries.SearchChunksByText(ctx, db.SearchChunksByTextParams{
		WorkspaceID: workspaceID,
		Limit:       int32(topK),
		Query:       query,
	})
	if err != nil {
		return nil, err
	}

	out := make([]rankedEvidence, 0, len(rows))
	for i, r := range rows {
		ev := rowToEvidence(r.ID, r.DocumentID, r.PageNumber, r.Text, r.Bbox)
		ev.MatchType = "fulltext"
		out = append(out, rankedEvidence{evidence: ev, rank: i + 1})
	}
	return out, nil
}

func (s *Service) textSearchInDocuments(ctx context.Context, workspaceID pgtype.UUID, documentIDs []pgtype.UUID, query string, topK int) ([]rankedEvidence, error) {
	rows, err := s.queries.SearchChunksByTextInDocuments(ctx, db.SearchChunksByTextInDocumentsParams{
		WorkspaceID: workspaceID,
		Limit:       int32(topK),
		Query:       query,
		DocumentIds: documentIDs,
	})
	if err != nil {
		return nil, err
	}

	out := make([]rankedEvidence, 0, len(rows))
	for i, r := range rows {
		ev := rowToEvidence(r.ID, r.DocumentID, r.PageNumber, r.Text, r.Bbox)
		ev.MatchType = "fulltext"
		out = append(out, rankedEvidence{evidence: ev, rank: i + 1})
	}
	return out, nil
}

func (s *Service) trigramSearch(ctx context.Context, workspaceID pgtype.UUID, query string, topK int) ([]rankedEvidence, error) {
	normalizedQuery := normalizeQuery(query)
	if normalizedQuery == "" {
		return nil, nil
	}

	rows, err := s.queries.SearchChunksByTrigram(ctx, db.SearchChunksByTrigramParams{
		WorkspaceID: workspaceID,
		Limit:       int32(topK),
		Query:       normalizedQuery,
	})
	if err != nil {
		return nil, err
	}

	out := make([]rankedEvidence, 0, len(rows))
	for i, r := range rows {
		ev := rowToEvidence(r.ID, r.DocumentID, r.PageNumber, r.Text, r.Bbox)
		ev.MatchType = "exact"
		out = append(out, rankedEvidence{evidence: ev, rank: i + 1})
	}
	return out, nil
}

func (s *Service) trigramSearchInDocuments(ctx context.Context, workspaceID pgtype.UUID, documentIDs []pgtype.UUID, query string, topK int) ([]rankedEvidence, error) {
	normalizedQuery := normalizeQuery(query)
	if normalizedQuery == "" {
		return nil, nil
	}

	rows, err := s.queries.SearchChunksByTrigramInDocuments(ctx, db.SearchChunksByTrigramInDocumentsParams{
		WorkspaceID: workspaceID,
		Limit:       int32(topK),
		Query:       normalizedQuery,
		DocumentIds: documentIDs,
	})
	if err != nil {
		return nil, err
	}

	out := make([]rankedEvidence, 0, len(rows))
	for i, r := range rows {
		ev := rowToEvidence(r.ID, r.DocumentID, r.PageNumber, r.Text, r.Bbox)
		ev.MatchType = "exact"
		out = append(out, rankedEvidence{evidence: ev, rank: i + 1})
	}
	return out, nil
}

func rowToEvidence(chunkID, docID pgtype.UUID, pageNumber int32, text string, bbox []byte) Evidence {
	ev := Evidence{
		ChunkID:    pgUUIDToString(chunkID),
		DocumentID: pgUUIDToString(docID),
		PageNumber: pageNumber,
		Quote:      text,
	}
	if len(bbox) > 0 {
		// Try parsing as normalized bbox {x,y,w,h}
		var box BoundingBox
		if err := json.Unmarshal(bbox, &box); err == nil && box.W > 0 && box.H > 0 {
			ev.Boxes = []BoundingBox{box}
		}
	}
	return ev
}

// rrfFuse combines multiple ranked lists using Reciprocal Rank Fusion.
// score = Σ 1/(k + rank_i) for each list where the chunk appears.
func rrfFuse(topK int, lists ...[]rankedEvidence) []Evidence {
	scores := make(map[string]float64)
	evidenceMap := make(map[string]Evidence)

	for _, list := range lists {
		for _, re := range list {
			id := re.evidence.ChunkID
			if id == "" {
				continue
			}
			scores[id] += 1.0 / float64(rrfK+re.rank)
			// Keep first occurrence as the base evidence
			if _, ok := evidenceMap[id]; !ok {
				evidenceMap[id] = re.evidence
			}
			// Prefer evidence that already has boxes
			if existing, ok := evidenceMap[id]; ok && len(re.evidence.Boxes) > 0 && len(existing.Boxes) == 0 {
				evidenceMap[id] = re.evidence
			}
		}
	}

	// Build result slice sorted by RRF score
	result := make([]Evidence, 0, len(scores))
	for id, score := range scores {
		ev := evidenceMap[id]
		ev.Score = score
		result = append(result, ev)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	if len(result) > topK {
		result = result[:topK]
	}
	return result
}

func normalizeQuery(s string) string {
	var out []rune
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= 0x4e00 && r <= 0x9fff) {
			out = append(out, r)
		} else if r >= 'A' && r <= 'Z' {
			out = append(out, r+32) // to lowercase
		} else {
			out = append(out, ' ')
		}
	}
	// Trim and collapse spaces
	result := make([]rune, 0, len(out))
	prevSpace := false
	for _, r := range out {
		if r == ' ' {
			if !prevSpace && len(result) > 0 {
				result = append(result, r)
			}
			prevSpace = true
		} else {
			result = append(result, r)
			prevSpace = false
		}
	}
	// Trim trailing space
	if len(result) > 0 && result[len(result)-1] == ' ' {
		result = result[:len(result)-1]
	}
	return string(result)
}

func pgUUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return u.String()
}
