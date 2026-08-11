import { useEffect, useMemo, useRef } from "react";
import { useTranslation } from "react-i18next";
import { Check, X, UserPlus } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { formatRelativeTime } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import type { LinkAccessRequest } from "@/types";

export type AccessRequestInboxItem = LinkAccessRequest & {
  documentLabel?: string;
  /** True when applicant email matches a workspace member (not shown on Deal Radar). */
  is_workspace_member?: boolean;
};

interface AccessRequestsInboxProps {
  title: string;
  description: string;
  requests: AccessRequestInboxItem[];
  busyId: string | null;
  /** Highlight / scroll to the first pending request for this link (dashboard deep-link). */
  focusLinkId?: string;
  testId?: string;
  itemTestIdPrefix: string;
  onApprove: (request: AccessRequestInboxItem) => void;
  onReject: (request: AccessRequestInboxItem) => void;
}

export function AccessRequestsInbox({
  title,
  description,
  requests,
  busyId,
  focusLinkId,
  testId,
  itemTestIdPrefix,
  onApprove,
  onReject,
}: AccessRequestsInboxProps) {
  const { t } = useTranslation("linkShare");
  const focusRef = useRef<HTMLDivElement | null>(null);
  const focusRequestId = useMemo(() => {
    if (!focusLinkId) return null;
    return requests.find((r) => r.link_id === focusLinkId)?.id ?? null;
  }, [focusLinkId, requests]);

  const memberCount = useMemo(
    () => requests.filter((r) => r.is_workspace_member).length,
    [requests],
  );

  const radarHonestyHint = useMemo(() => {
    if (memberCount === 0) return null;
    if (memberCount === requests.length) {
      return t("accessRequests.internalOnlyHint");
    }
    return t("accessRequests.mixedInternalHint", { count: memberCount });
  }, [memberCount, requests.length, t]);

  useEffect(() => {
    const el = focusRef.current;
    if (!focusRequestId || !el || typeof el.scrollIntoView !== "function") return;
    el.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, [focusRequestId, requests]);

  return (
    <Card className="border-amber-500/30 bg-amber-500/5" data-testid={testId}>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-h3">
          <UserPlus size={20} />
          {title}
          <Badge variant="warm">{requests.length}</Badge>
        </CardTitle>
        <CardDescription>{description}</CardDescription>
        {radarHonestyHint ? (
          <p
            className="pt-1 text-sm text-muted-foreground"
            data-testid="access-requests-radar-honesty-hint"
          >
            {radarHonestyHint}
          </p>
        ) : null}
      </CardHeader>
      <CardContent className="space-y-3">
        {requests.map((request) => {
          const isFocused = request.id === focusRequestId;
          const isMember = Boolean(request.is_workspace_member);
          return (
            <div
              key={request.id}
              ref={isFocused ? focusRef : undefined}
              className={cn(
                "flex flex-col gap-3 rounded-lg border bg-background p-3 sm:flex-row sm:items-start sm:justify-between",
                isFocused && "ring-2 ring-primary/40",
              )}
              data-testid={`${itemTestIdPrefix}-${request.id}`}
              data-link-id={request.link_id}
              data-focused={isFocused ? "true" : undefined}
              data-workspace-member={isMember ? "true" : undefined}
            >
              <div className="min-w-0 space-y-1">
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  <p className="truncate text-sm font-medium">
                    {request.signer_name
                      ? t("accessRequests.applicantWithName", {
                          name: request.signer_name,
                          email: request.email,
                        })
                      : request.email}
                  </p>
                  {isMember ? (
                    <Badge
                      variant="secondary"
                      className="shrink-0 font-normal"
                      data-testid={`${itemTestIdPrefix}-${request.id}-member-badge`}
                    >
                      {t("accessRequests.workspaceMemberBadge")}
                    </Badge>
                  ) : null}
                </div>
                {isMember ? (
                  <p className="text-xs text-muted-foreground">
                    {t("accessRequests.notOnRadar")}
                  </p>
                ) : null}
                {request.documentLabel ? (
                  <p className="truncate text-sm text-muted-foreground" title={request.documentLabel}>
                    {request.documentLabel}
                  </p>
                ) : null}
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
                  onClick={() => onApprove(request)}
                >
                  <Check size={14} />
                  {t("accessRequests.approve")}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="gap-1"
                  disabled={busyId === request.id}
                  onClick={() => onReject(request)}
                >
                  <X size={14} />
                  {t("accessRequests.reject")}
                </Button>
              </div>
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}
