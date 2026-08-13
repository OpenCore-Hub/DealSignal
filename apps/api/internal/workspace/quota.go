package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/billing"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type billingState struct {
	// limits are effective create-path caps (free after trial expiry).
	limits plan.Limits
	period string
	// storedPlan is the workspace_billing.plan_code (unchanged on read).
	storedPlan string
	// trialExpired is true when plan_code=trial and trial_ends_at has passed.
	trialExpired bool
	// trialEndsAt is set when the billing row has a valid trial_ends_at.
	trialEndsAt           time.Time
	hasTrialEnd           bool
	billingStatus         string
	hasStripeSubscription bool
	currentPeriodEnd      time.Time
	hasPeriodEnd          bool
}

const pastDueGrace = 72 * time.Hour

// resolveBilling maps a billing row to effective limits without rewriting the DB.
// Expired trial keeps storedPlan=trial but applies free caps/features.
// past_due keeps paid caps for 72h, then free. canceled is free.
func resolveBilling(row db.WorkspaceBilling, now time.Time) billingState {
	period := row.Period
	if period == "" {
		period = plan.PeriodMonthly
	}
	stored := strings.ToLower(strings.TrimSpace(row.PlanCode))
	if stored == "" {
		stored = plan.CodeFree
	}
	state := billingState{
		limits:                plan.Lookup(stored),
		period:                period,
		storedPlan:            stored,
		billingStatus:         strings.ToLower(strings.TrimSpace(row.BillingStatus.String)),
		hasStripeSubscription: billing.HasActiveSubscription(row),
	}
	if row.TrialEndsAt.Valid {
		ends := row.TrialEndsAt.Time.UTC()
		state.trialEndsAt = ends
		state.hasTrialEnd = true
	}
	if row.CurrentPeriodEnd.Valid {
		state.currentPeriodEnd = row.CurrentPeriodEnd.Time.UTC()
		state.hasPeriodEnd = true
	}
	if stored == plan.CodeTrial {
		// No clock (or a clock that has elapsed) is expired — never grant
		// unpaid Business capacity without a live trial_ends_at in the future.
		active := state.hasTrialEnd && now.UTC().Before(state.trialEndsAt)
		if !active {
			state.limits = plan.Lookup(plan.CodeFree)
			state.trialExpired = true
		}
	}
	switch state.billingStatus {
	case billing.StatusCanceled:
		state.limits = plan.Lookup(plan.CodeFree)
	case billing.StatusPastDue:
		graceOK := row.PastDueAt.Valid && now.UTC().Before(row.PastDueAt.Time.UTC().Add(pastDueGrace))
		if !graceOK {
			state.limits = plan.Lookup(plan.CodeFree)
		}
	}
	return state
}

// ownedWorkspaceLimit is the user's best effective owned-workspace cap.
// Cap comes from workspaces they own (what they pay for), not memberships in
// someone else's tenant. Missing billing rows fail-closed to Free (1).
// Enterprise 0 is unlimited.
func ownedWorkspaceLimit(rows []db.WorkspaceBilling, now time.Time) int64 {
	if len(rows) == 0 {
		return plan.Lookup(plan.CodeFree).OwnedWorkspaces
	}
	best := int64(1)
	for _, row := range rows {
		lim := resolveBilling(row, now).limits.OwnedWorkspaces
		if lim <= 0 {
			return 0
		}
		if lim > best {
			best = lim
		}
	}
	return best
}

