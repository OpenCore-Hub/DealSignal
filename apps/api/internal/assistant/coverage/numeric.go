package coverage

import (
	"regexp"
	"sort"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
)

// Supported Pack value_type values (P2.1b).
const (
	ValueTypePercent = "percent"
	ValueTypeMoney   = "money"
	ValueTypeShare   = "share"
)

var (
	// percent: 15%, 15.5％, 15 percent, 百分之12.5
	rePercent = regexp.MustCompile(`(?i)(?:百分之\s*(\d{1,3}(?:[.,]\d{1,4})?))|(?:(\d{1,3}(?:[.,]\d{1,4})?)\s*(?:%|％|percent|pct|个百分点))`)
	// money: $1.2M, USD 1,200,000, ¥120万, RMB 1.2亿, 100万美元
	reMoney = regexp.MustCompile(`(?i)(?:(?:USD|US\$|\$|EUR|€|GBP|£|CNY|RMB|¥|￥)\s*)?(\d{1,3}(?:,\d{3})*(?:\.\d+)?|\d+(?:\.\d+)?)(\s*[KMBT万亿])?\s*(?:USD|US\$|\$|EUR|€|GBP|£|CNY|RMB|¥|￥|美元|人民币|元)?`)
	// share: 1,234,567 shares, 100万股, 10 million shares
	reShare = regexp.MustCompile(`(?i)(\d{1,3}(?:,\d{3})+|\d+(?:\.\d+)?)\s*(?:[KMB万亿])?\s*(?:shares?|股|fully\s+diluted)`)
)

// AttachExtractedValues sets value_type from the pack and, for supported rows with
// clues, fills extracted_value via deterministic regex (no table index / no LLM).
func AttachExtractedValues(rows []CoverageRow, pack jobs.Pack) []CoverageRow {
	if len(rows) == 0 {
		return rows
	}
	byID := make(map[string]jobs.PackItem, len(pack.Items))
	for _, it := range pack.Items {
		byID[it.ID] = it
	}
	out := make([]CoverageRow, len(rows))
	copy(out, rows)
	for i := range out {
		it, ok := byID[out[i].ItemID]
		if !ok {
			continue
		}
		vt := strings.TrimSpace(it.ValueType)
		if vt == "" {
			continue
		}
		out[i].ValueType = vt
		if out[i].Status != StatusSupported || len(out[i].Clues) == 0 {
			continue
		}
		if v, ok := extractValue(vt, out[i].Clues); ok {
			out[i].ExtractedValue = v
		}
	}
	return out
}

func extractValue(valueType string, clues []search.Evidence) (string, bool) {
	ordered := make([]search.Evidence, len(clues))
	copy(ordered, clues)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Score > ordered[j].Score
	})
	for _, c := range ordered {
		if v, ok := extractFromText(valueType, c.Quote); ok {
			return v, true
		}
	}
	return "", false
}

func extractFromText(valueType, text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	switch valueType {
	case ValueTypePercent:
		return extractPercent(text)
	case ValueTypeMoney:
		return extractMoney(text)
	case ValueTypeShare:
		return extractShare(text)
	default:
		return "", false
	}
}

func extractPercent(text string) (string, bool) {
	m := rePercent.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	raw := strings.TrimSpace(m[0])
	raw = collapseSpaces(raw)
	return raw, true
}

func extractMoney(text string) (string, bool) {
	all := reMoney.FindAllStringSubmatchIndex(text, -1)
	if len(all) == 0 {
		return "", false
	}
	// Prefer matches that include a currency marker or large magnitude suffix.
	best := ""
	bestScore := -1
	for _, loc := range all {
		raw := strings.TrimSpace(text[loc[0]:loc[1]])
		raw = collapseSpaces(raw)
		if !looksLikeMoney(raw) {
			continue
		}
		score := 0
		lower := strings.ToLower(raw)
		if strings.ContainsAny(raw, "$€£¥￥") ||
			strings.Contains(lower, "usd") ||
			strings.Contains(lower, "rmb") ||
			strings.Contains(lower, "cny") ||
			strings.Contains(lower, "eur") ||
			strings.Contains(lower, "gbp") ||
			strings.Contains(raw, "美元") ||
			strings.Contains(raw, "人民币") ||
			strings.Contains(raw, "元") {
			score += 2
		}
		if strings.ContainsAny(strings.ToUpper(raw), "KMBT") ||
			strings.Contains(raw, "万") ||
			strings.Contains(raw, "亿") ||
			strings.Contains(raw, ",") {
			score++
		}
		if score > bestScore {
			bestScore = score
			best = raw
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}

func looksLikeMoney(s string) bool {
	lower := strings.ToLower(s)
	if strings.ContainsAny(s, "$€£¥￥") {
		return true
	}
	if strings.Contains(lower, "usd") || strings.Contains(lower, "rmb") ||
		strings.Contains(lower, "cny") || strings.Contains(lower, "eur") ||
		strings.Contains(lower, "gbp") || strings.Contains(s, "美元") ||
		strings.Contains(s, "人民币") || strings.Contains(s, "元") {
		return true
	}
	// Bare numbers only count with magnitude suffix (avoid page numbers).
	return strings.ContainsAny(strings.ToUpper(s), "KMBT") ||
		strings.Contains(s, "万") ||
		strings.Contains(s, "亿")
}

func extractShare(text string) (string, bool) {
	m := reShare.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	raw := strings.TrimSpace(m[0])
	return collapseSpaces(raw), true
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
