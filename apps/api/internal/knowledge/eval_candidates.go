package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	EvalReviewPending  = "pending"
	EvalReviewAccepted = "accepted"
	EvalReviewRejected = "rejected"

	EvalExpectRejectOrRebind = "reject_or_rebind"
	EvalExpectRefuseOrGround = "refuse_or_ground"

	evalCandidateListDefaultLimit = 50
	evalCandidateListMaxLimit     = 200
	evalSnapshotExcerptMaxRunes   = 280
)

// EvalHitSnapshot is a scrubbed hit for gold review (no visitor PII).
type EvalHitSnapshot struct {
	ChunkID    string `json:"chunkId,omitempty"`
	SourceName string `json:"sourceName,omitempty"`
	Pages      []int  `json:"pages,omitempty"`
	Sheet      string `json:"sheet,omitempty"`
	Excerpt    string `json:"excerpt,omitempty"`
}

// EvalCandidateSnapshot captures citation-binding context at feedback time.
type EvalCandidateSnapshot struct {
	Hits            []EvalHitSnapshot `json:"hits,omitempty"`
	Claims          []AnswerClaim     `json:"claims,omitempty"`
	Unresolved      []string          `json:"unresolved,omitempty"`
	Judgment        *JudgmentInfo     `json:"judgment,omitempty"`
	Refusal         *RefusalInfo      `json:"refusal,omitempty"`
	ExpectedSources []string          `json:"expectedSourceNames,omitempty"`
}

// EvalCandidate is the API shape for a feedback→eval row.
type EvalCandidate struct {
	ID                string                 `json:"id"`
	RoomID            string                 `json:"roomId"`
	TurnID            string                 `json:"turnId"`
	FeedbackKind      string                 `json:"feedbackKind"`
	Question          string                 `json:"question"`
	Answer            string                 `json:"answer,omitempty"`
	Note              string                 `json:"note,omitempty"`
	CorpusFingerprint string                 `json:"corpusFingerprint,omitempty"`
	ReviewStatus      string                 `json:"reviewStatus"`
	Expect            string                 `json:"expect,omitempty"`
	Snapshot          *EvalCandidateSnapshot `json:"snapshot,omitempty"`
	CreatedAt         time.Time              `json:"createdAt"`
	ReviewedAt        *time.Time             `json:"reviewedAt,omitempty"`
}

// EvalCandidateListResponse is a room-scoped candidate page.
type EvalCandidateListResponse struct {
	Items []EvalCandidate `json:"items"`
}

// ReviewEvalCandidateRequest accepts or rejects a pending candidate.
type ReviewEvalCandidateRequest struct {
	ReviewStatus string `json:"reviewStatus"`
	Expect       string `json:"expect,omitempty"`
}

// EvalSeedExport is seeds.json-shaped output for accepted gold (ceiling Phase O).
type EvalSeedExport struct {
	Description string          `json:"description"`
	Seeds       []EvalSeedEntry `json:"seeds"`
}

// EvalSeedEntry is one exportable / CI gold case.
type EvalSeedEntry struct {
	ID              string            `json:"id"`
	Kind            string            `json:"kind"`
	Question        string            `json:"question"`
	Answer          string            `json:"answer,omitempty"`
	Note            string            `json:"note,omitempty"`
	Expect          string            `json:"expect"`
	Hits            []EvalHitSnapshot `json:"hits,omitempty"`
	Claims          []AnswerClaim     `json:"claims,omitempty"`
	ExpectedSources []string          `json:"expectedSourceNames,omitempty"`
}

