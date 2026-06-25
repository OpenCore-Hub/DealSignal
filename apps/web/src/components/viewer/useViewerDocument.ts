import { useCallback, useEffect, useRef, useState } from "react";
import { useParams, useSearchParams } from "react-router";
import { api, type PublicLinkCredentials } from "@/lib/api";
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
  publicAccessCredentials?: PublicLinkCredentials;
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
  publicAccessCredentials,
}: UseViewerDocumentOptions = {}): ViewerDocumentResult {
  const { documentId: routeDocumentId } = useParams<{ documentId: string }>();
  const documentId = publicDocument?.id ?? routeDocumentId;
  const [searchParams] = useSearchParams();
  const initialPage = Math.max(
    1,
    Number.parseInt(searchParams.get("page") || "1", 10) || 1
  );

  const [page, setPage] = useState(initialPage);
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
        const pagesRes = await api.getPublicDocumentPages(id, publicToken, publicAccessCredentials);
        return { doc: publicDocument, pages: pagesRes.pages, analytics: [] };
      }
      return { doc: publicDocument, pages: [], analytics: [] };
    }
    const [d, a] = await Promise.all([
      api.getDocumentById(id),
      api.getPageAnalytics(id),
    ]);
    if (d.status === "ready") {
      const pagesRes = await api.getDocumentPages(id);
      return { doc: d, pages: pagesRes.pages, analytics: a.data };
    }
    return { doc: d, pages: [], analytics: a.data };
  }, [documentId, publicDocument, publicToken, publicAccessCredentials]);

  const { data, loading, error, refetch } = useAsyncData(loadDocument, [loadDocument]);

  const doc = data?.doc ?? null;
  const pages = data?.pages ?? [];
  const analytics = data?.analytics ?? [];

  // Clamp the page to the valid range once pages are known.
  useEffect(() => {
    if (pages.length === 0) return;
    const validPage = Math.min(Math.max(1, page), pages.length);
    if (validPage !== page) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- derived state after load
      setPage(validPage);
    }
  }, [pages.length, page]);

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
          ? await api.getPublicPageSignedUrl(id!, publicToken, page, publicAccessCredentials)
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
  }, [documentId, page, pages.length, publicToken, publicAccessCredentials]);

  // Report public viewer page view duration.
  const pageStartRef = useRef<number>(0);
  useEffect(() => {
    if (!publicToken || !documentId) return;
    pageStartRef.current = Date.now();
    return () => {
      const duration = Math.max(0, Math.round((Date.now() - pageStartRef.current) / 1000));
      if (duration <= 0) return;
      void api.recordPublicEvent(
        {
          event_type: "page_viewed",
          public_token: publicToken,
          visitor_id: publicVisitorId,
          page_number: page,
          duration_seconds: duration,
        },
        publicAccessCredentials
      );
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
