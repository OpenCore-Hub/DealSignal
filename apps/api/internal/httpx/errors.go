// Package httpx provides client-safe HTTP error helpers.
// Handlers must never return infrastructure/database error text to clients.
package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MsgInternal is the only message clients should see for unexpected failures.
const MsgInternal = "an unexpected error occurred"

// IsInfrastructure reports whether err is a database/driver/connectivity failure
// whose details must never be returned to clients.
func IsInfrastructure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return true
	}
	msg := err.Error()
	needles := []string{
		"SQLSTATE",
		"ERROR:",
		"violates foreign key",
		"violates unique constraint",
		"violates check constraint",
		"violates not-null constraint",
		"duplicate key value",
		"connection refused",
		"connection reset",
		"driver: bad connection",
		"failed to connect to",
		"no rows in result set",
		"pq:",
		"pgconn:",
		"pgx:",
	}
	lower := strings.ToLower(msg)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

// PublicMessage returns a client-safe message.
// Infrastructure errors always resolve to fallback (or MsgInternal).
// Domain/sentinel errors may pass through when they are not infrastructure.
func PublicMessage(err error, fallback string) string {
	if err == nil {
		if fallback != "" {
			return fallback
		}
		return MsgInternal
	}
	if IsInfrastructure(err) {
		if fallback != "" {
			return fallback
		}
		return MsgInternal
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		if fallback != "" {
			return fallback
		}
		return MsgInternal
	}
	return msg
}

// Internal writes a generic 500 response and logs the real error server-side.
func Internal(c *gin.Context, err error, logMsg string) {
	if logMsg == "" {
		logMsg = "request failed"
	}
	if err != nil {
		logger.ErrorCtx(c.Request.Context(), logMsg, err)
	} else {
		logger.ErrorCtx(c.Request.Context(), logMsg, errors.New("unknown error"))
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"code":    "internal_error",
		"message": MsgInternal,
	})
}

// AbortInternal is Internal followed by c.Abort().
func AbortInternal(c *gin.Context, err error, logMsg string) {
	Internal(c, err, logMsg)
	c.Abort()
}

// JSON writes a structured error with a scrubbed message.
func JSON(c *gin.Context, status int, code string, err error, fallback string) {
	c.JSON(status, gin.H{
		"code":    code,
		"message": PublicMessage(err, fallback),
	})
}

// serverOnlyCodes must never expose err.Error() to clients.
var serverOnlyCodes = map[string]string{
	"internal_error":          MsgInternal,
	"storage_error":           "storage operation failed",
	"signature_error":         "failed to generate signed url",
	"watermark_failed":        "failed to apply watermark",
	"oauth_failed":            "oauth operation failed",
	"oauth_not_configured":    "integration oauth is not configured",
	"sync_failed":             "sync failed",
	"export_failed":           "export failed",
	"anonymize_failed":        "anonymize failed",
	"delete_failed":           "delete failed",
	"render_error":            "render failed",
	"ai_unavailable":          "ai service unavailable",
	"upstream_error":          "upstream service error",
	"access_code_send_failed": "could not send verification code",
}