func buildEvalCandidateSnapshot(turn db.KnowledgeQaTurn) (EvalCandidateSnapshot, []byte) {
	mapped := mapQATurn(turn)
	snap := EvalCandidateSnapshot{
		Claims:     append([]AnswerClaim(nil), mapped.Claims...),
		Unresolved: append([]string(nil), mapped.Unresolved...),
		Judgment:   mapped.Judgment,
		Refusal:    mapped.Refusal,
	}
	for _, h := range mapped.Hits {
		snap.Hits = append(snap.Hits, EvalHitSnapshot{
			ChunkID:    h.ChunkID,
			SourceName: h.SourceName,
			Pages:      append([]int(nil), h.Pages...),
			Sheet:      h.Sheet,
			Excerpt:    truncateRunes(h.Text, evalSnapshotExcerptMaxRunes),
		})
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return snap, nil
	}
	return snap, raw
}

func (s *Service) recordEvalCandidate(
	ctx context.Context,
	roomID, workspaceID, userID string,
	turn db.KnowledgeQaTurn,
	kind, note string,
) {
	if s.queries == nil {
		return
	}
	_, snapJSON := buildEvalCandidateSnapshot(turn)
	_, err := s.queries.UpsertKnowledgeQAEvalCandidate(ctx, db.UpsertKnowledgeQAEvalCandidateParams{
		RoomID:            pgUUID(roomID),
		WorkspaceID:       pgUUID(workspaceID),
		TurnID:            turn.ID,
		FeedbackKind:      kind,
		Question:          turn.Question,
		Answer:            turn.Answer,
		Note:              pgtype.Text{String: note, Valid: note != ""},
		Snapshot:          snapJSON,
		CorpusFingerprint: turn.CorpusFingerprint,
		CreatedBy:         pgUUID(userID),
	})
	if err != nil {
		// Soft-fail: feedback already persisted; eval sampling must not break UX.
		return
	}
	recordKnowledgeQAEvalCandidate(kind)
}

// ListEvalCandidates returns room feedback candidates filtered by kind/status.
func (s *Service) ListEvalCandidates(
	ctx context.Context,
	roomID, workspaceID, userID, kind, status string,
	limit int,
) (EvalCandidateListResponse, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return EvalCandidateListResponse{}, err
	}
	if limit <= 0 {
		limit = evalCandidateListDefaultLimit
	}
	if limit > evalCandidateListMaxLimit {
		limit = evalCandidateListMaxLimit
	}
	kind = strings.TrimSpace(kind)
	status = strings.TrimSpace(status)
	if kind != "" && kind != FeedbackKindWrongCitation && kind != FeedbackKindNotAnswering {
		return EvalCandidateListResponse{}, fmt.Errorf("%w: invalid feedback kind filter", ErrInvalidInput)
	}
	if status != "" && !validEvalReviewStatus(status) {
		return EvalCandidateListResponse{}, fmt.Errorf("%w: invalid review status filter", ErrInvalidInput)
	}
	rows, err := s.queries.ListKnowledgeQAEvalCandidatesForRoom(ctx, db.ListKnowledgeQAEvalCandidatesForRoomParams{
		RoomID:       pgUUID(roomID),
		FeedbackKind: pgtype.Text{String: kind, Valid: kind != ""},
		ReviewStatus: pgtype.Text{String: status, Valid: status != ""},
		LimitN:       int32(limit),
	})
	if err != nil {
		return EvalCandidateListResponse{}, err
	}
	items := make([]EvalCandidate, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapEvalCandidateRow(
			row.ID, row.RoomID, row.TurnID, row.FeedbackKind, row.Question, row.Answer, row.Note,
			row.Snapshot, row.CorpusFingerprint, row.ReviewStatus, row.Expect, row.CreatedAt, row.ReviewedAt,
		))
	}
	return EvalCandidateListResponse{Items: items}, nil
}