// assertCanAddOwnedWorkspace gates creating a workspace the user will own.
// Joining someone else's tenant as admin/member is paid by that workspace's
// InternalSeats and must not call this. Cap is from owned billing
// (fail-closed Free). requireVerified applies to Create of a second owned
// workspace; the first owned workspace (including after invite-join) does not.
func (s *Service) assertCanAddOwnedWorkspace(ctx context.Context, q *db.Queries, userID pgtype.UUID, requireVerified bool) error {
	count, err := q.CountOwnedWorkspacesByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("count owned workspaces: %w", err)
	}
	rows, err := q.ListOwnedWorkspaceBillingByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("list owned billing: %w", err)
	}
	if plan.OverLimit(count, 1, ownedWorkspaceLimit(rows, time.Now())) {
		return plan.ErrLimitWorkspaces
	}
	if !requireVerified || count == 0 {
		return nil
	}
	user, err := q.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if !user.EmailVerified {
		return ErrEmailUnverified
	}
	return nil
}

type billingReader interface {
	GetWorkspaceBilling(ctx context.Context, workspaceID pgtype.UUID) (db.WorkspaceBilling, error)
}

func loadBillingFrom(ctx context.Context, q billingReader, workspaceID string) (billingState, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return billingState{}, err
	}
	row, err := q.GetWorkspaceBilling(ctx, wsUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Fail-closed: a missing row is a defect, not a new 14-day trial.
		return billingState{
			limits:     plan.Lookup(plan.CodeFree),
			period:     plan.PeriodMonthly,
			storedPlan: plan.CodeFree,
		}, nil
	}
	if err != nil {
		return billingState{}, err
	}
	return resolveBilling(row, time.Now().UTC()), nil
}

func (s *Service) loadBilling(ctx context.Context, workspaceID string) (billingState, error) {
	return loadBillingFrom(ctx, s.queries, workspaceID)
}

// lockWorkspaceBillingSeatQuota takes FOR UPDATE on workspace_billing so
// seat-consuming and seat-freeing invite/add/promote/demote/remove/accept/revoke
// paths cannot TOCTOU-oversubscribe or false-deny after a free.
// Missing rows are seeded as free (same fail-closed as loadBilling) then locked.
func (s *Service) lockWorkspaceBillingSeatQuota(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID) error {
	_, err := q.LockWorkspaceBillingForUpdate(ctx, workspaceID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("lock workspace billing: %w", err)
	}
	_, err = q.InsertWorkspaceBilling(ctx, db.InsertWorkspaceBillingParams{
		WorkspaceID: workspaceID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	})
	if err != nil && !isUniqueViolation(err) {
		return fmt.Errorf("seed workspace billing for seat lock: %w", err)
	}
	if _, err := q.LockWorkspaceBillingForUpdate(ctx, workspaceID); err != nil {
		return fmt.Errorf("lock workspace billing after seed: %w", err)
	}
	return nil
}

// withBillingMutation serializes plan-quota writes when a DB pool is configured.
// Unit tests without WithDBPool keep the unlocked path (still assert-then-write).
func (s *Service) withBillingMutation(ctx context.Context, workspaceID string, fn func(q *db.Queries) error) error {
	if s.dbPool == nil {
		return fn(s.queries)
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin billing quota tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)
	if err := s.lockWorkspaceBillingSeatQuota(ctx, qtx, wsUUID); err != nil {
		return err
	}
	if err := fn(qtx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit billing quota tx: %w", err)
	}
	return nil
}

// withSeatMutation serializes seat-consuming writes (alias of withBillingMutation).
func (s *Service) withSeatMutation(ctx context.Context, workspaceID string, fn func(q *db.Queries) error) error {
	return s.withBillingMutation(ctx, workspaceID, fn)
}

// AssertCanCreateRoom implements plan.Checker.
func (s *Service) AssertCanCreateRoom(ctx context.Context, workspaceID string) error {
	return s.assertCanCreateRoomQ(ctx, s.queries, workspaceID)
}

