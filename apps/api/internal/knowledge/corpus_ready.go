package knowledge

import (
	"context"
	"errors"
)

// ErrCorpusNotReady is returned when the room corpus cannot support grounded asks.
var ErrCorpusNotReady = errors.New("knowledge corpus not ready")

// displayCorpusStatusFromDocs mirrors FE displayCorpusStatus (heal stuck provisioning/syncing).
func displayCorpusStatusFromDocs(status string, docs []DocumentSyncItem) string {
	if status != "provisioning" && status != "syncing" {
		return status
	}
	if len(docs) == 0 {
		return status
	}
	for _, d := range docs {
		if d.Status == "pending" || d.Status == "syncing" {
			return status
		}
	}
	for _, d := range docs {
		if d.Status == "failed" {
			return "degraded"
		}
	}
	for _, d := range docs {
		if d.Status != "synced" {
			return status
		}
	}
	return "ready"
}

// corpusAskReady mirrors FE resolveCorpusAttentionStage(corpus) == "ready".
// Empty, building, degraded, or failed corpora must not burn answer quota.
func corpusAskReady(c CorpusStatus) bool {
	if !c.Enabled || len(c.Documents) == 0 {
		return false
	}
	status := displayCorpusStatusFromDocs(c.Status, c.Documents)
	status = reconcileCorpusStatus(status, c.Progress)

	if status == "degraded" || status == "failed" {
		return false
	}
	for _, d := range c.Documents {
		switch d.Status {
		case "pending", "syncing", "failed":
			return false
		}
	}
	jobBusy := c.Progress.JobStatus == "pending" || c.Progress.JobStatus == "running"
	if jobBusy && status != "ready" {
		return false
	}
	if status == "none" || status == "provisioning" || status == "syncing" {
		return false
	}
	if status != "ready" {
		return false
	}
	for _, d := range c.Documents {
		if d.Status == "synced" {
			return true
		}
	}
	return false
}

// enforceCorpusReady loads the room corpus snapshot and rejects asks when not ready.
func (s *Service) enforceCorpusReady(ctx context.Context, roomID, workspaceID, userID string) error {
	if s == nil {
		return nil
	}
	status, err := s.GetCorpus(ctx, roomID, workspaceID, userID)
	if err != nil {
		return err
	}
	if !corpusAskReady(status) {
		return ErrCorpusNotReady
	}
	return nil
}
