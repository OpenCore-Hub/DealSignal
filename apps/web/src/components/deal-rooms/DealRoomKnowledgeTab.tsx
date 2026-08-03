import { useEffect, useState } from "react";
import { Books, MagnifyingGlass, ArrowsClockwise } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { formatRelativeTime } from "@/lib/formatters";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useKnowledgeQueryStore } from "@/stores/knowledgeQueryStore";
import type {
  DealRoomKnowledgeQueryHit,
  DealRoomKnowledgeQueryResult,
} from "@/types";
import { cn } from "@/lib/utils";

interface DealRoomKnowledgeTabProps {
  roomId: string;
}

function isKnowledgeBusy(status?: string, jobStatus?: string) {
  if (status === "syncing" || status === "provisioning") return true;
  return jobStatus === "pending" || jobStatus === "running";
}

/** docling-rag grounded-answer refusals — retrieval may still return low-score hits. */
export function isUngroundedKnowledgeAnswer(answer?: string | null): boolean {
  const text = (answer ?? "").trim().toLowerCase();
  if (!text) return false;
  const needles = [
    "does not contain an answer",
    "do not contain an answer",
    "no relevant information",
    "cannot answer based on the",
    "can't answer based on the",
    "未找到相关",
    "没有匹配",
    "无法从提供的",
    "上下文中没有",
    "资料中没有",
  ];
  return needles.some((n) => text.includes(n));
}

/** Format page numbers without implying missing pages exist in the span. */
export function formatPagesLabel(pages: number[]): string {
  const sorted = [...new Set(pages.filter((p) => p > 0))].sort((a, b) => a - b);
  if (sorted.length === 0) return "";
  // Page numbers are viewer/preview pages (native PDF or OnlyOffice preview PDF).
  if (sorted.length === 1) return `第${sorted[0]}页`;
  const lo = sorted[0];
  const hi = sorted[sorted.length - 1];
  const contiguous = hi - lo + 1 === sorted.length;
  if (contiguous) return `第${lo}–${hi}页`;
  return `第${sorted.join("、")}页`;
}

/** Human-readable citation locus: file · pages|sheet. Never invents missing pages. */
export function formatHitLocusLabel(
  hit: DealRoomKnowledgeQueryHit,
  opts?: { sheetPrefix?: string },
): string | null {
  const parts: string[] = [];
  if (hit.sourceName) parts.push(hit.sourceName);
  if (hit.pages && hit.pages.length > 0) {
    const pagesLabel = formatPagesLabel(hit.pages);
    if (pagesLabel) parts.push(pagesLabel);
  } else if (hit.sheet) {
    const prefix = opts?.sheetPrefix?.trim() || "Sheet";
    parts.push(`${prefix} ${hit.sheet}`);
  }
  return parts.length ? parts.join(" · ") : null;
}

/** Same-tab viewer path (keeps in-memory workspace slug for authenticated APIs). */
export function viewerPath(documentId: string, page?: number): string {
  const qs = page && page > 0 ? `?page=${page}` : "";
  return `/viewer/${documentId}${qs}`;
}

/** Split answer text so `[n]` citations become clickable markers. */
export function renderAnswerWithCitations(
  answer: string,
  onCite: (n: number) => void,
) {
  const parts = answer.split(/(\[\d+\])/g);
  return parts.map((part, i) => {
    const m = /^\[(\d+)\]$/.exec(part);
    if (!m) return <span key={i}>{part}</span>;
    const n = Number(m[1]);
    return (
      <button
        key={i}
        type="button"
        className="mx-0.5 rounded bg-primary/10 px-1 font-medium text-primary hover:bg-primary/20"
        data-testid={`knowledge-cite-${n}`}
        onClick={() => onCite(n)}
      >
        [{n}]
      </button>
    );
  });
}

