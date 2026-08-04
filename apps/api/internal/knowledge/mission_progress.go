package knowledge

import (
	"context"
	"fmt"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge/missions"
	"github.com/google/uuid"
)

// MissionProgressItem is one checklist row with coverage against audited session state.
type MissionProgressItem struct {
	ID      string `json:"id"`
	Prompt  string `json:"prompt"`
	Covered bool   `json:"covered"`
}

// MissionProgress is the room mission pack scored against a session (ceiling Phase N).
type MissionProgress struct {
	PackID  string                `json:"packId"`
	Title   string                `json:"title"`
	Source  string                `json:"source"` // room | template_default
	Covered int                   `json:"covered"`
	Total   int                   `json:"total"`
	Items   []MissionProgressItem `json:"items"`
}

// buildMissionProgress scores pack items against audited session state + all turns.
// Coverage accumulates across the session (not only the latest question).
func buildMissionProgress(pack missions.Pack, source, loc string, state SessionState, turns []QATurn) MissionProgress {
	corpus := missionCoverageCorpusAll(state, turns)
	items := make([]MissionProgressItem, 0, len(pack.Items))
	covered := 0
	for _, it := range pack.Items {
		ok := missionItemCovered(it, corpus)
		if ok {
			covered++
		}
		items = append(items, MissionProgressItem{
			ID:      it.ID,
			Prompt:  it.Prompts.For(loc),
			Covered: ok,
		})
	}
	return MissionProgress{
		PackID:  pack.ID,
		Title:   pack.Title.For(loc),
		Source:  source,
		Covered: covered,
		Total:   len(items),
		Items:   items,
	}
}

// missionCoverageCorpusAll joins durable state with every turn’s Q/A/claims.
func missionCoverageCorpusAll(state SessionState, turns []QATurn) string {
	var b strings.Builder
	b.WriteString(missionCoverageCorpus(state, QATurn{}))
	for _, turn := range turns {
		b.WriteString(missionCoverageCorpus(SessionState{}, turn))
	}
	return b.String()
}

// GetMissionProgress returns checklist coverage for the room pack vs an optional session.
func (s *Service) GetMissionProgress(
	ctx context.Context,
	roomID, workspaceID, userID, sessionID, loc string,
) (MissionProgress, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return MissionProgress{}, err
	}
	pack, packID, err := s.resolveMissionPack(ctx, roomID, workspaceID)
	if err != nil {
		return MissionProgress{}, err
	}
	if pack == nil {
		return MissionProgress{}, fmt.Errorf("%w: mission pack unavailable", ErrUnavailable)
	}
	source := "template_default"
	if s.queries != nil {
		if _, gerr := s.queries.GetKnowledgeQARoomMission(ctx, pgUUID(roomID)); gerr == nil {
			source = "room"
		}
	}

	var state SessionState
	var turns []QATurn
	sessionID = strings.TrimSpace(sessionID)
	if sessionID != "" {
		if _, err := uuid.Parse(sessionID); err != nil {
			return MissionProgress{}, fmt.Errorf("%w: invalid sessionId", ErrInvalidInput)
		}
		detail, err := s.GetSession(ctx, roomID, workspaceID, userID, sessionID)
		if err != nil {
			return MissionProgress{}, err
		}
		if detail.Session.State != nil {
			state = *detail.Session.State
		}
		turns = detail.Turns
	}

	out := buildMissionProgress(*pack, source, loc, state, turns)
	if out.PackID == "" {
		out.PackID = packID
	}
	return out, nil
}
