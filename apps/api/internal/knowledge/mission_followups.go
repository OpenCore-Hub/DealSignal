package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge/missions"
	"github.com/jackc/pgx/v5"
)

const missionFollowUpMaxChips = 3

// buildMissionFollowUps produces checklist prompts from open gaps + mission pack.
// Composer chips no longer use this path (Phase Z); keep it for rail/tests/leak gates.
// Inputs are audited session state, the current turn, and an optional pack.
func buildMissionFollowUps(
	state SessionState,
	turn QATurn,
	pack *missions.Pack,
	loc string,
) []FollowUpSuggestion {
	var out []FollowUpSuggestion
	seen := map[string]struct{}{}
	add := func(id, text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		key := strings.ToLower(text)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, FollowUpSuggestion{ID: id, Text: text})
	}

	// 1) Provenanced open questions from the state machine.
	for i, oq := range state.OpenQuestions {
		if len(out) >= missionFollowUpMaxChips {
			break
		}
		if !isPromotableFollowUpText(oq.Text) {
			continue
		}
		add("mission-open-"+strconv.Itoa(i+1), oq.Text)
	}

	// 2) Unresolved bound-answer sentences from this turn (quality-gated).
	for i, u := range turn.Unresolved {
		if len(out) >= missionFollowUpMaxChips {
			break
		}
		if !isActionableUnresolvedGap(u) {
			continue
		}
		prompt := u
		if utf8.RuneCountInString(prompt) > 120 {
			prompt = truncateRunes(prompt, 120)
		}
		if loc == "zh-CN" {
			add("mission-unresolved-"+strconv.Itoa(i+1), "请在本室文档中核对："+prompt)
		} else {
			add("mission-unresolved-"+strconv.Itoa(i+1), "Verify in this room’s docs: "+prompt)
		}
	}

	// 3) Pack checklist items not yet covered by entities / recent Q.
	if pack != nil {
		covered := missionCoverageCorpus(state, turn)
		for _, item := range pack.Items {
			if len(out) >= missionFollowUpMaxChips {
				break
			}
			if missionItemCovered(item, covered) {
				continue
			}
			add("mission-"+item.ID, item.Prompts.For(loc))
		}
	}

	if len(out) > missionFollowUpMaxChips {
		out = out[:missionFollowUpMaxChips]
	}
	return out
}

// isPromotableFollowUpText rejects debris and non-room-fact meta that must never
// become a chip, while allowing short real room questions.
func isPromotableFollowUpText(text string) bool {
	t := strings.TrimSpace(text)
	n := utf8.RuneCountInString(t)
	if n < 6 || n > unresolvedGapMaxRunes {
		return false
	}
	if looksLikeMarkdownScaffold(t) || looksLikeBrokenFragment(t) {
		return false
	}
	// Red line: follow-ups (except template narrowing) stay inside room facts.
	if looksLikeNonRoomFactMeta(t) {
		return false
	}
	if looksLikeOutOfRoomGeneralKnowledge(t) {
		return false
	}
	return true
}

func missionCoverageCorpus(state SessionState, turn QATurn) string {
	var b strings.Builder
	b.WriteString(" ")
	b.WriteString(strings.ToLower(turn.Question))
	b.WriteString(" ")
	b.WriteString(strings.ToLower(turn.Answer))
	for _, e := range state.Entities {
		b.WriteString(" ")
		b.WriteString(strings.ToLower(e.Name))
	}
	for _, oq := range state.OpenQuestions {
		b.WriteString(" ")
		b.WriteString(strings.ToLower(oq.Text))
	}
	for _, h := range state.CoverageHints {
		for _, n := range h.SourceNames {
			b.WriteString(" ")
			b.WriteString(strings.ToLower(n))
		}
	}
	for _, c := range turn.Claims {
		b.WriteString(" ")
		b.WriteString(strings.ToLower(c.Text))
	}
	return b.String()
}

func missionItemCovered(item missions.Item, corpus string) bool {
	if corpus == "" {
		return false
	}
	// Asking the checklist prompt itself counts as covering the item (zh/en).
	for _, p := range []string{item.Prompts.EN, item.Prompts.ZhCN} {
		p = strings.ToLower(strings.TrimSpace(p))
		if utf8.RuneCountInString(p) >= 8 && strings.Contains(corpus, p) {
			return true
		}
	}
	hits := 0
	strong := false
	for _, kw := range item.Keywords {
		kw = strings.ToLower(strings.TrimSpace(kw))
		if kw == "" {
			continue
		}
		if strings.Contains(corpus, kw) {
			hits++
			if isStrongMissionKeyword(kw) {
				strong = true
			}
		}
	}
	if hits == 0 {
		return false
	}
	// One strong hit (CJK compound / longer EN token) is enough; short EN
	// tokens like "cap"/"pool" still need two hits to avoid false positives.
	if strong {
		return true
	}
	need := 2
	if len(item.Keywords) == 1 {
		need = 1
	}
	return hits >= need
}

