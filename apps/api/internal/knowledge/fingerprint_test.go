package knowledge

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestComputeCorpusFingerprintStableAndOrderIndependent(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	a := db.DealRoomRagDocument{
		ExternalDocumentID: pgtype.Text{String: "doc-b", Valid: true},
		Status:             "synced",
		ChunkCount:         3,
		UpdatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
	}
	b := db.DealRoomRagDocument{
		ExternalDocumentID: pgtype.Text{String: "doc-a", Valid: true},
		Status:             "synced",
		ChunkCount:         1,
		UpdatedAt:          pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
	}
	deleted := db.DealRoomRagDocument{
		ExternalDocumentID: pgtype.Text{String: "doc-z", Valid: true},
		Status:             "deleted",
		ChunkCount:         9,
		UpdatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
	}
	fp1 := computeCorpusFingerprint("ready", []db.DealRoomRagDocument{a, b, deleted})
	fp2 := computeCorpusFingerprint("ready", []db.DealRoomRagDocument{b, a, deleted})
	if fp1 == "" || fp1 != fp2 {
		t.Fatalf("fingerprint unstable: %q vs %q", fp1, fp2)
	}
	fp3 := computeCorpusFingerprint("degraded", []db.DealRoomRagDocument{a, b})
	if fp3 == fp1 {
		t.Fatal("status change must change fingerprint")
	}
	b.ChunkCount = 2
	fp4 := computeCorpusFingerprint("ready", []db.DealRoomRagDocument{a, b})
	if fp4 == fp1 {
		t.Fatal("chunk count change must change fingerprint")
	}
}

func TestBuildDiligencePackUsesTurnFingerprintFallback(t *testing.T) {
	t.Parallel()
	pack := buildDiligencePack("ws-1", SessionDetail{
		Session: QASession{ID: "sess-1", RoomID: "room-1", Status: "active"},
		Turns: []QATurn{
			{ID: "t1", Sequence: 1, Question: "q1", CorpusFingerprint: "abc"},
			{ID: "t2", Sequence: 2, Question: "q2", CorpusFingerprint: "def"},
		},
	}, "")
	if pack.SchemaVersion != diligenceExportSchemaVersion {
		t.Fatalf("schema=%s", pack.SchemaVersion)
	}
	if pack.CorpusFingerprint != "def" {
		t.Fatalf("want latest turn fingerprint, got %q", pack.CorpusFingerprint)
	}
	if pack.WorkspaceID != "ws-1" || pack.SessionID != "sess-1" || len(pack.Turns) != 2 {
		t.Fatalf("pack=%+v", pack)
	}
}
