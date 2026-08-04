package knowledge

import (
	"encoding/json"
	"strings"
)

const (
	sessionStateMaxEntities        = 24
	sessionStateMaxOpenQuestions   = 12
	sessionStateMaxCoverageHints   = 5
	sessionStateMaxHitIDsPerEntity = 8

	rewriteBasisState     = "state"
	rewriteBasisPriorOnly = "prior_only"
)

// SessionEntity is a provenanced name reused across turns (documents, parties, clauses).
type SessionEntity struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // document | clause | other
	FirstTurnID string   `json:"firstTurnId"`
	HitIDs      []string `json:"hitIds,omitempty"`
}

// SessionOpenQuestion tracks an unresolved desk question for later follow-ups.
type SessionOpenQuestion struct {
	Text         string `json:"text"`
	SourceTurnID string `json:"sourceTurnId"`
}

// SessionCoverageHint summarizes the coverage set from a recent turn.
type SessionCoverageHint struct {
	SourceNames []string `json:"sourceNames"`
	TurnID      string   `json:"turnId"`
}

// SessionState is the auditable desk state machine (ceiling §3.2).
// Rewrite may only consume this JSON plus the prior turn — never opaque chat memory.
type SessionState struct {
	Entities      []SessionEntity       `json:"entities,omitempty"`
	OpenQuestions []SessionOpenQuestion `json:"openQuestions,omitempty"`
	CoverageHints []SessionCoverageHint `json:"coverageHints,omitempty"`
}

func parseSessionState(raw []byte) SessionState {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return SessionState{}
	}
	var st SessionState
	if err := json.Unmarshal(raw, &st); err != nil {
		return SessionState{}
	}
	return normalizeSessionState(st)
}

