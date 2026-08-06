import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, X, UserPlus } from "@phosphor-icons/react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { api } from "@/lib/api";
import { accessRequestReviewErrorMessage } from "@/lib/accessRequestErrors";
import { useAccessRequestReview } from "@/hooks/useAccessRequestReview";
import { useAsyncData } from "@/hooks/useAsyncData";

type PendingAccessRequest = {
  id: string;
  email: string;
  reason?: string;
  signerName?: string;
  /** Present for share-link requests; absent for room-membership requests. */
  linkId?: string;
  linkName?: string;
  source: "room" | "link";
};

interface DealRoomAccessRequestsPanelProps {
  roomId: string;
  /** When set (e.g. dashboard deep link), highlight matching share-link applicants. */
  focusLinkId?: string;
  onChanged?: (detail?: {
    email?: string;
    action: "approve" | "reject";
    source: "room" | "link";
  }) => void;
}

export function DealRoomAccessRequestsPanel({
  roomId,
  focusLinkId,
  onChanged,
}: DealRoomAccessRequestsPanelProps) {
  const { t } = useTranslation(["dealRooms", "linkShare", "common"]);
  const [busyId, setBusyId] = useState<string | null>(null);
  const {
    data: requests,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    // Room membership requests + deal-room-scoped link inbox (creator-only emails).
    const [roomRes, pendingLinkRes] = await Promise.all([
      api.getDealRoomAccessRequests(roomId),
      api.getPendingLinkAccessRequests({ scope: "deal_room", dealRoomId: roomId }),
    ]);
    const roomPending: PendingAccessRequest[] = (roomRes.data ?? [])
      .filter((r) => r.status === "pending")
      .map((r) => ({
        id: r.id,
        email: r.email,
        reason: r.reason,
        source: "room" as const,
      }));

    const linkPending: PendingAccessRequest[] = (pendingLinkRes.data ?? []).map((r) => ({
      id: r.id,
      email: r.email,
      reason: r.reason,
      signerName: r.signer_name,
      linkId: r.link_id,
      linkName: r.link_name || undefined,
      source: "link" as const,
    }));

    return [...roomPending, ...linkPending];
  }, [roomId]);

  const pending = requests ?? [];

  const afterLinkReview = useCallback(
    async (detail: { email: string; action: "approve" | "reject" }) => {
      await refetch();
      onChanged?.({ ...detail, source: "link" });
    },
    [onChanged, refetch],
  );
  const {
    busyId: linkBusyId,
    approve: approveLink,
    reject: rejectLink,
  } = useAccessRequestReview(afterLinkReview);

  const handleApprove = useCallback(
    async (request: PendingAccessRequest) => {
      if (request.source === "link" && request.linkId) {
        await approveLink(request.linkId, request);
        return;
      }
      setBusyId(request.id);
      try {
        await api.approveDealRoomAccessRequest(roomId, request.id);
        toast.success(t("dealRooms:accessRequests.approveSuccess"));
        await refetch();
        onChanged?.({
          email: request.email,
          action: "approve",
          source: "room",
        });
      } catch (err) {
        toast.error(
          accessRequestReviewErrorMessage(
            err,
            (key) => t(`linkShare:${key}`),
            "accessRequests.approveError",
          ),
        );
      } finally {
        setBusyId(null);
      }
    },
    [approveLink, onChanged, refetch, roomId, t],
  );

  const handleReject = useCallback(
    async (request: PendingAccessRequest) => {
      if (request.source === "link" && request.linkId) {
        await rejectLink(request.linkId, request);
        return;
      }
      setBusyId(request.id);
      try {
        await api.rejectDealRoomAccessRequest(roomId, request.id);
        toast.success(t("dealRooms:accessRequests.rejectSuccess"));
        await refetch();
        onChanged?.({
          email: request.email,
          action: "reject",
          source: "room",
        });
      } catch (err) {
        toast.error(
          accessRequestReviewErrorMessage(
            err,
            (key) => t(`linkShare:${key}`),
            "accessRequests.rejectError",
          ),
        );
      } finally {
        setBusyId(null);
      }
    },
    [onChanged, refetch, rejectLink, roomId, t],
  );

  const activeBusyId = busyId ?? linkBusyId;

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
        data-testid="deal-room-access-requests-error"
      >
        <p className="text-destructive">{t("dealRooms:accessRequests.loadFailed")}</p>
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
    <Card
      className="border-amber-500/30 bg-amber-500/5"
      data-testid="deal-room-access-requests-panel"
    >
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-h3">
          <UserPlus size={20} />
          {t("dealRooms:accessRequests.title")}
          <Badge variant="warm">{pending.length}</Badge>
        </CardTitle>
        <CardDescription>{t("dealRooms:accessRequests.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {pending.map((request) => {
          const focused =
            Boolean(focusLinkId) &&
            request.source === "link" &&
            request.linkId === focusLinkId;
          return (
          <div
            key={`${request.source}-${request.id}`}
            className={`flex flex-col gap-3 rounded-lg border bg-background p-3 sm:flex-row sm:items-start sm:justify-between${
              focused ? " ring-2 ring-primary/40" : ""
            }`}
            data-testid={`deal-room-access-request-${request.id}`}
            data-focused={focused ? "true" : undefined}
          >
            <div className="min-h-0 min-w-0 space-y-1">
              <p className="truncate text-sm font-medium">{request.email}</p>
              {request.signerName ? (
                <p className="text-sm text-muted-foreground">
                  {t("dealRooms:accessRequests.signerName", { name: request.signerName })}
                </p>
              ) : null}
              {request.linkName ? (
                <p className="text-sm text-muted-foreground">
                  {t("dealRooms:accessRequests.linkLabel", { name: request.linkName })}
                </p>
              ) : null}
              {request.reason ? (
                <p className="text-sm text-muted-foreground">{request.reason}</p>
              ) : null}
            </div>
            <div className="flex shrink-0 gap-2">
              <Button
                size="sm"
                className="gap-1"
                disabled={activeBusyId === request.id}
                onClick={() => { void handleApprove(request); }}
              >
                <Check size={14} />
                {t("dealRooms:accessRequests.approve")}
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="gap-1"
                disabled={activeBusyId === request.id}
                onClick={() => { void handleReject(request); }}
              >
                <X size={14} />
                {t("dealRooms:accessRequests.reject")}
              </Button>
            </div>
          </div>
          );
        })}
      </CardContent>
    </Card>
  );
}
