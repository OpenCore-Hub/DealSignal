package workspace

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/billing"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// MemberDetail is the public view of a workspace member with user profile.
type MemberDetail struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	JoinedAt  string `json:"joined_at"`
	Status    string `json:"status"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// Settings is the public view of workspace general settings.
type Settings struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	BrandColor   string `json:"brand_color"`
	ViewerDomain string `json:"viewer_domain"`
	LogoURL      string `json:"logo_url,omitempty"`
}

// SecuritySettings is the public view of workspace security settings.
type SecuritySettings struct {
	ForceEmailVerification bool `json:"force_email_verification"`
	WatermarkDownloads     bool `json:"watermark_downloads"`
	TwoFactorEnabled       bool `json:"two_factor_enabled"`
}

// Billing is the public view of workspace billing usage.
// Limits are effective (free caps after trial expiry). Plan stays the stored plan_code.
type Billing struct {
	Plan                  string `json:"plan"`
	Period                string `json:"period"`
	TrialExpired          bool   `json:"trial_expired"`
	TrialEndsAt           string `json:"trial_ends_at,omitempty"`
	StorageUsed           int64  `json:"storage_used"`
	StorageLimit          int64  `json:"storage_limit"`
	LinksUsed             int64  `json:"links_used"`
	LinksLimit            int64  `json:"links_limit"`
	RoomsUsed             int64  `json:"rooms_used"`
	RoomsLimit            int64  `json:"rooms_limit"`
	SeatsUsed             int64  `json:"seats_used"`
	SeatsLimit            int64  `json:"seats_limit"`
	CustomDomainEnabled   bool   `json:"custom_domain_enabled"`
	WatermarkEnabled      bool   `json:"watermark_enabled"`
	NDAEnabled            bool   `json:"nda_enabled"`
	VisitorAskAIEnabled   bool   `json:"visitor_ask_ai_enabled"`
	BrandingEnabled       bool   `json:"branding_enabled"`
	AccessControlsEnabled bool   `json:"access_controls_enabled"`
	KnowledgeDeskEnabled  bool   `json:"knowledge_desk_enabled"`
	WebhooksEnabled       bool   `json:"webhooks_enabled"`
	HubSpotEnabled        bool   `json:"hubspot_enabled"`
	DailyDigestEnabled    bool   `json:"daily_digest_enabled"`
	SlackAlertsEnabled    bool   `json:"slack_alerts_enabled"`
	RoomAnalyticsEnabled  bool   `json:"room_analytics_enabled"`
	RoomInsightsEnabled   bool   `json:"room_insights_enabled"`
	FormalAskEnabled      bool   `json:"formal_ask_enabled"`
	DocumentsUsed         int64  `json:"documents_used"`
	DocumentsLimit        int64  `json:"documents_limit"`
	AskAIUsed             int32  `json:"ask_ai_used"`
	AskAILimit            int32  `json:"ask_ai_limit"`
	KnowledgeAnswersUsed  int32  `json:"knowledge_answers_used"`
	KnowledgeAnswersLimit int32  `json:"knowledge_answers_limit"`
	MaxUploadBytes        int64  `json:"max_upload_bytes"`
	BillingStatus         string `json:"billing_status,omitempty"`
	HasStripeSubscription bool   `json:"has_stripe_subscription"`
	CurrentPeriodEnd      string `json:"current_period_end,omitempty"`
}

// ListMembers returns workspace members with basic profile info.
func (s *Service) ListMembers(ctx context.Context, workspaceID string) ([]MemberDetail, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListWorkspaceMembers(ctx, wsUUID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberDetail, 0, len(rows))
	activeEmails := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		email := strings.ToLower(strings.TrimSpace(r.Email))
		activeEmails[email] = struct{}{}
		out = append(out, MemberDetail{
			ID:       uuidToString(r.UserID),
			UserID:   uuidToString(r.UserID),
			Email:    r.Email,
			Name:     "",
			Role:     r.Role,
			JoinedAt: r.JoinedAt.Time.Format(time.RFC3339),
			Status:   "active",
		})
	}

	pending, err := s.queries.ListPendingWorkspaceInvitations(ctx, wsUUID)
	if err != nil {
		return nil, err
	}
	for _, inv := range pending {
		email := strings.ToLower(strings.TrimSpace(inv.Email))
		if _, exists := activeEmails[email]; exists {
			continue
		}
		out = append(out, MemberDetail{
			ID:       uuidToString(inv.Token),
			UserID:   "",
			Email:    inv.Email,
			Name:     "",
			Role:     inv.Role,
			JoinedAt: inv.CreatedAt.Time.Format(time.RFC3339),
			Status:   "pending",
		})
	}
	return out, nil
}

// GetSettings returns workspace general settings.
func (s *Service) GetSettings(ctx context.Context, workspaceID string) (Settings, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Settings{}, err
	}
	ws, err := s.queries.GetWorkspaceByID(ctx, wsUUID)
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		Name:         ws.Name,
		Slug:         ws.Slug,
		BrandColor:   ws.BrandColor.String,
		ViewerDomain: s.verifiedViewerHostname(ctx, workspaceID),
		LogoURL:      s.resolveLogoURL(ctx, workspaceID),
	}, nil
}

// UpdateSettings updates workspace general settings.
func (s *Service) UpdateSettings(ctx context.Context, workspaceID, name, brandColor string) (Settings, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Settings{}, err
	}
	current, err := s.GetSettings(ctx, workspaceID)
	if err != nil {
		return Settings{}, err
	}
	if brandColor != "" && brandColor != current.BrandColor {
		if err := s.AssertCanUseBranding(ctx, workspaceID); err != nil {
			return Settings{}, err
		}
	}
	ws, err := s.queries.UpdateWorkspace(ctx, db.UpdateWorkspaceParams{
		ID:         wsUUID,
		Name:       name,
		BrandColor: pgtype.Text{String: brandColor, Valid: brandColor != ""},
	})
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		Name:         ws.Name,
		Slug:         ws.Slug,
		BrandColor:   ws.BrandColor.String,
		ViewerDomain: s.verifiedViewerHostname(ctx, workspaceID),
		LogoURL:      s.resolveLogoURL(ctx, workspaceID),
	}, nil
}

// GetSecurity returns workspace security settings.
func (s *Service) GetSecurity(ctx context.Context, workspaceID string) (SecuritySettings, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return SecuritySettings{}, err
	}
	ws, err := s.queries.GetWorkspaceByID(ctx, wsUUID)
	if err != nil {
		return SecuritySettings{}, err
	}
	return SecuritySettings{
		ForceEmailVerification: ws.ForceEmailVerification,
		WatermarkDownloads:     ws.WatermarkDownloads,
		TwoFactorEnabled:       ws.TwoFactorEnabled,
	}, nil
}

// UpdateSecurity updates workspace security settings.
func (s *Service) UpdateSecurity(ctx context.Context, workspaceID string, req SecuritySettings) (SecuritySettings, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return SecuritySettings{}, err
	}
	if req.WatermarkDownloads {
		current, err := s.GetSecurity(ctx, workspaceID)
		if err != nil {
			return SecuritySettings{}, err
		}
		// Grandfather: already-on stays writable; only false→true is gated.
		if !current.WatermarkDownloads {
			if err := s.AssertCanUseWatermark(ctx, workspaceID); err != nil {
				return SecuritySettings{}, err
			}
		}
	}
	if req.ForceEmailVerification {
		current, err := s.GetSecurity(ctx, workspaceID)
		if err != nil {
			return SecuritySettings{}, err
		}
		if !current.ForceEmailVerification {
			if err := s.AssertCanUseAccessControls(ctx, workspaceID); err != nil {
				return SecuritySettings{}, err
			}
		}
	}
	ws, err := s.queries.UpdateWorkspaceSecurity(ctx, db.UpdateWorkspaceSecurityParams{
		ForceEmailVerification: req.ForceEmailVerification,
		WatermarkDownloads:     req.WatermarkDownloads,
		TwoFactorEnabled:       req.TwoFactorEnabled,
		ID:                     wsUUID,
	})
	if err != nil {
		return SecuritySettings{}, err
	}
	return SecuritySettings{
		ForceEmailVerification: ws.ForceEmailVerification,
		WatermarkDownloads:     ws.WatermarkDownloads,
		TwoFactorEnabled:       ws.TwoFactorEnabled,
	}, nil
}

// GetBilling returns persisted plan_code catalog limits plus live usage counts.
func (s *Service) GetBilling(ctx context.Context, workspaceID string) (Billing, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Billing{}, err
	}
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return Billing{}, fmt.Errorf("load billing: %w", err)
	}
	linksUsed, err := s.queries.CountLinksByWorkspace(ctx, wsUUID)
	if err != nil {
		return Billing{}, fmt.Errorf("count links: %w", err)
	}
	roomsUsed, err := s.queries.CountDealRoomsByWorkspace(ctx, db.CountDealRoomsByWorkspaceParams{
		WorkspaceID: wsUUID,
		Query:       "",
	})
	if err != nil {
		return Billing{}, fmt.Errorf("count data rooms: %w", err)
	}
	storageUsage, err := s.queries.GetWorkspaceStorageUsage(ctx, wsUUID)
	if err != nil {
		return Billing{}, fmt.Errorf("get storage usage: %w", err)
	}
	seatsUsed, err := s.queries.CountInternalSeatsByWorkspace(ctx, wsUUID)
	if err != nil {
		return Billing{}, fmt.Errorf("count seats: %w", err)
	}
	docsUsed, err := s.queries.CountDocumentsByWorkspace(ctx, wsUUID)
	if err != nil {
		return Billing{}, fmt.Errorf("count documents: %w", err)
	}
	askUsed, err := s.queries.CountWorkspaceAskAITurnsThisMonth(ctx, wsUUID)
	if err != nil {
		return Billing{}, fmt.Errorf("count ask ai turns: %w", err)
	}
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	knowledgeUsed64, err := s.queries.CountKnowledgeQATurnsForWorkspaceSince(ctx, db.CountKnowledgeQATurnsForWorkspaceSinceParams{
		WorkspaceID: wsUUID,
		Since:       pgtype.Timestamptz{Time: monthStart, Valid: true},
	})
	if err != nil {
		return Billing{}, fmt.Errorf("count knowledge answers: %w", err)
	}
	knowledgeUsed := int32(knowledgeUsed64)
	if knowledgeUsed64 > math.MaxInt32 {
		knowledgeUsed = math.MaxInt32
	}

	out := Billing{
		Plan:                  state.storedPlan,
		Period:                state.period,
		TrialExpired:          state.trialExpired,
		StorageUsed:           storageUsage,
		StorageLimit:          state.limits.StorageBytes,
		LinksUsed:             linksUsed,
		LinksLimit:            state.limits.Links,
		RoomsUsed:             roomsUsed,
		RoomsLimit:            state.limits.Rooms,
		SeatsUsed:             seatsUsed,
		SeatsLimit:            state.limits.InternalSeats,
		DocumentsUsed:         docsUsed,
		DocumentsLimit:        state.limits.Documents,
		AskAIUsed:             askUsed,
		AskAILimit:            state.limits.VisitorAskAIMonthly,
		KnowledgeAnswersUsed:  knowledgeUsed,
		KnowledgeAnswersLimit: state.limits.KnowledgeAnswersMonthly,
		MaxUploadBytes:        state.limits.MaxUploadBytes,
		CustomDomainEnabled:   state.limits.CustomDomain,
		WatermarkEnabled:      state.limits.Watermark,
		NDAEnabled:            state.limits.NDA,
		VisitorAskAIEnabled:   state.limits.VisitorAskAI,
		BrandingEnabled:       state.limits.Branding,
		AccessControlsEnabled: state.limits.AccessControls,
		KnowledgeDeskEnabled:  state.limits.KnowledgeDesk,
		WebhooksEnabled:       state.limits.Webhooks,
		HubSpotEnabled:        state.limits.HubSpot,
		DailyDigestEnabled:    state.limits.DailyDigest,
		SlackAlertsEnabled:    state.limits.SlackAlerts,
		RoomAnalyticsEnabled:  state.limits.RoomAnalytics,
		RoomInsightsEnabled:   state.limits.RoomInsights,
		FormalAskEnabled:      state.limits.FormalAsk,
		BillingStatus:         state.billingStatus,
		HasStripeSubscription: state.hasStripeSubscription,
	}
	if state.hasTrialEnd {
		out.TrialEndsAt = state.trialEndsAt.UTC().Format(time.RFC3339)
	}
	if state.hasPeriodEnd {
		out.CurrentPeriodEnd = state.currentPeriodEnd.UTC().Format(time.RFC3339)
	}
	return out, nil
}

// BillingPlansResponse is GET /billing/plans — catalog + current workspace plan.
type BillingPlansResponse struct {
	CurrentPlan           string       `json:"current_plan"`
	CurrentPeriod         string       `json:"current_period"`
	TrialExpired          bool         `json:"trial_expired"`
	TrialEndsAt           string       `json:"trial_ends_at,omitempty"`
	BillingStatus         string       `json:"billing_status,omitempty"`
	HasStripeSubscription bool         `json:"has_stripe_subscription"`
	Plans                 []plan.Offer `json:"plans"`
}

// ListBillingPlans returns purchasable offers (not trial) plus the workspace's current plan.
func (s *Service) ListBillingPlans(ctx context.Context, workspaceID string) (BillingPlansResponse, error) {
	billing, err := s.GetBilling(ctx, workspaceID)
	if err != nil {
		return BillingPlansResponse{}, err
	}
	out := BillingPlansResponse{
		CurrentPlan:           billing.Plan,
		CurrentPeriod:         billing.Period,
		TrialExpired:          billing.TrialExpired,
		BillingStatus:         billing.BillingStatus,
		HasStripeSubscription: billing.HasStripeSubscription,
		Plans:                 plan.Offers(),
	}
	if billing.TrialEndsAt != "" {
		out.TrialEndsAt = billing.TrialEndsAt
	}
	return out, nil
}

// ChangePlan upserts workspace_billing to a listed SKU under the billing lock.
// Trial cannot be selected here (signup only). Leaving trial clears trial_ends_at.
// Enterprise is never self-serve. Paid SKUs require checkout unless unpaid
// self-serve is explicitly enabled (non-production).
func (s *Service) ChangePlan(ctx context.Context, workspaceID, planCode, period string) (Billing, error) {
	code := strings.ToLower(strings.TrimSpace(planCode))
	if !plan.Purchasable(code) {
		return Billing{}, ErrInvalidPlanCode
	}
	if code == plan.CodeEnterprise {
		return Billing{}, ErrPlanSalesAssisted
	}
	if code != plan.CodeFree && !s.allowUnpaidPlanChange {
		return Billing{}, ErrPlanPaymentRequired
	}
	normPeriod := plan.NormalizePeriod(period)
	if normPeriod == "" {
		return Billing{}, ErrInvalidBillingPeriod
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Billing{}, err
	}
	if err := s.withBillingMutation(ctx, workspaceID, func(q *db.Queries) error {
		row, err := q.GetWorkspaceBilling(ctx, wsUUID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("load billing for plan change: %w", err)
		}
		if err == nil && billing.HasActiveSubscription(row) && code == plan.CodeFree {
			return ErrPlanManageViaPortal
		}
		_, err = q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
			WorkspaceID: wsUUID,
			PlanCode:    code,
			Period:      normPeriod,
			// Paid/free selection ends the trial clock; expiry semantics only apply to plan=trial.
			TrialEndsAt: pgtype.Timestamptz{},
		})
		return err
	}); err != nil {
		if errors.Is(err, ErrPlanManageViaPortal) {
			return Billing{}, err
		}
		return Billing{}, fmt.Errorf("upsert billing plan: %w", err)
	}
	return s.GetBilling(ctx, workspaceID)
}
