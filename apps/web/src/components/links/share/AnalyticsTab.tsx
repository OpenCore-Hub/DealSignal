import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Eye,
  Users,
  Clock,
  Calendar,
  FileText,
  ChatCenteredDots,
  Target,
  EnvelopeSimple,
} from "@phosphor-icons/react";
import { toast } from "sonner";
import { StatCard } from "@/components/common/StatCard";
import { EmptyState } from "@/components/common/EmptyState";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatDuration, formatRelativeTime } from "@/lib/formatters";
import { calculateUniqueVisitors } from "@/lib/calculations";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { LinkAccessLog } from "../LinkAccessLog";
import { ManagementTab } from "./ManagementTab";
import type { AccessLog, FileRequest, Link, LinkAnalytics, VisitorQuestion } from "@/types";

interface AnalyticsTabProps {
  link: Link;
  logs: AccessLog[];
}

const RECENT_LOG_LIMIT = 20;

function codeSendBadgeVariant(
  status: string,
): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "sent":
      return "default";
    case "failed":
      return "destructive";
    case "pending":
      return "secondary";
    default:
      return "outline";
  }
}

async function fetchManagementData(linkId: string) {
  const [qRes, fRes] = await Promise.all([
    api.listLinkQuestions(linkId),
    api.listLinkFileRequests(linkId),
  ]);
  return { questions: qRes.data ?? [], fileRequests: fRes.data ?? [] };
}

