import { useEffect, useRef, useState } from "react";
import { useParams } from "react-router";
import { FileText } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { useAIStore } from "@/stores/aiStore";
import { ViewerToolbar } from "./ViewerToolbar";
import { ViewerCanvas } from "./ViewerCanvas";
import type { WatermarkInfo } from "./WatermarkOverlay";
import type { Document, Evidence, PageAnalytics } from "@/types";

interface CanvasViewerProps {
  evidence?: Evidence[];
  watermark?: WatermarkInfo | null;
  publicToken?: string;
  publicLink?: { id: string; downloadEnabled: boolean; watermarkEnabled: boolean };
  publicDocument?: Document;
  publicVisitorId?: string;
}

interface PageInfo {
  pageNumber: number;
  width: number;
  height: number;
}

export function CanvasViewer({
  evidence,
  watermark,
  publicToken,
  publicLink,
  publicDocument,
  publicVisitorId,
}: CanvasViewerProps = {}) {
  const { documentId: routeDocumentId } = useParams<{ documentId: string }>();
  const documentId = publicDocument?.id ?? routeDocumentId;
  const { t } = useTranslation(["documents", "common"]);
  const [doc, setDoc] = useState<Document | null>(null);
  const [analytics, setAnalytics] = useState<PageAnalytics[]>([]);
  const [pages, setPages] = useState<PageInfo[]>([]);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryTick, setRetryTick] = useState(0);
  const [page, setPage] = useState(1);
  const [zoom, setZoom] = useState(100);
  const { highlightedPage } = useAIStore();

  useEffect(() => {
    let cancelled = false;
    const id = documentId;
    if (!id) return;
    async function load() {
      try {
        setLoading(true);
        setError(null);
        if (publicDocument) {
          setDoc(publicDocument);
          setAnalytics([]);
          setPage(1);
          if (publicDocument.status === "ready" && publicToken) {
            const pagesRes = await api.getPublicDocumentPages(id!, publicToken);
            if (!cancelled) {
              setPages(pagesRes.pages);
            }
          } else {
            setPages([]);
          }
        } else {
          const [d, a] = await Promise.all([api.getDocumentById(id!), api.getPageAnalytics(id!)]);
          if (!cancelled) {
            setDoc(d);
            setAnalytics(a.data);
            setPage(1);
            if (d.status === "ready") {
              const pagesRes = await api.getDocumentPages(id!);
              if (!cancelled) {
                setPages(pagesRes.pages);
              }
            } else {
              setPages([]);
            }
          }
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : t("common:error.loadFailed"));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [documentId, retryTick, t, publicDocument, publicToken]);

  // Synchronize the viewer page with the AI highlight from the global store.
  useEffect(() => {
    if (highlightedPage && highlightedPage !== page) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- external store synchronization
      setPage(highlightedPage);
    }
  }, [highlightedPage, page]);

  useEffect(() => {
    let cancelled = false;
    const id = documentId;
    if (!id || pages.length === 0) return;
    async function loadSignedUrl() {
      try {
        const res = publicToken
          ? await api.getPublicPageSignedUrl(id!, publicToken, page)
          : await api.getPageSignedUrl(id!, page);
        if (!cancelled) setImageUrl(res.image_url);
      } catch {
        if (!cancelled) setImageUrl(null);
      }
    }
    loadSignedUrl();
    return () => {
      cancelled = true;
    };
  }, [documentId, page, pages.length, publicToken]);

  const pageStartRef = useRef<number>(0);
  useEffect(() => {
    if (!publicToken || !documentId) return;
    pageStartRef.current = Date.now();
    return () => {
      const duration = Math.max(0, Math.round((Date.now() - pageStartRef.current) / 1000));
      if (duration <= 0) return;
      void api.recordPublicEvent({
        event_type: "page_viewed",
        public_token: publicToken,
        visitor_id: publicVisitorId,
        page_number: page,
        duration_seconds: duration,
      });
    };
  }, [publicToken, documentId, page, publicVisitorId]);

  // For authenticated viewers, report a page view after the user has dwelled on the page.
  useEffect(() => {
    if (publicToken || !documentId || !doc || doc.status !== "ready") return;
    pageStartRef.current = Date.now();
    const timer = setTimeout(() => {
      void api.recordViewerEvent({
        documentId,
        eventType: "page_viewed",
        pageNumber: page,
        durationSeconds: Math.max(1, Math.round((Date.now() - pageStartRef.current) / 1000)),
      });
    }, 2000);
    return () => clearTimeout(timer);
  }, [publicToken, documentId, page, doc]);

  const handleDownload = async () => {
    if (!documentId || !doc) return;
    try {
      const res = publicToken
        ? await api.getPublicDocumentDownloadUrl(documentId, publicToken)
        : await api.getDocumentDownloadUrl(documentId);
      if (publicToken && publicLink && !publicLink.downloadEnabled) {
        setError(t("documents:viewer.downloadDisabled"));
        return;
      }
      const a = document.createElement("a");
      a.href = res.download_url;
      a.download = res.filename || doc.title;
      a.target = "_blank";
      a.rel = "noopener noreferrer";
      document.body.appendChild(a);
      a.click();
      a.remove();
      if (publicToken) {
        void api.recordPublicEvent({
          event_type: "download_attempted",
          public_token: publicToken,
          visitor_id: publicVisitorId,
        });
      } else {
        void api.recordViewerEvent({
          documentId,
          eventType: "download_attempted",
        });
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : t("common:error.loadFailed"));
    }
  };

  if (loading) {
    return (
      <div className="flex flex-1 flex-col bg-neutral-50 dark:bg-background">
        <header className="flex h-14 items-center border-b border-border bg-background px-4">
          <Skeleton className="h-8 w-64" />
        </header>
        <div className="flex flex-1">
          <Skeleton className="m-8 h-full w-48" />
          <Skeleton className="m-8 h-full flex-1" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-4 bg-neutral-50 dark:bg-background">
        <FileText size={48} className="text-muted-foreground/50" />
        <p className="text-body text-destructive">{t("documents:viewer.loadFailed", { error })}</p>
        <Button onClick={() => setRetryTick((x) => x + 1)}>{t("common:retry")}</Button>
      </div>
    );
  }

  if (!doc) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center bg-neutral-50 dark:bg-background">
        <FileText size={48} className="text-muted-foreground/50" />
        <p className="mt-4 text-body text-muted-foreground">{t("documents:viewer.notFound")}</p>
      </div>
    );
  }

  const totalPages = pages.length > 0 ? pages.length : doc.pageCount;

  return (
    <div className="flex flex-1 flex-col bg-neutral-50 dark:bg-background">
      <ViewerToolbar
        doc={doc}
        page={page}
        totalPages={totalPages}
        zoom={zoom}
        onZoomOut={() => setZoom((z) => Math.max(50, z - 10))}
        onZoomIn={() => setZoom((z) => Math.min(200, z + 10))}
        onPreviousPage={() => setPage((p) => Math.max(1, p - 1))}
        onNextPage={() => setPage((p) => Math.min(totalPages, p + 1))}
        onDownload={handleDownload}
      />
      <ViewerCanvas
        doc={doc}
        page={page}
        zoom={zoom}
        pages={pages}
        analytics={analytics}
        imageUrl={imageUrl}
        evidence={evidence}
        watermark={watermark}
        onSelectPage={setPage}
      />
    </div>
  );
}
