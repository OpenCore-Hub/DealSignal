import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "react-router";
import { api } from "@/lib/api";
import { useAIStore } from "@/stores/aiStore";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { Document, Evidence, PageAnalytics } from "@/types";
import type { WatermarkInfo } from "./WatermarkOverlay";

interface PageInfo {
  pageNumber: number;
  width: number;
  height: number;
}

interface ViewerDocumentData {
  doc: Document | null;
  pages: PageInfo[];
  analytics: PageAnalytics[];
}

interface UseViewerDocumentOptions {
  evidence?: Evidence[];
  watermark?: WatermarkInfo | null;
  publicToken?: string;
  publicLink?: { id: string; downloadEnabled: boolean; watermarkEnabled: boolean };
  publicDocument?: Document;
  publicVisitorId?: string;
}

interface ViewerDocumentResult {
  documentId: string | undefined;
  doc: Document | null;
  pages: PageInfo[];
  analytics: PageAnalytics[];
  imageUrl: string | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
  page: number;
  setPage: (page: number | ((prev: number) => number)) => void;
  zoom: number;
  setZoom: (zoom: number | ((prev: number) => number)) => void;
}

export function useViewerDocument({
  publicToken,
  publicDocument,
  publicVisitorId,
}: UseViewerDocumentOptions = {}): ViewerDocumentResult {
  const { documentId: routeDocumentId } = useParams<{ documentId: string }>();
  const documentId = publicDocument?.id ?? routeDocumentId;

  const [page, setPage] = useState(1);
  const [zoom, setZoom] = useState(100);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const { highlightedPage } = useAIStore();

  const loadDocument = useCallback(async (): Promise<ViewerDocumentData> => {
    const id = documentId;
    if (!id) {
      return { doc: null, pages: [], analytics: [] };
    }
    if (publicDocument) {
      if (publicDocument.status === "ready" && publicToken) {
        const pagesRes = await api.getPublicDocumentPages(id, publicToken);
        setPage(1);
        return { doc: publicDocument, pages: pagesRes.pages, analytics: [] };
      }
      setPage(1);
      return { doc: publicDocument, pages: [], analytics: [] };
    }
    const [d, a] = await Promise.all([
      api.getDocumentById(id),
      api.getPageAnalytics(id),
    ]);
    if (d.status === "ready") {
      const pagesRes = await api.getDocumentPages(id);
      setPage(1);
      return { doc: d, pages: pagesRes.pages, analytics: a.data };
    }
    setPage(1);
    return { doc: d, pages: [], analytics: a.data };
  }, [documentId, publicDocument, publicToken]);

  const { data, loading, error, refetch } = useAsyncData(loadDocument, [loadDocument]);

  const doc = data?.doc ?? null;
  const pages = data?.pages ?? [];
  const analytics = data?.analytics ?? [];

  // Synchronize the viewer page with the AI highlight from the global store.
  useEffect(() => {
    if (highlightedPage && highlightedPage !== page) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- external store synchronization
      setPage(highlightedPage);
    }
  }, [highlightedPage, page]);

  // Fetch a signed URL for the current page.
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

  // Report public viewer page view duration.
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

  // Report authenticated viewer page view after a short dwell.
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

  return {
    documentId,
    doc,
    pages,
    analytics,
    imageUrl,
    loading,
    error,
    refetch,
    page,
    setPage,
    zoom,
    setZoom,
  };
}
