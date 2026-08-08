package radar

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// LinkMeta carries display fields resolved for a link.
type LinkMeta struct {
	ID         string
	Name       string
	DealRoomID string
	DocumentID string
}

// CompileInput is the pure-compiler input (no I/O).
type CompileInput struct {
	WorkspaceSlug string
	Now           time.Time
	Circle        heat.Circle
	Actions       []db.ActionItem
	Signals       []db.Signal
	Links         map[string]LinkMeta // linkID → meta
	Rooms         map[string]string   // roomID → name
	Metrics       map[string]LinkMetrics24h
	// OutcomeDemote soft-demotes products with high false_positive rates (from LearnFromOutcomes).
	OutcomeDemote map[Product]int
	NoiseHints    []NoiseHint
}

// EvidenceChip is a structured evidence marker for FE i18n (no free-text labels).
type EvidenceChip struct {
	Kind  string `json:"kind"`
	Count int    `json:"count,omitempty"`
}

// WorkItem is one productized radar card.
type WorkItem struct {
	ID            string         `json:"id"`
	Product       Product        `json:"product"`
	Headline      string         `json:"headline"`
	Subtitle      string         `json:"subtitle"`
	Actor         string         `json:"actor,omitempty"`
	Verb          Verb           `json:"verb"`
	Priority      Priority       `json:"priority"`
	Confidence    Confidence     `json:"confidence,omitempty"`
	SlaDueAt      string         `json:"slaDueAt"`
	CreatedAt     string         `json:"createdAt"`
	DealKey       string         `json:"dealKey"`
	DealName      string         `json:"dealName"`
	DealRoomID    string         `json:"dealRoomId,omitempty"`
	LinkID        string         `json:"linkId,omitempty"`
	DocumentID    string         `json:"documentId,omitempty"`
	ContactID     string         `json:"contactId,omitempty"`
	ActionID      string         `json:"actionId"`
	SignalID      string         `json:"signalId,omitempty"`
	NavigatePath  string         `json:"navigatePath,omitempty"`
	EvidencePath  string         `json:"evidencePath,omitempty"`
	ContactEmail  string         `json:"contactEmail,omitempty"`
	DocumentTitle string         `json:"documentTitle,omitempty"`
	CoalescedFrom []string       `json:"coalescedFrom,omitempty"`
	WhyNowCode    string         `json:"whyNowCode,omitempty"`
	WhyNowHours   int            `json:"whyNowHours,omitempty"`
	Evidence      []EvidenceChip `json:"evidence,omitempty"`
	State         string         `json:"state"`
}

// Strand groups work items under one deal surface.
type Strand struct {
	DealKey    string     `json:"dealKey"`
	DealName   string     `json:"dealName"`
	DealRoomID string     `json:"dealRoomId,omitempty"`
	Items      []WorkItem `json:"items"`
}

// Feed is the compiled radar response body.
type Feed struct {
	NextUp       *WorkItem      `json:"nextUp"`
	Strands      []Strand       `json:"strands"`
	Items        []WorkItem     `json:"items"`
	ClearedToday int            `json:"clearedToday"`
	Counts       map[string]int `json:"counts"`
	Lens         string         `json:"lens"`
	NoiseHints   []NoiseHint    `json:"noiseHints,omitempty"`
}

type draft struct {
	item      WorkItem
	created   time.Time
	slaDue    time.Time
	coalesceK string
	rankBoost int // added to productRank (soft demote)
	microRank int // within-product tie-break (higher first)
}

