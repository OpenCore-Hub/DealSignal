package plan

import (
	"context"
	"errors"
	"net/http"
)

var (
	ErrLimitRooms           = errors.New("plan limit: data rooms")
	ErrLimitLinks           = errors.New("plan limit: share links")
	ErrLimitStorage         = errors.New("plan limit: storage")
	ErrLimitSeats           = errors.New("plan limit: internal seats")
	ErrLimitDocuments       = errors.New("plan limit: documents")
	ErrLimitUpload          = errors.New("plan limit: upload size")
	ErrLimitWorkspaces      = errors.New("plan limit: owned workspaces")
	ErrFeatureCustomDomain  = errors.New("plan feature: custom domain")
	ErrFeatureWatermark     = errors.New("plan feature: watermark downloads")
	ErrFeatureNDA           = errors.New("plan feature: nda")
	ErrFeatureVisitorAskAI  = errors.New("plan feature: visitor ask ai")
	ErrFeatureBranding      = errors.New("plan feature: branding")
	ErrFeatureAccessControl = errors.New("plan feature: access controls")
	ErrFeatureWebhooks      = errors.New("plan feature: webhooks")
	ErrFeatureHubSpot       = errors.New("plan feature: hubspot")
	ErrFeatureDailyDigest   = errors.New("plan feature: daily digest")
	ErrFeatureSlackAlerts   = errors.New("plan feature: slack alerts")
	ErrFeatureRoomInsights  = errors.New("plan feature: room insights")
	ErrFeatureRoomAnalytics = errors.New("plan feature: room analytics")
	ErrFeatureFormalAsk     = errors.New("plan feature: formal ask")
)

const (
	CodeLimitRooms           = "plan_limit_rooms"
	CodeLimitLinks           = "plan_limit_links"
	CodeLimitStorage         = "plan_limit_storage"
	CodeLimitSeats           = "plan_limit_seats"
	CodeLimitDocuments       = "plan_limit_documents"
	CodeLimitUpload          = "plan_limit_upload"
	CodeLimitWorkspaces      = "plan_limit_workspaces"
	CodeFeatureCustomDomain  = "plan_feature_custom_domain"
	CodeFeatureWatermark     = "plan_feature_watermark"
	CodeFeatureNDA           = "plan_feature_nda"
	CodeFeatureVisitorAskAI  = "plan_feature_visitor_ask_ai"
	CodeFeatureBranding      = "plan_feature_branding"
	CodeFeatureAccessControl = "plan_feature_access_controls"
	CodeFeatureWebhooks      = "plan_feature_webhooks"
	CodeFeatureHubSpot       = "plan_feature_hubspot"
	CodeFeatureDailyDigest   = "plan_feature_daily_digest"
	CodeFeatureSlackAlerts   = "plan_feature_slack_alerts"
	CodeFeatureRoomInsights  = "plan_feature_room_insights"
	CodeFeatureRoomAnalytics = "plan_feature_room_analytics"
	CodeFeatureFormalAsk     = "plan_feature_formal_ask"
)

// Checker enforces workspace plan limits on create paths.
// A nil checker must be treated as a no-op by callers.
type Checker interface {
	AssertCanCreateRoom(ctx context.Context, workspaceID string) error
	AssertCanCreateLink(ctx context.Context, workspaceID string) error
	AssertCanAddStorage(ctx context.Context, workspaceID string, additionalBytes int64) error
	AssertCanCreateDocument(ctx context.Context, workspaceID string) error
	AssertCanUploadFile(ctx context.Context, workspaceID string, size int64) error
	AssertCanUseWatermark(ctx context.Context, workspaceID string) error
	AssertCanUseNDA(ctx context.Context, workspaceID string) error
	AssertCanUseVisitorAskAI(ctx context.Context, workspaceID string) error
	AssertCanUseBranding(ctx context.Context, workspaceID string) error
	AssertCanUseAccessControls(ctx context.Context, workspaceID string) error
	AssertCanUseWebhooks(ctx context.Context, workspaceID string) error
	AssertCanUseHubSpot(ctx context.Context, workspaceID string) error
	AssertCanUseDailyDigest(ctx context.Context, workspaceID string) error
	AssertCanUseSlackAlerts(ctx context.Context, workspaceID string) error
	AssertCanUseRoomInsights(ctx context.Context, workspaceID string) error
	AssertCanUseRoomAnalytics(ctx context.Context, workspaceID string) error
	AssertCanUseFormalAsk(ctx context.Context, workspaceID string) error
	// AskAIMonthlyLimit is the workspace visitor-AI monthly cap.
	// 0 = unlimited only when Visitor Ask AI is included on the plan.
	AskAIMonthlyLimit(ctx context.Context, workspaceID string) (int32, error)
	// WithCreateRoomQuota asserts room capacity then runs fn while holding the
	// workspace billing lock (when the implementation supports it). fn should
	// perform the durable room insert before returning.
	WithCreateRoomQuota(ctx context.Context, workspaceID string, fn func(ctx context.Context) error) error
	// WithCreateLinkQuota asserts link capacity then runs fn while holding the
	// workspace billing lock (when the implementation supports it).
	WithCreateLinkQuota(ctx context.Context, workspaceID string, fn func(ctx context.Context) error) error
	// WithBillingLock runs fn while holding the workspace billing lock without
	// asserting capacity. Use for mutations that free quota (revoke/archive/delete)
	// so they serialize with create/renew consumers.
	WithBillingLock(ctx context.Context, workspaceID string, fn func(ctx context.Context) error) error
	// WithAddStorageQuota asserts storage headroom for additionalBytes then runs
	// fn while holding the workspace billing lock. additionalBytes <= 0 still
	// holds the lock (net-zero / shrink free paths) so they serialize with grows.
	WithAddStorageQuota(ctx context.Context, workspaceID string, additionalBytes int64, fn func(ctx context.Context) error) error
}