func isStrongMissionKeyword(kw string) bool {
	for _, r := range kw {
		if r >= 0x4E00 && r <= 0x9FFF {
			return utf8.RuneCountInString(kw) >= 2
		}
	}
	return len(kw) >= 5
}

func (s *Service) resolveMissionPack(
	ctx context.Context,
	roomID, workspaceID string,
) (*missions.Pack, string, error) {
	if s.queries == nil {
		return nil, "", nil
	}
	row, err := s.queries.GetKnowledgeQARoomMission(ctx, pgUUID(roomID))
	if err == nil && strings.TrimSpace(row.PackID) != "" {
		if p, ok := missions.Get(row.PackID); ok {
			cp := p
			return &cp, p.ID, nil
		}
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, "", err
	}

	room, err := s.access.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		if p, ok := missions.Get(missions.FinancingDDV1); ok {
			cp := p
			return &cp, p.ID, nil
		}
		return nil, "", nil
	}
	templateType := ""
	if room.TemplateType.Valid {
		templateType = room.TemplateType.String
	}
	id := missions.DefaultForRoomTemplate(templateType)
	if p, ok := missions.Get(id); ok {
		cp := p
		return &cp, p.ID, nil
	}
	return nil, "", nil
}

// MissionPackInfo is the API shape for the room’s active mission pack.
type MissionPackInfo struct {
	PackID string            `json:"packId"`
	Title  string            `json:"title"`
	Source string            `json:"source"` // room | template_default | catalog
	Items  []MissionPackItem `json:"items,omitempty"`
}

// MissionPackItem is a checklist row for the desk.
type MissionPackItem struct {
	ID     string `json:"id"`
	Prompt string `json:"prompt"`
}

// MissionCatalogResponse lists builtin packs.
type MissionCatalogResponse struct {
	Items []MissionPackInfo `json:"items"`
}

// SetMissionPackRequest sets the room’s active mission pack.
type SetMissionPackRequest struct {
	PackID string `json:"packId"`
}

// GetRoomMissionPack returns the effective pack for a room.
func (s *Service) GetRoomMissionPack(
	ctx context.Context,
	roomID, workspaceID, userID, loc string,
) (MissionPackInfo, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return MissionPackInfo{}, err
	}
	pack, id, err := s.resolveMissionPack(ctx, roomID, workspaceID)
	if err != nil {
		return MissionPackInfo{}, err
	}
	if pack == nil {
		return MissionPackInfo{}, fmt.Errorf("%w: mission pack unavailable", ErrUnavailable)
	}
	source := "template_default"
	if _, gerr := s.queries.GetKnowledgeQARoomMission(ctx, pgUUID(roomID)); gerr == nil {
		source = "room"
	}
	return missionPackInfo(*pack, id, source, loc), nil
}

// ListMissionPacks returns builtin catalog entries.
func (s *Service) ListMissionPacks(ctx context.Context, roomID, workspaceID, userID, loc string) (MissionCatalogResponse, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return MissionCatalogResponse{}, err
	}
	list, err := missions.List()
	if err != nil {
		return MissionCatalogResponse{}, err
	}
	items := make([]MissionPackInfo, 0, len(list))
	for _, p := range list {
		items = append(items, missionPackInfo(p, p.ID, "catalog", loc))
	}
	return MissionCatalogResponse{Items: items}, nil
}

// SetRoomMissionPack binds a builtin pack to the room.
func (s *Service) SetRoomMissionPack(
	ctx context.Context,
	roomID, workspaceID, userID, loc string,
	req SetMissionPackRequest,
) (MissionPackInfo, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return MissionPackInfo{}, err
	}
	packID := strings.TrimSpace(req.PackID)
	if !missions.IsBuiltin(packID) {
		return MissionPackInfo{}, fmt.Errorf("%w: unknown mission pack", ErrInvalidInput)
	}
	if _, err := s.access.GetRoom(ctx, roomID, workspaceID); err != nil {
		return MissionPackInfo{}, err
	}
	_, err := s.queries.UpsertKnowledgeQARoomMission(ctx, db.UpsertKnowledgeQARoomMissionParams{
		RoomID:      pgUUID(roomID),
		WorkspaceID: pgUUID(workspaceID),
		PackID:      packID,
		UpdatedBy:   pgUUID(userID),
	})
	if err != nil {
		return MissionPackInfo{}, err
	}
	p, _ := missions.Get(packID)
	return missionPackInfo(p, packID, "room", loc), nil
}

func missionPackInfo(p missions.Pack, id, source, loc string) MissionPackInfo {
	items := make([]MissionPackItem, 0, len(p.Items))
	for _, it := range p.Items {
		items = append(items, MissionPackItem{ID: it.ID, Prompt: it.Prompts.For(loc)})
	}
	return MissionPackInfo{
		PackID: id,
		Title:  p.Title.For(loc),
		Source: source,
		Items:  items,
	}
}
