import { useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { Lightning, Envelope, Lightbulb, X, ClockCounterClockwise, ChatTeardropText } from "@phosphor-icons/react";
import { toast } from "sonner";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { HeatBadge } from "@/components/common/HeatBadge";
import { EmptyState } from "@/components/common/EmptyState";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import { formalAskSuggestionPath } from "@/lib/actionNavigation";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useTranslation } from "react-i18next";
import type { Suggestion } from "@/types";

type SnoozeHours = 24 | 72 | 168;

function buildFollowUpMailto(s: Suggestion, subject: string, body: string): string {
  const params = new URLSearchParams();
  params.set("subject", subject);
  params.set("body", body);
  return `mailto:${s.contactEmail}?${params.toString()}`;
}

export function InsightsSuggestionsPage() {
  const { t } = useTranslation("insights");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const location = useLocation();
  const [busyId, setBusyId] = useState<string | null>(null);
  const [hiddenIds, setHiddenIds] = useState<Set<string>>(() => new Set());

  const {
    data: suggestions,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    const res = await api.getSuggestions();
    return res.data;
  }, []);

  const visible = (suggestions ?? []).filter((s) => !hiddenIds.has(s.id));

  const hideLocally = (id: string) => {
    setHiddenIds((prev) => new Set(prev).add(id));
  };

  const handleDismiss = async (s: Suggestion) => {
    if (!s.linkId || busyId) return;
    setBusyId(s.id);
    try {
      await api.dismissSuggestion(s.linkId, s.id);
      hideLocally(s.id);
      toast.success(t("suggestions.dismissed"));
    } catch (e) {
      toast.error(apiErrorMessage(e));
    } finally {
      setBusyId(null);
    }
  };

  const handleSnooze = async (s: Suggestion, hours: SnoozeHours) => {
    if (busyId) return;
    setBusyId(s.id);
    try {
      await api.snoozeSuggestion(s.id, hours);
      hideLocally(s.id);
      toast.success(t("suggestions.snoozed", { hours }));
    } catch (e) {
      toast.error(apiErrorMessage(e));
    } finally {
      setBusyId(null);
    }
  };

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>{tc("retry")}</Button>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-32" />
        <Skeleton className="h-32" />
        <Skeleton className="h-32" />
      </div>
    );
  }

  if (visible.length === 0) {
    return (
      <EmptyState
        icon={<Lightbulb size={48} />}
        title={t("suggestions.emptyTitle")}
        description={t("suggestions.emptyDescription")}
        size="large"
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="grid gap-4">
        {visible.map((s) => {
          const mailto = buildFollowUpMailto(
            s,
            t("suggestions.emailSubject", { document: s.documentTitle }),
            t("suggestions.emailBody", {
              email: s.contactEmail,
              document: s.documentTitle,
              action: s.action,
            }),
          );
          const busy = busyId === s.id;
          return (
            <Card key={s.id} className="transition-colors hover:bg-muted/50">
              <CardContent className="p-5">
                <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                  <div className="space-y-2">
                    <div className="flex items-center gap-2">
                      <Lightning size={18} className="text-warning-500" />
                      <h3 className="text-h3">{s.action}</h3>
                    </div>
                    <p className="text-body text-muted-foreground">{s.reason}</p>
                    <div className="flex flex-wrap items-center gap-3 text-caption text-muted-foreground">
                      <span>{s.contactEmail}</span>
                      <span>·</span>
                      <span>{s.documentTitle}</span>
                      <span>·</span>
                      <HeatBadge level={s.heatLevel} />
                      <span className="tabular-nums">
                        {t("suggestions.score", { score: s.score })}
                      </span>
                    </div>
                  </div>
                  <div className="flex shrink-0 flex-wrap gap-2">
                    {s.kind === "formal_ask" && s.linkId && workspaceSlug ? (
                      <Button
                        className="gap-1.5"
                        onClick={() => {
                          const path = formalAskSuggestionPath(workspaceSlug, s);
                          if (!path) return;
                          navigate(path, {
                            state: {
                              returnTo: location.pathname + location.search,
                              returnLabel: tc("back"),
                            },
                          });
                        }}
                      >
                        <ChatTeardropText size={16} />
                        {t("suggestions.openFormalAsk")}
                      </Button>
                    ) : null}
                    <Button
                      variant="outline"
                      onClick={() =>
                        navigate(`/${workspaceSlug}/contacts/${s.contactId}`, {
                          state: {
                            returnTo: location.pathname + location.search,
                            returnLabel: tc("back"),
                          },
                        })
                      }
                    >
                      {t("suggestions.viewContact")}
                    </Button>
                    {s.kind !== "formal_ask" ? (
                      <Button
                        className="gap-1.5"
                        onClick={() => {
                          window.open(mailto, "_blank", "noopener,noreferrer");
                        }}
                      >
                        <Envelope size={16} />
                        {t("suggestions.writeEmail")}
                      </Button>
                    ) : null}
                    <DropdownMenu>
                      <DropdownMenuTrigger
                        render={(props) => (
                          <Button
                            variant="outline"
                            className="gap-1.5"
                            disabled={busy}
                            {...props}
                          >
                            <ClockCounterClockwise size={16} />
                            {t("suggestions.snooze")}
                          </Button>
                        )}
                      />
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem disabled={busy} onClick={() => void handleSnooze(s, 24)}>
                          {t("suggestions.snooze1d")}
                        </DropdownMenuItem>
                        <DropdownMenuItem disabled={busy} onClick={() => void handleSnooze(s, 72)}>
                          {t("suggestions.snooze3d")}
                        </DropdownMenuItem>
                        <DropdownMenuItem disabled={busy} onClick={() => void handleSnooze(s, 168)}>
                          {t("suggestions.snooze7d")}
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                    <Button
                      variant="ghost"
                      className="gap-1.5"
                      disabled={busy}
                      onClick={() => void handleDismiss(s)}
                    >
                      <X size={16} />
                      {t("suggestions.dismiss")}
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          );
        })}
      </div>
    </div>
  );
}