// Compile turns pending actions (+ linked signals) into a ranked radar feed.
// Bounce risk_alert items are never emitted.
func Compile(in CompileInput) Feed {
	now := in.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	sigByID := make(map[string]db.Signal, len(in.Signals))
	for _, s := range in.Signals {
		sigByID[uuidString(s.ID)] = s
	}

	drafts := make([]draft, 0, len(in.Actions))
	clearedToday := 0
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	for _, a := range in.Actions {
		if a.Status == "done" && a.UpdatedAt.Valid && !a.UpdatedAt.Time.Before(dayStart) {
			clearedToday++
		}
		if a.Status != "pending" {
			continue
		}

		var sig *db.Signal
		if a.SignalID.Valid {
			if s, ok := sigByID[uuidString(a.SignalID)]; ok {
				sig = &s
			}
		}
		if sig != nil && strings.EqualFold(sig.Subtype.String, suggestions.SubtypeBounce) {
			continue
		}

		product, verb, ok := classify(a, sig)
		if !ok {
			continue
		}

		item := buildItem(in, a, sig, product, verb, now)
		created := a.CreatedAt.Time
		if !a.CreatedAt.Valid {
			created = now
		}
		slaDue := parseRFC3339(item.SlaDueAt, slaDueAt(product, created, now))
		subtype := ""
		if sig != nil {
			subtype = sig.Subtype.String
		}
		d := draft{
			item:      item,
			created:   created,
			slaDue:    slaDue,
			coalesceK: coalesceKey(product, item.DealKey, item.ContactID, item.LinkID, item.Actor),
			microRank: microRank(product, isFormalAsk(a), subtype, sig != nil && reasonLooksLikeAskEscalation(sig)),
		}
		applyLeakConfidence(&d, in.Metrics, now)
		if boost := in.OutcomeDemote[product]; boost > 0 {
			d.rankBoost += boost
		}
		drafts = append(drafts, d)
	}

	circle := in.Circle
	if circle == "" {
		circle = heat.CircleDefault
	}

	merged := coalesce(drafts, now)
	sort.SliceStable(merged, func(i, j int) bool {
		// Spec Rank: SLA overdue first, then product band / priority / ties.
		oi, oj := merged[i].slaDue.Before(now), merged[j].slaDue.Before(now)
		if oi != oj {
			return oi
		}
		pi := productRankForCircle(circle, merged[i].item.Product) + merged[i].rankBoost
		pj := productRankForCircle(circle, merged[j].item.Product) + merged[j].rankBoost
		if pi != pj {
			return pi < pj
		}
		ri, rj := priorityRank(merged[i].item.Priority), priorityRank(merged[j].item.Priority)
		if ri != rj {
			return ri > rj
		}
		// Outcome demotion loses same-band ties (noise should not steal Next Up).
		if merged[i].rankBoost != merged[j].rankBoost {
			return merged[i].rankBoost < merged[j].rankBoost
		}
		if ci, cj := confidenceRank(merged[i].item.Confidence), confidenceRank(merged[j].item.Confidence); ci != cj {
			return ci > cj
		}
		if merged[i].microRank != merged[j].microRank {
			return merged[i].microRank > merged[j].microRank
		}
		if !merged[i].slaDue.Equal(merged[j].slaDue) {
			return merged[i].slaDue.Before(merged[j].slaDue)
		}
		return merged[i].created.After(merged[j].created)
	})

	items := make([]WorkItem, 0, len(merged))
	for _, d := range merged {
		finalizeWorkItem(&d.item, d, now)
		items = append(items, d.item)
	}

	var nextUp *WorkItem
	if len(items) > 0 {
		cp := items[0]
		nextUp = &cp
	}

	strands := buildStrands(items)
	counts := map[string]int{"all": len(items)}
	for _, p := range AllProducts {
		counts[string(p)] = 0
	}
	for _, it := range items {
		counts[string(it.Product)]++
	}

	return Feed{
		NextUp:       nextUp,
		Strands:      strands,
		Items:        items,
		ClearedToday: clearedToday,
		Counts:       counts,
		Lens:         string(circle),
		NoiseHints:   in.NoiseHints,
	}
}

func confidenceRank(c Confidence) int {
	switch c {
	case ConfidenceHigh:
		return 3
	case ConfidenceMedium:
		return 2
	case ConfidenceLow:
		return 1
	default:
		return 0
	}
}

