// Package plan is the workspace billing catalog and create-path quota errors.
// Limit <= 0 means unlimited. Formal Ask is Limits.FormalAsk (trial + enterprise);
// Docling FORMAL_ASK_ENTITLED_PLAN_CODES remains a second AND gate in production.
package plan

import (
	"strings"
	"time"
)

const (
	CodeFree       = "free"
	CodePro        = "pro"
	CodeBusiness   = "business"
	CodeEnterprise = "enterprise"
	CodeTrial      = "trial"

	PeriodMonthly = "monthly"
	PeriodYearly  = "yearly"

	TrialDuration = 14 * 24 * time.Hour

	bytesGiB = 1 << 30
	bytesMiB = 1 << 20
)

// Limits are create-path caps for a plan_code. Zero means unlimited
// (except VisitorAskAIMonthly when VisitorAskAI is false, and
// KnowledgeAnswersMonthly when KnowledgeDesk is false: feature off).
type Limits struct {
	Code          string
	InternalSeats int64
	StorageBytes  int64
	Documents     int64
	Links         int64
	Rooms         int64
	// OwnedWorkspaces caps workspaces the user owns (create path). Membership
	// in someone else's tenant is paid by that workspace's InternalSeats and
	// does not consume this cap. Zero is unlimited.
	OwnedWorkspaces int64
	// MaxUploadBytes is the per-file cap. Zero means the platform hard cap.
	MaxUploadBytes int64
	// VisitorAskAIMonthly is workspace-aggregated visitor AI turns per calendar
	// month. Zero with VisitorAskAI true means unlimited; ignored when false.
	VisitorAskAIMonthly int32
	// KnowledgeAnswersMonthly is host Knowledge Desk answers per calendar month.
	// Zero with KnowledgeDesk true means unlimited; ignored when false.
	KnowledgeAnswersMonthly int32
	// KnowledgeDesk enables the host Deal Room research desk (pro+ / trial).
	KnowledgeDesk bool
	// CustomDomain enables Brand viewer hostname registration (business+ / trial).
	CustomDomain bool
	// Watermark enables workspace download watermarks and link viewer
	// watermark / screenshot-protection toggles (pro+ / trial).
	Watermark bool
	// NDA enables requiring NDA on share links / room NDA floors (business+ / trial).
	NDA bool
	// VisitorAskAI enables turning on deal-room link ask_ai_enabled (pro+ / trial).
	VisitorAskAI bool
	// FormalAsk enables Formal Q&A (trial evaluation + enterprise). Pro/Business
	// and expired trial (free caps) are off. Production still ANDs Docling
	// FORMAL_ASK_ENTITLED_PLAN_CODES — this flag is the workspace billing valve.
	FormalAsk bool
	// Branding enables logo / brand color (pro+ / trial).
	Branding bool
	// AccessControls enables email verification and allow/block lists (business+ / trial).
	AccessControls bool
	// Webhooks enables workspace outbound HTTPS webhooks (business+ / trial).
	Webhooks bool
	// HubSpot enables HubSpot OAuth connect and CRM sync (business+ / trial).
	HubSpot bool
	// DailyDigest enables the Insights daily email digest (business+ / trial).
	DailyDigest bool
	// SlackAlerts enables sensitive-page Slack alerts (business+ / trial).
	SlackAlerts bool
	// RoomInsights enables workspace Insights overview (business+ / trial).
	RoomInsights bool
	// RoomAnalytics enables deal-room Analytics tab aggregates (pro+ / trial).
	RoomAnalytics bool
}