// ReviewEvalCandidate marks a candidate accepted/rejected for gold promotion.
func (s *Service) ReviewEvalCandidate(
	ctx context.Context,
	roomID, workspaceID, userID, candidateID string,
	req ReviewEvalCandidateRequest,
) (EvalCandidate, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return EvalCandidate{}, err
	}
	if _, err := uuid.Parse(strings.TrimSpace(candidateID)); err != nil {
		return EvalCandidate{}, fmt.Errorf("%w: invalid candidate id", ErrInvalidInput)
	}
	status := strings.TrimSpace(req.ReviewStatus)
	if status != EvalReviewAccepted && status != EvalReviewRejected {
		return EvalCandidate{}, fmt.Errorf("%w: reviewStatus must be accepted or rejected", ErrInvalidInput)
	}
	expect := strings.TrimSpace(req.Expect)
	if status == EvalReviewAccepted {
		if expect == "" {
			// Default expect from feedback kind.
			row, gerr := s.queries.GetKnowledgeQAEvalCandidateForRoom(ctx, db.GetKnowledgeQAEvalCandidateForRoomParams{
				ID:     pgUUID(candidateID),
				RoomID: pgUUID(roomID),
			})
			if gerr != nil {
				if errors.Is(gerr, pgx.ErrNoRows) {
					return EvalCandidate{}, ErrNotFound
				}
				return EvalCandidate{}, gerr
			}
			expect = defaultExpectForFeedbackKind(row.FeedbackKind)
		}
		if !validEvalExpect(expect) {
			return EvalCandidate{}, fmt.Errorf("%w: invalid expect", ErrInvalidInput)
		}
	} else {
		expect = ""
	}

	row, err := s.queries.ReviewKnowledgeQAEvalCandidate(ctx, db.ReviewKnowledgeQAEvalCandidateParams{
		ReviewStatus: status,
		Expect:       pgtype.Text{String: expect, Valid: expect != ""},
		ReviewedBy:   pgUUID(userID),
		ID:           pgUUID(candidateID),
		RoomID:       pgUUID(roomID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvalCandidate{}, ErrNotFound
		}
		return EvalCandidate{}, err
	}
	return mapEvalCandidateRow(
		row.ID, row.RoomID, row.TurnID, row.FeedbackKind, row.Question, row.Answer, row.Note,
		row.Snapshot, row.CorpusFingerprint, row.ReviewStatus, row.Expect, row.CreatedAt, row.ReviewedAt,
	), nil
}

// ExportAcceptedEvalSeeds returns seeds.json-shaped accepted gold for the room.
func (s *Service) ExportAcceptedEvalSeeds(
	ctx context.Context,
	roomID, workspaceID, userID string,
	limit int,
) (EvalSeedExport, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return EvalSeedExport{}, err
	}
	if limit <= 0 {
		limit = evalCandidateListDefaultLimit
	}
	if limit > evalCandidateListMaxLimit {
		limit = evalCandidateListMaxLimit
	}
	rows, err := s.queries.ListAcceptedKnowledgeQAEvalCandidatesForRoom(ctx, db.ListAcceptedKnowledgeQAEvalCandidatesForRoomParams{
		RoomID:  pgUUID(roomID),
		LimitN:  int32(limit),
	})
	if err != nil {
		return EvalSeedExport{}, err
	}
	out := EvalSeedExport{
		Description: "Accepted knowledge desk eval seeds exported from room feedback (ceiling Phase O).",
		Seeds:       make([]EvalSeedEntry, 0, len(rows)),
	}
	for _, row := range rows {
		id := "cand_" + uuid.UUID(row.ID.Bytes).String()
		expect := textOrEmpty(row.Expect)
		if expect == "" {
			expect = defaultExpectForFeedbackKind(row.FeedbackKind)
		}
		entry := EvalSeedEntry{
			ID:       id,
			Kind:     row.FeedbackKind,
			Question: row.Question,
			Answer:   textOrEmpty(row.Answer),
			Note:     textOrEmpty(row.Note),
			Expect:   expect,
		}
		if snap := parseEvalSnapshot(row.Snapshot); snap != nil {
			entry.Hits = snap.Hits
			entry.Claims = snap.Claims
			entry.ExpectedSources = snap.ExpectedSources
		}
		out.Seeds = append(out.Seeds, entry)
	}
	return out, nil
}

