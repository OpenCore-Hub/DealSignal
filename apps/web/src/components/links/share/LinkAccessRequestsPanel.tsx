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
import { formatRelativeTime } from "@/lib/formatters";
import type { LinkAccessRequest } from "@/types";

interface LinkAccessRequestsPanelProps {
  linkId: string;
  /** Called after approve/reject. Approve passes the granted email so parents can sync allowlists. */
  onChanged?: (detail?: { email?: string; action: "approve" | "reject" }) => void;
}

export function LinkAccessRequestsPanel({ linkId, onChanged }: LinkAccessRequestsPanelProps) {
  const { t } = useTranslation("linkShare");
  const [busyId, setBusyId] = useState<string | null>(null);
  const {
    data: requests,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    const res = await api.getLinkAccessRequests(linkId);
    return res.data ?? [];
  }, [linkId]);

  const pending = (requests ?? []).filter((r) => r.status === "pending");

  const handleApprove = useCallback(
    async (request: LinkAccessRequest) => {
      setBusyId(request.id);
      try {
        await api.approveLinkAccessRequest(linkId, request.id);
        toast.success(t("accessRequests.approveSuccess"));
        await refetch();
        onChanged?.({ email: request.email, action: "approve" });
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("accessRequests.approveError"));
      } finally {
        setBusyId(null);
      }
    },
    [linkId, onChanged, refetch, t]
  );

  const handleReject = useCallback(
    async (request: LinkAccessRequest) => {
      setBusyId(request.id);
      try {
        await api.rejectLinkAccessRequest(linkId, request.id);
        toast.success(t("accessRequests.rejectSuccess"));
        await refetch();
        onChanged?.({ email: request.email, action: "reject" });
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("accessRequests.rejectError"));
      } finally {
        setBusyId(null);
      }
    },
    [linkId, onChanged, refetch, t]
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
        data-testid="link-access-requests-error"
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
    <Card className="border-amber-500/30 bg-amber-500/5">
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
            key={request.id}
            className="flex flex-col gap-3 rounded-lg border bg-background p-3 sm:flex-row sm:items-start sm:justify-between"
            data-testid={`link-access-request-${request.id}`}
          >
            <div className="min-w-0 space-y-1">
              <p className="truncate text-sm font-medium">
                {request.signer_name ? `${request.signer_name} · ${request.email}` : request.email}
              </p>
              {request.reason ? (
                <p className="text-sm text-muted-foreground">{request.reason}</p>
              ) : null}
              <p className="text-xs text-muted-foreground">
                {formatRelativeTime(request.created_at)}
              </p>
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
