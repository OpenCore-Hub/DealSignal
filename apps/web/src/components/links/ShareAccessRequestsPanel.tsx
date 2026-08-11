import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { AccessRequestsInbox } from "@/components/links/AccessRequestsInbox";
import { api } from "@/lib/api";
import { useAccessRequestReview } from "@/hooks/useAccessRequestReview";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { PendingLinkAccessRequest } from "@/types";

interface ShareAccessRequestsPanelProps {
  /** When set, only show requests for these link ids (document filter). */
  linkIds?: string[];
  onChanged?: () => void;
}

export function ShareAccessRequestsPanel({
  linkIds,
  onChanged,
}: ShareAccessRequestsPanelProps) {
  const { t } = useTranslation(["links", "linkShare", "common"]);
  const [searchParams] = useSearchParams();
  const focusLinkId = searchParams.get("linkId") ?? undefined;
  const {
    data: requests,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    // Document Library surface: never include deal-room share applicants.
    const res = await api.getPendingLinkAccessRequests({ scope: "document" });
    return res.data ?? [];
  }, []);

  const pending = useMemo(() => {
    const all = requests ?? [];
    // undefined = workspace-wide inbox; [] = filtered to no links → empty.
    if (linkIds === undefined) return all;
    const allowed = new Set(linkIds);
    return all.filter((r) => allowed.has(r.link_id));
  }, [requests, linkIds]);

  const inboxItems = useMemo(
    () =>
      pending.map((request) => {
        const title = request.document_title || request.link_name || request.short_url;
        return {
          ...request,
          is_workspace_member: Boolean(request.is_workspace_member),
          documentLabel: title
            ? t("links:accessRequests.forDocument", { title })
            : undefined,
        };
      }),
    [pending, t],
  );

  const afterReview = useCallback(async () => {
    await refetch();
    onChanged?.();
  }, [onChanged, refetch]);
  const { busyId, approve, reject } = useAccessRequestReview(afterReview);

  if (loading && !requests && !error) {
    return (
      <p className="py-2 text-sm text-muted-foreground" data-testid="share-access-requests-loading">
        {t("common:loading")}
      </p>
    );
  }

  if (error) {
    return (
      <div
        className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm"
        role="alert"
        data-testid="share-access-requests-error"
      >
        <p className="text-destructive">{t("linkShare:accessRequests.loadFailed")}</p>
        <Button size="sm" variant="outline" className="mt-2" onClick={() => { void refetch(); }}>
          {t("common:retry")}
        </Button>
      </div>
    );
  }

  if (inboxItems.length === 0) {
    return null;
  }

  return (
    <AccessRequestsInbox
      title={t("links:accessRequests.inboxTitle")}
      description={t("links:accessRequests.inboxDescription")}
      requests={inboxItems}
      busyId={busyId}
      focusLinkId={focusLinkId}
      testId="share-access-requests-panel"
      itemTestIdPrefix="share-access-request"
      onApprove={(request: PendingLinkAccessRequest) => {
        void approve(request.link_id, request);
      }}
      onReject={(request: PendingLinkAccessRequest) => {
        void reject(request.link_id, request);
      }}
    />
  );
}