var catalog = map[string]Limits{
	CodeFree: {
		Code:            CodeFree,
		InternalSeats:   1,
		StorageBytes:    2 * bytesGiB,
		Documents:       50,
		Links:           20,
		Rooms:           1,
		OwnedWorkspaces: 1,
		MaxUploadBytes:  25 * bytesMiB,
	},
	CodePro: {
		Code:                    CodePro,
		InternalSeats:           3,
		StorageBytes:            50 * bytesGiB,
		Documents:               200,
		Links:                   0,
		Rooms:                   5,
		OwnedWorkspaces:         3,
		MaxUploadBytes:          100 * bytesMiB,
		VisitorAskAIMonthly:     200,
		KnowledgeAnswersMonthly: 100,
		KnowledgeDesk:           true,
		Watermark:               true,
		VisitorAskAI:            true,
		Branding:                true,
		RoomAnalytics:           true,
	},
	CodeBusiness: {
		Code:                    CodeBusiness,
		InternalSeats:           10,
		StorageBytes:            500 * bytesGiB,
		Documents:               1000,
		Links:                   0,
		Rooms:                   0,
		OwnedWorkspaces:         10,
		MaxUploadBytes:          250 * bytesMiB,
		VisitorAskAIMonthly:     1000,
		KnowledgeAnswersMonthly: 200,
		KnowledgeDesk:           true,
		CustomDomain:            true,
		Watermark:               true,
		NDA:                     true,
		VisitorAskAI:            true,
		Branding:                true,
		AccessControls:          true,
		Webhooks:                true,
		HubSpot:                 true,
		DailyDigest:             true,
		SlackAlerts:             true,
		RoomInsights:            true,
		RoomAnalytics:           true,
	},
	CodeEnterprise: {
		Code:            CodeEnterprise,
		InternalSeats:   0,
		StorageBytes:    0,
		Documents:       0,
		Links:           0,
		Rooms:           0,
		OwnedWorkspaces: 0,
		MaxUploadBytes:  0,
		KnowledgeDesk:   true,
		CustomDomain:    true,
		Watermark:       true,
		NDA:             true,
		VisitorAskAI:    true,
		FormalAsk:       true,
		Branding:        true,
		AccessControls:  true,
		Webhooks:        true,
		HubSpot:         true,
		DailyDigest:     true,
		SlackAlerts:     true,
		RoomInsights:    true,
		RoomAnalytics:   true,
	},
	// Trial mirrors Business capacity for rooms/docs/storage/features.
	// OwnedWorkspaces stays 1 so the evaluation window cannot farm tenants.
	CodeTrial: {
		Code:                    CodeTrial,
		InternalSeats:           10,
		StorageBytes:            500 * bytesGiB,
		Documents:               1000,
		Links:                   0,
		Rooms:                   0,
		OwnedWorkspaces:         1,
		MaxUploadBytes:          250 * bytesMiB,
		VisitorAskAIMonthly:     1000,
		KnowledgeAnswersMonthly: 200,
		KnowledgeDesk:           true,
		CustomDomain:            true,
		Watermark:               true,
		NDA:                     true,
		VisitorAskAI:            true,
		Branding:                true,
		AccessControls:          true,
		Webhooks:                true,
		HubSpot:                 true,
		DailyDigest:             true,
		SlackAlerts:             true,
		RoomInsights:            true,
		RoomAnalytics:           true,
		FormalAsk:               true,
	},
}

// Lookup returns catalog limits. Unknown or empty codes fail-closed to free
// so a corrupt row cannot grant trial/Business capacity.
func Lookup(code string) Limits {
	key := strings.ToLower(strings.TrimSpace(code))
	if limits, ok := catalog[key]; ok {
		return limits
	}
	return catalog[CodeFree]
}

// OverLimit reports whether used+additional exceeds a finite cap.
func OverLimit(used, additional, limit int64) bool {
	if limit <= 0 {
		return false
	}
	return used+additional > limit
}