func (s *Service) assertCanCreateRoomQ(ctx context.Context, q *db.Queries, workspaceID string) error {
	state, err := loadBillingFrom(ctx, q, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.Rooms <= 0 {
		return nil
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	used, err := q.CountDealRoomsByWorkspace(ctx, db.CountDealRoomsByWorkspaceParams{
		WorkspaceID: wsUUID,
		Query:       "",
	})
	if err != nil {
		return fmt.Errorf("count data rooms: %w", err)
	}
	if plan.OverLimit(used, 1, state.limits.Rooms) {
		return plan.ErrLimitRooms
	}
	return nil
}

// WithCreateRoomQuota implements plan.Checker.
func (s *Service) WithCreateRoomQuota(ctx context.Context, workspaceID string, fn func(ctx context.Context) error) error {
	return s.withBillingMutation(ctx, workspaceID, func(q *db.Queries) error {
		if err := s.assertCanCreateRoomQ(ctx, q, workspaceID); err != nil {
			return err
		}
		return fn(ctx)
	})
}

// AssertCanCreateLink implements plan.Checker.
func (s *Service) AssertCanCreateLink(ctx context.Context, workspaceID string) error {
	return s.assertCanCreateLinkQ(ctx, s.queries, workspaceID)
}

func (s *Service) assertCanCreateLinkQ(ctx context.Context, q *db.Queries, workspaceID string) error {
	state, err := loadBillingFrom(ctx, q, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.Links <= 0 {
		return nil
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	used, err := q.CountLinksByWorkspace(ctx, wsUUID)
	if err != nil {
		return fmt.Errorf("count links: %w", err)
	}
	if plan.OverLimit(used, 1, state.limits.Links) {
		return plan.ErrLimitLinks
	}
	return nil
}

// WithCreateLinkQuota implements plan.Checker.
func (s *Service) WithCreateLinkQuota(ctx context.Context, workspaceID string, fn func(ctx context.Context) error) error {
	return s.withBillingMutation(ctx, workspaceID, func(q *db.Queries) error {
		if err := s.assertCanCreateLinkQ(ctx, q, workspaceID); err != nil {
			return err
		}
		return fn(ctx)
	})
}

// WithBillingLock implements plan.Checker.
func (s *Service) WithBillingLock(ctx context.Context, workspaceID string, fn func(ctx context.Context) error) error {
	return s.withBillingMutation(ctx, workspaceID, func(q *db.Queries) error {
		return fn(ctx)
	})
}

// AssertCanAddStorage implements plan.Checker.
func (s *Service) AssertCanAddStorage(ctx context.Context, workspaceID string, additionalBytes int64) error {
	return s.assertCanAddStorageQ(ctx, s.queries, workspaceID, additionalBytes)
}

func (s *Service) assertCanAddStorageQ(ctx context.Context, q *db.Queries, workspaceID string, additionalBytes int64) error {
	if additionalBytes <= 0 {
		return nil
	}
	state, err := loadBillingFrom(ctx, q, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.StorageBytes <= 0 {
		return nil
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	used, err := q.GetWorkspaceStorageUsage(ctx, wsUUID)
	if err != nil {
		return fmt.Errorf("get storage usage: %w", err)
	}
	if plan.OverLimit(used, additionalBytes, state.limits.StorageBytes) {
		return plan.ErrLimitStorage
	}
	return nil
}

// AssertCanCreateDocument implements plan.Checker.
func (s *Service) AssertCanCreateDocument(ctx context.Context, workspaceID string) error {
	return s.assertCanCreateDocumentQ(ctx, s.queries, workspaceID)
}

func (s *Service) assertCanCreateDocumentQ(ctx context.Context, q *db.Queries, workspaceID string) error {
	state, err := loadBillingFrom(ctx, q, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.Documents <= 0 {
		return nil
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	used, err := q.CountDocumentsByWorkspace(ctx, wsUUID)
	if err != nil {
		return fmt.Errorf("count documents: %w", err)
	}
	if plan.OverLimit(used, 1, state.limits.Documents) {
		return plan.ErrLimitDocuments
	}
	return nil
}

// AssertCanUploadFile implements plan.Checker.
func (s *Service) AssertCanUploadFile(ctx context.Context, workspaceID string, size int64) error {
	if size <= 0 {
		return nil
	}
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.MaxUploadBytes <= 0 {
		return nil
	}
	if size > state.limits.MaxUploadBytes {
		return plan.ErrLimitUpload
	}
	return nil
}

// WithAddStorageQuota implements plan.Checker.
// Shrink / net-zero deltas (additionalBytes <= 0) still take the billing lock so
// they serialize with grow paths — otherwise a concurrent grow can false-deny or,
// with looser callers, race past a shrink that has not yet become visible.
func (s *Service) WithAddStorageQuota(ctx context.Context, workspaceID string, additionalBytes int64, fn func(ctx context.Context) error) error {
	return s.withBillingMutation(ctx, workspaceID, func(q *db.Queries) error {
		if err := s.assertCanAddStorageQ(ctx, q, workspaceID, additionalBytes); err != nil {
			return err
		}
		return fn(ctx)
	})
}

func isInternalSeatRole(role string) bool {
	return role == RoleOwner || role == RoleAdmin || role == RoleMember
}

// AssertCanAddInternalSeat blocks adding another owner/admin/member seat when capped.
// Guests are unlimited and must not call this.
func (s *Service) AssertCanAddInternalSeat(ctx context.Context, workspaceID string) error {
	return s.assertCanAddInternalSeatQ(ctx, s.queries, workspaceID)
}

func (s *Service) assertCanAddInternalSeatQ(ctx context.Context, q *db.Queries, workspaceID string) error {
	state, err := loadBillingFrom(ctx, q, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.InternalSeats <= 0 {
		return nil
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	used, err := q.CountInternalSeatsByWorkspace(ctx, wsUUID)
	if err != nil {
		return fmt.Errorf("count internal seats: %w", err)
	}
	if plan.OverLimit(used, 1, state.limits.InternalSeats) {
		return plan.ErrLimitSeats
	}
	return nil
}

// seatQuotaDB counts seats and reads billing inside the caller's transaction.
type seatQuotaDB interface {
	billingReader
	CountInternalSeatsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) (int64, error)
}

// assertReservedInternalSeatAcceptable gates AcceptInvitation for internal roles.
// Pending invites already reserve a seat, so accept is net-zero when used <= limit.
// After a plan downgrade that leaves used > limit, accept is denied until seats are freed.
// Guests never consume seats.
func (s *Service) assertReservedInternalSeatAcceptable(
	ctx context.Context,
	q seatQuotaDB,
	workspaceID, role string,
) error {
	if !isInternalSeatRole(role) {
		return nil
	}
	state, err := loadBillingFrom(ctx, q, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.InternalSeats <= 0 {
		return nil
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	used, err := q.CountInternalSeatsByWorkspace(ctx, wsUUID)
	if err != nil {
		return fmt.Errorf("count internal seats: %w", err)
	}
	if used > state.limits.InternalSeats {
		return plan.ErrLimitSeats
	}
	return nil
}

// AssertCanUseCustomDomain blocks Brand viewer-domain registration on plans without the feature.
func (s *Service) AssertCanUseCustomDomain(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.CustomDomain {
		return nil
	}
	return plan.ErrFeatureCustomDomain
}

// AssertCanUseWatermark blocks enabling watermark downloads and link viewer
// protections (overlay watermark / screenshot protection) on plans without the feature.
func (s *Service) AssertCanUseWatermark(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.Watermark {
		return nil
	}
	return plan.ErrFeatureWatermark
}

// AssertCanUseNDA blocks enabling link/room NDA on plans without the feature.
func (s *Service) AssertCanUseNDA(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.NDA {
		return nil
	}
	return plan.ErrFeatureNDA
}

// AssertCanUseVisitorAskAI blocks enabling deal-room ask_ai_enabled on plans without the feature.
func (s *Service) AssertCanUseVisitorAskAI(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.VisitorAskAI {
		return nil
	}
	return plan.ErrFeatureVisitorAskAI
}

// AssertCanUseBranding blocks logo / brand-color changes on plans without the feature.
func (s *Service) AssertCanUseBranding(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.Branding {
		return nil
	}
	return plan.ErrFeatureBranding
}

// AssertCanUseAccessControls blocks email verification and allow/block lists
// on plans without the feature.
func (s *Service) AssertCanUseAccessControls(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	if state.limits.AccessControls {
		return nil
	}
	return plan.ErrFeatureAccessControl
}

func (s *Service) assertLoadedFeature(enabled bool, deny error) error {
	if enabled {
		return nil
	}
	return deny
}

// AssertCanUseWebhooks blocks saving outbound webhooks on plans without the feature.
func (s *Service) AssertCanUseWebhooks(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	return s.assertLoadedFeature(state.limits.Webhooks, plan.ErrFeatureWebhooks)
}

// AssertCanUseHubSpot blocks HubSpot connect/sync on plans without the feature.
func (s *Service) AssertCanUseHubSpot(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	return s.assertLoadedFeature(state.limits.HubSpot, plan.ErrFeatureHubSpot)
}

// AssertCanUseDailyDigest blocks enabling the Insights daily digest on plans without the feature.
func (s *Service) AssertCanUseDailyDigest(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	return s.assertLoadedFeature(state.limits.DailyDigest, plan.ErrFeatureDailyDigest)
}

// AssertCanUseSlackAlerts blocks enabling sensitive-page Slack alerts on plans without the feature.
func (s *Service) AssertCanUseSlackAlerts(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	return s.assertLoadedFeature(state.limits.SlackAlerts, plan.ErrFeatureSlackAlerts)
}

// AssertCanUseRoomInsights blocks workspace Insights overview on plans without the feature.
func (s *Service) AssertCanUseRoomInsights(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	return s.assertLoadedFeature(state.limits.RoomInsights, plan.ErrFeatureRoomInsights)
}

// AssertCanUseRoomAnalytics blocks deal-room Analytics tab aggregates on plans without the feature.
func (s *Service) AssertCanUseRoomAnalytics(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	return s.assertLoadedFeature(state.limits.RoomAnalytics, plan.ErrFeatureRoomAnalytics)
}

// AssertCanUseFormalAsk blocks Formal Q&A on plans without the feature.
// Expired trial and missing billing fail-closed to Free (off).
func (s *Service) AssertCanUseFormalAsk(ctx context.Context, workspaceID string) error {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	return s.assertLoadedFeature(state.limits.FormalAsk, plan.ErrFeatureFormalAsk)
}

// AskAIMonthlyLimit implements plan.Checker.
// Zero means unlimited only when Visitor Ask AI is included (see AssertCanUseVisitorAskAI).
// When the feature is off this still returns 0 — callers must not treat that as unlimited.
func (s *Service) AskAIMonthlyLimit(ctx context.Context, workspaceID string) (int32, error) {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	if !state.limits.VisitorAskAI {
		return 0, nil
	}
	return state.limits.VisitorAskAIMonthly, nil
}

// KnowledgeAnswersQuota is the host Knowledge Desk calendar-month cap.
// Limit 0 is unlimited only when included is true (enterprise). Free returns
// included=false so callers must not treat 0 as unlimited. Expired trial is Free.
func (s *Service) KnowledgeAnswersQuota(ctx context.Context, workspaceID string) (limit int32, included bool, err error) {
	state, err := s.loadBilling(ctx, workspaceID)
	if err != nil {
		return 0, false, err
	}
	return state.limits.KnowledgeAnswersMonthly, state.limits.KnowledgeDesk, nil
}

// KnowledgeAnswersMonthly is the numeric cap. Zero is unlimited only when the
// desk is included; use KnowledgeAnswersQuota when feature-off vs unlimited matters.
func (s *Service) KnowledgeAnswersMonthly(ctx context.Context, workspaceID string) (int32, error) {
	n, _, err := s.KnowledgeAnswersQuota(ctx, workspaceID)
	return n, err
}
