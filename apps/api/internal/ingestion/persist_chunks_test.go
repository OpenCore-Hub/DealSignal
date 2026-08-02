package ingestion

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Seam: persistTypedChunks → CreateChunkWithBBox (preview text/bbox only).
// Invariant: ingest must never INSERT embeddings into chunks.
func TestPersistTypedChunksOmitsEmbeddings(t *testing.T) {
	rec := &chunkInsertRecorder{}
	svc := NewService(db.New(rec), nil, nil)

	docID := uuid.New()
	pageID := uuid.New()
	doc := db.GetDocumentByIDRow{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: uuid.New(), Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
	}

	err := svc.persistTypedChunks(
		context.Background(),
		doc,
		pgtype.UUID{Bytes: pageID, Valid: true},
		1,
		[]Chunk{{Text: "revenue grew 12%", Bbox: []byte(`{"x":0.1,"y":0.2,"w":0.3,"h":0.05}`)}},
		[]string{chunkTypeParagraph},
		0,
	)
	if err != nil {
		t.Fatalf("persistTypedChunks: %v", err)
	}
	if rec.chunkInserts != 1 {
		t.Fatalf("expected 1 chunk insert, got %d", rec.chunkInserts)
	}
	for _, sql := range rec.insertSQLs {
		lower := strings.ToLower(sql)
		if !strings.Contains(lower, "insert into chunks") {
			continue
		}
		// Column list before VALUES must not include embedding.
		cols := lower
		if i := strings.Index(lower, "values"); i >= 0 {
			cols = lower[:i]
		}
		for _, banned := range []string{"embedding", "search_vector", "normalized_text"} {
			if strings.Contains(cols, banned) {
				t.Fatalf("ingest chunk insert must omit %s:\n%s", banned, sql)
			}
		}
	}
}

type chunkInsertRecorder struct {
	chunkInserts int
	insertSQLs   []string
}

func (r *chunkInsertRecorder) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	r.insertSQLs = append(r.insertSQLs, sql)
	return pgconn.CommandTag{}, nil
}

func (r *chunkInsertRecorder) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return &fakeRows{}, nil
}

func (r *chunkInsertRecorder) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	r.insertSQLs = append(r.insertSQLs, sql)
	lower := strings.ToLower(sql)
	if strings.Contains(lower, "insert into chunks") {
		r.chunkInserts++
		return chunkInsertRow{}
	}
	return fakeRow{err: pgx.ErrNoRows}
}

type chunkInsertRow struct{}

func (chunkInsertRow) Scan(dest ...interface{}) error {
	// CreateChunkWithBBox RETURNING: id, tenant, workspace, page, document,
	// chunk_index, chunk_type, text, bbox
	if len(dest) != 9 {
		return errors.New("unexpected CreateChunkWithBBox scan arity")
	}
	if id, ok := dest[0].(*pgtype.UUID); ok {
		*id = pgtype.UUID{Bytes: uuid.New(), Valid: true}
	}
	return nil
}
