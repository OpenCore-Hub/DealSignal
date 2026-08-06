package config

import "time"

const (
	// WriteDeadlineSlack covers audit/tokenization after upstream RAG returns.
	WriteDeadlineSlack = 45 * time.Second
	// WriteDeadlineMin is the floor for long-running knowledge asks / SSE streams.
	WriteDeadlineMin = 210 * time.Second
	// KnowledgeSSEKeepaliveInterval prevents proxy idle timeouts during retrieve.
	KnowledgeSSEKeepaliveInterval = 15 * time.Second
)

// DefaultHTTPWriteTimeout is the fallback HTTP write deadline when unset.
func DefaultHTTPWriteTimeout() time.Duration {
	return WriteDeadlineMin
}

// LongRunningWriteDeadline sizes per-flush SSE deadlines and the HTTP server
// WriteTimeout. It must exceed DOCLING_RAG_HTTP_TIMEOUT_MS plus headroom.
func LongRunningWriteDeadline(upstream time.Duration) time.Duration {
	if upstream <= 0 {
		return WriteDeadlineMin
	}
	d := upstream + WriteDeadlineSlack
	if d < WriteDeadlineMin {
		return WriteDeadlineMin
	}
	return d
}

// resolveHTTPWriteTimeout sizes the HTTP server write deadline for long-running
// knowledge asks and SSE streams. Overrides below the docling-derived minimum
// are clamped so misconfiguration cannot reintroduce 30s stream kills.
func resolveHTTPWriteTimeout(docling DoclingRAGConfig, overrideSeconds int) time.Duration {
	min := LongRunningWriteDeadline(docling.HTTPTimeout)
	if overrideSeconds > 0 {
		d := time.Duration(overrideSeconds) * time.Second
		if d < min {
			return min
		}
		return d
	}
	return min
}

// StreamWriteBudget sizes per-flush SSE write deadlines. It is the larger of
// the resolved server WriteTimeout and the docling-derived budget so operator
// overrides cannot leave flushes shorter than the HTTP server deadline.
func StreamWriteBudget(serverWrite, upstream time.Duration) time.Duration {
	fromUpstream := LongRunningWriteDeadline(upstream)
	if serverWrite > fromUpstream {
		return serverWrite
	}
	return fromUpstream
}
