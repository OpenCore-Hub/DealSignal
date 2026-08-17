import { useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { AccessRequestsInbox } from "@/components/links/AccessRequestsInbox";
import { api } from "@/lib/api";
import { useAccessRequestReview } from "@/hooks/useAccessRequestReview";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { LinkAccessRequest } from "@/types";

interface LinkAccessRequestsPanelProps {
  linkId: string;
  /** Called after approve/reject. Approve passes the granted email so parents can sync allowlists. */
  onChanged?: (detail?: { email?: string; action: "approve" | "reject" }) => void;
  canReview?: boolean;
}

export function LinkAccessRequestsPanel({ linkId, onChanged, canReview }: LinkAccessRequestsPanelProps) {
  const { t } = useTranslation(["linkShare", "common"]);
  const {
    data: requests,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    const res = await api.getLinkAccessRequests(linkId);
    return res.data ?? [];
  }, [linkId]);

  const pending = useMemo(
    () => (requests ?? []).filter((r) => r.status === "pending"),
    [requests],
  );

  const afterReview = useCallback(
    async (detail: { email: string; action: "approve" | "reject" }) => {
      await refetch();
      onChanged?.({ email: detail.email, action: detail.action });
    },
    [onChanged, refetch],
  );
  const { busyId, approve, reject } = useAccessRequestReview(afterReview);

  if (loading && !requests && !error) {
    return (
      <p className="py-2 text-sm text-muted-foreground">{t("common:loading")}</p>
    );
  }

  if (error) {
    return (
      <div
        className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm"
        role="alert"
        data-testid="link-access-requests-error"
      >
        <p className="text-destructive">{t("linkShare:accessRequests.loadFailed")}</p>
        <Button
          size="sm"
          variant="outline"
          className="mt-2"
          onClick={() => { void refetch(); }}
        >
          {t("common:retry")}
        </Button>
      </div>
    );
  }

  if (pending.length === 0) {
    return null;
  }

  return (
    <AccessRequestsInbox
      title={t("linkShare:accessRequests.title")}
      description={t("linkShare:accessRequests.description")}
      requests={pending}
      busyId={busyId}
      itemTestIdPrefix="link-access-request"
      onApprove={(request: LinkAccessRequest) => { void approve(linkId, request); }}
      onReject={(request: LinkAccessRequest) => { void reject(linkId, request); }}
      canReview={canReview}
    />
  );
}