func mapEvalCandidateRow(
	id, roomID, turnID pgtype.UUID,
	kind, question string,
	answer, note pgtype.Text,
	snapshot []byte,
	fp pgtype.Text,
	reviewStatus string,
	expect pgtype.Text,
	createdAt, reviewedAt pgtype.Timestamptz,
) EvalCandidate {
	out := EvalCandidate{
		ID:                uuid.UUID(id.Bytes).String(),
		RoomID:            uuid.UUID(roomID.Bytes).String(),
		TurnID:            uuid.UUID(turnID.Bytes).String(),
		FeedbackKind:      kind,
		Question:          question,
		Answer:            textOrEmpty(answer),
		Note:              textOrEmpty(note),
		CorpusFingerprint: textOrEmpty(fp),
		ReviewStatus:      reviewStatus,
		Expect:            textOrEmpty(expect),
		Snapshot:          parseEvalSnapshot(snapshot),
	}
	if createdAt.Valid {
		out.CreatedAt = createdAt.Time.UTC()
	}
	if reviewedAt.Valid {
		t := reviewedAt.Time.UTC()
		out.ReviewedAt = &t
	}
	return out
}

func parseEvalSnapshot(raw []byte) *EvalCandidateSnapshot {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var snap EvalCandidateSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil
	}
	return &snap
}

func validEvalReviewStatus(s string) bool {
	switch s {
	case EvalReviewPending, EvalReviewAccepted, EvalReviewRejected:
		return true
	default:
		return false
	}
}

func validEvalExpect(s string) bool {
	switch s {
	case EvalExpectRejectOrRebind, EvalExpectRefuseOrGround:
		return true
	default:
		return false
	}
}

func defaultExpectForFeedbackKind(kind string) string {
	switch kind {
	case FeedbackKindWrongCitation:
		return EvalExpectRejectOrRebind
	case FeedbackKindNotAnswering:
		return EvalExpectRefuseOrGround
	default:
		return ""
	}
}

// wrongCitationMismatch reports whether claims cite sources outside expectedSourceNames.
// Used by CI gold gate and review helpers (deterministic — no LLM).
func wrongCitationMismatch(hits []EvalHitSnapshot, claims []AnswerClaim, expectedSources []string) bool {
	if len(hits) == 0 || len(claims) == 0 || len(expectedSources) == 0 {
		return false
	}
	byID := make(map[string]string, len(hits))
	for _, h := range hits {
		id := strings.TrimSpace(h.ChunkID)
		if id == "" {
			continue
		}
		byID[id] = strings.TrimSpace(h.SourceName)
	}
	expected := map[string]struct{}{}
	for _, s := range expectedSources {
		s = strings.TrimSpace(s)
		if s != "" {
			expected[s] = struct{}{}
		}
	}
	if len(expected) == 0 || len(byID) == 0 {
		return false
	}
	for _, c := range claims {
		for _, hid := range c.HitIDs {
			src, ok := byID[strings.TrimSpace(hid)]
			if !ok || src == "" {
				continue
			}
			if _, good := expected[src]; !good {
				return true
			}
		}
	}
	return false
}

// claimHitIDsIntact ensures every claim hitId exists in the hit set (binding integrity).
func claimHitIDsIntact(hits []EvalHitSnapshot, claims []AnswerClaim) bool {
	if len(claims) == 0 {
		return true
	}
	ids := map[string]struct{}{}
	for _, h := range hits {
		id := strings.TrimSpace(h.ChunkID)
		if id != "" {
			ids[id] = struct{}{}
		}
	}
	for _, c := range claims {
		for _, hid := range c.HitIDs {
			hid = strings.TrimSpace(hid)
			if hid == "" {
				continue
			}
			if _, ok := ids[hid]; !ok {
				return false
			}
		}
	}
	return true
}
