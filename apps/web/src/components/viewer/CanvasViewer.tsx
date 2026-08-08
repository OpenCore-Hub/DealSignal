import { useCallback, useEffect, useState } from "react";
import { useParams } from "react-router";
import { FileText, LockSimple, Prohibit } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { api, type PublicLinkCredentials } from "@/lib/api";
import { getCachedAccountEmail } from "@/lib/authAccount";
import { isViewerAccessErrorKind, viewerPolicyBlockI18nKeys } from "@/lib/viewerAccessErrors";
import { apiErrorMessage } from "@/lib/apiErrors";
import { ViewerToolbar } from "./ViewerToolbar";
import { PublicViewerToolbar } from "./PublicViewerToolbar";
import { ViewerCanvas } from "./ViewerCanvas";
import { ViewerKnowledgeRail } from "./ViewerKnowledgeRail";
import { useViewerDocument } from "./useViewerDocument";
import type { WatermarkInfo } from "./WatermarkOverlay";
import type { Document, Evidence } from "@/types";

interface CanvasViewerProps {
  evidence?: Evidence[];
  watermark?: WatermarkInfo | null;
  publicToken?: string;
  publicLink?: {
    id: string;
    name?: string;
    downloadEnabled: boolean;
    watermarkEnabled: boolean;
    screenshotProtectionEnabled?: boolean;
  };
  publicDocument?: Document;
  publicVisitorId?: string;
  publicAccessCredentials?: PublicLinkCredentials;
  sidebarOpen?: boolean;
  onToggleSidebar?: () => void;
  sidebar?: React.ReactNode;
  requestedPage?: number | null;
  onRequestedPageApplied?: () => void;
  /**
   * Authenticated owner Viewer only: deal-room id for grounded knowledge rail.
   * Ignored when `publicToken` is set (Visitor channel stays separate).
   */
  knowledgeRoomId?: string;
}