export function AnalyticsTab({ link, logs }: AnalyticsTabProps) {
  const { t } = useTranslation("linkShare");
  const [analytics, setAnalytics] = useState<LinkAnalytics | null>(null);
  const [analyticsLoading, setAnalyticsLoading] = useState(true);
  const [questions, setQuestions] = useState<VisitorQuestion[]>([]);
  const [fileRequests, setFileRequests] = useState<FileRequest[]>([]);
  const [managementLoading, setManagementLoading] = useState(true);
  const [resendingEmail, setResendingEmail] = useState<string | null>(null);
  const [resendingAll, setResendingAll] = useState(false);

  const uniqueVisitors = useMemo(() => calculateUniqueVisitors(logs), [logs]);
  const recentLogs = useMemo(() => {
    return [...logs]
      .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
      .slice(0, RECENT_LOG_LIMIT);
  }, [logs]);

  const accessCodeContacts = analytics?.access_code_contacts ?? [];
  const showAccessCodeSection =
    Boolean(link.requireEmailVerification) || accessCodeContacts.length > 0;
  const remediableCount = accessCodeContacts.filter((c) => c.can_resend).length;

  const refreshAnalytics = async () => {
    const res = await api.getLinkAnalytics(link.id);
    setAnalytics(res.data);
  };

  const handleResendToast = (err: unknown) => {
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
    toast.error(t("analytics.resendFailed"));
  };

  const handleResendOne = async (email: string) => {
    setResendingEmail(email);
    try {
      await api.resendLinkAccessCode(link.id, email);
      toast.success(t("analytics.resendSuccess"));
      await refreshAnalytics();
    } catch (err) {
      handleResendToast(err);
      try {
        await refreshAnalytics();
      } catch {
        /* ignore refresh errors after failure */
      }
    } finally {
      setResendingEmail(null);
    }
  };

  const handleResendAllFailed = async () => {
    setResendingAll(true);
    try {
      const res = await api.resendFailedLinkAccessCodes(link.id);
      const summary = res.data;
      if (summary.attempted === 0) {
        toast.message(t("analytics.resendAllNone"));
      } else if (summary.failed === 0) {
        toast.success(
          t("analytics.resendAllSuccess", {
            sent: summary.sent,
            attempted: summary.attempted,
          }),
        );
      } else {
        toast.error(
          t("analytics.resendAllPartial", {
            sent: summary.sent,
            attempted: summary.attempted,
            failed: summary.failed,
          }),
        );
      }
      await refreshAnalytics();
    } catch (err) {
      handleResendToast(err);
    } finally {
      setResendingAll(false);
    }
  };

  useEffect(() => {
    let cancelled = false;
    api
      .getLinkAnalytics(link.id)
      .then((res) => {
        if (!cancelled) setAnalytics(res.data);
      })
      .catch(() => {
        if (!cancelled) toast.error(t("analytics.loadFailed"));
      })
      .finally(() => {
        if (!cancelled) setAnalyticsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [link.id, t]);

  useEffect(() => {
    let cancelled = false;
    fetchManagementData(link.id)
      .then((data) => {
        if (!cancelled) {
          setQuestions(data.questions);
          setFileRequests(data.fileRequests);
        }
      })
      .catch(() => {
        if (!cancelled) toast.error(t("management.loadFailed"));
      })
      .finally(() => {
        if (!cancelled) setManagementLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [link.id, t]);

  const refreshManagement = async () => {
    setManagementLoading(true);
    try {
      const data = await fetchManagementData(link.id);
      setQuestions(data.questions);
      setFileRequests(data.fileRequests);
    } catch {
      toast.error(t("management.loadFailed"));
    } finally {
      setManagementLoading(false);
    }
  };

  const handleAnswer = async (questionId: string, answer: string) => {
    try {
      await api.answerQuestion(link.id, questionId, answer);
      toast.success(t("management.answerSuccess"));
      await refreshManagement();
    } catch {
      toast.error(t("management.answerFailed"));
      throw new Error("answer failed");
    }
  };

  const handleUpdateFileRequest = async (
    requestId: string,
    status: FileRequest["status"],
  ) => {
    try {
      await api.updateFileRequestStatus(link.id, requestId, status);
      toast.success(t("management.fileRequestUpdateSuccess"));
      await refreshManagement();
    } catch {
      toast.error(t("management.fileRequestUpdateFailed"));
      throw new Error("update failed");
    }
  };

  const stats = [
    {
      label: t("analytics.views"),
      value: analytics?.total_views ?? link.accessCount ?? 0,
      icon: <Eye size={18} />,
    },
    {
      label: t("analytics.uniqueVisitors"),
      value: analytics?.unique_visitors ?? uniqueVisitors,
      icon: <Users size={18} />,
    },
    {
      label: t("analytics.avgDuration"),
      value: formatDuration(
        analytics?.average_duration_seconds ?? link.avgDurationSeconds ?? 0,
      ),
      icon: <Clock size={18} />,
    },
    {
      label: t("analytics.lastVisit"),
      value: analytics?.last_access_at
        ? formatRelativeTime(analytics.last_access_at)
        : link.lastViewedAt
          ? formatRelativeTime(link.lastViewedAt)
          : "—",
      icon: <Calendar size={18} />,
    },
  ];

  return (
    <div className="space-y-6 py-2">
      <div className="grid grid-cols-2 gap-3">
        {stats.map((stat) => (
          <StatCard
            key={stat.label}
            label={stat.label}
            value={stat.value}
            icon={stat.icon}
            size="sm"
          />
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-h3">{t("analytics.recentActivity")}</CardTitle>
        </CardHeader>
        <CardContent>
          {recentLogs.length === 0 ? (
            <EmptyState
              icon={<Eye size={48} />}
              title={t("analytics.emptyTitle")}
              description={t("analytics.emptyDescription")}
            />
          ) : (
            <div className="max-h-[320px] overflow-auto">
              <LinkAccessLog logs={recentLogs} />
            </div>
          )}
        </CardContent>
      </Card>

      {showAccessCodeSection && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between gap-3">
              <CardTitle className="flex items-center gap-2 text-h3">
                <EnvelopeSimple size={18} />
                {t("analytics.accessCodeContacts")}
              </CardTitle>
              {remediableCount > 0 && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={resendingAll || resendingEmail !== null}
                  onClick={() => void handleResendAllFailed()}
                >
                  {resendingAll
                    ? t("analytics.resending")
                    : t("analytics.resendAllFailed")}
                </Button>
              )}
            </div>
          </CardHeader>
          <CardContent>
            {!analytics || analyticsLoading ? (
              <p className="py-4 text-center text-sm text-muted-foreground">
                {t("common:loading")}
              </p>
            ) : accessCodeContacts.length === 0 ? (
              <p className="py-4 text-center text-sm text-muted-foreground">
                {t("analytics.noAccessCodeContacts")}
              </p>
            ) : (
              <div className="space-y-3">
                {accessCodeContacts.map((c) => {
                  const statusKey =
                    c.send_status === "sent" ||
                    c.send_status === "failed" ||
                    c.send_status === "pending"
                      ? c.send_status
                      : "pending";
                  const busy =
                    resendingAll ||
                    resendingEmail === c.email ||
                    resendingEmail !== null;
                  return (
                    <div
                      key={c.email}
                      className="flex items-center justify-between gap-3 rounded-lg border p-3"
                    >
                      <div className="min-w-0 flex-1 space-y-0.5">
                        <p className="truncate text-sm font-medium">
                          {c.name?.trim()
                            ? `${c.name.trim()} · ${c.email}`
                            : c.email}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {c.code_sent_at
                            ? t("analytics.codeSentAt", {
                                time: formatRelativeTime(c.code_sent_at),
                              })
                            : t(`analytics.codeSendStatus.${statusKey}`)}
                          {c.used_at ? ` · ${t("analytics.codeUsed")}` : ""}
                        </p>
                        {c.send_status === "failed" && c.send_error ? (
                          <p className="truncate text-xs text-destructive">
                            {c.send_error}
                          </p>
                        ) : null}
                      </div>
                      <div className="flex shrink-0 items-center gap-2">
                        <Badge variant={codeSendBadgeVariant(c.send_status)}>
                          {t(`analytics.codeSendStatus.${statusKey}`)}
                        </Badge>
                        {c.can_resend ? (
                          <Button
                            type="button"
                            variant="secondary"
                            size="sm"
                            disabled={busy}
                            onClick={() => void handleResendOne(c.email)}
                          >
                            {resendingEmail === c.email
                              ? t("analytics.resending")
                              : t("analytics.resend")}
                          </Button>
                        ) : null}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-h3">
            <Users size={18} />
            {t("analytics.recentVisitors")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {!analytics || analyticsLoading ? (
            <p className="py-4 text-center text-sm text-muted-foreground">
              {t("common:loading")}
            </p>
          ) : analytics.recent_visitors.length === 0 ? (
            <p className="py-4 text-center text-sm text-muted-foreground">
              {t("analytics.noRecentVisitors")}
            </p>
          ) : (
            <div className="space-y-3">
              {analytics.recent_visitors.map((v) => (
                <div
                  key={v.visitor_id}
                  className="flex items-center justify-between rounded-lg border p-3"
                >
                  <div className="min-w-0 flex-1 space-y-0.5">
                    <p className="truncate text-sm font-medium">
                      {v.visitor_email || t("management.anonymous")}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {t("analytics.lastVisit")}:{" "}
                      {formatRelativeTime(v.last_access_at)}
                    </p>
                  </div>
                  <Badge variant="secondary">
                    {t("analytics.visitorViews", { count: v.total_views })}
                  </Badge>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-h3">
            <Target size={18} />
            {t("analytics.keyPages")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {!analytics || analyticsLoading ? (
            <p className="py-4 text-center text-sm text-muted-foreground">
              {t("common:loading")}
            </p>
          ) : analytics.key_pages.length === 0 ? (
            <p className="py-4 text-center text-sm text-muted-foreground">
              {t("analytics.noKeyPages")}
            </p>
          ) : (
            <div className="space-y-3">
              {analytics.key_pages.map((p) => (
                <div
                  key={p.page_number}
                  className="flex items-center justify-between rounded-lg border p-3"
                >
                  <div className="flex items-center gap-2 text-sm">
                    <FileText size={16} className="text-muted-foreground" />
                    {t("analytics.pageLabel", { pageNumber: p.page_number })}
                  </div>
                  <div className="flex items-center gap-3 text-xs text-muted-foreground">
                    <span>
                      {t("analytics.visitorViews", { count: p.views })}
                    </span>
                    <span>·</span>
                    <span>{formatDuration(p.average_duration_seconds)}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-h3">
            <ChatCenteredDots size={18} />
            {t("analytics.qaRecords")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {!analytics || analyticsLoading ? (
            <p className="py-4 text-center text-sm text-muted-foreground">
              {t("common:loading")}
            </p>
          ) : analytics.qa_records.length === 0 ? (
            <p className="py-4 text-center text-sm text-muted-foreground">
              {t("analytics.noQARecords")}
            </p>
          ) : (
            <div className="space-y-3">
              {analytics.qa_records.map((q, idx) => (
                <div key={idx} className="rounded-lg border p-3">
                  <p className="text-sm font-medium">
                    {q.visitor_email || t("management.anonymous")}
                  </p>
                  <p className="text-sm text-muted-foreground">{q.question}</p>
                  {q.answer && (
                    <p className="mt-2 text-sm">
                      <span className="font-medium">
                        {t("management.answerLabel")}
                      </span>{" "}
                      {q.answer}
                    </p>
                  )}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <ManagementTab
        questions={questions}
        fileRequests={fileRequests}
        onAnswer={handleAnswer}
        onUpdateFileRequest={handleUpdateFileRequest}
      />
      {managementLoading && (
        <div className="sr-only" role="status">
          {t("management.loading")}
        </div>
      )}
    </div>
  );
}
