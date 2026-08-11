import { useCallback, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import {
  accessRequestReviewErrorMessage,
  isAccessCodeSendFailedAfterApprove,
  isAccessCodeSendFailedWarning,
} from "@/lib/accessRequestErrors";
import { ApiError } from "@/lib/apiClient";
import { apiErrorMessage } from "@/lib/apiErrors";

export type AccessRequestReviewDetail = {
  email: string;
  action: "approve" | "reject";
  linkId: string;
  requestId: string;
};

type Reviewable = {
  id: string;
  email: string;
};

async function resendAccessCodeFromToast(
  linkId: string,
  email: string,
  t: (key: string) => string,
) {
  try {
    await api.resendLinkAccessCode(linkId, email, true);
    toast.success(t("accessRequests.resendCodeSuccess"));
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.code === "rate_limited" || err.status === 429) {
        toast.error(t("analytics.resendRateLimited"));
        return;
      }
      if (err.code === "resend_not_needed" || err.status === 409) {
        toast.message(t("analytics.resendNotNeeded"));
        return;
      }
    }
    toast.error(
      apiErrorMessage(err, { messageKey: "linkShare:accessRequests.resendCodeFailed" }),
    );
  }
}

function toastApprovedButCodeSendFailed(
  linkId: string,
  email: string,
  t: (key: string) => string,
) {
  toast.warning(t("accessRequests.codeSendFailed"), {
    duration: 12_000,
    action: {
      label: t("accessRequests.resendCode"),
      onClick: () => {
        void resendAccessCodeFromToast(linkId, email, t);
      },
    },
  });
}

/**
 * Shared approve/reject flow for link access requests (toasts + i18n-safe errors).
 * Callers own refetch / parent sync via afterReview.
 */
export function useAccessRequestReview(
  afterReview?: (detail: AccessRequestReviewDetail) => void | Promise<void>,
) {
  const { t } = useTranslation("linkShare");
  const [busyId, setBusyId] = useState<string | null>(null);
  const afterReviewRef = useRef(afterReview);
  afterReviewRef.current = afterReview;

  const translate = useCallback((key: string) => t(key), [t]);

  const approve = useCallback(
    async (linkId: string, request: Reviewable) => {
      setBusyId(request.id);
      try {
        const res = await api.approveLinkAccessRequest(linkId, request.id);
        if (isAccessCodeSendFailedWarning(res.warning)) {
          toastApprovedButCodeSendFailed(linkId, request.email, translate);
        } else {
          toast.success(t("accessRequests.approveSuccess"));
        }
        await afterReviewRef.current?.({
          email: request.email,
          action: "approve",
          linkId,
          requestId: request.id,
        });
      } catch (err) {
        // Legacy 502 path (older API): approval was still committed server-side.
        if (isAccessCodeSendFailedAfterApprove(err)) {
          toastApprovedButCodeSendFailed(linkId, request.email, translate);
          await afterReviewRef.current?.({
            email: request.email,
            action: "approve",
            linkId,
            requestId: request.id,
          });
          return;
        }
        toast.error(
          accessRequestReviewErrorMessage(
            err,
            translate,
            "accessRequests.approveError",
          ),
        );
      } finally {
        setBusyId(null);
      }
    },
    [t, translate],
  );

  const reject = useCallback(
    async (linkId: string, request: Reviewable) => {
      setBusyId(request.id);
      try {
        await api.rejectLinkAccessRequest(linkId, request.id);
        toast.success(t("accessRequests.rejectSuccess"));
        await afterReviewRef.current?.({
          email: request.email,
          action: "reject",
          linkId,
          requestId: request.id,
        });
      } catch (err) {
        toast.error(
          accessRequestReviewErrorMessage(
            err,
            translate,
            "accessRequests.rejectError",
          ),
        );
      } finally {
        setBusyId(null);
      }
    },
    [t, translate],
  );

  return { busyId, approve, reject };
}
