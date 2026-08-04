import { useCallback, useEffect, useState } from "react";
import {
  Archive,
  CircleNotch,
  DownloadSimple,
  Eye,
} from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { downloadDiligencePack } from "@/lib/knowledge/downloadDiligence";
import type {
  DealRoomKnowledgeDiligencePack,
  DealRoomKnowledgeSessionArchive,
} from "@/types";
import { cn } from "@/lib/utils";

interface ColdArchivePanelProps {
  roomId: string;
  /** Bump after ops refresh. */
  refreshKey?: number | string;
  className?: string;
}

/**
 * Read-only cold-archive browser (ceiling Phase U / H).
 * Loads diligence packs from object storage tombstones — never mutates live session state.
 */
export function ColdArchivePanel({
  roomId,
  refreshKey,
  className,
}: ColdArchivePanelProps) {
  const { t, i18n } = useTranslation("dealRooms");
  const [items, setItems] = useState<DealRoomKnowledgeSessionArchive[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [hydrated, setHydrated] = useState(false);
  const [actingId, setActingId] = useState<string | null>(null);
  const [preview, setPreview] = useState<{
    archive: DealRoomKnowledgeSessionArchive;
    pack: DealRoomKnowledgeDiligencePack;
  } | null>(null);

  const load = useCallback(() => {
    let cancelled = false;
    setLoading(true);
    setError(false);
    void api
      .listDealRoomKnowledgeArchives(roomId, { limit: 20 })
      .then((res) => {
        if (cancelled) return;
        setItems(res.items ?? []);
      })
      .catch(() => {
        if (!cancelled) {
          setItems([]);
          setError(true);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
          setHydrated(true);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [roomId]);

  useEffect(() => load(), [load, refreshKey]);

  const formatArchivedAt = (iso: string) => {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    try {
      return new Intl.DateTimeFormat(i18n.language || "en", {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(d);
    } catch {
      return iso;
    }
  };

  const onOpen = async (id: string) => {
    if (actingId) return;
    setActingId(id);
    try {
      const detail = await api.getDealRoomKnowledgeArchive(roomId, id);
      setPreview({ archive: detail.archive, pack: detail.pack });
      setItems((prev) =>
        prev.map((row) =>
          row.id === detail.archive.id ? { ...row, ...detail.archive } : row,
        ),
      );
    } catch {
      toast.error(t("knowledge.archivesOpenFailed"));
    } finally {
      setActingId(null);
    }
  };

  const onDownload = async (id: string) => {
    if (actingId) return;
    setActingId(id);
    try {
      const detail =
        preview?.archive.id === id
          ? preview
          : await api.getDealRoomKnowledgeArchive(roomId, id);
      downloadDiligencePack(
        detail.pack,
        `diligence-archive-${detail.archive.sessionId.slice(0, 8)}.json`,
      );
      toast.success(t("knowledge.archivesDownloadSuccess"));
      setItems((prev) =>
        prev.map((row) =>
          row.id === detail.archive.id ? { ...row, ...detail.archive } : row,
        ),
      );
      if (preview?.archive.id === id) {
        setPreview({ archive: detail.archive, pack: detail.pack });
      }
    } catch {
      toast.error(t("knowledge.archivesDownloadFailed"));
    } finally {
      setActingId(null);
    }
  };

  if (!hydrated && loading) {
    return (
      <aside
        className={cn(
          "rounded-xl border border-border/60 bg-muted/[0.15] px-3.5 py-3 text-[11px] text-muted-foreground",
          className,
        )}
        data-testid="knowledge-cold-archives"
      >
        <span className="inline-flex items-center gap-1.5">
          <CircleNotch size={12} className="animate-spin" />
          {t("knowledge.archivesLoading")}
        </span>
      </aside>
    );
  }

  if (!error && items.length === 0) return null;

  return (
    <aside
      className={cn(
        "rounded-xl border border-border/60 bg-muted/[0.15] px-3.5 py-3",
        className,
      )}
      data-testid="knowledge-cold-archives"
      aria-label={t("knowledge.archivesTitle")}
    >
      <div className="mb-2 min-w-0">
        <p className="inline-flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
          <Archive size={12} weight="bold" />
          {t("knowledge.archivesTitle")}
        </p>
        <p className="mt-0.5 text-[11px] leading-relaxed text-muted-foreground">
          {t("knowledge.archivesHint")}
        </p>
      </div>

      {error && items.length === 0 ? (
        <p className="text-[11px] text-muted-foreground" data-testid="knowledge-cold-archives-error">
          {t("knowledge.archivesLoadFailed")}
        </p>
      ) : null}

      {items.length > 0 ? (
        <ul className="space-y-2" data-testid="knowledge-cold-archives-list">
          {items.map((row) => (
            <li
              key={row.id}
              className="rounded-lg border border-border/50 bg-background/80 px-2.5 py-2"
              data-testid={`knowledge-cold-archive-${row.id}`}
            >
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="text-[12px] font-medium leading-snug text-foreground/90">
                    {row.title?.trim() || t("knowledge.archivesUntitled")}
                  </p>
                  <p className="mt-0.5 text-[10px] text-muted-foreground">
                    {t("knowledge.archivesMeta", {
                      turns: row.turnCount,
                      status:
                        row.status === "restored_readonly"
                          ? t("knowledge.archivesStatusRestored")
                          : t("knowledge.archivesStatusCold"),
                      when: formatArchivedAt(row.archivedAt),
                    })}
                  </p>
                </div>
                <div className="flex flex-wrap gap-1.5">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="h-7 gap-1 px-2 text-[11px]"
                    disabled={actingId === row.id}
                    onClick={() => {
                      void onOpen(row.id);
                    }}
                    data-testid={`knowledge-cold-archive-open-${row.id}`}
                  >
                    {actingId === row.id ? (
                      <CircleNotch size={13} className="animate-spin" />
                    ) : (
                      <Eye size={13} weight="bold" />
                    )}
                    {t("knowledge.archivesOpen")}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    className="h-7 gap-1 px-2 text-[11px]"
                    disabled={actingId === row.id}
                    onClick={() => {
                      void onDownload(row.id);
                    }}
                    data-testid={`knowledge-cold-archive-download-${row.id}`}
                  >
                    <DownloadSimple size={13} weight="bold" />
                    {t("knowledge.archivesDownload")}
                  </Button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      ) : null}

      {preview ? (
        <div
          className="mt-3 rounded-lg border border-border/50 bg-background/90 px-2.5 py-2"
          data-testid="knowledge-cold-archive-preview"
        >
          <div className="mb-1.5 flex flex-wrap items-center justify-between gap-2">
            <p className="text-[11px] font-semibold text-foreground/80">
              {t("knowledge.archivesPreviewTitle")}
            </p>
            <button
              type="button"
              className="text-[10px] text-muted-foreground underline-offset-2 hover:underline"
              onClick={() => setPreview(null)}
              data-testid="knowledge-cold-archive-preview-close"
            >
              {t("knowledge.archivesPreviewClose")}
            </button>
          </div>
          <p className="text-[10px] text-muted-foreground">
            {t("knowledge.archivesPreviewMeta", {
              turns: preview.pack.turns?.length ?? 0,
              fingerprint: preview.pack.corpusFingerprint
                ? `${preview.pack.corpusFingerprint.slice(0, 8)}…`
                : "—",
            })}
          </p>
          <ul className="mt-2 max-h-40 space-y-1.5 overflow-y-auto">
            {(preview.pack.turns ?? []).slice(0, 5).map((turn) => (
              <li
                key={turn.id}
                className="rounded-md border border-border/40 bg-muted/20 px-2 py-1.5 text-[10px] leading-snug text-muted-foreground"
              >
                <span className="block font-medium text-foreground/75">
                  {turn.question}
                </span>
                {turn.answer ? (
                  <span className="mt-0.5 line-clamp-2 block">{turn.answer}</span>
                ) : null}
              </li>
            ))}
          </ul>
          {(preview.pack.turns?.length ?? 0) > 5 ? (
            <p className="mt-1 text-[10px] text-muted-foreground">
              {t("knowledge.archivesPreviewMore", {
                count: (preview.pack.turns?.length ?? 0) - 5,
              })}
            </p>
          ) : null}
        </div>
      ) : null}
    </aside>
  );
}
