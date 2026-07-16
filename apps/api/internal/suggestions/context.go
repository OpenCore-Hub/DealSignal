package suggestions

import (
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
)

// Context is the trigger snapshot persisted with a suggestion/signal.
// It is also returned to the frontend inside the signal feed.
type Context struct {
	Opens           int      `json:"opens"`
	UniqueVisitors  int      `json:"uniqueVisitors"`
	DurationSeconds int      `json:"durationSeconds"`
	KeyPageCount    int      `json:"keyPageCount"`
	KeyPageTitles   []string `json:"keyPageTitles"`
	ContactName     string   `json:"contactName,omitempty"`
	ContactEmail    string   `json:"contactEmail,omitempty"`
	VisitorEmail    string   `json:"visitorEmail,omitempty"`
	DocumentTitle   string   `json:"documentTitle,omitempty"`
	// Question-specific fields.
	Question string `json:"question,omitempty"`
	Intent   string `json:"intent,omitempty"`
	Actor    string `json:"actor,omitempty"`
}

// ToJSONB marshals the context to a JSONB byte slice for sqlc.
func (c Context) ToJSONB() []byte {
	if c.IsZero() {
		return []byte("{}")
	}
	b, _ := json.Marshal(c)
	return b
}

// IsZero reports whether the context carries no meaningful data.
func (c Context) IsZero() bool {
	return c.Opens == 0 && c.UniqueVisitors == 0 && c.DurationSeconds == 0 &&
		c.KeyPageCount == 0 && len(c.KeyPageTitles) == 0 &&
		c.ContactName == "" && c.ContactEmail == "" && c.VisitorEmail == "" &&
		c.DocumentTitle == "" && c.Question == "" && c.Intent == "" && c.Actor == ""
}

// pgText returns a pgtype.Text from a string.
func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
