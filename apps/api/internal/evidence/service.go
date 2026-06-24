// Package evidence formats retrieved chunks into answer-ready context.
package evidence

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
)

// Formatter turns evidence into a prompt context block.
type Formatter struct{}

// NewFormatter creates an evidence formatter.
func NewFormatter() *Formatter {
	return &Formatter{}
}

// BuildContext serializes evidence for inclusion in an LLM prompt.
func (f *Formatter) BuildContext(evidence []search.Evidence) string {
	if len(evidence) == 0 {
		return "No relevant evidence was found in the workspace documents."
	}

	var b strings.Builder
	b.WriteString("Use the following evidence from workspace documents to answer the question. " +
		"Each item includes the page number, bounding box, and text. " +
		"If the evidence does not contain the answer, say you could not find a basis for the answer.\n\n")
	for i, e := range evidence {
		b.WriteString(fmt.Sprintf("[%d] Page %d", i+1, e.PageNumber))
		if len(e.Boxes) > 0 {
			b.WriteString(fmt.Sprintf(" %v", e.Boxes))
		}
		b.WriteString(fmt.Sprintf(": %s\n", strings.ReplaceAll(e.Quote, "\n", " ")))
	}
	return b.String()
}

// Marshal serializes evidence for API responses or database storage.
func Marshal(evidence []search.Evidence) ([]byte, error) {
	return json.Marshal(evidence)
}
