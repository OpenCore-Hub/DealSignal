// Package search implements hybrid retrieval over a workspace's document chunks.
package search

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

const (
	defaultTopK = 5
	maxTopK     = 20
)

// Embedder creates vector embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Evidence is a single retrieved chunk with its source location.
type Evidence struct {
	ChunkID    string      `json:"chunk_id"`
	DocumentID string      `json:"document_id"`
	PageNumber int32       `json:"page_number"`
	Text       string      `json:"text"`
	Bbox       interface{} `json:"bbox,omitempty"`
}

// Service performs hybrid vector + full-text search.
type Service struct {
	queries  *db.Queries
	embedder Embedder
}

// NewService creates a search service.
func NewService(q *db.Queries, e Embedder) *Service {
	return &Service{queries: q, embedder: e}
}

// Search retrieves the most relevant evidence for a query within a workspace.
func (s *Service) Search(ctx context.Context, workspaceID pgtype.UUID, query string, topK int) ([]Evidence, error) {
	if topK <= 0 {
		topK = defaultTopK
	}
	if topK > maxTopK {
		topK = maxTopK
	}

	vectorResults, err := s.vectorSearch(ctx, workspaceID, query, topK)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	textResults, err := s.textSearch(ctx, workspaceID, query, topK)
	if err != nil {
		return nil, fmt.Errorf("text search: %w", err)
	}

	return mergeEvidence(vectorResults, textResults, topK), nil
}

func (s *Service) vectorSearch(ctx context.Context, workspaceID pgtype.UUID, query string, topK int) ([]Evidence, error) {
	if s.embedder == nil {
		return nil, nil
	}

	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.SearchChunksByVector(ctx, db.SearchChunksByVectorParams{
		WorkspaceID: workspaceID,
		Limit:       int32(topK),
		Embedding:   pgvector.NewVector(vec),
	})
	if err != nil {
		return nil, err
	}

	out := make([]Evidence, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowToEvidence(r.ID, r.DocumentID, r.PageNumber, r.Text, r.Bbox))
	}
	return out, nil
}

func (s *Service) textSearch(ctx context.Context, workspaceID pgtype.UUID, query string, topK int) ([]Evidence, error) {
	rows, err := s.queries.SearchChunksByText(ctx, db.SearchChunksByTextParams{
		WorkspaceID: workspaceID,
		Limit:       int32(topK),
		Query:       query,
	})
	if err != nil {
		return nil, err
	}

	out := make([]Evidence, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowToEvidence(r.ID, r.DocumentID, r.PageNumber, r.Text, r.Bbox))
	}
	return out, nil
}

func rowToEvidence(chunkID, docID pgtype.UUID, pageNumber int32, text string, bbox []byte) Evidence {
	ev := Evidence{
		ChunkID:    pgUUIDToString(chunkID),
		DocumentID: pgUUIDToString(docID),
		PageNumber: pageNumber,
		Text:       text,
	}
	if len(bbox) > 0 {
		var parsed interface{}
		if err := json.Unmarshal(bbox, &parsed); err == nil {
			ev.Bbox = parsed
		}
	}
	return ev
}

func mergeEvidence(vector, text []Evidence, topK int) []Evidence {
	seen := make(map[string]struct{}, len(vector)+len(text))
	out := make([]Evidence, 0, topK)

	for _, e := range vector {
		if _, ok := seen[e.ChunkID]; ok {
			continue
		}
		seen[e.ChunkID] = struct{}{}
		out = append(out, e)
		if len(out) == topK {
			return out
		}
	}

	for _, e := range text {
		if _, ok := seen[e.ChunkID]; ok {
			continue
		}
		seen[e.ChunkID] = struct{}{}
		out = append(out, e)
		if len(out) == topK {
			break
		}
	}
	return out
}

func pgUUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return u.String()
}