// Unrestricted is a Checker that never denies. Tests embed it and override methods.
type Unrestricted struct{}

func (Unrestricted) AssertCanCreateRoom(context.Context, string) error        { return nil }
func (Unrestricted) AssertCanCreateLink(context.Context, string) error        { return nil }
func (Unrestricted) AssertCanAddStorage(context.Context, string, int64) error { return nil }
func (Unrestricted) AssertCanCreateDocument(context.Context, string) error    { return nil }
func (Unrestricted) AssertCanUploadFile(context.Context, string, int64) error { return nil }
func (Unrestricted) AssertCanUseWatermark(context.Context, string) error      { return nil }
func (Unrestricted) AssertCanUseNDA(context.Context, string) error            { return nil }
func (Unrestricted) AssertCanUseVisitorAskAI(context.Context, string) error   { return nil }
func (Unrestricted) AssertCanUseBranding(context.Context, string) error       { return nil }
func (Unrestricted) AssertCanUseAccessControls(context.Context, string) error { return nil }
func (Unrestricted) AssertCanUseWebhooks(context.Context, string) error       { return nil }
func (Unrestricted) AssertCanUseHubSpot(context.Context, string) error        { return nil }
func (Unrestricted) AssertCanUseDailyDigest(context.Context, string) error    { return nil }
func (Unrestricted) AssertCanUseSlackAlerts(context.Context, string) error    { return nil }
func (Unrestricted) AssertCanUseRoomInsights(context.Context, string) error   { return nil }
func (Unrestricted) AssertCanUseRoomAnalytics(context.Context, string) error  { return nil }
func (Unrestricted) AssertCanUseFormalAsk(context.Context, string) error      { return nil }
func (Unrestricted) AskAIMonthlyLimit(context.Context, string) (int32, error) { return 0, nil }
func (Unrestricted) WithCreateRoomQuota(ctx context.Context, _ string, fn func(context.Context) error) error {
	return fn(ctx)
}
func (Unrestricted) WithCreateLinkQuota(ctx context.Context, _ string, fn func(context.Context) error) error {
	return fn(ctx)
}
func (Unrestricted) WithBillingLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	return fn(ctx)
}
func (Unrestricted) WithAddStorageQuota(ctx context.Context, _ string, _ int64, fn func(context.Context) error) error {
	return fn(ctx)
}

// HTTPError maps quota/feature errors to a client status and code.
func HTTPError(err error) (status int, code string, ok bool) {
	switch {
	case errors.Is(err, ErrLimitRooms):
		return http.StatusForbidden, CodeLimitRooms, true
	case errors.Is(err, ErrLimitLinks):
		return http.StatusForbidden, CodeLimitLinks, true
	case errors.Is(err, ErrLimitStorage):
		return http.StatusForbidden, CodeLimitStorage, true
	case errors.Is(err, ErrLimitSeats):
		return http.StatusForbidden, CodeLimitSeats, true
	case errors.Is(err, ErrLimitDocuments):
		return http.StatusForbidden, CodeLimitDocuments, true
	case errors.Is(err, ErrLimitUpload):
		return http.StatusForbidden, CodeLimitUpload, true
	case errors.Is(err, ErrLimitWorkspaces):
		return http.StatusForbidden, CodeLimitWorkspaces, true
	case errors.Is(err, ErrFeatureCustomDomain):
		return http.StatusForbidden, CodeFeatureCustomDomain, true
	case errors.Is(err, ErrFeatureWatermark):
		return http.StatusForbidden, CodeFeatureWatermark, true
	case errors.Is(err, ErrFeatureNDA):
		return http.StatusForbidden, CodeFeatureNDA, true
	case errors.Is(err, ErrFeatureVisitorAskAI):
		return http.StatusForbidden, CodeFeatureVisitorAskAI, true
	case errors.Is(err, ErrFeatureBranding):
		return http.StatusForbidden, CodeFeatureBranding, true
	case errors.Is(err, ErrFeatureAccessControl):
		return http.StatusForbidden, CodeFeatureAccessControl, true
	case errors.Is(err, ErrFeatureWebhooks):
		return http.StatusForbidden, CodeFeatureWebhooks, true
	case errors.Is(err, ErrFeatureHubSpot):
		return http.StatusForbidden, CodeFeatureHubSpot, true
	case errors.Is(err, ErrFeatureDailyDigest):
		return http.StatusForbidden, CodeFeatureDailyDigest, true
	case errors.Is(err, ErrFeatureSlackAlerts):
		return http.StatusForbidden, CodeFeatureSlackAlerts, true
	case errors.Is(err, ErrFeatureRoomInsights):
		return http.StatusForbidden, CodeFeatureRoomInsights, true
	case errors.Is(err, ErrFeatureRoomAnalytics):
		return http.StatusForbidden, CodeFeatureRoomAnalytics, true
	case errors.Is(err, ErrFeatureFormalAsk):
		return http.StatusForbidden, CodeFeatureFormalAsk, true
	default:
		return 0, "", false
	}
}