// Offer is a purchasable plan shown on the in-app subscription page.
// Trial is not listed — it is the evaluation state, not a checkout SKU.
type Offer struct {
	Code                    string `json:"code"`
	InternalSeats           int64  `json:"internal_seats"`
	StorageBytes            int64  `json:"storage_bytes"`
	Documents               int64  `json:"documents"`
	Links                   int64  `json:"links"`
	Rooms                   int64  `json:"rooms"`
	OwnedWorkspaces         int64  `json:"owned_workspaces"`
	MaxUploadBytes          int64  `json:"max_upload_bytes"`
	VisitorAskAIMonthly     int32  `json:"visitor_ask_ai_monthly"`
	KnowledgeAnswersMonthly int32  `json:"knowledge_answers_monthly"`
	KnowledgeDesk           bool   `json:"knowledge_desk"`
	CustomDomain            bool   `json:"custom_domain"`
	Watermark               bool   `json:"watermark"`
	NDA                     bool   `json:"nda"`
	VisitorAskAI            bool   `json:"visitor_ask_ai"`
	Branding                bool   `json:"branding"`
	AccessControls          bool   `json:"access_controls"`
	Webhooks                bool   `json:"webhooks"`
	HubSpot                 bool   `json:"hubspot"`
	DailyDigest             bool   `json:"daily_digest"`
	SlackAlerts             bool   `json:"slack_alerts"`
	RoomInsights            bool   `json:"room_insights"`
	RoomAnalytics           bool   `json:"room_analytics"`
	FormalAsk               bool   `json:"formal_ask"`
	PriceMonthlyUSD         int    `json:"price_monthly_usd"`
	CustomPricing           bool   `json:"custom_pricing"`
	Highlighted             bool   `json:"highlighted"`
}

// Purchasable reports whether plan_code is a listed SKU (not trial).
// ChangePlan still rejects enterprise (sales-assisted) and unpaid pro/business
// unless the process allows unpaid self-serve (non-production only).
func Purchasable(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case CodeFree, CodePro, CodeBusiness, CodeEnterprise:
		return true
	default:
		return false
	}
}

// SelfServe reports whether ChangePlan may persist the code without a sales desk.
// Enterprise is listed on the pricing page but never self-serve.
func SelfServe(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case CodeFree, CodePro, CodeBusiness:
		return true
	default:
		return false
	}
}

// NormalizePeriod returns monthly|yearly or empty when invalid.
func NormalizePeriod(period string) string {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case PeriodMonthly, "":
		return PeriodMonthly
	case PeriodYearly, "annual", "annually":
		return PeriodYearly
	default:
		return ""
	}
}

// Offers returns Free → Pro → Business → Enterprise in display order.
// Prices are list USD/month (enterprise is custom). Yearly billing is period-only
// until Stripe ships. Production ChangePlan does not persist paid SKUs unpaid.
func Offers() []Offer {
	order := []string{CodeFree, CodePro, CodeBusiness, CodeEnterprise}
	prices := map[string]int{
		CodeFree:       0,
		CodePro:        49,
		CodeBusiness:   99,
		CodeEnterprise: 0,
	}
	out := make([]Offer, 0, len(order))
	for _, code := range order {
		lim := catalog[code]
		out = append(out, Offer{
			Code:                    lim.Code,
			InternalSeats:           lim.InternalSeats,
			StorageBytes:            lim.StorageBytes,
			Documents:               lim.Documents,
			Links:                   lim.Links,
			Rooms:                   lim.Rooms,
			OwnedWorkspaces:         lim.OwnedWorkspaces,
			MaxUploadBytes:          lim.MaxUploadBytes,
			VisitorAskAIMonthly:     lim.VisitorAskAIMonthly,
			KnowledgeAnswersMonthly: lim.KnowledgeAnswersMonthly,
			KnowledgeDesk:           lim.KnowledgeDesk,
			CustomDomain:            lim.CustomDomain,
			Watermark:               lim.Watermark,
			NDA:                     lim.NDA,
			VisitorAskAI:            lim.VisitorAskAI,
			Branding:                lim.Branding,
			AccessControls:          lim.AccessControls,
			Webhooks:                lim.Webhooks,
			HubSpot:                 lim.HubSpot,
			DailyDigest:             lim.DailyDigest,
			SlackAlerts:             lim.SlackAlerts,
			RoomInsights:            lim.RoomInsights,
			RoomAnalytics:           lim.RoomAnalytics,
			FormalAsk:               lim.FormalAsk,
			PriceMonthlyUSD:         prices[code],
			CustomPricing:           code == CodeEnterprise,
			Highlighted:             code == CodeBusiness,
		})
	}
	return out
}