func marshalSessionState(st SessionState) []byte {
	st = normalizeSessionState(st)
	b, err := json.Marshal(st)
	if err != nil {
		return []byte("{}")
	}
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

func normalizeSessionState(st SessionState) SessionState {
	st.Entities = st.Entities[:min(len(st.Entities), sessionStateMaxEntities)]
	st.OpenQuestions = st.OpenQuestions[:min(len(st.OpenQuestions), sessionStateMaxOpenQuestions)]
	st.CoverageHints = st.CoverageHints[:min(len(st.CoverageHints), sessionStateMaxCoverageHints)]
	return st
}

func sessionStateHasRewriteHints(st SessionState) bool {
	return len(st.Entities) > 0 || len(st.CoverageHints) > 0 || len(st.OpenQuestions) > 0
}

// evolveSessionState merges the completed turn into prior audited state.
// Deterministic — no LLM; source names / hit ids / open gaps only.
func evolveSessionState(prev SessionState, turn QATurn) SessionState {
	next := SessionState{
		Entities:      append([]SessionEntity(nil), prev.Entities...),
		OpenQuestions: append([]SessionOpenQuestion(nil), prev.OpenQuestions...),
		CoverageHints: append([]SessionCoverageHint(nil), prev.CoverageHints...),
	}

	for _, h := range turn.Hits {
		name := strings.TrimSpace(h.SourceName)
		if name == "" {
			continue
		}
		chunkID := strings.TrimSpace(h.ChunkID)
		upsertSessionEntity(&next, SessionEntity{
			Name:        name,
			Type:        "document",
			FirstTurnID: turn.ID,
			HitIDs:      nonEmptyStrings(chunkID),
		})
	}

	// Phase I3: definition/attachment anchors become clause entities for later hops.
	if turn.MultiHop != nil {
		for _, hq := range turn.MultiHop.Queries {
			anchor := strings.TrimSpace(hq.Anchor)
			if anchor == "" {
				continue
			}
			entType := "clause"
			if hq.Kind == multiHopKindAttachment {
				entType = "attachment"
			}
			upsertSessionEntity(&next, SessionEntity{
				Name:        anchor,
				Type:        entType,
				FirstTurnID: turn.ID,
				HitIDs:      append([]string(nil), hq.FromHitIDs...),
			})
		}
	}

	coverage := coverageSourceNames(turn.Hits, followUpCoverageMax)
	if len(coverage) > 0 && turn.ID != "" {
		next.CoverageHints = append(next.CoverageHints, SessionCoverageHint{
			SourceNames: coverage,
			TurnID:      turn.ID,
		})
		if len(next.CoverageHints) > sessionStateMaxCoverageHints {
			next.CoverageHints = next.CoverageHints[len(next.CoverageHints)-sessionStateMaxCoverageHints:]
		}
	}

	switch turn.ResultStatus {
	case "refused", "no_hits", "error":
		q := strings.TrimSpace(turn.Question)
		if q != "" && turn.ID != "" {
			upsertOpenQuestion(&next, SessionOpenQuestion{Text: q, SourceTurnID: turn.ID})
		}
	case "answered":
		resolveOpenQuestions(&next, turn.Question)
		// Phase J: only actionable unbound sentences become open gaps
		// (rejects markdown / mid-token debris from bad sentence splits).
		for _, gap := range turn.Unresolved {
			gap = strings.TrimSpace(gap)
			if gap == "" || turn.ID == "" || !isActionableUnresolvedGap(gap) {
				continue
			}
			upsertOpenQuestion(&next, SessionOpenQuestion{Text: gap, SourceTurnID: turn.ID})
		}
	}

	return normalizeSessionState(next)
}

func upsertSessionEntity(st *SessionState, e SessionEntity) {
	key := strings.ToLower(strings.TrimSpace(e.Name))
	if key == "" {
		return
	}
	for i := range st.Entities {
		if strings.ToLower(st.Entities[i].Name) != key {
			continue
		}
		// Preserve first sighting turn; merge hit ids.
		seen := map[string]struct{}{}
		var hits []string
		for _, id := range append(st.Entities[i].HitIDs, e.HitIDs...) {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			hits = append(hits, id)
			if len(hits) >= sessionStateMaxHitIDsPerEntity {
				break
			}
		}
		st.Entities[i].HitIDs = hits
		if st.Entities[i].Type == "" {
			st.Entities[i].Type = e.Type
		}
		return
	}
	if len(st.Entities) >= sessionStateMaxEntities {
		st.Entities = st.Entities[1:]
	}
	st.Entities = append(st.Entities, e)
}

func upsertOpenQuestion(st *SessionState, q SessionOpenQuestion) {
	key := strings.ToLower(strings.TrimSpace(q.Text))
	if key == "" {
		return
	}
	for _, existing := range st.OpenQuestions {
		if strings.ToLower(strings.TrimSpace(existing.Text)) == key {
			return
		}
	}
	if len(st.OpenQuestions) >= sessionStateMaxOpenQuestions {
		st.OpenQuestions = st.OpenQuestions[1:]
	}
	st.OpenQuestions = append(st.OpenQuestions, q)
}

// resolveOpenQuestions drops open items that share a distinctive token with the new question
// (user continued / answered that gap).
func resolveOpenQuestions(st *SessionState, question string) {
	tokens := distinctiveEvidenceTokens(strings.ToLower(question))
	if len(tokens) == 0 || len(st.OpenQuestions) == 0 {
		return
	}
	var kept []SessionOpenQuestion
	for _, oq := range st.OpenQuestions {
		ql := strings.ToLower(oq.Text)
		if containsAnyToken(ql, tokens) {
			continue
		}
		kept = append(kept, oq)
	}
	st.OpenQuestions = kept
}

func nonEmptyStrings(vals ...string) []string {
	var out []string
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// sessionStateRewriteSurface flattens state names for rewrite grounding / prompt.
func sessionStateRewriteSurface(st SessionState) string {
	var b strings.Builder
	for _, e := range st.Entities {
		b.WriteString(" ")
		b.WriteString(strings.ToLower(e.Name))
	}
	for _, h := range st.CoverageHints {
		for _, n := range h.SourceNames {
			b.WriteString(" ")
			b.WriteString(strings.ToLower(n))
		}
	}
	for _, oq := range st.OpenQuestions {
		b.WriteString(" ")
		b.WriteString(strings.ToLower(oq.Text))
	}
	return b.String()
}
