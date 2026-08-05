import { ApiError } from "@/lib/apiClient";

type Translate = (key: string) => string;

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
      case "access_code_send_failed":
        return t("accessRequests.codeSendFailed");
      default:
        break;
    }
  }
  return t(fallbackKey);
}
