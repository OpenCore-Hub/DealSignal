package knowledge

import (
	"context"
	"errors"
	"net/http"
)

// knowledgeErrorBody is the stable JSON/SSE error contract for knowledge handlers.
type knowledgeErrorBody struct {
	Status  int
	Code    string
	Message string
}

func mapKnowledgeError(err error) knowledgeErrorBody {
	switch {
	case errors.Is(err, ErrUnavailable):
		return knowledgeErrorBody{http.StatusServiceUnavailable, "knowledge_unavailable", "knowledge base is not available"}
	case errors.Is(err, ErrForbidden):
		return knowledgeErrorBody{http.StatusForbidden, "forbidden", "forbidden"}
	case errors.Is(err, ErrNotFound):
		return knowledgeErrorBody{http.StatusNotFound, "not_found", "not found"}
	case errors.Is(err, ErrInvalidInput):
		return knowledgeErrorBody{http.StatusBadRequest, "invalid_input", "invalid input"}
	case errors.Is(err, ErrAnswerRequiresSession):
		return knowledgeErrorBody{http.StatusBadRequest, "answer_requires_session", "answer=true requires the session query API"}
	case errors.Is(err, ErrCorpusNotReady):
		return knowledgeErrorBody{http.StatusConflict, "knowledge_corpus_not_ready", "knowledge corpus is not ready for questions"}
	case errors.Is(err, ErrQueryBusy):
		return knowledgeErrorBody{http.StatusTooManyRequests, "knowledge_query_busy", "a question is already in progress"}
	case errors.Is(err, ErrQueryRateLimited):
		return knowledgeErrorBody{http.StatusTooManyRequests, "knowledge_query_rate_limited", "too many questions, please try again shortly"}
	case errors.Is(err, ErrQueryQuotaExceeded):
		return knowledgeErrorBody{http.StatusTooManyRequests, "knowledge_query_quota_exceeded", "answer quota for this plan is exhausted"}
	case errors.Is(err, ErrQueryQuotaCheckFailed):
		return knowledgeErrorBody{http.StatusServiceUnavailable, "knowledge_query_quota_unavailable", "answer quota could not be verified"}
	case errors.Is(err, errStreamClientCancelled):
		return knowledgeErrorBody{http.StatusOK, "client_cancelled", "client disconnected"}
	case errors.Is(err, context.Canceled):
		return knowledgeErrorBody{http.StatusOK, "client_cancelled", "client disconnected"}
	default:
		return knowledgeErrorBody{http.StatusInternalServerError, "internal_error", "internal error"}
	}
}
