package coverage

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
)

const (
	maxForkItems = 50
	minForkItems = 1
)

var itemIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// PackView is the Owner-facing effective pack (builtin or room fork).
type PackView struct {
	PackID       string          `json:"pack_id"`
	PackVersion  string          `json:"pack_version"`
	BasePackID   string          `json:"base_pack_id"`
	Forked       bool            `json:"forked"`
	ForkRevision int             `json:"fork_revision,omitempty"`
	Items        []jobs.PackItem `json:"items"`
}

// PutPackRequest replaces the room fork with the given items.
type PutPackRequest struct {
	Items []jobs.PackItem `json:"items"`
}

// ValidateForkItems checks Owner-edited pack items (P2.1c).
func ValidateForkItems(items []jobs.PackItem) error {
	if len(items) < minForkItems {
		return fmt.Errorf("%w: at least one checklist item required", ErrInvalidInput)
	}
	if len(items) > maxForkItems {
		return fmt.Errorf("%w: at most %d checklist items", ErrInvalidInput, maxForkItems)
	}
	seen := make(map[string]struct{}, len(items))
	for i, it := range items {
		id := strings.TrimSpace(it.ID)
		if !itemIDPattern.MatchString(id) {
			return fmt.Errorf("%w: item[%d] id must match %s", ErrInvalidInput, i, itemIDPattern.String())
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%w: duplicate item id %q", ErrInvalidInput, id)
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(it.LabelEN) == "" && strings.TrimSpace(it.LabelZH) == "" {
			return fmt.Errorf("%w: item %s label required", ErrInvalidInput, id)
		}
		if strings.TrimSpace(it.QueryEN) == "" && strings.TrimSpace(it.QueryZH) == "" {
			return fmt.Errorf("%w: item %s query required", ErrInvalidInput, id)
		}
		switch vt := strings.TrimSpace(it.ValueType); vt {
		case "", ValueTypePercent, ValueTypeMoney, ValueTypeShare:
			// ok
		default:
			return fmt.Errorf("%w: item %s invalid value_type %q", ErrInvalidInput, id, vt)
		}
	}
	return nil
}

// NormalizeForkItems trims fields and clears unknown value_type whitespace.
func NormalizeForkItems(items []jobs.PackItem) []jobs.PackItem {
	out := make([]jobs.PackItem, 0, len(items))
	for _, it := range items {
		out = append(out, jobs.PackItem{
			ID:        strings.TrimSpace(it.ID),
			LabelEN:   strings.TrimSpace(it.LabelEN),
			LabelZH:   strings.TrimSpace(it.LabelZH),
			QueryEN:   strings.TrimSpace(it.QueryEN),
			QueryZH:   strings.TrimSpace(it.QueryZH),
			ValueType: strings.TrimSpace(it.ValueType),
		})
	}
	return out
}

func packFromItems(base jobs.Pack, version string, items []jobs.PackItem) jobs.Pack {
	return jobs.Pack{
		PackID:      base.PackID,
		PackVersion: version,
		Items:       items,
	}
}

func forkVersion(baseVersion string, revision int) string {
	baseVersion = strings.TrimSpace(baseVersion)
	if baseVersion == "" {
		baseVersion = "1"
	}
	return fmt.Sprintf("%s.f%d", baseVersion, revision)
}

func decodePackItems(raw []byte) ([]jobs.PackItem, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty pack items")
	}
	var items []jobs.PackItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// ValidItemInPack reports whether itemID exists in the given pack items.
func ValidItemInPack(pack jobs.Pack, itemID string) bool {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return false
	}
	for _, it := range pack.Items {
		if it.ID == itemID {
			return true
		}
	}
	return false
}

// ChipsFromPack returns visitor chips from an effective pack (labels only).
func ChipsFromPack(pack jobs.Pack, lang string) []Chip {
	out := make([]Chip, 0, len(pack.Items))
	for _, it := range pack.Items {
		label := it.LabelFor(lang)
		if strings.TrimSpace(label) == "" {
			continue
		}
		out = append(out, Chip{ItemID: it.ID, Label: label})
	}
	return out
}
