import i18n from "@/i18n/config";
import { ApiError } from "@/lib/apiClient";
import { accessRequestReviewErrorMessage } from "@/lib/accessRequestErrors";
import { agreementCategoryErrorCode } from "@/lib/documentCategory";
import { knowledgeErrorMessage } from "@/lib/knowledge/errors";
import {
  viewerAccessErrorKind,
  viewerPolicyBlockI18nKeys,
} from "@/lib/viewerAccessErrors";

export type ApiErrorFallbackKey =
  | "loadFailed"
  | "saveFailed"
  | "deleteFailed"
  | "uploadFailed"
  | "copyFailed";

export type ApiErrorContext =
  | "login"
  | "register"
  | "verifyEmail"
  | "viewerGate"
  | "acceptInvitation"
  | "knowledge"
  | "accessRequestApprove"
  | "accessRequestReject";

export interface ApiErrorMessageOptions {
  fallback?: ApiErrorFallbackKey;
  context?: ApiErrorContext;
  /** Full i18n key used when no API code mapping matches. */
  messageKey?: string;
  messageKeyParams?: Record<string, unknown>;
}

const VIEWER_INLINE_KEYS: Record<string, string> = {
  invalid_password: "documents:viewer.invalidPassword",
  invalid_email_code: "documents:viewer.invalidEmailCode",
  whitelist_denied: "documents:viewer.whitelistDenied",
  invalid_signer_name: "documents:viewer.signerNameRequired",
  email_mismatch: "documents:viewer.emailMismatch",
  not_allowed: "documents:viewer.emailNotAllowed",
  blocked_email: "documents:viewer.emailBlocked",
};

const LINK_UNAVAILABLE_CODES = new Set([
  "link_not_found",
  "link_expired",
  "link_revoked",
  "link_disabled",
  "link_max_access_reached",
  "invite_expired",
  "invite_revoked",
  "invite_already_used",
]);

const AUTH_LOGIN_CODES: Record<string, string> = {
  unauthorized: "auth:login.errorInvalidCredentials",
  invalid_email: "auth:login.errorInvalidEmail",
  invalid_user: "auth:login.errorInvalidCredentials",
};

const AUTH_REGISTER_CODES: Record<string, string> = {
  unauthorized: "auth:register.errorRegistrationFailed",
  invalid_email: "auth:register.errorInvalidEmail",
  duplicate_email: "auth:register.errorEmailTaken",
  already_member: "auth:register.errorEmailTaken",
};

function translate(key: string, options?: Record<string, unknown>): string {
  return i18n.t(key, { defaultValue: "", ...options });
}

function hasKey(key: string): boolean {
  return i18n.exists(key);
}

function resolveFallback(options: ApiErrorMessageOptions): string {
  if (options.messageKey && hasKey(options.messageKey)) {
    return translate(options.messageKey, options.messageKeyParams);
  }
  return translate(`common:error.${options.fallback ?? "loadFailed"}`);
}

/** Never surface raw server strings; mirror backend httpx.SafeMessage on the client. */
export function apiErrorMessage(
  err: unknown,
  options: ApiErrorMessageOptions = {},
): string {
  const fallback = resolveFallback(options);

  if (!(err instanceof ApiError)) {
    return fallback;
  }

  const code = err.code?.trim();
  if (!code) {
    return fallback;
  }

  if (options.context === "login") {
    const authKey = AUTH_LOGIN_CODES[code];
    if (authKey && hasKey(authKey)) {
      return translate(authKey);
    }
  }

  if (options.context === "register") {
    const authKey = AUTH_REGISTER_CODES[code];
    if (authKey && hasKey(authKey)) {
      return translate(authKey);
    }
  }

  if (options.context === "verifyEmail") {
    return translate("auth:verifyEmail.error");
  }

  if (options.context === "acceptInvitation") {
    if (code === "invitation_email_mismatch" || code === "email_mismatch") {
      return translate("auth:acceptInvitation.emailMismatch");
    }
  }

  if (options.context === "knowledge") {
    return knowledgeErrorMessage((key) => translate(`dealRooms:${key}`), code);
  }

  if (options.context === "accessRequestApprove") {
    return accessRequestReviewErrorMessage(
      err,
      (key) => translate(`linkShare:${key}`),
      "accessRequests.approveError",
    );
  }

  if (options.context === "accessRequestReject") {
    return accessRequestReviewErrorMessage(
      err,
      (key) => translate(`linkShare:${key}`),
      "accessRequests.rejectError",
    );
  }

  if (code === "duplicate_slug" || code === "slug_conflict") {
    return translate("common:error.duplicateSlug");
  }
  if (code === "invalid_slug") {
    return translate("common:error.invalidSlug");
  }
  if (code === "duplicate_name") {
    return translate("linkShare:share.linkNameDuplicate");
  }

  if (agreementCategoryErrorCode(code)) {
    return translate(`documents:detail.categoryErrors.${code}`);
  }

  // Viewer gate codes (e.g. email_mismatch) must not leak into workspace-invite UX.
  if (options.context === "viewerGate") {
    const viewerInlineKey = VIEWER_INLINE_KEYS[code];
    if (viewerInlineKey && hasKey(viewerInlineKey)) {
      return translate(viewerInlineKey);
    }
  }

  const policyKeys = viewerPolicyBlockI18nKeys(viewerAccessErrorKind(code));
  if (policyKeys && hasKey(policyKeys.descriptionKey)) {
    return translate(policyKeys.descriptionKey);
  }

  if (LINK_UNAVAILABLE_CODES.has(code)) {
    const unavailableKey = `documents:viewer.${code}Description`;
    if (hasKey(unavailableKey)) {
      return translate(unavailableKey);
    }
  }

  const codeKey = `common:error.codes.${code}`;
  if (hasKey(codeKey)) {
    return translate(codeKey);
  }

  return fallback;
}

/** Friendly label for ingestion / processing failures (never show pipeline internals). */
export function ingestionErrorLabel(message?: string | null): string | undefined {
  const trimmed = message?.trim();
  if (!trimmed) {
    return undefined;
  }
  return translate("common:error.codes.document_processing_failed");
}
