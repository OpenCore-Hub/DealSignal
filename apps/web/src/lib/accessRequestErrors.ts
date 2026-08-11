import { ApiError } from "@/lib/apiClient";

type Translate = (key: string) => string;

export const ACCESS_CODE_SEND_FAILED = "access_code_send_failed";

/**
 * Legacy approve path: older APIs returned 502 access_code_send_failed after
 * the approval row was already committed. Current API returns 200 + warning.
 */
export function isAccessCodeSendFailedAfterApprove(err: unknown): boolean {
  return err instanceof ApiError && err.code === ACCESS_CODE_SEND_FAILED;
}

/** Current approve path: soft warning on 200 when code email failed. */
export function isAccessCodeSendFailedWarning(
  warning: { code?: string } | null | undefined,
): boolean {
  return warning?.code === ACCESS_CODE_SEND_FAILED;
}

/** Map approve/reject API failures to i18n-safe messages (no raw server strings). */
export function accessRequestReviewErrorMessage(
  err: unknown,
  t: Translate,
  fallbackKey: "accessRequests.approveError" | "accessRequests.rejectError",
): string {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "access_request_forbidden":
      case "access_request_not_found":
      case "link_not_found":
        // Opaque: missing request, foreign workspace, and non-creator share one message.
        return t("accessRequests.notFound");
      case "access_request_not_pending":
        return t("accessRequests.notPending");
      case "access_request_blocked":
        return t("accessRequests.blocked");
      case ACCESS_CODE_SEND_FAILED:
        return t("accessRequests.codeSendFailed");
      default:
        break;
    }
  }
  return t(fallbackKey);
}