// clientFallbacks are used when an infrastructure error is incorrectly
// returned under a client-facing code (e.g. pgx.ErrNoRows as workspace_not_found).
var clientFallbacks = map[string]string{
	"workspace_not_found":                "workspace not found",
	"room_not_found":                     "data room not found",
	"link_not_found":                     "link not found",
	"link_not_renewable":                 "only archived or expired links can be renewed",
	"document_not_found":                 "document not found",
	"contact_not_found":                  "contact not found",
	"member_not_found":                   "member not found",
	"folder_not_found":                   "folder not found",
	"request_not_found":                  "request not found",
	"not_found":                          "not found",
	"forbidden":                          "forbidden",
	"access_denied":                      "access denied",
	"document_locked":                    "document is locked in this data room",
	"document_out_of_scope":              "document is not included in this link",
	"unauthorized":                       "unauthorized",
	"invalid_input":                      "invalid input",
	"invalid_webhook_url":                "invalid webhook url",
	"recipients_not_in_workspace":        "one or more recipients are not contacts in this workspace",
	"too_many_recipients":                "recipient list exceeds the maximum allowed for a single send",
	"invalid_request":                    "invalid request",
	"invalid_slug":                       "invalid slug",
	"invalid_name":                       "invalid data room name",
	"invalid_email":                      "invalid email",
	"invalid_file":                       "invalid file",
	"empty_file":                         "file is empty",
	"file_empty":                         "file is empty",
	"file_too_large":                     "file is too large",
	"unsupported_upload":                 "file cannot be uploaded",
	"unsupported_type":                   "unsupported file type",
	"not_file_request_link":              "this link does not accept file uploads",
	"invalid_workspace":                  "invalid workspace",
	"invalid_user":                       "invalid user",
	"invalid_signer_name":                "invalid signer name",
	"duplicate_slug":                     "slug already exists",
	"duplicate_name":                     "name already exists",
	"resource_locked":                    "resource is locked",
	"folder_exists":                      "folder already exists",
	"folder_not_empty":                   "folder is not empty",
	"rate_limited":                       "rate limited",
	"access_request_exists":              "access request already exists",
	"nda_not_required":                   "nda is not required",
	"nda_required":                       "nda required",
	"nda_agreement_required":             "nda agreement is required",
	"approval_required":                  "approval required",
	"template_archived":                  "template is archived",
	"template_locked":                    "template is locked",
	"payload_too_large":                  "payload too large",
	"unsupported_media_type":             "unsupported media type",
	"agreement_pdf_required":             "agreement documents must be PDF",
	"agreement_not_allowed_in_deal_room": "agreement documents cannot be added to a data room",
	"category_immutable":                 "document category cannot be changed",
	"category_deal_room_via_api":         "data room category is managed by membership",
	"category_while_in_room":             "remove the document from data rooms before changing category",
	"page_not_found":                     "page not found",
	"invalid_id":                         "invalid id",
	"invalid_payload":                    "invalid payload",
	"invalid_access_rules":               "invalid access rules",
	"document_not_ready":                 "document not ready",
	"link_disabled":                      "link is disabled",
	"link_expired":                       "link has expired",
	"invitation_not_found":               "invitation not found",
	"access_request_blocked":             "access request blocked",
	"access_request_not_found":           "access request not found",
	"access_request_forbidden":           "only the link creator can review access requests",
	"access_request_not_pending":         "access request is not pending",
	"access_already_allowed":             "access already allowed",
	"requires_email":                     "email required",
	"requires_email_code":                "email verification required",
	"invalid_email_code":                 "invalid email code",
	"requires_nda":                       "nda required",
	"requires_password":                  "password required",
	"invalid_password":                   "invalid password",
	"blocked_email":                      "email is blocked",
	"not_allowed_email":                  "email is not allowed",
	"delivery_email_mismatch":            "email does not match invitation",
	"invite_expired":                     "invitation expired",
	"invite_revoked":                     "invitation revoked",
	"invite_already_used":                "invitation already used",
	"invite_token_failed":                "invitation link is invalid or cannot be used",
	"link_revoked":                       "link has been revoked",
	"link_archived":                      "link is archived",
	"link_max_access_reached":            "link access limit reached",
	"not_allowed":                        "email is not allowed",
	"email_mismatch":                     "email does not match invitation",
	"invitation_email_mismatch":          "signed-in email does not match this workspace invitation",
	"slug_conflict":                      "slug already exists",
	"already_member":                     "already a member",
	"invalid_domain":                     "invalid domain",
	"domain_exists":                      "domain already registered",
	"not_verified":                       "domain is not verified",
	"cname_missing":                      "no cname record found",
	"viewer_domain_not_configured":       "viewer domain is not configured",
	"invalid_role":                       "invalid role",
	"invitation_expired":                 "invitation expired",
	"invitation_used":                    "invitation already used",
	"insufficient_role":                  "your role does not allow this action",
	"cannot_modify_owner":                "cannot modify the workspace owner",
	"cannot_modify_self":                 "cannot change your own membership here",
	"cannot_manage_member":               "cannot manage this member",
	"plan_limit_rooms":                   "data room limit reached for this plan",
	"plan_limit_links":                   "share link limit reached for this plan",
	"plan_limit_storage":                 "storage limit reached for this plan",
	"plan_limit_seats":                   "internal seat limit reached for this plan",
	"plan_limit_documents":               "document limit reached for this plan",
	"plan_limit_upload":                  "file exceeds the upload size limit for this plan",
	"plan_limit_workspaces":              "owned workspace limit reached for this plan",
	"email_unverified":                   "verify your email before creating another workspace",
	"plan_feature_custom_domain":         "custom viewer domain is not available on this plan",
	"plan_feature_watermark":             "watermark and viewer protection features are not available on this plan",
	"plan_feature_nda":                   "nda requirements are not available on this plan",
	"plan_feature_visitor_ask_ai":        "visitor ask ai is not available on this plan",
	"plan_feature_branding":              "workspace branding is not available on this plan",
	"plan_feature_access_controls":       "email verification and allow/block lists are not available on this plan",
	"plan_feature_webhooks":              "outbound webhooks are not available on this plan",
	"plan_feature_hubspot":               "hubspot crm is not available on this plan",
	"plan_feature_daily_digest":          "the insights daily digest is not available on this plan",
	"plan_feature_slack_alerts":          "sensitive-page slack alerts are not available on this plan",
	"plan_feature_room_insights":         "data room insights are not available on this plan",
	"plan_feature_room_analytics":        "data room analytics are not available on this plan",
	"plan_feature_formal_ask":            "formal q&a is not available on this plan",
	"invalid_plan":                       "that plan cannot be selected",
	"invalid_period":                     "invalid billing period",
	"plan_payment_required":              "this plan requires checkout before it can be activated",
	"plan_sales_assisted":                "enterprise plans are provisioned with sales, not self-serve checkout",
	"plan_manage_via_portal":             "manage this subscription in the billing portal",
	"plan_no_stripe_customer":            "this workspace has no billing customer yet",
	"stripe_not_configured":              "checkout is not configured",
}

