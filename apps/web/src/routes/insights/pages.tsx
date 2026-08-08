import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { Combobox } from "@base-ui/react/combobox";
import { CaretDown, Check, FileText } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DocumentAnalytics } from "@/components/documents/DocumentAnalytics";
import { DocumentVisitorsCard } from "@/components/documents/DocumentVisitorsCard";
import {
  InsightsRangeControls,
  useInsightsRange,
} from "@/components/insights/InsightsRangeControls";
import { ReadingFunnelCard } from "@/components/insights/ReadingFunnelCard";
import { ReadingSessionsCard } from "@/components/insights/ReadingSessionsCard";
import { EmptyState } from "@/components/common/EmptyState";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import { documentDetailPath } from "@/lib/documentDetailNav";
import { cn } from "@/lib/utils";
import { useTranslation } from "react-i18next";
import type { Document } from "@/types";

export function mergeInsightDocuments(general: Document[], dealRoom: Document[]): Document[] {
  const byId = new Map<string, Document>();
  for (const d of [...general, ...dealRoom]) {
    byId.set(d.id, d);
  }
  return Array.from(byId.values()).sort((a, b) =>
    a.title.localeCompare(b.title, undefined, { sensitivity: "base" }),
  );
}

function documentLabel(
  doc: Document,
  untitled: string,
  dealRoomSuffix: string,
): string {
  const title = doc.title.trim() || untitled;
  return doc.category === "deal_room" ? `${title} · ${dealRoomSuffix}` : title;
}

