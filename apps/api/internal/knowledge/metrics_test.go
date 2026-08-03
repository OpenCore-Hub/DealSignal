package knowledge

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordKnowledgeQATurn(t *testing.T) {
	// Not parallel: shared process-wide Prometheus registry.
	before := testutil.ToFloat64(knowledgeQATurnsTotal.WithLabelValues("answered", "stream"))
	recordKnowledgeQATurn("answered", "stream", time.Now().Add(-10*time.Millisecond))
	after := testutil.ToFloat64(knowledgeQATurnsTotal.WithLabelValues("answered", "stream"))
	if after < before+1 {
		t.Fatalf("turns counter did not increase: before=%v after=%v", before, after)
	}
}

func TestRecordKnowledgeQAStreamErrorAndFeedback(t *testing.T) {
	beforeErr := testutil.ToFloat64(knowledgeQAStreamErrorsTotal.WithLabelValues("forbidden"))
	recordKnowledgeQAStreamError("forbidden")
	if testutil.ToFloat64(knowledgeQAStreamErrorsTotal.WithLabelValues("forbidden")) < beforeErr+1 {
		t.Fatal("stream error counter")
	}

	beforeFB := testutil.ToFloat64(knowledgeQAFeedbackTotal.WithLabelValues("helpful"))
	recordKnowledgeQAFeedback("helpful")
	if testutil.ToFloat64(knowledgeQAFeedbackTotal.WithLabelValues("helpful")) < beforeFB+1 {
		t.Fatal("feedback counter")
	}
}

func TestRecordKnowledgeQACiteOpen(t *testing.T) {
	before := testutil.ToFloat64(knowledgeQACiteOpensTotal.WithLabelValues("refused"))
	recordKnowledgeQACiteOpen("refused")
	if testutil.ToFloat64(knowledgeQACiteOpensTotal.WithLabelValues("refused")) < before+1 {
		t.Fatal("cite open counter")
	}
}

func TestRecordKnowledgeQARetention(t *testing.T) {
	before := testutil.ToFloat64(knowledgeQARetentionDeletedTotal)
	recordKnowledgeQARetentionDeleted(0)
	if testutil.ToFloat64(knowledgeQARetentionDeletedTotal) != before {
		t.Fatal("zero delete should not increment")
	}
	recordKnowledgeQARetentionDeleted(3)
	if testutil.ToFloat64(knowledgeQARetentionDeletedTotal) < before+3 {
		t.Fatal("deleted counter")
	}
	beforeErr := testutil.ToFloat64(knowledgeQARetentionErrorsTotal)
	recordKnowledgeQARetentionError()
	if testutil.ToFloat64(knowledgeQARetentionErrorsTotal) < beforeErr+1 {
		t.Fatal("retention error counter")
	}
}
