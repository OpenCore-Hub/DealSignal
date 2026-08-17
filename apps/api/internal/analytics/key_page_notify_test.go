package analytics

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestResolveKeyPageNotificationGatesHonestly(t *testing.T) {
	doc := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	link := db.Link{
		ID:         pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		DocumentID: doc,
	}
	docID := uuidToString(doc)

	t.Run("short dwell skipped", func(t *testing.T) {
		svc := NewService(&mockAnalyticsQuerier{pageTitleByNumber: "Financial Projections"}, nil, testCfg())
		_, ok, err := svc.ResolveKeyPageNotification(context.Background(), link, "v1", 4, 2, "")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})

	t.Run("non key title skipped", func(t *testing.T) {
		svc := NewService(&mockAnalyticsQuerier{pageTitleByNumber: "Appendix"}, nil, testCfg())
		_, ok, err := svc.ResolveKeyPageNotification(context.Background(), link, "v1", 4, 12, "")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})

	t.Run("no resolvable document skipped", func(t *testing.T) {
		svc := NewService(&mockAnalyticsQuerier{pageTitleByNumber: "Financial Projections"}, nil, testCfg())
		bundle := db.Link{ID: link.ID}
		_, ok, err := svc.ResolveKeyPageNotification(context.Background(), bundle, "v1", 4, 12, "")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})

	t.Run("bundle with event document matches", func(t *testing.T) {
		ws := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
		svc := NewService(&mockAnalyticsQuerier{
			pageTitleByNumber:   "财务模型",
			visitorKeyPageCount: 1,
			document:            db.GetDocumentByIDRow{Title: "Memo.pdf"},
		}, nil, testCfg())
		bundle := db.Link{ID: link.ID, WorkspaceID: ws}
		alert, ok, err := svc.ResolveKeyPageNotification(context.Background(), bundle, "v1", 4, 15, docID)
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if alert.EventType != "key_page" {
			t.Fatalf("event=%s", alert.EventType)
		}
		if alert.Metadata["document_id"] != docID {
			t.Fatalf("document_id meta=%s", alert.Metadata["document_id"])
		}
		if alert.Metadata["document_title"] != "Memo.pdf" {
			t.Fatalf("document_title meta=%s", alert.Metadata["document_title"])
		}
		if alert.Metadata["category"] != "financials" {
			t.Fatalf("category=%s", alert.Metadata["category"])
		}
	})

	t.Run("first engaged key page", func(t *testing.T) {
		svc := NewService(&mockAnalyticsQuerier{
			pageTitleByNumber:   "财务模型",
			visitorKeyPageCount: 1,
		}, nil, testCfg())
		alert, ok, err := svc.ResolveKeyPageNotification(context.Background(), link, "v1", 4, 15, "")
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if alert.EventType != "key_page" {
			t.Fatalf("event=%s", alert.EventType)
		}
		if alert.Metadata["category"] != "financials" {
			t.Fatalf("category=%s", alert.Metadata["category"])
		}
		if alert.Metadata["page_title"] != "财务模型" {
			t.Fatalf("title=%s", alert.Metadata["page_title"])
		}
		if _, ok := alert.Metadata["document_title"]; ok {
			t.Fatalf("primary document must not set document_title, got %v", alert.Metadata)
		}
	})

	t.Run("repeat engaged key page", func(t *testing.T) {
		svc := NewService(&mockAnalyticsQuerier{
			pageTitleByNumber:   "Financial Projections",
			visitorKeyPageCount: 3,
		}, nil, testCfg())
		alert, ok, err := svc.ResolveKeyPageNotification(context.Background(), link, "v1", 4, 20, "")
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if alert.EventType != "repeat_key_page" {
			t.Fatalf("event=%s", alert.EventType)
		}
		if alert.Metadata["engaged_key_page_views"] != "3" {
			t.Fatalf("count meta=%s", alert.Metadata["engaged_key_page_views"])
		}
	})
}