export function DealRoomKnowledgeTab({ roomId }: DealRoomKnowledgeTabProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const openViewer = (documentId: string, page?: number) => {
    navigate(viewerPath(documentId, page));
  };
  const { data, loading, error, refetch } = useAsyncData(
    () => api.getDealRoomKnowledge(roomId),
    [roomId],
  );
  const [syncing, setSyncing] = useState(false);
  const [asking, setAsking] = useState(false);
  // Keep Q&A in a room-scoped store so viewer → browser Back remount restores it.
  const query = useKnowledgeQueryStore((s) => s.byRoom[roomId]?.query ?? "");
  const result = useKnowledgeQueryStore((s) => s.byRoom[roomId]?.result ?? null);
  const activeCite = useKnowledgeQueryStore(
    (s) => s.byRoom[roomId]?.activeCite ?? null,
  );
  const setDraft = useKnowledgeQueryStore((s) => s.setDraft);
  const setQuery = (value: string) => setDraft(roomId, { query: value });
  const setResult = (value: DealRoomKnowledgeQueryResult | null) =>
    setDraft(roomId, { result: value });
  const setActiveCite = (value: number | null) =>
    setDraft(roomId, { activeCite: value });

  useEffect(() => {
    if (!isKnowledgeBusy(data?.status, data?.progress?.jobStatus)) return;
    const timer = window.setInterval(() => {
      void refetch();
    }, 2500);
    return () => window.clearInterval(timer);
  }, [data?.status, data?.progress?.jobStatus, refetch]);

  const onSync = async () => {
    setSyncing(true);
    try {
      await api.syncDealRoomKnowledge(roomId);
      toast.success(t("knowledge.syncQueued"));
      await refetch();
    } catch (e) {
      if (e instanceof ApiError && e.status === 503) {
        toast.error(t("knowledge.unavailable"));
      } else {
        toast.error(t("knowledge.syncFailed"));
      }
    } finally {
      setSyncing(false);
    }
  };

  const onQuery = async () => {
    const q = query.trim();
    if (!q) return;
    setAsking(true);
    setActiveCite(null);
    try {
      const res = await api.queryDealRoomKnowledge(roomId, {
        query: q,
        answer: true,
        top_k: 8,
      });
      setResult(res);
    } catch (e) {
      if (e instanceof ApiError && e.status === 503) {
        toast.error(t("knowledge.unavailable"));
      } else {
        toast.error(t("knowledge.queryFailed"));
      }
    } finally {
      setAsking(false);
    }
  };

  if (loading && !data) {
    return (
      <div className="rounded-lg border border-border px-4 py-10 text-center text-sm text-muted-foreground">
        {tc("loading")}
      </div>
    );
  }

  if (error && !data) {
    return (
      <div
        className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-6 text-center"
        role="alert"
      >
        <p className="text-sm text-destructive">{t("knowledge.loadFailed")}</p>
        <Button size="sm" variant="outline" className="mt-3" onClick={() => void refetch()}>
          {tc("retry")}
        </Button>
      </div>
    );
  }

  const corpus = data!;
  if (!corpus.enabled) {
    return (
      <Card data-testid="deal-room-knowledge-tab">
        <CardContent className="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
          <span className="flex h-12 w-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
            <Books size={24} />
          </span>
          <div className="space-y-1.5">
            <p className="text-sm font-medium text-foreground">{t("knowledge.disabledTitle")}</p>
            <p className="max-w-md text-sm text-muted-foreground">
              {t("knowledge.disabledDescription")}
            </p>
          </div>
        </CardContent>
      </Card>
    );
  }

  const statusLabel = t(`knowledge.status.${corpus.status}`, {
    defaultValue: corpus.status,
  });
  const ungroundedAnswer = isUngroundedKnowledgeAnswer(result?.answer);
  const showKnowledgeSources =
    !!result && !ungroundedAnswer && result.results.length > 0;
  const showKnowledgeNoHits =
    !!result && !showKnowledgeSources && !result.answer;

  return (
    <div className="space-y-4" data-testid="deal-room-knowledge-tab">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-3 space-y-0 pb-3">
          <div className="space-y-1">
            <CardTitle className="flex items-center gap-2 text-h3">
              <Books size={20} />
              {t("knowledge.title")}
            </CardTitle>
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <Badge variant="secondary">{statusLabel}</Badge>
              {corpus.lastSyncedAt ? (
                <span>
                  {t("knowledge.lastSynced", {
                    time: formatRelativeTime(corpus.lastSyncedAt),
                  })}
                </span>
              ) : null}
            </div>
            {corpus.status === "degraded" || corpus.status === "failed" ? (
              <p className="text-xs text-destructive">{t("knowledge.syncIssue")}</p>
            ) : null}
            {isKnowledgeBusy(corpus.status, corpus.progress?.jobStatus) ? (
              <p className="text-xs text-muted-foreground">
                {t("knowledge.syncProgress", {
                  synced: corpus.progress?.synced ?? 0,
                  total: corpus.progress?.total ?? 0,
                  pending: corpus.progress?.pending ?? 0,
                  failed: corpus.progress?.failed ?? 0,
                })}
              </p>
            ) : null}
          </div>
          <Button
            size="sm"
            variant="outline"
            disabled={syncing}
            onClick={() => {
              void onSync();
            }}
            data-testid="deal-room-knowledge-sync"
          >
            <ArrowsClockwise size={16} className="mr-1.5" />
            {syncing ? t("knowledge.syncing") : t("knowledge.sync")}
          </Button>
        </CardHeader>
        <CardContent>
          {corpus.documents.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">
              {t("knowledge.emptyDocuments")}
            </p>
          ) : (
            <ul className="divide-y divide-border rounded-lg border border-border">
              {corpus.documents.map((doc) => (
                <li
                  key={doc.documentId}
                  className="flex items-start justify-between gap-3 px-3 py-2.5 text-sm"
                  data-testid="deal-room-knowledge-doc-row"
                >
                  <div className="min-w-0">
                    <p className="truncate font-medium">
                      {doc.title || doc.documentId}
                    </p>
                    {doc.status === "failed" ? (
                      <p className="text-xs text-destructive">{t("knowledge.docSyncFailed")}</p>
                    ) : (
                      <p className="text-xs text-muted-foreground">
                        {t("knowledge.chunkCount", { count: doc.chunkCount })}
                      </p>
                    )}
                  </div>
                  <Badge variant="outline">
                    {t(`knowledge.docStatus.${doc.status}`, {
                      defaultValue: doc.status,
                    })}
                  </Badge>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-h3">{t("knowledge.queryTitle")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-1.5">
            <Label htmlFor="deal-room-knowledge-query">{t("knowledge.queryLabel")}</Label>
            <div className="flex gap-2">
              <Input
                id="deal-room-knowledge-query"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t("knowledge.queryPlaceholder")}
                onKeyDown={(e) => {
                  if (e.key === "Enter") void onQuery();
                }}
              />
              <Button
                disabled={asking || !query.trim()}
                onClick={() => {
                  void onQuery();
                }}
                data-testid="deal-room-knowledge-ask"
              >
                <MagnifyingGlass size={16} className="mr-1.5" />
                {asking ? t("knowledge.querying") : t("knowledge.ask")}
              </Button>
            </div>
          </div>

          {result ? (
            <div className="space-y-3">
              {result.answer ? (
                <div className="rounded-lg border border-border bg-muted/30 p-3 text-sm whitespace-pre-wrap">
                  {renderAnswerWithCitations(result.answer, (n) => {
                    // Spec: [n] highlights the hit card; jump is via the card button.
                    setActiveCite(n);
                  })}
                </div>
              ) : null}
              {showKnowledgeSources ? (
                <ul className="space-y-2">
                  {result.results.map((hit, idx) => {
                    const n = idx + 1;
                    const locus = formatHitLocusLabel(hit, {
                      sheetPrefix: t("knowledge.sheetLabel", { defaultValue: "Sheet" }),
                    });
                    const canJump = !!(hit.documentId && hit.viewerPage);
                    // DOCX / unmapped sheet: open the document home — never invent a page.
                    const canOpenDoc = !!(hit.documentId && !hit.viewerPage);
                    return (
                      <li
                        key={hit.chunkId || idx}
                        className={cn(
                          "rounded-lg border border-border p-3 text-sm",
                          activeCite === n ? "border-primary ring-1 ring-primary/40" : null,
                        )}
                        data-testid="deal-room-knowledge-hit"
                      >
                        <div className="mb-1 flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
                          <span>[{n}]</span>
                          <span>{hit.score.toFixed(3)}</span>
                        </div>
                        {locus ? (
                          <p
                            className="mb-1 text-xs font-medium text-foreground/80"
                            data-testid="deal-room-knowledge-locus"
                          >
                            {locus}
                          </p>
                        ) : null}
                        <p className="whitespace-pre-wrap">{hit.text}</p>
                        {canJump ? (
                          <Button
                            size="sm"
                            variant="outline"
                            className="mt-2"
                            data-testid="deal-room-knowledge-jump"
                            onClick={() => openViewer(hit.documentId!, hit.viewerPage)}
                          >
                            {t("knowledge.openPage", {
                              page: hit.viewerPage,
                            })}
                          </Button>
                        ) : null}
                        {canOpenDoc ? (
                          <Button
                            size="sm"
                            variant="outline"
                            className="mt-2"
                            title={
                              hit.sheet
                                ? t("knowledge.sheetMapMissing")
                                : t("knowledge.noPageLocus")
                            }
                            data-testid="deal-room-knowledge-jump-doc"
                            onClick={() => openViewer(hit.documentId!)}
                          >
                            {t("knowledge.openDocument")}
                          </Button>
                        ) : null}
                      </li>
                    );
                  })}
                </ul>
              ) : null}
              {showKnowledgeNoHits ? (
                <p className="text-sm text-muted-foreground">{t("knowledge.noHits")}</p>
              ) : null}
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
