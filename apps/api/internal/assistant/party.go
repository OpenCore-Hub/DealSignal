package assistant

import (
	"regexp"
	"strings"
)

// Stable party slot values for Ask Docs (P1d / H7). Not a primary DocIntent.
const (
	PartyInvestor = "investor"
	PartyFounder  = "founder"
	PartyBuyer    = "buyer"
	PartySeller   = "seller"
	PartyGP       = "gp"
	PartyLP       = "lp"
)

type partyDictEntry struct {
	party   string
	needles []string
}

// Dictionary order: financing investor first (acceptance), then founder, M&A, fund.
var partyDictionary = []partyDictEntry{
	{PartyInvestor, []string{
		"preferred shareholder", "preferred stockholders", "preferred shareholders",
		"investors", "investor",
		"优先股股东", "投资人", "投资者",
	}},
	{PartyFounder, []string{
		"founders", "founder", "founding team",
		"创始团队", "创始人",
	}},
	{PartyBuyer, []string{
		"purchasers", "purchaser", "acquirers", "acquirer", "buyers", "buyer",
		"购买方", "买方",
	}},
	{PartySeller, []string{
		"vendors", "vendor", "sellers", "seller",
		"出售方", "卖方",
	}},
	{PartyGP, []string{
		"general partners", "general partner",
		"普通合伙人",
	}},
	{PartyLP, []string{
		"limited partners", "limited partner",
		"有限合伙人",
	}},
}

var (
	// Latin GP/LP tokens, including when glued to CJK (e.g. "GP有哪些").
	partyGPRE = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])gp(?:[^a-z0-9]|$)`)
	partyLPRE = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])lp(?:[^a-z0-9]|$)`)
	partyGPHanRE = regexp.MustCompile(`(?i)gp\p{Han}|\p{Han}gp(?:[^a-z0-9]|$)`)
	partyLPHanRE = regexp.MustCompile(`(?i)lp\p{Han}|\p{Han}lp(?:[^a-z0-9]|$)`)
)

func applyPartySlot(d IntentDecision, message string) IntentDecision {
	if party := extractParty(message); party != "" {
		d.Party = party
	}
	return d
}

// extractParty returns a stable party id from rule/dictionary match (empty if none).
func extractParty(message string) string {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return ""
	}
	lower := strings.ToLower(msg)

	bestParty := ""
	bestLen := 0
	for _, entry := range partyDictionary {
		for _, needle := range entry.needles {
			n := strings.TrimSpace(needle)
			if n == "" {
				continue
			}
			hit := false
			if isASCIIWordNeedle(n) {
				hit = strings.Contains(lower, strings.ToLower(n))
			} else {
				hit = strings.Contains(msg, n)
			}
			if !hit {
				continue
			}
			if len(n) > bestLen {
				bestLen = len(n)
				bestParty = entry.party
			}
		}
	}
	if bestParty != "" {
		return bestParty
	}
	if partyGPRE.MatchString(msg) || partyGPHanRE.MatchString(msg) {
		return PartyGP
	}
	if partyLPRE.MatchString(msg) || partyLPHanRE.MatchString(msg) {
		return PartyLP
	}
	return ""
}

func isASCIIWordNeedle(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

func partyFocusLabel(party string) string {
	switch party {
	case PartyInvestor:
		return "investor"
	case PartyFounder:
		return "founder"
	case PartyBuyer:
		return "buyer"
	case PartySeller:
		return "seller"
	case PartyGP:
		return "general partner (GP)"
	case PartyLP:
		return "limited partner (LP)"
	default:
		return party
	}
}