// SafeMessage returns a client-safe message for the given API error code.
// Server-only codes always use a fixed public string; other codes pass through
// domain errors but scrub infrastructure/database details.
func SafeMessage(code string, err error) string {
	if fixed, ok := serverOnlyCodes[code]; ok {
		return fixed
	}
	fallback := clientFallbacks[code]
	if fallback == "" {
		fallback = MsgInternal
	}
	return PublicMessage(err, fallback)
}

// WriteIfPlanLimit writes a 403 plan-limit response. Returns true when err is a quota error.
func WriteIfPlanLimit(c *gin.Context, err error) bool {
	status, code, ok := plan.HTTPError(err)
	if !ok {
		return false
	}
	plan.RecordQuotaDenial(code)
	attrs := make([]slog.Attr, 0, 4)
	attrs = append(attrs, logger.Attr("code", code))
	// workspaceID matches middleware.workspaceIDKey without importing middleware (cycle).
	if v, exists := c.Get("workspaceID"); exists {
		if wsID, ok := v.(string); ok && wsID != "" {
			attrs = append(attrs, logger.Attr("workspace_id", wsID))
		}
	}
	ctx := context.Background()
	if c.Request != nil {
		ctx = c.Request.Context()
		attrs = append(attrs, logger.Attr("method", c.Request.Method))
		if c.Request.URL != nil {
			attrs = append(attrs, logger.Attr("path", c.Request.URL.Path))
		}
	}
	logger.InfoCtx(ctx, "plan quota denied", attrs...)
	c.JSON(status, gin.H{"code": code, "message": SafeMessage(code, err)})
	return true
}
