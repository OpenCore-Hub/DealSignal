import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Books, ChatCenteredDots } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { formatRelativeTime } from "@/lib/formatters";
import type { AskDocsAuditDetail, AskDocsAuditEntry } from "@/types";

export type AskDocsAuditPanelProps =
  | { mode: "link"; linkId: string }
  | {
      mode: "room";
      roomId: string;
      links?: Array<{ id: string; name?: string }>;
    };

type LoadError = "forbidden" | "generic" | null;

export function AskDocsAuditPanel(props: AskDocsAuditPanelProps) {
  const { t } = useTranslation("linkShare");
  const [entries, setEntries] = useState<AskDocsAuditEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<LoadError>(null);
  const [includeArchived, setIncludeArchived] = useState(false);
  const [linkFilter, setLinkFilter] = useState<string>("all");
  const [selected, setSelected] = useState<AskDocsAuditEntry | null>(null);
  const [detail, setDetail] = useState<AskDocsAuditDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState(false);

  const scopeId = props.mode === "link" ? props.linkId : props.roomId;
  const isRoom = props.mode === "room";

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const res = isRoom
          ? await api.listRoomAskDocsAudit(scopeId, {
              archived: includeArchived,
              linkId: linkFilter === "all" ? undefined : linkFilter,
            })
          : await api.listLinkAskDocsAudit(scopeId, {
              archived: includeArchived,
            });
        if (cancelled) return;
        setEntries(res.data ?? []);
      } catch (e) {
        if (cancelled) return;
        if (e instanceof ApiError && e.status === 403) {
          setError("forbidden");
        } else {
          setError("generic");
        }
        setEntries([]);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [isRoom, scopeId, includeArchived, linkFilter]);

  useEffect(() => {
    if (!selected) {
      setDetail(null);
      setDetailError(false);
      return;
    }
    const detailLinkId = isRoom ? selected.link_id : scopeId;
    if (!detailLinkId) {
      setDetailError(true);
      return;
    }
    let cancelled = false;
    async function loadDetail() {
      setDetailLoading(true);
      setDetailError(false);
      try {
        const res = await api.getLinkAskDocsAudit(detailLinkId!, selected!.session_id);
        if (cancelled) return;
        setDetail(res);
      } catch {
        if (cancelled) return;
        setDetailError(true);
        setDetail(null);
      } finally {
        if (!cancelled) setDetailLoading(false);
      }
    }
    void loadDetail();
    return () => {
      cancelled = true;
    };
  }, [selected, isRoom, scopeId]);

  const title =
    props.mode === "room" ? t("askDocsAudit.roomTitle") : t("askDocsAudit.title");
  const description =
    props.mode === "room"
      ? t("askDocsAudit.roomDescription")
      : t("askDocsAudit.description");

  const linkName = (linkId?: string) => {
    if (!linkId || props.mode !== "room") return null;
    const match = props.links?.find((l) => l.id === linkId);
    return match?.name || linkId;
  };

  const resultLabel = (status?: string) => {
    if (!status) return null;
    const key = `askDocsAudit.resultStatuses.${status}`;
    const translated = t(key);
    return translated === key ? status : translated;
  };

  return (
    <Card data-testid="ask-docs-audit-panel">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-h3">
          <Books size={20} />
          {title}
        </CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {selected ? (
          <div className="space-y-4">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="gap-1.5 px-0"
              onClick={() => setSelected(null)}
            >
              <ArrowLeft size={16} />
              {t("askDocsAudit.detailBack")}
            </Button>
            <h3 className="text-sm font-medium">{t("askDocsAudit.detailTitle")}</h3>
            {detailLoading ? (
              <p className="py-4 text-center text-sm text-muted-foreground">
                {t("askDocsAudit.detailLoading")}
              </p>
            ) : detailError || !detail ? (
              <p className="py-4 text-center text-sm text-muted-foreground">
                {t("askDocsAudit.detailFailed")}
              </p>
            ) : (
              <div className="space-y-4">
                <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  <span>{detail.visitor_id || t("askDocsAudit.anonymous")}</span>
                  <span>·</span>
                  <span>{formatRelativeTime(detail.created_at)}</span>
                  {detail.result_status ? (
                    <>
                      <span>·</span>
                      <Badge variant="outline">
                        {t("askDocsAudit.resultStatus")}: {resultLabel(detail.result_status)}
                      </Badge>
                    </>
                  ) : null}
                  {detail.archived ? (
                    <Badge variant="secondary">{t("askDocsAudit.archivedBadge")}</Badge>
                  ) : null}
                </div>

                <div>
                  <p className="mb-2 text-sm font-medium">{t("askDocsAudit.messages")}</p>
                  <div className="space-y-2">
                    {detail.messages.map((m, idx) => (
                      <div key={`${m.role}-${idx}`} className="rounded-lg border p-3">
                        <p className="mb-1 text-xs font-medium uppercase text-muted-foreground">
                          {m.role}
                        </p>
                        <p className="text-sm whitespace-pre-wrap">{m.content}</p>
                      </div>
                    ))}
                  </div>
                </div>

                <div>
                  <p className="mb-2 text-sm font-medium">{t("askDocsAudit.evidence")}</p>
                  {detail.evidence.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                      {t("askDocsAudit.noEvidence")}
                    </p>
                  ) : (
                    <div className="space-y-2">
                      {detail.evidence.map((ev) => (
                        <div key={ev.chunk_id} className="rounded-lg border p-3 text-sm">
                          <p className="text-muted-foreground">
                            {ev.document_id ? `${ev.document_id} · ` : ""}
                            p.{ev.page_number}
                          </p>
                          <p className="mt-1">{ev.quote}</p>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        ) : (
          <>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <p className="text-xs text-muted-foreground">
                {t("askDocsAudit.hotWindowHint")}
              </p>
              <div className="flex items-center gap-2">
                <Switch
                  id="ask-docs-audit-archived"
                  checked={includeArchived}
                  onCheckedChange={setIncludeArchived}
                />
                <Label htmlFor="ask-docs-audit-archived" className="text-sm font-normal">
                  {t("askDocsAudit.showArchived")}
                </Label>
              </div>
            </div>

            {props.mode === "room" && (props.links?.length ?? 0) > 0 ? (
              <div className="space-y-1.5">
                <Label htmlFor="ask-docs-audit-link-filter">
                  {t("askDocsAudit.filterByLink")}
                </Label>
                <select
                  id="ask-docs-audit-link-filter"
                  aria-label={t("askDocsAudit.filterByLink")}
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
                  value={linkFilter}
                  onChange={(e) => setLinkFilter(e.target.value)}
                >
                  <option value="all">{t("askDocsAudit.filterAllLinks")}</option>
                  {props.links!.map((link) => (
                    <option key={link.id} value={link.id}>
                      {link.name || link.id}
                    </option>
                  ))}
                </select>
              </div>
            ) : null}

            {loading ? (
              <p className="py-4 text-center text-sm text-muted-foreground">
                {t("askDocsAudit.loading")}
              </p>
            ) : error === "forbidden" ? (
              <p className="py-4 text-center text-sm text-muted-foreground">
                {t("askDocsAudit.forbidden")}
              </p>
            ) : error === "generic" ? (
              <p className="py-4 text-center text-sm text-muted-foreground">
                {t("askDocsAudit.loadFailed")}
              </p>
            ) : entries.length === 0 ? (
              <p className="py-4 text-center text-sm text-muted-foreground">
                {includeArchived
                  ? t("askDocsAudit.emptyArchived")
                  : t("askDocsAudit.empty")}
              </p>
            ) : (
              <div className="space-y-3">
                {entries.map((entry) => (
                  <button
                    key={entry.session_id}
                    type="button"
                    className="w-full rounded-lg border p-3 text-left transition-colors hover:bg-muted/40"
                    onClick={() => setSelected(entry)}
                    aria-label={entry.question_preview}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0 flex-1 space-y-1">
                        <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                          <ChatCenteredDots size={14} />
                          <span>{entry.visitor_id || t("askDocsAudit.anonymous")}</span>
                          <span>·</span>
                          <span>{t("askDocsAudit.channel")}</span>
                          {linkName(entry.link_id) ? (
                            <>
                              <span>·</span>
                              <span>{linkName(entry.link_id)}</span>
                            </>
                          ) : null}
                        </div>
                        <p className="text-sm font-medium">「{entry.question_preview}」</p>
                        <p className="text-xs text-muted-foreground">
                          {t("askDocsAudit.evidenceCount", {
                            count: entry.evidence_count,
                          })}{" "}
                          · {formatRelativeTime(entry.created_at)}
                        </p>
                      </div>
                      <div className="flex shrink-0 flex-col items-end gap-1">
                        {entry.result_status ? (
                          <Badge variant="outline">{resultLabel(entry.result_status)}</Badge>
                        ) : null}
                        {entry.archived ? (
                          <Badge variant="secondary">
                            {t("askDocsAudit.archivedBadge")}
                          </Badge>
                        ) : null}
                      </div>
                    </div>
                  </button>
                ))}
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