func classify(a db.ActionItem, sig *db.Signal) (Product, Verb, bool) {
	src := textOrEmpty(a.SourceType)
	actType := a.ActionType

	if isApprove(a) {
		return ProductDiligenceGate, VerbApprove, true
	}
	if isAsk(a) {
		return ProductCommitmentAsk, VerbReply, true
	}
	if actType == "renew" || src == action.SourceTypeExpiringLink || src == action.SourceTypeExpiringRoom {
		return ProductAccessDecay, VerbRenew, true
	}

	if sig != nil {
		subtype := sig.Subtype.String
		switch sig.Type {
		case "risk_alert":
			switch subtype {
			case suggestions.SubtypeBounce:
				return "", "", false
			case suggestions.SubtypeForward, suggestions.SubtypeDownload,
				suggestions.SubtypeBlockedAttempt, suggestions.SubtypeCaptureAttempt:
				return ProductLeakWatch, VerbReview, true
			case suggestions.SubtypeExpired, suggestions.SubtypeAccessExhausted, suggestions.SubtypeAccessRevoked:
				return ProductAccessDecay, VerbReview, true
			case suggestions.SubtypeAnomaly:
				// Ask rate-limit / escalate land as anomaly; escalate reasons prefer commitment.
				if reasonLooksLikeAskEscalation(sig) {
					return ProductCommitmentAsk, VerbReply, true
				}
				if reasonLooksLikeAskAbuse(sig) {
					return ProductAbuseGuard, VerbReview, true
				}
				return ProductLeakWatch, VerbReview, true
			default:
				return ProductLeakWatch, VerbReview, true
			}
		case "hot_signal", "follow_up":
			if subtype == suggestions.SubtypeFormalAsk || subtype == suggestions.SubtypeQuestion {
				return ProductCommitmentAsk, VerbReply, true
			}
			return ProductBuyingWindow, VerbEmail, true
		}
	}

	switch actType {
	case "email", "call", "share":
		return ProductBuyingWindow, VerbEmail, true
	case "review":
		return ProductLeakWatch, VerbReview, true
	default:
		// Unknown operational types stay off radar (no silent buying_window).
		return "", "", false
	}
}

func isApprove(a db.ActionItem) bool {
	src := textOrEmpty(a.SourceType)
	switch a.ActionType {
	case "approve", "sign", "verify":
		return true
	}
	switch src {
	case action.SourceTypeLinkAccessRequest, action.SourceTypeDealRoomLinkAccessRequest,
		action.SourceTypeRoomAccessRequest, action.SourceTypeRoomNDA:
		return true
	}
	return false
}

func isAsk(a db.ActionItem) bool {
	src := textOrEmpty(a.SourceType)
	if a.ActionType == "answer" {
		return true
	}
	if src == action.SourceTypeLinkQuestion || src == action.SourceTypeDealRoomLinkQuestion {
		return true
	}
	return false
}

func isFormalAsk(a db.ActionItem) bool {
	src := textOrEmpty(a.SourceType)
	return (src == action.SourceTypeLinkQuestion || src == action.SourceTypeDealRoomLinkQuestion) &&
		a.ActionType == "review"
}