export function InsightsPagesPage() {
  const { t } = useTranslation("insights");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [selectedDocId, setSelectedDocId] = useState("");
  const [docQuery, setDocQuery] = useState("");
  const rangeCtl = useInsightsRange(30);
  const {
    data: documents,
    loading: loadingDocs,
    error,
    refetch,
  } = useAsyncData(async () => {
    // Insights covers library + data-room docs; agreements stay in the agreements surface.
    const [generalRes, dealRoomRes] = await Promise.all([
      api.getDocuments("all", "general"),
      api.getDocuments("all", "deal_room"),
    ]);
    return mergeInsightDocuments(generalRes.data, dealRoomRes.data);
  }, []);

  useEffect(() => {
    if (!documents?.length) {
      setSelectedDocId("");
      return;
    }
    setSelectedDocId((prev) => {
      if (prev && documents.some((d) => d.id === prev)) return prev;
      return documents[0]?.id || "";
    });
  }, [documents]);

  const {
    data: analytics,
    loading: loadingAnalytics,
  } = useAsyncData(
    async () => {
      if (!selectedDocId) return [];
      const res = await api.getPageAnalytics(selectedDocId, rangeCtl.apiParams);
      return res.data;
    },
    [selectedDocId, rangeCtl.apiParams],
  );

  const {
    data: funnel,
    loading: loadingFunnel,
  } = useAsyncData(
    async () => {
      if (!selectedDocId) return null;
      return api.getDocumentReadingFunnel(selectedDocId, rangeCtl.apiParams);
    },
    [selectedDocId, rangeCtl.apiParams],
  );

  const {
    data: sessions,
    loading: loadingSessions,
  } = useAsyncData(
    async () => {
      if (!selectedDocId) return null;
      return api.getDocumentReadingSessions(selectedDocId, 40, rangeCtl.apiParams);
    },
    [selectedDocId, rangeCtl.apiParams],
  );

  const {
    data: visitors,
    loading: loadingVisitors,
  } = useAsyncData(
    async () => {
      if (!selectedDocId) return [];
      const res = await api.getDocumentVisitors(selectedDocId, rangeCtl.apiParams);
      return res.data;
    },
    [selectedDocId, rangeCtl.apiParams],
  );

  const openPage =
    workspaceSlug && selectedDocId
      ? (pageNumber: number) =>
          navigate(
            documentDetailPath(workspaceSlug, selectedDocId, {
              tab: "content",
              page: pageNumber,
            }),
          )
      : undefined;

  const untitled = t("pages.untitledDocument");
  const dealRoomSuffix = t("pages.categoryDealRoom");

  const filteredDocuments = useMemo(() => {
    if (!documents) return [];
    const q = docQuery.trim().toLowerCase();
    if (!q) return documents;
    return documents.filter((doc) => {
      const label = documentLabel(doc, untitled, dealRoomSuffix).toLowerCase();
      return label.includes(q) || doc.title.toLowerCase().includes(q);
    });
  }, [documents, docQuery, untitled, dealRoomSuffix]);

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>{tc("retry")}</Button>
      </div>
    );
  }

  if (loadingDocs) {
    return <Skeleton className="h-80" />;
  }

  if (!documents || documents.length === 0) {
    return (
      <EmptyState
        icon={<FileText size={48} />}
        title={t("pages.emptyTitle")}
        description={t("pages.emptyDescription")}
      />
    );
  }

  const selectedDoc = documents.find((d) => d.id === selectedDocId);
  const selectedDocLabel = selectedDoc
    ? documentLabel(selectedDoc, untitled, dealRoomSuffix)
    : t("pages.selectPlaceholder");

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <InsightsRangeControls
          variant="chips"
          className="w-full sm:w-auto sm:max-w-xl"
          range={rangeCtl.range}
          customOpen={rangeCtl.customOpen}
          draftFrom={rangeCtl.draftFrom}
          draftTo={rangeCtl.draftTo}
          rangeError={rangeCtl.rangeError}
          onSelectPreset={rangeCtl.selectPreset}
          onOpenCustom={rangeCtl.openCustom}
          onDraftFromChange={rangeCtl.setDraftFrom}
          onDraftToChange={rangeCtl.setDraftTo}
          onApplyCustom={rangeCtl.applyCustom}
        />

        <Combobox.Root
          value={selectedDocId || null}
          onValueChange={(next) => {
            if (next) setSelectedDocId(next);
          }}
          onInputValueChange={setDocQuery}
          onOpenChange={(open) => {
            if (!open) setDocQuery("");
          }}
        >
          <Combobox.Trigger
            data-testid="insights-document-picker"
            className={cn(
              "flex h-9 w-full items-center justify-between gap-2 rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none transition-colors",
              "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
              "sm:w-80 sm:max-w-[22rem] sm:self-end",
            )}
          >
            <span className="min-w-0 flex-1 truncate text-left">
              {selectedDocLabel}
            </span>
            <Combobox.Icon
              render={<CaretDown size={16} className="shrink-0 text-muted-foreground" />}
            />
          </Combobox.Trigger>

          <Combobox.Portal>
            <Combobox.Positioner
              className="isolate z-50"
              align="end"
              side="bottom"
              sideOffset={4}
            >
              <Combobox.Popup
                className={cn(
                  "z-50 w-[min(22rem,var(--available-width))] min-w-[var(--anchor-width)] origin-[var(--transform-origin)] overflow-hidden rounded-lg border border-border bg-popover text-popover-foreground shadow-md outline-none",
                  "data-[side=bottom]:slide-in-from-top-2 data-[side=top]:slide-in-from-bottom-2",
                  "data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95",
                  "data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95",
                )}
              >
                <div className="border-b border-border px-2 py-1.5">
                  <Combobox.Input
                    placeholder={t("pages.searchPlaceholder")}
                    className={cn(
                      "flex h-8 w-full bg-transparent px-1.5 text-sm outline-none",
                      "placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-0",
                    )}
                  />
                </div>
                <Combobox.List className="max-h-64 overflow-auto p-1">
                  {filteredDocuments.map((doc) => {
                    const label = documentLabel(doc, untitled, dealRoomSuffix);
                    return (
                      <Combobox.Item
                        key={doc.id}
                        value={doc.id}
                        className={cn(
                          "group relative flex w-full cursor-default items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none select-none",
                          "hover:bg-accent hover:text-accent-foreground data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground",
                        )}
                      >
                        <FileText
                          size={16}
                          className="shrink-0 text-muted-foreground group-hover:text-accent-foreground group-data-[highlighted]:text-accent-foreground"
                        />
                        <span className="min-w-0 flex-1 truncate">{label}</span>
                        <Combobox.ItemIndicator className="ml-auto text-foreground">
                          <Check size={16} weight="bold" />
                        </Combobox.ItemIndicator>
                      </Combobox.Item>
                    );
                  })}
                  {filteredDocuments.length === 0 ? (
                    <Combobox.Empty className="px-2 py-4 text-center text-sm text-muted-foreground">
                      {t("pages.noSearchResults")}
                    </Combobox.Empty>
                  ) : null}
                </Combobox.List>
              </Combobox.Popup>
            </Combobox.Positioner>
          </Combobox.Portal>
        </Combobox.Root>
      </div>

      <ReadingFunnelCard
        funnel={funnel}
        loading={loadingFunnel}
        onOpenPage={openPage}
      />

      <ReadingSessionsCard
        data={sessions}
        loading={loadingSessions}
        onOpenPage={openPage}
      />

      {loadingVisitors ? (
        <Skeleton className="h-48" />
      ) : (
        <DocumentVisitorsCard
          visitors={visitors ?? []}
          title={t("pages.visitorsTitle")}
          linkToContacts={false}
          emptyTitle={t("pages.visitorsEmpty")}
          emptyDescription={t("pages.noAnalyticsDescription")}
          anonymousLabel={t("pages.anonymousVisitor")}
          metaLabel={({ pages, duration }) =>
            t("pages.visitorMeta", { pages, duration })
          }
        />
      )}

      {loadingAnalytics ? (
        <Skeleton className="h-80" />
      ) : analytics?.length === 0 ? (
        <EmptyState
          icon={<FileText size={48} />}
          title={t("pages.noAnalyticsTitle")}
          description={t("pages.noAnalyticsDescription")}
        />
      ) : (
        <DocumentAnalytics
          analytics={analytics ?? []}
          variant="detail"
          onOpenPage={openPage}
        />
      )}
    </div>
  );
}
