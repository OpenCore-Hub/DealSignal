import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Eye,
  Users,
  Clock,
  Calendar,
  ChatCenteredDots,
  EnvelopeSimple,
} from "@phosphor-icons/react";
import { toast } from "sonner";
import { StatCard } from "@/components/common/StatCard";
import { EmptyState } from "@/components/common/EmptyState";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { formatDuration, formatRelativeTime } from "@/lib/formatters";
import { calculateUniqueVisitors } from "@/lib/calculations";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { LinkAccessLog } from "../LinkAccessLog";
import { ManagementTab } from "./ManagementTab";
import type {
  AccessLog,
  FileRequest,
  Link,
  LinkAccessCodeContact,
  LinkAnalytics,
  LinkRecentVisitor,
  VisitorQuestion,
} from "@/types";

type ActivitySection = "visitors" | "activity" | "delivery" | "engage";

interface AnalyticsTabProps {
  link: Link;
  logs: AccessLog[];
}

const RECENT_VISITORS_PAGE_SIZE = 10;
const ACCESS_LOG_PAGE_SIZE = 10;
const ACCESS_CODE_CONTACTS_PAGE_SIZE = 10;

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
  const [section, setSection] = useState<ActivitySection>("visitors");
  const prioritizedSection = useRef(false);
  const [visitors, setVisitors] = useState<LinkRecentVisitor[]>([]);
  const [visitorsHasMore, setVisitorsHasMore] = useState(false);
  const [loadingMoreVisitors, setLoadingMoreVisitors] = useState(false);
  const visitorsSeededForLinkRef = useRef<string | null>(null);
  const visitorsNextOffsetRef = useRef(0);
  const visitorsLoadLock = useRef(false);
  const visitorsScrollRef = useRef<HTMLDivElement | null>(null);
  const visitorsSentinelRef = useRef<HTMLDivElement | null>(null);

  const [activityLogs, setActivityLogs] = useState<AccessLog[]>([]);
  const [activityHasMore, setActivityHasMore] = useState(false);
  const [activityLoading, setActivityLoading] = useState(true);
  const [loadingMoreActivity, setLoadingMoreActivity] = useState(false);
  const activityNextOffsetRef = useRef(0);
  const activityLoadLock = useRef(false);
  const activityScrollRef = useRef<HTMLDivElement | null>(null);
  const activitySentinelRef = useRef<HTMLDivElement | null>(null);

  const [deliveryContacts, setDeliveryContacts] = useState<LinkAccessCodeContact[]>([]);
  const [deliveryHasMore, setDeliveryHasMore] = useState(false);
  const [loadingMoreDelivery, setLoadingMoreDelivery] = useState(false);
  const deliverySeededForLinkRef = useRef<string | null>(null);
  const deliveryNextOffsetRef = useRef(0);
  const deliveryLoadLock = useRef(false);
  const deliveryScrollRef = useRef<HTMLDivElement | null>(null);
  const deliverySentinelRef = useRef<HTMLDivElement | null>(null);

  const uniqueVisitors = useMemo(() => calculateUniqueVisitors(logs), [logs]);

  const showDeliveryTab =
    Boolean(link.requireEmailVerification) ||
    deliveryContacts.length > 0 ||
    deliveryHasMore;
  const remediableCount =
    analytics?.access_code_remediable_count ??
    deliveryContacts.filter((c) => c.can_resend).length;
  const pendingEngageCount = useMemo(() => {
    const pendingQuestions = questions.filter((q) => q.status === "pending").length;
    const pendingFiles = fileRequests.filter((r) => r.status === "pending").length;
    return pendingQuestions + pendingFiles;
  }, [questions, fileRequests]);

  useEffect(() => {
    if (analyticsLoading || managementLoading || prioritizedSection.current) return;
    prioritizedSection.current = true;
    if (remediableCount > 0 && showDeliveryTab) {
      setSection("delivery");
      return;
    }
    if (pendingEngageCount > 0) {
      setSection("engage");
    }
  }, [
    analyticsLoading,
    managementLoading,
    remediableCount,
    pendingEngageCount,
    showDeliveryTab,
  ]);

  useEffect(() => {
    visitorsSeededForLinkRef.current = null;
    setVisitors([]);
    setVisitorsHasMore(false);
    visitorsNextOffsetRef.current = 0;
    deliverySeededForLinkRef.current = null;
    setDeliveryContacts([]);
    setDeliveryHasMore(false);
    deliveryNextOffsetRef.current = 0;
  }, [link.id]);

  useEffect(() => {
    if (!analytics) return;
    // Seed visitors only once per link so refreshAnalytics (e.g. resend)
    // does not wipe pages already loaded by infinite scroll.
    if (visitorsSeededForLinkRef.current === link.id) return;
    visitorsSeededForLinkRef.current = link.id;
    setVisitors(analytics.recent_visitors);
    setVisitorsHasMore(Boolean(analytics.recent_visitors_has_more));
    visitorsNextOffsetRef.current = analytics.recent_visitors.length;
  }, [analytics, link.id]);

  useEffect(() => {
    if (!analytics) return;
    if (deliverySeededForLinkRef.current === link.id) return;
    deliverySeededForLinkRef.current = link.id;
    const firstPage = analytics.access_code_contacts ?? [];
    setDeliveryContacts(firstPage);
    setDeliveryHasMore(Boolean(analytics.access_code_contacts_has_more));
    deliveryNextOffsetRef.current = firstPage.length;
  }, [analytics, link.id]);

  const loadMoreVisitors = async () => {
    if (
      visitorsLoadLock.current ||
      loadingMoreVisitors ||
      !visitorsHasMore ||
      analyticsLoading
    ) {
      return;
    }
    visitorsLoadLock.current = true;
    setLoadingMoreVisitors(true);
    const offset = visitorsNextOffsetRef.current;
    const linkId = link.id;
    try {
      const res = await api.listLinkRecentVisitors(linkId, {
        limit: RECENT_VISITORS_PAGE_SIZE,
        offset,
      });
      if (linkId !== link.id) return;
      const page = res.data ?? [];
      visitorsNextOffsetRef.current = offset + page.length;
      setVisitors((prev) => {
        const seen = new Set(prev.map((v) => v.visitor_id));
        const next = [...prev];
        for (const item of page) {
          if (!seen.has(item.visitor_id)) {
            seen.add(item.visitor_id);
            next.push(item);
          }
        }
        return next;
      });
      setVisitorsHasMore(Boolean(res.has_more));
    } catch {
      toast.error(t("analytics.loadMoreVisitorsFailed"));
    } finally {
      setLoadingMoreVisitors(false);
      visitorsLoadLock.current = false;
    }
  };

  useEffect(() => {
    if (section !== "visitors" || !visitorsHasMore) return;
    const root = visitorsScrollRef.current;
    const node = visitorsSentinelRef.current;
    if (!root || !node) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          void loadMoreVisitors();
        }
      },
      { root, rootMargin: "40px", threshold: 0 },
    );
    observer.observe(node);
    return () => observer.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- sentinel rebind; loadMore uses refs
  }, [section, visitorsHasMore, loadingMoreVisitors, link.id, visitors.length]);

  useEffect(() => {
    let cancelled = false;
    setActivityLoading(true);
    activityNextOffsetRef.current = 0;
    api
      .getAccessLogs(link.id, { limit: ACCESS_LOG_PAGE_SIZE, offset: 0 })
      .then((res) => {
        if (cancelled) return;
        setActivityLogs(res.data ?? []);
        activityNextOffsetRef.current = (res.data ?? []).length;
        setActivityHasMore(Boolean(res.has_more));
      })
      .catch(() => {
        if (!cancelled) {
          setActivityLogs([]);
          setActivityHasMore(false);
          toast.error(t("analytics.loadFailed"));
        }
      })
      .finally(() => {
        if (!cancelled) setActivityLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [link.id, t]);

  const loadMoreActivity = async () => {
    if (
      activityLoadLock.current ||
      loadingMoreActivity ||
      !activityHasMore ||
      activityLoading
    ) {
      return;
    }
    activityLoadLock.current = true;
    setLoadingMoreActivity(true);
    const offset = activityNextOffsetRef.current;
    const linkId = link.id;
    try {
      const res = await api.getAccessLogs(linkId, {
        limit: ACCESS_LOG_PAGE_SIZE,
        offset,
      });
      if (linkId !== link.id) return;
      const page = res.data ?? [];
      activityNextOffsetRef.current = offset + page.length;
      setActivityLogs((prev) => {
        const seen = new Set(prev.map((log) => log.id));
        const next = [...prev];
        for (const item of page) {
          if (!seen.has(item.id)) {
            seen.add(item.id);
            next.push(item);
          }
        }
        return next;
      });
      setActivityHasMore(Boolean(res.has_more));
    } catch {
      toast.error(t("analytics.loadMoreActivityFailed"));
    } finally {
      setLoadingMoreActivity(false);
      activityLoadLock.current = false;
    }
  };

  useEffect(() => {
    if (section !== "activity" || !activityHasMore) return;
    const root = activityScrollRef.current;
    const node = activitySentinelRef.current;
    if (!root || !node) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          void loadMoreActivity();
        }
      },
      { root, rootMargin: "40px", threshold: 0 },
    );
    observer.observe(node);
    return () => observer.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- sentinel rebind; loadMore uses refs
  }, [section, activityHasMore, loadingMoreActivity, link.id, activityLogs.length]);

  const reloadDeliveryContacts = async () => {
    const limit = Math.max(deliveryNextOffsetRef.current, ACCESS_CODE_CONTACTS_PAGE_SIZE);
    const res = await api.listLinkAccessCodeContacts(link.id, {
      limit,
      offset: 0,
    });
    const page = res.data ?? [];
    setDeliveryContacts(page);
    deliveryNextOffsetRef.current = page.length;
    setDeliveryHasMore(Boolean(res.has_more));
  };

  const loadMoreDelivery = async () => {
    if (
      deliveryLoadLock.current ||
      loadingMoreDelivery ||
      !deliveryHasMore ||
      analyticsLoading
    ) {
      return;
    }
    deliveryLoadLock.current = true;
    setLoadingMoreDelivery(true);
    const offset = deliveryNextOffsetRef.current;
    const linkId = link.id;
    try {
      const res = await api.listLinkAccessCodeContacts(linkId, {
        limit: ACCESS_CODE_CONTACTS_PAGE_SIZE,
        offset,
      });
      if (linkId !== link.id) return;
      const page = res.data ?? [];
      deliveryNextOffsetRef.current = offset + page.length;
      setDeliveryContacts((prev) => {
        const seen = new Set(prev.map((c) => c.email));
        const next = [...prev];
        for (const item of page) {
          if (!seen.has(item.email)) {
            seen.add(item.email);
            next.push(item);
          }
        }
        return next;
      });
      setDeliveryHasMore(Boolean(res.has_more));
    } catch {
      toast.error(t("analytics.loadMoreDeliveryFailed"));
    } finally {
      setLoadingMoreDelivery(false);
      deliveryLoadLock.current = false;
    }
  };

  useEffect(() => {
    if (section !== "delivery" || !deliveryHasMore) return;
    const root = deliveryScrollRef.current;
    const node = deliverySentinelRef.current;
    if (!root || !node) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          void loadMoreDelivery();
        }
      },
      { root, rootMargin: "40px", threshold: 0 },
    );
    observer.observe(node);
    return () => observer.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- sentinel rebind; loadMore uses refs
  }, [section, deliveryHasMore, loadingMoreDelivery, link.id, deliveryContacts.length]);

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
      await reloadDeliveryContacts();
    } catch (err) {
      handleResendToast(err);
      try {
        await refreshAnalytics();
        await reloadDeliveryContacts();
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
      await reloadDeliveryContacts();
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
    <div className="space-y-4 py-2">
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

      <Tabs
        value={section}
        onValueChange={(value) => setSection(value as ActivitySection)}
        className="gap-3"
      >
        <TabsList
          variant="line"
          className="w-full justify-start overflow-x-auto"
          aria-label={t("analytics.tabs.ariaLabel")}
        >
          <TabsTrigger value="visitors">{t("analytics.tabs.visitors")}</TabsTrigger>
          <TabsTrigger value="activity">{t("analytics.tabs.activity")}</TabsTrigger>
          {showDeliveryTab ? (
            <TabsTrigger value="delivery" className="gap-1.5">
              {t("analytics.tabs.delivery")}
              {remediableCount > 0 ? (
                <Badge variant="destructive" className="h-5 min-w-5 px-1.5 text-[10px]">
                  {remediableCount}
                </Badge>
              ) : null}
            </TabsTrigger>
          ) : null}
          <TabsTrigger value="engage" className="gap-1.5">
            {t("analytics.tabs.engage")}
            {pendingEngageCount > 0 ? (
              <Badge variant="secondary" className="h-5 min-w-5 px-1.5 text-[10px]">
                {pendingEngageCount}
              </Badge>
            ) : null}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="visitors" className="space-y-3">
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
              ) : visitors.length === 0 ? (
                <p className="py-4 text-center text-sm text-muted-foreground">
                  {t("analytics.noRecentVisitors")}
                </p>
              ) : (
                <div
                  ref={visitorsScrollRef}
                  className="max-h-[320px] space-y-3 overflow-y-auto pr-1"
                  aria-busy={loadingMoreVisitors}
                >
                  {visitors.map((v) => (
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
                  {visitorsHasMore ? (
                    <div
                      ref={visitorsSentinelRef}
                      className="flex h-8 items-center justify-center"
                      aria-hidden={!loadingMoreVisitors}
                    >
                      {loadingMoreVisitors ? (
                        <p className="text-xs text-muted-foreground">
                          {t("analytics.loadingMoreVisitors")}
                        </p>
                      ) : null}
                    </div>
                  ) : null}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="activity">
          <Card>
            <CardHeader>
              <CardTitle className="text-h3">{t("analytics.recentActivity")}</CardTitle>
            </CardHeader>
            <CardContent>
              {activityLoading ? (
                <p className="py-4 text-center text-sm text-muted-foreground">
                  {t("common:loading")}
                </p>
              ) : activityLogs.length === 0 ? (
                <EmptyState
                  icon={<Eye size={48} />}
                  title={t("analytics.emptyTitle")}
                  description={t("analytics.emptyDescription")}
                />
              ) : (
                <div
                  ref={activityScrollRef}
                  className="max-h-[320px] space-y-3 overflow-y-auto pr-1"
                  aria-busy={loadingMoreActivity}
                >
                  <LinkAccessLog logs={activityLogs} />
                  {activityHasMore ? (
                    <div
                      ref={activitySentinelRef}
                      className="flex h-8 items-center justify-center"
                      aria-hidden={!loadingMoreActivity}
                    >
                      {loadingMoreActivity ? (
                        <p className="text-xs text-muted-foreground">
                          {t("analytics.loadingMoreActivity")}
                        </p>
                      ) : null}
                    </div>
                  ) : null}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {showDeliveryTab ? (
          <TabsContent value="delivery">
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
                ) : deliveryContacts.length === 0 ? (
                  <p className="py-4 text-center text-sm text-muted-foreground">
                    {t("analytics.noAccessCodeContacts")}
                  </p>
                ) : (
                  <div
                    ref={deliveryScrollRef}
                    className="max-h-[320px] space-y-3 overflow-y-auto pr-1"
                    aria-busy={loadingMoreDelivery}
                  >
                    {deliveryContacts.map((c) => {
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
                    {deliveryHasMore ? (
                      <div
                        ref={deliverySentinelRef}
                        className="flex h-8 items-center justify-center"
                        aria-hidden={!loadingMoreDelivery}
                      >
                        {loadingMoreDelivery ? (
                          <p className="text-xs text-muted-foreground">
                            {t("analytics.loadingMoreDelivery")}
                          </p>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        ) : null}

        <TabsContent value="engage" className="space-y-4">
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
        </TabsContent>
      </Tabs>
    </div>
  );
}
