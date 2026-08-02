package jobs

import (
	"bytes"
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed financing_dd_v1.yaml ma_redflag_v1.yaml
var packFS embed.FS

// Pack is a versioned DD checklist definition (P2 / P2.2).
type Pack struct {
	PackID      string     `yaml:"pack_id" json:"pack_id"`
	PackVersion string     `yaml:"pack_version" json:"pack_version"`
	Items       []PackItem `yaml:"items" json:"items"`
}

// PackItem is one checklist row template.
type PackItem struct {
	ID        string `yaml:"id" json:"id"`
	LabelEN   string `yaml:"label_en" json:"label_en"`
	LabelZH   string `yaml:"label_zh" json:"label_zh"`
	QueryEN   string `yaml:"query_en" json:"query_en"`
	QueryZH   string `yaml:"query_zh" json:"query_zh"`
	ValueType string `yaml:"value_type,omitempty" json:"value_type,omitempty"`
}

// QueryFor returns the keyword query for the given UI language.
func (it PackItem) QueryFor(lang string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "zh") {
		if q := strings.TrimSpace(it.QueryZH); q != "" {
			return q
		}
	}
	if q := strings.TrimSpace(it.QueryEN); q != "" {
		return q
	}
	return strings.TrimSpace(it.QueryZH)
}

// LabelFor returns the display label for the given UI language.
func (it PackItem) LabelFor(lang string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "zh") {
		if l := strings.TrimSpace(it.LabelZH); l != "" {
			return l
		}
	}
	if l := strings.TrimSpace(it.LabelEN); l != "" {
		return l
	}
	return strings.TrimSpace(it.LabelZH)
}

// PackRegistry holds built-in packs by pack_id.
type PackRegistry struct {
	byID map[string]Pack
}

// LoadBuiltinPacks loads embedded YAML packs and validates them.
func LoadBuiltinPacks() (*PackRegistry, error) {
	files := []string{"financing_dd_v1.yaml", "ma_redflag_v1.yaml"}
	byID := make(map[string]Pack, len(files))
	for _, name := range files {
		raw, err := packFS.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		pack, err := parsePackYAML(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if err := validatePack(pack); err != nil {
			return nil, err
		}
		if _, ok := byID[pack.PackID]; ok {
			return nil, fmt.Errorf("duplicate pack_id %q", pack.PackID)
		}
		byID[pack.PackID] = pack
	}
	return &PackRegistry{byID: byID}, nil
}

// MustLoadBuiltinPacks panics if embedded packs fail to load (startup).
func MustLoadBuiltinPacks() *PackRegistry {
	reg, err := LoadBuiltinPacks()
	if err != nil {
		panic(err)
	}
	return reg
}

// Get returns a pack by id.
func (r *PackRegistry) Get(packID string) (Pack, bool) {
	if r == nil {
		return Pack{}, false
	}
	p, ok := r.byID[packID]
	return p, ok
}

// List returns built-in packs in stable order.
func (r *PackRegistry) List() []Pack {
	if r == nil {
		return nil
	}
	order := []string{FinancingDDV1, MARedflagV1}
	out := make([]Pack, 0, len(order))
	for _, id := range order {
		if p, ok := r.byID[id]; ok {
			out = append(out, p)
		}
	}
	return out
}

// FinancingDDV1 is the default P2 wedge pack id.
const FinancingDDV1 = "financing_dd_v1"

// MARedflagV1 is the P2.2 M&A red-flag pack id (built-in read-only).
const MARedflagV1 = "ma_redflag_v1"

// IsBuiltinPackID reports whether packID is a known built-in checklist.
func IsBuiltinPackID(packID string) bool {
	switch strings.TrimSpace(packID) {
	case FinancingDDV1, MARedflagV1:
		return true
	default:
		return false
	}
}

// ForkAllowed reports whether room-level Owner fork is allowed for packID.
func ForkAllowed(packID string) bool {
	return strings.TrimSpace(packID) == FinancingDDV1
}

func parsePackYAML(raw []byte) (Pack, error) {
	var pack Pack
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&pack); err != nil {
		return Pack{}, fmt.Errorf("parse pack yaml: %w", err)
	}
	return pack, nil
}

func validatePack(pack Pack) error {
	if strings.TrimSpace(pack.PackID) == "" {
		return fmt.Errorf("pack_id required")
	}
	if strings.TrimSpace(pack.PackVersion) == "" {
		return fmt.Errorf("pack_version required")
	}
	if len(pack.Items) == 0 {
		return fmt.Errorf("pack %s has no items", pack.PackID)
	}
	seen := make(map[string]struct{}, len(pack.Items))
	for _, it := range pack.Items {
		id := strings.TrimSpace(it.ID)
		if id == "" {
			return fmt.Errorf("pack %s: empty item id", pack.PackID)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("pack %s: duplicate item id %q", pack.PackID, id)
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(it.QueryEN) == "" && strings.TrimSpace(it.QueryZH) == "" {
			return fmt.Errorf("pack %s item %s: query required", pack.PackID, id)
		}
		for _, q := range []string{it.QueryEN, it.QueryZH} {
			if strings.ContainsAny(q, "？?") {
				return fmt.Errorf("pack %s item %s: query must be keyword string, not a question (D15/D17)", pack.PackID, id)
			}
		}
		if strings.TrimSpace(it.LabelEN) == "" && strings.TrimSpace(it.LabelZH) == "" {
			return fmt.Errorf("pack %s item %s: label required", pack.PackID, id)
		}
		switch vt := strings.TrimSpace(it.ValueType); vt {
		case "", "percent", "money", "share":
			// ok
		default:
			return fmt.Errorf("pack %s item %s: invalid value_type %q", pack.PackID, id, vt)
		}
	}
	switch pack.PackID {
	case FinancingDDV1:
		if len(pack.Items) != 20 {
			return fmt.Errorf("financing_dd_v1 must have exactly 20 items, got %d", len(pack.Items))
		}
	case MARedflagV1:
		if len(pack.Items) != 18 {
			return fmt.Errorf("ma_redflag_v1 must have exactly 18 items, got %d", len(pack.Items))
		}
	}
	return nil
}