func buildItem(in CompileInput, a db.ActionItem, sig *db.Signal, product Product, verb Verb, now time.Time) WorkItem {
	actionID := uuidString(a.ID)
	created := now
	if a.CreatedAt.Valid {
		created = a.CreatedAt.Time.UTC()
	}

	var (
		linkID, docID, contactID, actor, email, docTitle, page string
		signalID                                               string
	)
	headline := a.Title
	subtitle := ""

	if sig != nil {
		signalID = uuidString(sig.ID)
		if headline == "" {
			headline = sig.Title
		}
		subtitle = firstNonEmpty(sig.Suggestion, sig.Description)
		if sig.LinkID.Valid {
			linkID = uuidString(sig.LinkID)
		}
		if sig.DocumentID.Valid {
			docID = uuidString(sig.DocumentID)
		}
		if sig.ContactID.Valid {
			contactID = uuidString(sig.ContactID)
		}
		actor, email, docTitle = contextActor(sig.Context)
		page = metadataPage(sig.Metadata)
	}

	src := textOrEmpty(a.SourceType)
	sourceID := textOrEmpty(a.SourceID)
	targetID := textOrEmpty(a.TargetID)

	// Operational actions often carry link/room ids on source/target.
	if linkID == "" {
		switch src {
		case action.SourceTypeLinkAccessRequest, action.SourceTypeDealRoomLinkAccessRequest,
			action.SourceTypeExpiringLink, action.SourceTypeUploadedFile:
			linkID = sourceID
		case action.SourceTypeLinkQuestion:
			linkID = targetID
		case action.SourceTypeDealRoomLinkQuestion:
			_, linkID = parseDealRoomAskTarget(targetID)
		}
	}

	dealRoomID := ""
	dealName := ""
	if src == action.SourceTypeRoomAccessRequest || src == action.SourceTypeRoomNDA || src == action.SourceTypeExpiringRoom {
		dealRoomID = sourceID
	}
	if src == action.SourceTypeDealRoomLinkAccessRequest {
		dealRoomID = targetID
	}
	if src == action.SourceTypeDealRoomLinkQuestion {
		dealRoomID, _ = parseDealRoomAskTarget(targetID)
	}
	if linkID != "" {
		if meta, ok := in.Links[linkID]; ok {
			if dealRoomID == "" {
				dealRoomID = meta.DealRoomID
			}
			if docID == "" {
				docID = meta.DocumentID
			}
			if dealName == "" {
				dealName = meta.Name
			}
			if docTitle == "" {
				docTitle = meta.Name
			}
		}
	}
	if dealRoomID != "" {
		if name, ok := in.Rooms[dealRoomID]; ok && name != "" {
			dealName = name
		}
	}
	if dealName == "" {
		// Empty → FE i18n fallback (never hardcode English "Deal").
		dealName = firstNonEmpty(docTitle, "")
	}

	dealKey := "workspace"
	if dealRoomID != "" {
		dealKey = "room:" + dealRoomID
	} else if linkID != "" {
		dealKey = "link:" + linkID
	}

	slaDue := slaDueAt(product, created, now)
	if a.DueAt.Valid && product == ProductAccessDecay {
		slaDue = a.DueAt.Time.UTC()
	}

	nav := navigatePath(in.WorkspaceSlug, src, sourceID, targetID, isFormalAsk(a))
	ev := evidencePath(in.WorkspaceSlug, docID, linkID, contactID, page)
	if nav == "" {
		nav = ev
	}
	if verb == VerbEmail && email == "" {
		verb = VerbOpen
	}

	pri := Priority(a.Impact)
	if pri != PriorityHigh && pri != PriorityMedium && pri != PriorityLow {
		pri = PriorityMedium
	}
	if sig != nil && sig.Priority != "" {
		pri = Priority(sig.Priority)
	}

	var conf Confidence
	if product == ProductLeakWatch && sig != nil {
		switch sig.Subtype.String {
		case suggestions.SubtypeForward, suggestions.SubtypeDownload,
			suggestions.SubtypeCaptureAttempt:
			conf = ConfidenceMedium
		case suggestions.SubtypeBlockedAttempt:
			conf = ConfidenceLow
		default:
			conf = ConfidenceLow
		}
	}

	chips := evidenceChips(product, sig, 0)
	return WorkItem{
		ID:            actionID,
		Product:       product,
		Headline:      headline,
		Subtitle:      subtitle,
		Actor:         actor,
		Verb:          verb,
		Priority:      pri,
		Confidence:    conf,
		SlaDueAt:      slaDue.Format(time.RFC3339),
		CreatedAt:     created.Format(time.RFC3339),
		DealKey:       dealKey,
		DealName:      dealName,
		DealRoomID:    dealRoomID,
		LinkID:        linkID,
		DocumentID:    docID,
		ContactID:     contactID,
		ActionID:      actionID,
		SignalID:      signalID,
		NavigatePath:  nav,
		EvidencePath:  ev,
		ContactEmail:  email,
		DocumentTitle: docTitle,
		Evidence:      chips,
		State:         "open",
	}
}

