package ingestion

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestProcessDocumentMaxAttempts(t *testing.T) {
	job := db.IngestionJob{
		ID:       pgtype.UUID{Valid: true},
		DocumentID: pgtype.UUID{Valid: true},
		Status:   "failed",
		Attempts: pgtype.Int4{Int32: 3, Valid: true},
	}
	fake := &fakeDB{job: job}
	svc := NewService(db.New(fake), nil, nil, nil)

	err := svc.ProcessDocument(context.Background(), db.Document{ID: job.DocumentID})
	if !errors.Is(err, ErrMaxAttemptsExceeded) {
		t.Fatalf("expected ErrMaxAttemptsExceeded, got %v", err)
	}
}

type fakeDB struct {
	job db.IngestionJob
}

func (f *fakeDB) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *fakeDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return &fakeRows{}, nil
}

func (f *fakeDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if strings.Contains(strings.ToLower(sql), "from ingestion_jobs") && strings.Contains(strings.ToLower(sql), "where document_id") {
		return fakeRow{job: f.job}
	}
	return fakeRow{err: pgx.ErrNoRows}
}

type fakeRow struct {
	job db.IngestionJob
	err error
}

func (r fakeRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	vals := []interface{}{r.job.ID, r.job.TenantID, r.job.WorkspaceID, r.job.DocumentID, r.job.Status, r.job.Attempts, r.job.ErrorMessage, r.job.CreatedAt, r.job.UpdatedAt}
	if len(dest) != len(vals) {
		return errors.New("scan count mismatch")
	}
	for i, v := range vals {
		switch d := dest[i].(type) {
		case *pgtype.UUID:
			*d = v.(pgtype.UUID)
		case *string:
			*d = v.(string)
		case *pgtype.Int4:
			*d = v.(pgtype.Int4)
		case *pgtype.Text:
			if s, ok := v.(string); ok {
				*d = pgtype.Text{String: s, Valid: true}
			}
		case *pgtype.Timestamptz:
			*d = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

type fakeRows struct{}

func (r *fakeRows) Next() bool                       { return false }
func (r *fakeRows) Err() error                       { return nil }
func (r *fakeRows) Close()                           {}
func (r *fakeRows) CommandTag() pgconn.CommandTag    { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)           { return nil, nil }
func (r *fakeRows) RawValues() [][]byte              { return nil }
func (r *fakeRows) Conn() *pgx.Conn                  { return nil }
func (r *fakeRows) Scan(dest ...interface{}) error   { return pgx.ErrNoRows }
