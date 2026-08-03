package knowledge

import (
	"errors"
	"unicode"
	"unicode/utf8"
)

// SSE payloads for Knowledge Tab grounded chat (philosophy §5 / §6).
// Phase-2 stream: after blocking QueryWithSession, emit
// phase → sources? → token* → done. Upstream docling is still sync JSON;
// tokens are chunked from the audited answer so the desk grows honestly.
// Never emit grounded sources before refuse classification.

const defaultAnswerTokenRunes = 36

type streamPhasePayload struct {
	Phase string `json:"phase"`
}

type streamSourcesPayload struct {
	Results  []QueryHit `json:"results"`
	Grounded bool       `json:"grounded"`
}

type streamTokenPayload struct {
	Text string `json:"text"`
}

type streamDonePayload struct {
	SessionID    string     `json:"sessionId"`
	Turn         QATurn     `json:"turn"`
	Query        string     `json:"query"`
	Mode         string     `json:"mode"`
	Answer       string     `json:"answer,omitempty"`
	Results      []QueryHit `json:"results"`
	Refused      bool       `json:"refused"`
	ResultStatus string     `json:"resultStatus"`
}

type streamErrorPayload struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// answerTokenChunks splits an audited answer into SSE token payloads.
// Prefers whitespace boundaries near maxRunes; never drops or reorders runes.
func answerTokenChunks(answer string, maxRunes int) []string {
	if answer == "" {
		return nil
	}
	if maxRunes < 8 {
		maxRunes = 8
	}
	if utf8.RuneCountInString(answer) <= maxRunes {
		return []string{answer}
	}
	runes := []rune(answer)
	out := make([]string, 0, (len(runes)/maxRunes)+1)
	for i := 0; i < len(runes); {
		end := i + maxRunes
		if end >= len(runes) {
			out = append(out, string(runes[i:]))
			break
		}
		j := end
		minKeep := i + maxRunes/2
		for j > minKeep && !unicode.IsSpace(runes[j-1]) {
			j--
		}
		if j <= minKeep {
			j = end
		}
		out = append(out, string(runes[i:j]))
		i = j
	}
	return out
}

func streamErrorFrom(err error) streamErrorPayload {
	switch {
	case errors.Is(err, ErrUnavailable):
		return streamErrorPayload{Code: "knowledge_unavailable", Message: "knowledge base is not available"}
	case errors.Is(err, ErrForbidden):
		return streamErrorPayload{Code: "forbidden", Message: "forbidden"}
	case errors.Is(err, ErrNotFound):
		return streamErrorPayload{Code: "not_found", Message: "not found"}
	case errors.Is(err, ErrInvalidInput):
		return streamErrorPayload{Code: "invalid_input", Message: "invalid input"}
	case errors.Is(err, ErrQueryBusy):
		return streamErrorPayload{Code: "knowledge_query_busy", Message: "a question is already in progress"}
	case errors.Is(err, ErrQueryRateLimited):
		return streamErrorPayload{Code: "knowledge_query_rate_limited", Message: "too many questions, please try again shortly"}
	case errors.Is(err, ErrQueryQuotaExceeded):
		return streamErrorPayload{Code: "knowledge_query_quota_exceeded", Message: "answer quota for this plan is exhausted"}
	default:
		return streamErrorPayload{Code: "internal_error", Message: "internal error"}
	}
}

func shouldEmitGroundedSources(turn QATurn) bool {
	if turn.Refused || turn.ResultStatus == "refused" || turn.ResultStatus == "error" {
		return false
	}
	return len(turn.Hits) > 0
}

func shouldEmitAnswerTokens(answer string, turn QATurn) bool {
	if turn.ResultStatus == "error" {
		return false
	}
	return answer != ""
}
