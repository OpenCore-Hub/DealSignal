// Package httpx provides client-safe HTTP error helpers.
// Handlers must never return infrastructure/database error text to clients.
package httpx

import (
	"errors"
	"net/http"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
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
	"internal_error":   MsgInternal,
	"storage_error":    "storage operation failed",
	"signature_error":  "failed to generate signed url",
	"watermark_failed": "failed to apply watermark",
	"oauth_failed":     "oauth operation failed",
	"sync_failed":      "sync failed",
	"export_failed":    "export failed",
	"anonymize_failed": "anonymize failed",
	"delete_failed":    "delete failed",
	"render_error":     "render failed",
	"ai_unavailable":   "ai service unavailable",
	"upstream_error":   "upstream service error",
}

// clientFallbacks are used when an infrastructure error is incorrectly
// returned under a client-facing code (e.g. pgx.ErrNoRows as workspace_not_found).
var clientFallbacks = map[string]string{
	"workspace_not_found":     "workspace not found",
	"room_not_found":          "deal room not found",
	"link_not_found":          "link not found",
	"document_not_found":      "document not found",
	"contact_not_found":       "contact not found",
	"member_not_found":        "member not found",
	"folder_not_found":        "folder not found",
	"request_not_found":       "request not found",
	"not_found":               "not found",
	"forbidden":               "forbidden",
	"access_denied":           "access denied",
	"document_locked":         "document is locked in this deal room",
	"document_out_of_scope":   "document is not included in this link",
	"unauthorized":            "unauthorized",
	"invalid_input":           "invalid input",
	"invalid_request":         "invalid request",
	"invalid_slug":            "invalid slug",
	"invalid_email":           "invalid email",
	"invalid_file":            "invalid file",
	"invalid_workspace":       "invalid workspace",
	"invalid_user":            "invalid user",
	"invalid_signer_name":     "invalid signer name",
	"duplicate_slug":          "slug already exists",
	"duplicate_name":          "name already exists",
	"resource_locked":         "resource is locked",
	"folder_exists":           "folder already exists",
	"folder_not_empty":        "folder is not empty",
	"rate_limited":            "rate limited",
	"access_request_exists":   "access request already exists",
	"nda_not_required":        "nda is not required",
	"nda_required":            "nda required",
	"approval_required":       "approval required",
	"template_archived":       "template is archived",
	"template_locked":         "template is locked",
	"payload_too_large":       "payload too large",
	"unsupported_media_type":  "unsupported media type",
	"agreement_pdf_required":            "agreement documents must be PDF",
	"agreement_not_allowed_in_deal_room": "agreement documents cannot be added to a deal room",
	"category_immutable":                 "document category cannot be changed",
	"category_deal_room_via_api":         "deal room category is managed by membership",
	"category_while_in_room":             "remove the document from deal rooms before changing category",
	"page_not_found":          "page not found",
	"invalid_id":              "invalid id",
	"invalid_payload":         "invalid payload",
	"invalid_access_rules":    "invalid access rules",
	"document_not_ready":      "document not ready",
	"link_disabled":           "link is disabled",
	"link_expired":            "link has expired",
	"invitation_not_found":    "invitation not found",
	"access_request_blocked":  "access request blocked",
	"access_request_not_found": "access request not found",
	"access_request_forbidden": "only the link creator can review access requests",
	"access_request_not_pending": "access request is not pending",
	"access_already_allowed":  "access already allowed",
	"requires_email":          "email required",
	"requires_email_code":     "email verification required",
	"invalid_email_code":      "invalid email code",
	"requires_nda":            "nda required",
	"requires_password":       "password required",
	"invalid_password":        "invalid password",
	"blocked_email":           "email is blocked",
	"not_allowed_email":       "email is not allowed",
	"delivery_email_mismatch": "email does not match invitation",
	"invite_expired":          "invitation expired",
	"invite_revoked":          "invitation revoked",
	"invite_already_used":     "invitation already used",
	"invite_token_failed":     "invitation link is invalid or cannot be used",
	"link_revoked":            "link has been revoked",
	"link_archived":           "link is archived",
	"link_max_access_reached": "link access limit reached",
	"not_allowed":             "email is not allowed",
	"email_mismatch":          "email does not match invitation",
	"slug_conflict":           "slug already exists",
	"already_member":          "already a member",
	"invalid_role":            "invalid role",
	"invitation_expired":      "invitation expired",
	"invitation_used":         "invitation already used",
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
