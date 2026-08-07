package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/docling"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// LinkScopedRequest is the visitor Ask AI retrieve body (link ACL scoped).
type LinkScopedRequest struct {
	Query  string
	Answer bool
	TopK   int
}

// QueryLinkScoped runs docling-rag search for a deal room and keeps only hits
// from authorizedDocIDs (public link ACL). Caller must verify visitor access first.
func (s *Service) QueryLinkScoped(
	ctx context.Context,
	roomID, workspaceID string,
	authorizedDocIDs []uuid.UUID,
	req LinkScopedRequest,
) (QueryResponse, error) {
	if !s.Enabled() {
		return QueryResponse{}, ErrUnavailable
	}
	q := strings.TrimSpace(req.Query)
	if q == "" {
		return QueryResponse{}, fmt.Errorf("query is required")
	}
	room, err := s.access.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return QueryResponse{}, err
	}
	cred, err := s.ensureProvisioned(ctx, room)
	if err != nil {
		return QueryResponse{}, err
	}
	topK := req.TopK
	if topK <= 0 {
		topK = s.cfg.DefaultTopK
	}
	mode := s.cfg.DefaultMode
	res, err := s.client.Search(ctx, cred.tenantSlug, cred.kbSlug, cred.apiKey, docling.SearchRequest{
		Query:  q,
		Mode:   mode,
		TopK:   topK,
		Answer: req.Answer,
	})
	if err != nil {
		var apiErr *docling.APIError
		if errors.As(err, &apiErr) && (apiErr.Code == "INDEX_NOT_READY" || apiErr.Status == http.StatusServiceUnavailable) {
			_, _ = s.queries.UpdateDealRoomRagCorpusStatus(ctx, db.UpdateDealRoomRagCorpusStatusParams{
				RoomID:       room.ID,
				Status:       "syncing",
				ErrorMessage: pgtype.Text{},
			})
		}
		return QueryResponse{}, mapUpstream(err)
	}

	allowed := authorizedDocIDSet(authorizedDocIDs)
	bindings, err := s.queries.ListDealRoomRagDocuments(ctx, room.ID)
	if err != nil {
		return QueryResponse{}, err
	}
	byExtID := map[string]string{}
	byName := map[string]string{}
	for _, b := range bindings {
		docID := uuid.UUID(b.DocumentID.Bytes).String()
		if !allowed[docID] {
			continue
		}
		byName[b.ExternalName] = docID
		if b.ExternalDocumentID.Valid {
			byExtID[b.ExternalDocumentID.String] = docID
		}
	}

	lockedIDs, err := s.lockedDocumentIDs(ctx, room)
	if err != nil {
		return QueryResponse{}, err
	}
	out := applyLinkScopedSearchFilter(res, byExtID, byName, allowed, lockedIDs)

	if tableHits, terr := s.retrieveTableLaneHits(ctx, room, lockedIDs, q, topK); terr != nil {
		logger.InfoCtx(ctx, "visitor ask table lane unavailable; continuing with hybrid only",
			logger.Attr("error", terr.Error()),
		)
	} else if n := applyTableLaneScoped(&out, tableHits, allowed, topK); n > 0 {
		recordKnowledgeQATableLaneHits(n)
	}

	s.enrichViewerPages(ctx, &out)
	s.enrichSourceDisplayNames(ctx, room.ID, &out)
	return out, nil
}

func authorizedDocIDSet(ids []uuid.UUID) map[string]bool {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id.String()] = true
	}
	return out
}

// applyLinkScopedSearchFilter keeps hits mapped to authorized documents and
// drops grounded answers when scope cannot be proven (P2 fail-closed).
func applyLinkScopedSearchFilter(
	res docling.SearchResponse,
	byExtID, byName map[string]string,
	allowedDocIDs map[string]bool,
	lockedIDs map[string]bool,
) QueryResponse {
	out := QueryResponse{
		Query:   res.Query,
		Mode:    res.Mode,
		Results: make([]QueryHit, 0, len(res.Results)),
	}
	sawOutOfScope := false
	sawLockedHit := false
	for _, hit := range res.Results {
		localDoc := mapHitLocalDocument(hit, byExtID, byName)
		if localDoc == "" || !allowedDocIDs[localDoc] {
			if localDoc != "" && !allowedDocIDs[localDoc] {
				sawOutOfScope = true
			} else if localDoc == "" && len(allowedDocIDs) > 0 {
				sawOutOfScope = true
			}
			continue
		}
		if lockedIDs[localDoc] {
			sawLockedHit = true
			continue
		}
		qh := QueryHit{
			ChunkID:    hit.Chunk.ID,
			DocumentID: localDoc,
			Text:       hit.Chunk.Text,
			Score:      hit.Score,
		}
		fillHitLocus(&qh, hit.Chunk.Metadata)
		out.Results = append(out.Results, qh)
	}
	if res.Answer != "" && !sawOutOfScope && !sawLockedHit {
		out.Answer = res.Answer
	}
	return out
}

func mapHitLocalDocument(hit docling.ScoredHit, byExtID, byName map[string]string) string {
	localDoc := byExtID[hit.Chunk.DocID]
	if localDoc == "" {
		if name, _ := hit.Chunk.Metadata["name"].(string); name != "" {
			localDoc = byName[name]
		}
		if localDoc == "" {
			if src, _ := hit.Chunk.Metadata["source_uri"].(string); src != "" {
				localDoc = byName[strings.TrimPrefix(src, "upload:///")]
			}
		}
	}
	return localDoc
}

func applyTableLaneScoped(out *QueryResponse, tableHits []QueryHit, allowed map[string]bool, topK int) int {
	if out == nil || len(tableHits) == 0 {
		return 0
	}
	filtered := make([]QueryHit, 0, len(tableHits))
	for _, h := range tableHits {
		if h.DocumentID != "" && allowed[h.DocumentID] {
			filtered = append(filtered, h)
		}
	}
	if len(filtered) == 0 {
		return 0
	}
	return applyTableLane(out, filtered, topK)
}

// ClassifyVisitorAskResult maps link-scoped retrieval to refuse/answered status.
func ClassifyVisitorAskResult(res QueryResponse, qerr error) (answer string, hits []QueryHit, refused bool, status string, refusal *RefusalInfo) {
	hits = res.Results
	if hits == nil {
		hits = []QueryHit{}
	}
	answer = res.Answer
	if qerr != nil {
		return "", []QueryHit{}, true, "error", refusalForError()
	}
	refused, status, refusal = classifyTurnResult(answer, len(hits))
	if refused {
		hits = []QueryHit{}
	}
	return answer, hits, refused, status, refusal
}

// BuildVisitorAskAIPayload is the JSON persisted on link_ask_turns.ai_payload.
func BuildVisitorAskAIPayload(answer string, hits []QueryHit, refused bool, status string, refusal *RefusalInfo) ([]byte, error) {
	if hits == nil {
		hits = []QueryHit{}
	}
	payload := map[string]any{
		"answer":       answer,
		"refused":      refused,
		"resultStatus": status,
		"hits":         hits,
	}
	if refusal != nil {
		payload["refusal"] = refusal
	}
	return json.Marshal(payload)
}
