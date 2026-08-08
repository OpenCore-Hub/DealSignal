package link

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestResolveEventDocumentID(t *testing.T) {
	docA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	primary := pgtype.UUID{Bytes: docA, Valid: true}

	t.Run("falls back to primary document", func(t *testing.T) {
		h := &Handler{service: &Service{}}
		got, err := h.resolveEventDocumentID(context.Background(), db.Link{DocumentID: primary}, "")
		if err != nil {
			t.Fatal(err)
		}
		if got != docA.String() {
			t.Fatalf("got=%s", got)
		}
	})

	t.Run("bundle without document stays empty", func(t *testing.T) {
		h := &Handler{service: &Service{}}
		got, err := h.resolveEventDocumentID(context.Background(), db.Link{}, "")
		if err != nil || got != "" {
			t.Fatalf("got=%q err=%v", got, err)
		}
	})

	t.Run("invalid uuid rejected", func(t *testing.T) {
		h := &Handler{service: &Service{}}
		_, err := h.resolveEventDocumentID(context.Background(), db.Link{}, "not-a-uuid")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
