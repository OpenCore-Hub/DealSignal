package link

import (
	"context"
	"errors"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrKnowledgeBaseRequired is returned when enabling Ask Docs on a deal-room
// link while the room knowledge base is not ready or stale.
var ErrKnowledgeBaseRequired = errors.New("create or rebuild the room knowledge base before enabling Ask Docs")

// AskDocsCoverageWarning is a soft warning when link-authorized material is
// outside the room KB selection (save still succeeds).
type AskDocsCoverageWarning struct {
	Code               string   `json:"code"`
	Message            string   `json:"message"`
	MissingFolderPaths []string `json:"missing_folder_paths,omitempty"`
	MissingDocumentIDs []string `json:"missing_document_ids,omitempty"`
}

// knowledgeBaseReader reads deal-room KB status for the Ask Docs save gate.
type knowledgeBaseReader interface {
	GetDealRoomKnowledgeBaseByRoom(ctx context.Context, roomID pgtype.UUID) (db.DealRoomKnowledgeBasis, error)
}

// ensureAskDocsKnowledgeBase rejects enabling Ask Docs on a deal-room link
// unless the room KB status is ready or stale. Non-deal-room links and
// Ask-Docs-off saves skip the gate.
func ensureAskDocsKnowledgeBase(ctx context.Context, q knowledgeBaseReader, dealRoomID pgtype.UUID, aiCopilotEnabled bool) error {
	if !aiCopilotEnabled || !dealRoomID.Valid {
		return nil
	}
	kb, err := q.GetDealRoomKnowledgeBaseByRoom(ctx, dealRoomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrKnowledgeBaseRequired
		}
		return err
	}
	switch kb.Status {
	case "ready", "stale":
		return nil
	default:
		return ErrKnowledgeBaseRequired
	}
}

// askDocsCoverageGaps returns a soft warning when authorized docs/folders are
// not covered by the KB checkbox selection. Nil means full coverage or N/A.
func askDocsCoverageGaps(authorized []authorizedDocument, kb db.DealRoomKnowledgeBasis) *AskDocsCoverageWarning {
	missingFolders := map[string]struct{}{}
	missingDocs := map[string]struct{}{}
	for _, doc := range authorized {
		if coveredByKBSelection(doc, kb) {
			continue
		}
		if doc.FolderPath != "" {
			missingFolders[doc.FolderPath] = struct{}{}
		} else {
			missingDocs[doc.ID.String()] = struct{}{}
		}
	}
	if len(missingFolders) == 0 && len(missingDocs) == 0 {
		return nil
	}
	folders := make([]string, 0, len(missingFolders))
	for p := range missingFolders {
		folders = append(folders, p)
	}
	docs := make([]string, 0, len(missingDocs))
	for id := range missingDocs {
		docs = append(docs, id)
	}
	return &AskDocsCoverageWarning{
		Code:               "ask_docs_scope_not_in_kb",
		Message:            "Some authorized folders or documents are outside the knowledge base selection; Ask Docs will only use the intersection.",
		MissingFolderPaths: folders,
		MissingDocumentIDs: docs,
	}
}

func coveredByKBSelection(doc authorizedDocument, kb db.DealRoomKnowledgeBasis) bool {
	folder := normalizeFolderPathForKB(doc.FolderPath)
	for _, scope := range kb.FolderPaths {
		scope = normalizeFolderPathForKB(scope)
		if folder == scope || strings.HasPrefix(folder, scope+"/") {
			return true
		}
	}
	for _, id := range kb.DocumentIds {
		if id.Valid && uuid.UUID(id.Bytes) == doc.ID {
			return true
		}
	}
	return false
}

func normalizeFolderPathForKB(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	return path
}

// AskDocsCoverageWarning returns a soft warning when Ask Docs is on and the
// link's authorized scope is not ⊆ the room KB selection. Nil if N/A or covered.
func (s *Service) AskDocsCoverageWarning(ctx context.Context, link db.Link) *AskDocsCoverageWarning {
	if !link.AiCopilotEnabled || !link.DealRoomID.Valid {
		return nil
	}
	kb, err := s.queries.GetDealRoomKnowledgeBaseByRoom(ctx, link.DealRoomID)
	if err != nil {
		return nil
	}
	docs, err := listAuthorizedDocuments(ctx, s.queries, link)
	if err != nil {
		return nil
	}
	return askDocsCoverageGaps(docs, kb)
}