func coalesce(drafts []draft, now time.Time) []draft {
	if len(drafts) == 0 {
		return nil
	}
	_ = now
	// Greedy merge: same coalesce key and created within 24h of each other.
	clusters := make([]draft, 0, len(drafts))
	for _, d := range drafts {
		merged := false
		for i := range clusters {
			c := clusters[i]
			if c.coalesceK != d.coalesceK || !withinCoalesceWindow(c.created, d.created) {
				continue
			}
			winner, loser := c, d
			if prefer(d, c) {
				winner, loser = d, c
			}
			ids := append([]string{}, winner.item.CoalescedFrom...)
			ids = append(ids, loser.item.ID)
			ids = append(ids, loser.item.CoalescedFrom...)
			winner.item.CoalescedFrom = uniqueStrings(ids)
			clusters[i] = winner
			merged = true
			break
		}
		if !merged {
			clusters = append(clusters, d)
		}
	}
	return clusters
}

func withinCoalesceWindow(a, b time.Time) bool {
	d := a.Sub(b)
	if d < 0 {
		d = -d
	}
	return d <= coalesceWindow
}

func finalizeWorkItem(item *WorkItem, d draft, now time.Time) {
	item.State = "open"
	coalesceCount := len(item.CoalescedFrom)
	item.WhyNowCode, item.WhyNowHours = whyNowCode(item.Product, d.slaDue, now, coalesceCount)
	if coalesceCount > 0 {
		item.Evidence = appendEvidence(item.Evidence, EvidenceChip{
			Kind:  "coalesced",
			Count: coalesceCount + 1,
		})
	}
}

func whyNowCode(p Product, slaDue, now time.Time, coalesceCount int) (string, int) {
	if slaDue.Before(now) {
		hours := int(now.Sub(slaDue).Hours())
		if hours < 1 {
			hours = 1
		}
		return "sla_overdue", hours
	}
	if coalesceCount > 0 {
		return "coalesced", coalesceCount + 1
	}
	switch p {
	case ProductBuyingWindow:
		return "buying_window", 0
	case ProductDiligenceGate:
		return "diligence_gate", 0
	case ProductCommitmentAsk:
		return "commitment_ask", 0
	case ProductLeakWatch:
		return "leak_watch", 0
	case ProductAccessDecay:
		return "access_decay", 0
	case ProductAbuseGuard:
		return "abuse_guard", 0
	default:
		return "", 0
	}
}

func evidenceChips(product Product, sig *db.Signal, coalesceCount int) []EvidenceChip {
	var chips []EvidenceChip
	if sig != nil {
		switch sig.Subtype.String {
		case suggestions.SubtypeForward:
			chips = append(chips, EvidenceChip{Kind: "forward", Count: 1})
		case suggestions.SubtypeDownload:
			chips = append(chips, EvidenceChip{Kind: "download", Count: 1})
		case suggestions.SubtypeCaptureAttempt:
			chips = append(chips, EvidenceChip{Kind: "capture", Count: 1})
		case suggestions.SubtypeKeyPage:
			chips = append(chips, EvidenceChip{Kind: "key_page", Count: 1})
		case suggestions.SubtypeFormalAsk, suggestions.SubtypeQuestion:
			chips = append(chips, EvidenceChip{Kind: "ask", Count: 1})
		case suggestions.SubtypeHot, suggestions.SubtypeRevisit:
			chips = append(chips, EvidenceChip{Kind: "engagement", Count: 1})
		}
	}
	switch product {
	case ProductDiligenceGate:
		chips = appendEvidence(chips, EvidenceChip{Kind: "gate", Count: 1})
	case ProductAccessDecay:
		chips = appendEvidence(chips, EvidenceChip{Kind: "access", Count: 1})
	case ProductAbuseGuard:
		chips = appendEvidence(chips, EvidenceChip{Kind: "abuse", Count: 1})
	}
	if coalesceCount > 0 {
		chips = appendEvidence(chips, EvidenceChip{Kind: "coalesced", Count: coalesceCount + 1})
	}
	return chips
}

