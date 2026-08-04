package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

const corpusFingerprintVersion = "v1"

// computeCorpusFingerprint returns a stable sha256 hex of the room's RAG sync generation.
// Inputs: corpus status + non-deleted docs (external id, status, chunk count, updated_at unix).
func computeCorpusFingerprint(corpusStatus string, rows []db.DealRoomRagDocument) string {
	status := strings.TrimSpace(corpusStatus)
	if status == "" {
		status = "unknown"
	}
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Status == "deleted" {
			continue
		}
		ext := ""
		if r.ExternalDocumentID.Valid {
			ext = strings.TrimSpace(r.ExternalDocumentID.String)
		}
		updated := int64(0)
		if r.UpdatedAt.Valid {
			updated = r.UpdatedAt.Time.UTC().Unix()
		}
		lines = append(lines, fmt.Sprintf("%s|%s|%d|%d", ext, r.Status, r.ChunkCount, updated))
	}
	sort.Strings(lines)

	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s|%s\n", corpusFingerprintVersion, status)
	for _, line := range lines {
		_, _ = h.Write([]byte(line))
		_, _ = h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}
