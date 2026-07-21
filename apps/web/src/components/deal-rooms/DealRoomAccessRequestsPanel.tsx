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
  onChanged?: () => void;
}

export function DealRoomAccessRequestsPanel({ roomId, onChanged }: DealRoomAccessRequestsPanelProps) {
  const { t } = useTranslation("dealRooms");
  const [busyId, setBusyId] = useState<string | null>(null);
  const {
    data: requests,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    const [roomRes, linksRes] = await Promise.all([
      api.getDealRoomAccessRequests(roomId),
      api.getDealRoomLinks(roomId),
    ]);
    const roomPending: PendingAccessRequest[] = (roomRes.data ?? [])
      .filter((r) => r.status === "pending")
      .map((r) => ({
        id: r.id,
        email: r.email,
        reason: r.reason,
        source: "room" as const,
      }));

    const links = linksRes.data ?? [];
    const linkEntries = await Promise.all(
      links.map(async (link) => {
        const res = await api.getLinkAccessRequests(link.id);
        return (res.data ?? [])
          .filter((r) => r.status === "pending")
          .map(
            (r): PendingAccessRequest => ({
              id: r.id,
              email: r.email,
              reason: r.reason,
              signerName: r.signer_name,
              linkId: link.id,
              linkName: link.name || undefined,
              source: "link",
            })
          );
      })
    );

    return [...roomPending, ...linkEntries.flat()];
  }, [roomId]);

  const pending = requests ?? [];

  const handleApprove = useCallback(
    async (request: PendingAccessRequest) => {
      setBusyId(request.id);
      try {
        if (request.source === "link" && request.linkId) {
          await api.approveLinkAccessRequest(request.linkId, request.id);
        } else {
          await api.approveDealRoomAccessRequest(roomId, request.id);
        }
        toast.success(t("accessRequests.approveSuccess"));
        await refetch();
        onChanged?.();
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("accessRequests.approveError"));
      } finally {
        setBusyId(null);
      }
    },
    [roomId, onChanged, refetch, t]
  );

  const handleReject = useCallback(
    async (request: PendingAccessRequest) => {
      setBusyId(request.id);
      try {
        if (request.source === "link" && request.linkId) {
          await api.rejectLinkAccessRequest(request.linkId, request.id);
        } else {
          await api.rejectDealRoomAccessRequest(roomId, request.id);
        }
        toast.success(t("accessRequests.rejectSuccess"));
        await refetch();
        onChanged?.();
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("accessRequests.rejectError"));
      } finally {
        setBusyId(null);
      }
    },
    [roomId, onChanged, refetch, t]
  );

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
        <p className="text-destructive">{t("accessRequests.loadFailed")}</p>
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
          {t("accessRequests.title")}
          <Badge variant="warm">{pending.length}</Badge>
        </CardTitle>
        <CardDescription>{t("accessRequests.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {pending.map((request) => (
          <div
            key={`${request.source}-${request.id}`}
            className="flex flex-col gap-3 rounded-lg border bg-background p-3 sm:flex-row sm:items-start sm:justify-between"
            data-testid={`deal-room-access-request-${request.id}`}
          >
            <div className="min-h-0 min-w-0 space-y-1">
              <p className="truncate text-sm font-medium">{request.email}</p>
              {request.signerName ? (
                <p className="text-sm text-muted-foreground">
                  {t("accessRequests.signerName", { name: request.signerName })}
                </p>
              ) : null}
              {request.linkName ? (
                <p className="text-sm text-muted-foreground">
                  {t("accessRequests.linkLabel", { name: request.linkName })}
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
                disabled={busyId === request.id}
                onClick={() => { void handleApprove(request); }}
              >
                <Check size={14} />
                {t("accessRequests.approve")}
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="gap-1"
                disabled={busyId === request.id}
                onClick={() => { void handleReject(request); }}
              >
                <X size={14} />
                {t("accessRequests.reject")}
              </Button>
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