func appendEvidence(chips []EvidenceChip, chip EvidenceChip) []EvidenceChip {
	for _, c := range chips {
		if c.Kind == chip.Kind {
			return chips
		}
	}
	return append(chips, chip)
}

func prefer(a, b draft) bool {
	ra, rb := priorityRank(a.item.Priority), priorityRank(b.item.Priority)
	if ra != rb {
		return ra > rb
	}
	if ca, cb := confidenceRank(a.item.Confidence), confidenceRank(b.item.Confidence); ca != cb {
		return ca > cb
	}
	if a.rankBoost != b.rankBoost {
		return a.rankBoost < b.rankBoost
	}
	if a.microRank != b.microRank {
		return a.microRank > b.microRank
	}
	if !a.slaDue.Equal(b.slaDue) {
		return a.slaDue.Before(b.slaDue)
	}
	return a.created.After(b.created)
}

func coalesceKey(p Product, dealKey, contactID, linkID, actor string) string {
	who := firstNonEmpty(contactID, actor, linkID, "unknown")
	return fmt.Sprintf("%s|%s|%s", p, dealKey, who)
}

func buildStrands(items []WorkItem) []Strand {
	idx := map[string]int{}
	var strands []Strand
	for _, it := range items {
		i, ok := idx[it.DealKey]
		if !ok {
			idx[it.DealKey] = len(strands)
			strands = append(strands, Strand{
				DealKey:    it.DealKey,
				DealName:   it.DealName,
				DealRoomID: it.DealRoomID,
				Items:      []WorkItem{it},
			})
			continue
		}
		strands[i].Items = append(strands[i].Items, it)
	}
	return strands
}

func reasonLooksLikeAskAbuse(sig *db.Signal) bool {
	blob := strings.ToLower(sig.Description + " " + sig.Title + " " + sig.Suggestion)
	return strings.Contains(blob, "rate limit") ||
		strings.Contains(blob, "ask_ai") ||
		strings.Contains(blob, "ask abuse") ||
		strings.Contains(blob, "visitor ask rate")
}

func reasonLooksLikeAskEscalation(sig *db.Signal) bool {
	blob := strings.ToLower(sig.Description + " " + sig.Title + " " + sig.Suggestion)
	return strings.Contains(blob, "escalat")
}

func contextActor(raw []byte) (actor, email, docTitle string) {
	ctx, ok := unmarshalMap(raw)
	if !ok {
		return "", "", ""
	}
	actor = firstNonEmpty(
		asString(ctx["contactName"]),
		asString(ctx["contactEmail"]),
		asString(ctx["visitorEmail"]),
		asString(ctx["actor"]),
	)
	email = firstNonEmpty(asString(ctx["contactEmail"]), asString(ctx["visitorEmail"]))
	docTitle = asString(ctx["documentTitle"])
	return actor, email, docTitle
}

func metadataPage(raw []byte) string {
	md, ok := unmarshalStringMap(raw)
	if !ok {
		return ""
	}
	return md["page_number"]
}

func unmarshalMap(b []byte) (map[string]any, bool) {
	if len(b) == 0 {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, false
	}
	return m, true
}

func unmarshalStringMap(b []byte) (map[string]string, bool) {
	if len(b) == 0 {
		return nil, false
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, false
	}
	return m, true
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func textOrEmpty(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func uuidString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseRFC3339(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fallback
	}
	return t
}

func uniqueStrings(ids []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range ids {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
