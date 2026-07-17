package signal

import (
	"encoding/json"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SignalItem returns the canonical JSON representation of a signal for clients.
// It is shared by the signal feed endpoint and the dashboard stats endpoint so
// the two surfaces stay in sync when new fields are added.
func SignalItem(s db.Signal) gin.H {
	item := gin.H{
		"id":          uuid.UUID(s.ID.Bytes).String(),
		"type":        s.Type,
		"subtype":     s.Subtype.String,
		"title":       s.Title,
		"description": s.Description,
		"explanation": s.Explanation,
		"suggestion":  s.Suggestion,
		"priority":    s.Priority,
		"createdAt":   s.CreatedAt.Time.Format(time.RFC3339),
	}
	if s.DocumentID.Valid {
		item["documentId"] = uuid.UUID(s.DocumentID.Bytes).String()
	}
	if s.ContactID.Valid {
		item["contactId"] = uuid.UUID(s.ContactID.Bytes).String()
	}
	if s.LinkID.Valid {
		item["linkId"] = uuid.UUID(s.LinkID.Bytes).String()
	}
	if ctx, ok := unmarshalJSONB[map[string]any](s.Context); ok && len(ctx) > 0 {
		item["context"] = ctx
	}
	if md, ok := unmarshalJSONB[map[string]string](s.Metadata); ok && len(md) > 0 {
		item["metadata"] = md
	}
	return item
}

// ActionItem returns the canonical JSON representation of an action item.
func ActionItem(a db.ActionItem) gin.H {
	return gin.H{
		"id":         uuid.UUID(a.ID.Bytes).String(),
		"signalId":   uuid.UUID(a.SignalID.Bytes).String(),
		"title":      a.Title,
		"impact":     a.Impact,
		"dueAt":      a.DueAt.Time.Format(time.RFC3339),
		"status":     a.Status,
		"actionType": a.ActionType,
		"createdAt":  a.CreatedAt.Time.Format(time.RFC3339),
		"updatedAt":  a.UpdatedAt.Time.Format(time.RFC3339),
	}
}

func unmarshalJSONB[T any](b []byte) (T, bool) {
	var v T
	if len(b) == 0 {
		return v, false
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return v, false
	}
	return v, true
}
