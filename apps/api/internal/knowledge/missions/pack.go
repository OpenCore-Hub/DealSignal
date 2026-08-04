package missions

import (
	"embed"
	"fmt"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Builtin pack IDs — one per deal-room template scenario.
const (
	FinancingDDV1   = "financing_dd_v1"
	FirstFundV1     = "first_fund_v1"
	MARedFlagV1     = "ma_redflag_v1"
	SeriesAPlusV1   = "series_a_plus_v1"
	RealEstateV1    = "real_estate_v1"
	FundMgmtV1      = "fund_mgmt_v1"
	PortfolioMgmtV1 = "portfolio_mgmt_v1"
	ProjectMgmtV1   = "project_mgmt_v1"
	SalesDataroomV1 = "sales_dataroom_v1"
)

// catalogOrder is the stable UI / API order (aligned with dealroom roomTemplates).
var catalogOrder = []string{
	FinancingDDV1,
	FirstFundV1,
	MARedFlagV1,
	SeriesAPlusV1,
	RealEstateV1,
	FundMgmtV1,
	PortfolioMgmtV1,
	ProjectMgmtV1,
	SalesDataroomV1,
}

//go:embed *.yaml
var packFS embed.FS

// LocalizedString is en / zh-CN copy.
type LocalizedString struct {
	EN   string `yaml:"en" json:"en"`
	ZhCN string `yaml:"zh-CN" json:"zh-CN"`
}

func (l LocalizedString) For(loc string) string {
	if loc == "zh-CN" && strings.TrimSpace(l.ZhCN) != "" {
		return l.ZhCN
	}
	if strings.TrimSpace(l.EN) != "" {
		return l.EN
	}
	return l.ZhCN
}

// Item is one diligence checklist prompt in a mission pack.
type Item struct {
	ID       string          `yaml:"id" json:"id"`
	Keywords []string        `yaml:"keywords" json:"keywords"`
	Prompts  LocalizedString `yaml:"prompts" json:"prompts"`
}

// Pack is a builtin room mission pack for knowledge follow-ups.
type Pack struct {
	ID    string          `yaml:"id" json:"id"`
	Title LocalizedString `yaml:"title" json:"title"`
	Items []Item          `yaml:"items" json:"items"`
}

var (
	loadOnce sync.Once
	loadErr  error
	byID     map[string]Pack
	ordered  []Pack
)

func load() {
	loadOnce.Do(func() {
		byID = map[string]Pack{}
		entries, err := packFS.ReadDir(".")
		if err != nil {
			loadErr = err
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			raw, err := packFS.ReadFile(e.Name())
			if err != nil {
				loadErr = err
				return
			}
			var p Pack
			if err := yaml.Unmarshal(raw, &p); err != nil {
				loadErr = fmt.Errorf("%s: %w", e.Name(), err)
				return
			}
			p.ID = strings.TrimSpace(p.ID)
			if p.ID == "" {
				loadErr = fmt.Errorf("%s: missing id", e.Name())
				return
			}
			if len(p.Items) == 0 {
				loadErr = fmt.Errorf("%s: empty items", e.Name())
				return
			}
			for i, item := range p.Items {
				if strings.TrimSpace(item.ID) == "" {
					loadErr = fmt.Errorf("%s: item %d missing id", e.Name(), i)
					return
				}
				if strings.TrimSpace(item.Prompts.EN) == "" || strings.TrimSpace(item.Prompts.ZhCN) == "" {
					loadErr = fmt.Errorf("%s: item %s missing en/zh-CN prompts", e.Name(), item.ID)
					return
				}
			}
			byID[p.ID] = p
		}
		if len(byID) == 0 {
			loadErr = fmt.Errorf("no mission packs embedded")
			return
		}
		// Stable catalog order; append any unexpected packs last.
		seen := map[string]struct{}{}
		for _, id := range catalogOrder {
			p, ok := byID[id]
			if !ok {
				loadErr = fmt.Errorf("catalog missing pack %s", id)
				return
			}
			ordered = append(ordered, p)
			seen[id] = struct{}{}
		}
		for id, p := range byID {
			if _, ok := seen[id]; ok {
				continue
			}
			ordered = append(ordered, p)
		}
	})
}

// List returns builtin packs in catalog order.
func List() ([]Pack, error) {
	load()
	if loadErr != nil {
		return nil, loadErr
	}
	out := make([]Pack, len(ordered))
	copy(out, ordered)
	return out, nil
}

// Get returns a builtin pack by id.
func Get(id string) (Pack, bool) {
	load()
	if loadErr != nil {
		return Pack{}, false
	}
	p, ok := byID[strings.TrimSpace(id)]
	return p, ok
}

// IsBuiltin reports whether id is a known pack.
func IsBuiltin(id string) bool {
	_, ok := Get(id)
	return ok
}

// DefaultForRoomTemplate maps a deal-room template/scenario to a mission pack.
// More specific scenario needles are matched before generic "fund" / "startup" tokens.
func DefaultForRoomTemplate(templateType string) string {
	t := strings.ToLower(strings.TrimSpace(templateType))
	t = strings.ReplaceAll(t, "_", "-")
	switch {
	case strings.Contains(t, "ma-acquisition") || strings.Contains(t, "acquisition") ||
		strings.Contains(t, "merger"):
		return MARedFlagV1
	case strings.Contains(t, "raising-first-fund") || strings.Contains(t, "first-fund"):
		return FirstFundV1
	case strings.Contains(t, "series-a"):
		return SeriesAPlusV1
	case strings.Contains(t, "real-estate"):
		return RealEstateV1
	case strings.Contains(t, "fund-management"):
		return FundMgmtV1
	case strings.Contains(t, "portfolio"):
		return PortfolioMgmtV1
	case strings.Contains(t, "project"):
		return ProjectMgmtV1
	case strings.Contains(t, "sales"):
		return SalesDataroomV1
	case strings.Contains(t, "fundraising") || strings.Contains(t, "startup"):
		return FinancingDDV1
	default:
		return FinancingDDV1
	}
}