export function CanvasViewer({
  evidence,
  watermark,
  publicToken,
  publicLink,
  publicDocument,
  publicVisitorId,
  publicAccessCredentials,
  sidebarOpen = false,
  onToggleSidebar,
  sidebar,
  requestedPage = null,
  onRequestedPageApplied,
  knowledgeRoomId,
}: CanvasViewerProps = {}) {
  const { t } = useTranslation(["documents", "common"]);
  const { documentId: routeDocumentId } = useParams<{ documentId: string }>();
  const documentId = publicDocument?.id ?? routeDocumentId;
  const [actionError, setActionError] = useState<string | null>(null);
  const ownerKnowledgeRoomId =
    !publicToken && knowledgeRoomId?.trim() ? knowledgeRoomId.trim() : "";
  const [knowledgeSidebarOpen, setKnowledgeSidebarOpen] = useState(
    () => Boolean(ownerKnowledgeRoomId),
  );
  // Owner /viewer: identity = logged-in account email (same shape as share-link).
  const [ownerEmail, setOwnerEmail] = useState<string | undefined>(() =>
    !publicToken && watermark === undefined ? getCachedAccountEmail() : undefined,
  );

  useEffect(() => {
    if (ownerKnowledgeRoomId) setKnowledgeSidebarOpen(true);
  }, [ownerKnowledgeRoomId]);

  useEffect(() => {
    if (publicToken || watermark !== undefined) return;
    let cancelled = false;
    const cached = getCachedAccountEmail();
    if (cached) setOwnerEmail(cached);
    api
      .getMe()
      .then((user) => {
        if (cancelled) return;
        const email = user.email?.trim();
        if (email) setOwnerEmail(email);
      })
      .catch(() => {
        // Keep session cache when /auth/me is unavailable (stale API image).
        if (cancelled) return;
        const fallback = getCachedAccountEmail();
        if (fallback) setOwnerEmail(fallback);
      });
    return () => {
      cancelled = true;
    };
  }, [publicToken, watermark]);

  const resolvedWatermark: WatermarkInfo | null | undefined =
    watermark !== undefined
      ? watermark
      : ownerEmail
        ? { email: ownerEmail }
        : {};

  const {
    doc,
    pages,
    analytics,
    imageUrl,
    loading,
    error: loadError,
    accessErrorKind,
    refetch,
    page,
    setPage,
    zoom,
    setZoom,
  } = useViewerDocument({
    publicToken,
    publicLink,
    publicDocument,
    publicVisitorId,
    publicAccessCredentials,
  });

  const totalPages = doc ? (pages.length > 0 ? pages.length : doc.pageCount) : 0;

  useEffect(() => {
    if (requestedPage == null || requestedPage < 1) return;
    setPage(Math.min(requestedPage, Math.max(totalPages, 1)));
    onRequestedPageApplied?.();
  }, [requestedPage, totalPages, setPage, onRequestedPageApplied]);

  const goToPreviousPage = useCallback(() => {
    setPage((p) => Math.max(1, p - 1));
  }, [setPage]);

  const goToNextPage = useCallback(() => {
    setPage((p) => Math.min(totalPages, p + 1));
  }, [setPage, totalPages]);

  const zoomOut = useCallback(() => {
    setZoom((z) => Math.max(50, z - 10));
  }, [setZoom]);

  const zoomIn = useCallback(() => {
    setZoom((z) => Math.min(200, z + 10));
  }, [setZoom]);

  const goToFirstPage = useCallback(() => setPage(1), [setPage]);
  const goToLastPage = useCallback(() => setPage(totalPages), [setPage, totalPages]);

  const handleDownload = useCallback(async () => {
    if (!documentId || !doc) return;
    try {
      if (publicToken && publicLink && !publicLink.downloadEnabled) {
        setActionError(t("documents:viewer.downloadDisabled"));
        return;
      }
      const res = publicToken
        ? await api.getPublicDocumentDownloadUrl(documentId, publicToken, publicAccessCredentials)
        : await api.getDocumentDownloadUrl(documentId);
      const a = document.createElement("a");
      a.href = res.download_url;
      a.download = res.filename || doc.title;
      a.target = "_blank";
      a.rel = "noopener noreferrer";
      document.body.appendChild(a);
      a.click();
      a.remove();
      if (publicToken) {
        void api.recordPublicEvent(
          {
            event_type: "download_attempted",
            public_token: publicToken,
            visitor_id: publicVisitorId,
          },
          publicAccessCredentials
        );
      } else {
        void api.recordViewerEvent({
          documentId,
          eventType: "download_attempted",
        });
      }
      setActionError(null);
    } catch (e) {
      setActionError(apiErrorMessage(e, { fallback: "loadFailed" }));
    }
  }, [documentId, doc, publicToken, publicLink, publicVisitorId, publicAccessCredentials, t]);

  // Keyboard shortcuts for viewer navigation.
  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      const target = event.target as HTMLElement;
      if (
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target.isContentEditable
      ) {
        return;
      }

      switch (event.key) {
        case "ArrowLeft":
        case "PageUp":
          event.preventDefault();
          goToPreviousPage();
          break;
        case "ArrowRight":
        case "PageDown":
          event.preventDefault();
          goToNextPage();
          break;
        case "Home":
          event.preventDefault();
          goToFirstPage();
          break;
        case "End":
          event.preventDefault();
          goToLastPage();
          break;
        case "+":
        case "=":
          event.preventDefault();
          zoomIn();
          break;
        case "-":
        case "_":
          event.preventDefault();
          zoomOut();
          break;
        case "d":
        case "D":
          if (event.ctrlKey || event.metaKey) {
            event.preventDefault();
            void handleDownload();
          }
          break;
        default:
          break;
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [goToPreviousPage, goToNextPage, goToFirstPage, goToLastPage, zoomIn, zoomOut, handleDownload]);

  const error = loadError || actionError;

  if (loading) {
    const shellClass = publicToken
      ? "public-viewer-shell flex min-h-0 flex-1 flex-col"
      : "flex min-h-0 flex-1 flex-col bg-neutral-50 dark:bg-background";
    return (
      <div className={shellClass}>
        <header className="public-viewer-glass mx-3 mt-3 flex h-14 items-center rounded-2xl px-4 sm:mx-4">
          <Skeleton className="h-8 w-64 rounded-xl" />
        </header>
        <div className="flex flex-1 gap-3 p-4">
          <Skeleton className="hidden h-full w-16 rounded-2xl md:block" />
          <Skeleton className="h-full flex-1 rounded-2xl" />
        </div>
      </div>
    );
  }

  if (error) {
    const accessBlocked = accessErrorKind && isViewerAccessErrorKind(accessErrorKind);
    const policyBlockKeys = accessErrorKind ? viewerPolicyBlockI18nKeys(accessErrorKind) : null;
    const Icon = accessErrorKind === "locked" ? LockSimple : accessBlocked ? Prohibit : FileText;

    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-4 bg-neutral-50 px-6 text-center dark:bg-background">
        <Icon
          size={48}
          className={accessBlocked ? "text-muted-foreground" : "text-muted-foreground/50"}
        />
        {policyBlockKeys ? (
          <>
            <div className="max-w-md space-y-2">
              <p className="text-base font-medium text-foreground">{t(policyBlockKeys.titleKey)}</p>
              <p className="text-sm text-muted-foreground">{t(policyBlockKeys.descriptionKey)}</p>
            </div>
          </>
        ) : (
          <p className="text-body text-destructive">{error || t("documents:viewer.loadFailed")}</p>
        )}
        {!accessBlocked ? (
          <Button onClick={() => { refetch(); setActionError(null); }}>{t("common:retry")}</Button>
        ) : null}
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

  const effectiveSidebarOpen = ownerKnowledgeRoomId
    ? knowledgeSidebarOpen
    : sidebarOpen;
  const effectiveToggleSidebar = ownerKnowledgeRoomId
    ? () => setKnowledgeSidebarOpen((v) => !v)
    : onToggleSidebar;
  const effectiveSidebar =
    ownerKnowledgeRoomId && knowledgeSidebarOpen && documentId ? (
      <ViewerKnowledgeRail
        roomId={ownerKnowledgeRoomId}
        documentId={documentId}
        onJumpToPage={setPage}
        onClose={() => setKnowledgeSidebarOpen(false)}
      />
    ) : (
      sidebar
    );

  return (
    <div
      className={
        publicToken
          ? "public-viewer-shell flex min-h-0 flex-1 flex-col overflow-hidden"
          : "flex min-h-0 flex-1 flex-col bg-neutral-50 dark:bg-background"
      }
    >
      {publicToken ? (
        <PublicViewerToolbar
          doc={doc}
          page={page}
          totalPages={totalPages}
          zoom={zoom}
          onZoomOut={zoomOut}
          onZoomIn={zoomIn}
          onPreviousPage={goToPreviousPage}
          onNextPage={goToNextPage}
          onDownload={handleDownload}
          sidebarOpen={effectiveSidebarOpen}
          onToggleSidebar={effectiveToggleSidebar}
          linkName={publicLink?.name}
        />
      ) : (
        <ViewerToolbar
          doc={doc}
          page={page}
          totalPages={totalPages}
          zoom={zoom}
          onZoomOut={zoomOut}
          onZoomIn={zoomIn}
          onPreviousPage={goToPreviousPage}
          onNextPage={goToNextPage}
          onDownload={handleDownload}
          sidebarOpen={effectiveSidebarOpen}
          onToggleSidebar={effectiveToggleSidebar}
          sidebarOpenLabel={
            ownerKnowledgeRoomId
              ? t("documents:viewer.knowledgeRailOpen")
              : undefined
          }
          sidebarCloseLabel={
            ownerKnowledgeRoomId
              ? t("documents:viewer.knowledgeRailClose")
              : undefined
          }
        />
      )}
      <ViewerCanvas
        doc={doc}
        page={page}
        zoom={zoom}
        pages={pages}
        analytics={analytics}
        imageUrl={imageUrl}
        evidence={evidence}
        watermark={resolvedWatermark}
        screenshotProtectionEnabled={publicLink?.screenshotProtectionEnabled}
        onSelectPage={setPage}
        onCaptureAttempt={
          publicToken && publicLink?.screenshotProtectionEnabled
            ? (reason) => {
                void api.recordPublicEvent(
                  {
                    event_type: "capture_attempt",
                    public_token: publicToken,
                    visitor_id: publicVisitorId,
                    reason,
                  },
                  publicAccessCredentials,
                );
              }
            : undefined
        }
        sidebar={effectiveSidebar}
        variant={publicToken ? "public" : "default"}
        publicToken={publicToken}
        publicAccessCredentials={publicAccessCredentials}
      />
    </div>
  );
}
