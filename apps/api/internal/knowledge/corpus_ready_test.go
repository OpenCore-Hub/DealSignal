package knowledge

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestCorpusAskReady(t *testing.T) {
	t.Parallel()
	synced := []DocumentSyncItem{
		{DocumentID: "d1", Status: "synced", ChunkCount: 2},
		{DocumentID: "d2", Status: "synced", ChunkCount: 7},
	}
	cases := []struct {
		name string
		c    CorpusStatus
		want bool
	}{
		{
			name: "disabled",
			c:    CorpusStatus{Enabled: false, Status: "ready", Documents: synced},
			want: false,
		},
		{
			name: "empty",
			c:    CorpusStatus{Enabled: true, Status: "ready", Documents: nil},
			want: false,
		},
		{
			name: "ready",
			c:    CorpusStatus{Enabled: true, Status: "ready", Documents: synced},
			want: true,
		},
		{
			name: "building pending doc",
			c: CorpusStatus{
				Enabled: true,
				Status:  "ready",
				Documents: []DocumentSyncItem{
					{DocumentID: "d1", Status: "pending"},
				},
			},
			want: false,
		},
		{
			name: "attention failed",
			c: CorpusStatus{
				Enabled: true,
				Status:  "degraded",
				Documents: []DocumentSyncItem{
					{DocumentID: "d1", Status: "failed"},
				},
			},
			want: false,
		},
		{
			name: "heal stuck provisioning when all synced",
			c: CorpusStatus{
				Enabled: true,
				Status:  "provisioning",
				Progress: SyncProgress{
					Total: 2, Synced: 2, JobStatus: "done",
				},
				Documents: synced,
			},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := corpusAskReady(tc.c); got != tc.want {
				t.Fatalf("corpusAskReady=%v want %v", got, tc.want)
			}
		})
	}
}

func TestRoomCorpusAskReady_NilService(t *testing.T) {
	t.Parallel()
	var s *Service
	if s.RoomCorpusAskReady(context.Background(), pgtype.UUID{Valid: true}, pgtype.UUID{Valid: true}) {
		t.Fatal("expected false for nil service")
	}
}

func TestMapKnowledgeErrorCorpusNotReady(t *testing.T) {
	t.Parallel()
	body := mapKnowledgeError(ErrCorpusNotReady)
	if body.Code != "knowledge_corpus_not_ready" || body.Status != 409 {
		t.Fatalf("%+v", body)
	}
}
