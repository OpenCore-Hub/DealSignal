import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import { motion, AnimatePresence } from "motion/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { PageHeader } from "@/components/common/PageHeader";
import { SkeletonDetail } from "@/components/common/SkeletonLayout";
import { api } from "@/lib/api";
import { useTranslation } from "react-i18next";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { toast } from "sonner";
import { DealRoomDocumentsDialog } from "@/components/deal-rooms/DealRoomDocumentsDialog";
import { DealRoomFolderTree } from "@/components/deal-rooms/DealRoomFolderTree";
import {
  DEAL_ROOM_PAGE_TAB_LABEL_KEY,
  isDealRoomPageTab,
  orderDealRoomPageTabs,
  useDealRoomTab,
} from "@/hooks/useDealRoomTab";
import type { DealRoomTab } from "@/hooks/useDealRoomTab";
import { FolderPermissionsSection } from "@/components/deal-rooms/FolderPermissionsSection";
import { DealRoomAccessControlTab } from "@/components/deal-rooms/DealRoomAccessControlTab";
import { DealRoomAnalyticsTab } from "@/components/deal-rooms/DealRoomAnalyticsTab";
import { DealRoomQATab } from "@/components/deal-rooms/DealRoomQATab";
import { DealRoomDocumentsHome } from "@/components/deal-rooms/DealRoomDocumentsHome";
import { DealRoomActivityTab } from "@/components/deal-rooms/DealRoomActivityTab";
import { DealRoomSettingsTab } from "@/components/deal-rooms/DealRoomSettingsTab";
import { DealRoomKnowledgeTab } from "@/components/deal-rooms/DealRoomKnowledgeTab";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  useDealRoomNavSignals,
  fetchDealRoomLinks,
  isLinkActive,
} from "@/hooks/useDealRoomNavSignals";
import { useDealRoomNavStore } from "@/stores/dealRoomNavStore";
import { useUIStore } from "@/stores/uiStore";
import { matchesRecommendedFile } from "@/lib/dealRoomReadiness";
import {
  UploadCancelledError,
  useDocumentUploadConflict,
} from "@/hooks/useDocumentUploadConflict";
import type { DealRoomFolderDocs, Link } from "@/types";

interface UploadProgressItem {
  id: string;
  fileName: string;
  folderPath: string;
  folderName: string;
  documentId?: string;
  status: "pending" | "uploading" | "processing" | "done" | "error";
  progress: number;
  error?: string;
}

const tabTransition = {
  initial: { opacity: 0, x: 8 },
  animate: { opacity: 1, x: 0 },
  exit: { opacity: 0, x: -8 },
  transition: { duration: 0.25, ease: [0.16, 1, 0.3, 1] as const },
};

const pageTransition = {
  initial: { opacity: 0, y: 12 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: 0.4, ease: [0.16, 1, 0.3, 1] as const },
};

export function DealRoomDetailPage() {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const { t: td } = useTranslation("documents");
  const { uploadDocument, conflictDialog } = useDocumentUploadConflict();
  const { workspaceSlug, roomId } = useParams<{ workspaceSlug: string; roomId: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const shouldOpenDocuments = searchParams.get("addDocuments") === "1";
  const [documentsDialogOpen, setDocumentsDialogOpen] = useState(shouldOpenDocuments);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const uploadTargetFolderRef = useRef<string | null>(null);
  const activeIntervalsRef = useRef<Set<ReturnType<typeof setInterval>>>(new Set());
  const activePollsRef = useRef<Map<string, ReturnType<typeof setInterval>>>(new Map());
  const [uploadItems, setUploadItems] = useState<UploadProgressItem[]>([]);
  const [linksRevision, setLinksRevision] = useState(0);
  const [roomLinks, setRoomLinks] = useState<Link[]>([]);
  const [descriptionExpanded, setDescriptionExpanded] = useState(false);
  const [accessLinkId, setAccessLinkId] = useState<string | undefined>();
  const { tab, setTab } = useDealRoomTab();
  const reducedMotion = useReducedMotion();
  const setBreadcrumbTail = useUIStore((state) => state.setBreadcrumbTail);
  const navSignals = useDealRoomNavStore();
  useDealRoomNavSignals(roomId, linksRevision);
  const bumpLinksRevision = useCallback(() => {
    setLinksRevision((n) => n + 1);
  }, []);

  // Cleanup all progress intervals and document-status polls on unmount to
  // prevent state updates on an unmounted component.
  useEffect(() => {
    const intervals = activeIntervalsRef.current;
    const polls = activePollsRef.current;
    return () => {
      for (const id of intervals) {
        clearInterval(id);
      }
      for (const poll of polls.values()) {
        clearInterval(poll);
      }
    };
  }, []);

  // Hide the main scrollbar on the deal room detail page.
  useEffect(() => {
    const main = document.querySelector("main");
    if (main) {
      main.classList.add("scrollbar-hide");
      return () => {
        main.classList.remove("scrollbar-hide");
      };
    }
  }, []);

  // Auto-open documents dialog from query param and reset selected folder when tab changes.
  useEffect(() => {
    if (shouldOpenDocuments) {
      setDocumentsDialogOpen(true);
      const next = new URLSearchParams(searchParams);
      next.delete("addDocuments");
      setSearchParams(next, { replace: true });
    }
  }, [shouldOpenDocuments, searchParams, setSearchParams]);

  const fetchRoom = useCallback(async () => {
    if (!roomId) {
      throw new Error(t("detail.notFound"));
    }
    const [r, tRes, docsRes] = await Promise.all([
      api.getDealRoomById(roomId),
      api.getDealRoomTemplates(),
      api.getDocuments(),
    ]);
    return { room: r, templates: tRes.data, workspaceDocs: docsRes.data };
  }, [roomId, t]);

  const { data, loading, error, refetch } = useAsyncData(fetchRoom, [roomId]);

  const room = data?.room ?? null;

  useEffect(() => {
    if (!roomId) {
      setRoomLinks([]);
      return;
    }
    let cancelled = false;
    void fetchDealRoomLinks(roomId)
      .then((links) => {
        if (!cancelled) setRoomLinks(links);
      })
      .catch(() => {
        if (!cancelled) setRoomLinks([]);
      });
    return () => {
      cancelled = true;
    };
  }, [roomId, linksRevision]);

  // Append room name after the shared workspace nav breadcrumbs (Home >> Deal rooms).
  useEffect(() => {
    if (!room?.name) {
      setBreadcrumbTail(null);
      return;
    }
    setBreadcrumbTail({ label: room.name });
    return () => setBreadcrumbTail(null);
  }, [room?.name, setBreadcrumbTail]);

  const allRoomDocuments = useMemo(
    () => (room?.documents ?? []).flatMap((fd: DealRoomFolderDocs) => fd.documents),
    [room]
  );

  const folderByPath = useMemo(() => {
    const map = new Map<string, string>();
    for (const folder of room?.folders ?? []) {
      map.set(folder.path, folder.name);
    }
    return map;
  }, [room?.folders]);

  // Default target folder for move dialog.

  const stopPolling = useCallback((itemId: string) => {
    const poll = activePollsRef.current.get(itemId);
    if (poll) {
      clearInterval(poll);
      activePollsRef.current.delete(itemId);
    }
  }, []);

  const pollDocumentStatus = useCallback(
    (itemId: string, documentId: string) => {
      let consecutiveErrors = 0;
      const check = async () => {
        try {
          const doc = await api.getDocumentById(documentId);
          consecutiveErrors = 0;
          setUploadItems((prev) =>
            prev.map((item) => {
              if (item.id !== itemId) return item;
              if (doc.status === "ready") {
                return { ...item, status: "done", progress: doc.progress ?? 100 };
              }
              if (doc.status === "failed") {
                return {
                  ...item,
                  status: "error",
                  progress: doc.progress ?? item.progress,
                  error: doc.ingestionJob?.errorMessage ?? tc("error.saveFailed"),
                };
              }
              return { ...item, status: "processing", progress: doc.progress ?? item.progress };
            })
          );
          if (doc.status === "ready") {
            stopPolling(itemId);
            window.dispatchEvent(new CustomEvent("documents:uploaded"));
            await refetch();
          } else if (doc.status === "failed") {
            stopPolling(itemId);
            toast.error(doc.ingestionJob?.errorMessage ?? tc("error.saveFailed"));
          }
        } catch (e) {
          consecutiveErrors++;
          if (consecutiveErrors >= 3) {
            stopPolling(itemId);
            setUploadItems((prev) =>
              prev.map((item) =>
                item.id === itemId
                  ? { ...item, status: "error", error: e instanceof Error ? e.message : tc("error.saveFailed") }
                  : item
              )
            );
            toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
          }
        }
      };
      check();
      const pollInterval = setInterval(check, 2500);
      activePollsRef.current.set(itemId, pollInterval);
    },
    [refetch, stopPolling, tc]
  );

  const uploadFileToFolder = useCallback(
    async (file: File, folderPath: string, sortOrder?: number) => {
      if (!roomId) return;
      const id = Math.random().toString(36).slice(2);
      const folderName = folderByPath.get(folderPath) ?? folderPath;

      setUploadItems((prev) => [
        ...prev,
        {
          id,
          fileName: file.name,
          folderPath,
          folderName,
          status: "uploading",
          progress: 0,
        },
      ]);

      const interval = setInterval(() => {
        setUploadItems((prev) =>
          prev.map((item) =>
            item.id === id && item.status === "uploading"
              ? { ...item, progress: Math.min(item.progress + Math.random() * 15, 95) }
              : item
          )
        );
      }, 300);

      activeIntervalsRef.current.add(interval);

      try {
        const doc = await uploadDocument(file);
        clearInterval(interval);
        activeIntervalsRef.current.delete(interval);

        const order =
          sortOrder ??
          (room?.documents ?? []).find((fd) => fd.folder === folderPath)?.documents.length ??
          0;
        // Backend AddDocument is idempotent: re-add after replace updates folder
        // placement instead of failing on UNIQUE(room_id, document_id).
        await api.addDealRoomDocument(roomId, {
          document_id: doc.id,
          folder_path: folderPath,
          sort_order: order,
        });

        // HTTP upload + room association succeeded, but the backend may still be
        // processing the document. Show the real backend status instead of jumping
        // straight to "done" so this popup stays in sync with the Documents page.
        setUploadItems((prev) =>
          prev.map((item) =>
            item.id === id
              ? { ...item, documentId: doc.id, status: "processing", progress: doc.progress ?? 95 }
              : item
          )
        );
        pollDocumentStatus(id, doc.id);
        void refetch();
      } catch (e) {
        clearInterval(interval);
        activeIntervalsRef.current.delete(interval);
        const cancelled = e instanceof UploadCancelledError;
        const message = cancelled
          ? td("upload.replaceCancelled")
          : e instanceof Error
            ? e.message
            : tc("error.saveFailed");
        setUploadItems((prev) =>
          prev.map((item) =>
            item.id === id
              ? { ...item, status: "error", progress: 0, error: message }
              : item
          )
        );
        if (!cancelled) {
          toast.error(message);
        }
        // Re-throw so batch callers can stop counting later files / refresh partials.
        throw e;
      } finally {
        if (fileInputRef.current) fileInputRef.current.value = "";
      }
    },
    [roomId, folderByPath, room?.documents, tc, td, pollDocumentStatus, uploadDocument, refetch]
  );

  const resolveTargetFolder = useCallback(
    (fileName: string): { path: string; name: string } => {
      const roomFolders = room?.folders ?? [];
      for (const folder of roomFolders) {
        if (matchesRecommendedFile(fileName, folder.name)) {
          return { path: folder.path, name: folder.name };
        }
      }
      const fallback = roomFolders[0];
      if (fallback) {
        return { path: fallback.path, name: fallback.name };
      }
      return { path: "/general", name: "General" };
    },
    [room?.folders]
  );

  const handleUploadFiles = useCallback(
    async (files: File[]) => {
      if (files.length === 0) return;
      const override = uploadTargetFolderRef.current;
      uploadTargetFolderRef.current = null;

      // Group by target folder so multi-select gets distinct sort_order ranks.
      const byFolder = new Map<string, File[]>();
      for (const file of files) {
        const path = override ?? resolveTargetFolder(file.name).path;
        const list = byFolder.get(path) ?? [];
        list.push(file);
        byFolder.set(path, list);
      }

      // Sequential uploads so replace/cancel dialogs never race.
      for (const [folderPath, folderFiles] of byFolder) {
        const base =
          (room?.documents ?? []).find((fd) => fd.folder === folderPath)?.documents.length ?? 0;
        for (let index = 0; index < folderFiles.length; index++) {
          try {
            await uploadFileToFolder(folderFiles[index]!, folderPath, base + index);
          } catch {
            // Stop the batch after cancel/error; earlier successes already refreshed.
            return;
          }
        }
      }
    },
    [resolveTargetFolder, uploadFileToFolder, room?.documents]
  );

  const onFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const files = Array.from(e.target.files ?? []);
      e.target.value = "";
      if (files.length > 0) void handleUploadFiles(files);
    },
    [handleUploadFiles]
  );

  const isUploading = uploadItems.some((item) => item.status === "uploading" || item.status === "processing");
  const overallProgress =
    uploadItems.length === 0
      ? 0
      : Math.round(uploadItems.reduce((sum, item) => sum + item.progress, 0) / uploadItems.length);

  // Hide the floating progress bar automatically once everything finishes,
  // keeping the UI minimal and integrated with the page.
  useEffect(() => {
    if (!isUploading && uploadItems.length > 0) {
      const timer = setTimeout(() => {
        for (const poll of activePollsRef.current.values()) {
          clearInterval(poll);
        }
        activePollsRef.current.clear();
        setUploadItems([]);
      }, 1500);
      return () => clearTimeout(timer);
    }
  }, [isUploading, uploadItems.length]);

  const handleFolderCreate = useCallback(
    async (name: string, parentPath?: string) => {
      if (!roomId) return;
      try {
        await api.createDealRoomFolder(roomId, { name, parent_path: parentPath });
        toast.success(t("folders.created", { name }));
        refetch();
      } catch (e) {
        toast.error(e instanceof Error ? e.message : t("folders.createFailed"));
      }
    },
    [roomId, t, refetch]
  );

  const handleFolderRename = useCallback(
    async (path: string, name: string) => {
      if (!roomId) return;
      try {
        await api.renameDealRoomFolder(roomId, path, { name });
        toast.success(t("folders.renamed"));
        refetch();
      } catch (e) {
        toast.error(e instanceof Error ? e.message : t("folders.renameFailed"));
      }
    },
    [roomId, t, refetch]
  );

  const handleFolderDelete = useCallback(
    async (path: string) => {
      if (!roomId) return;
      try {
        await api.deleteDealRoomFolder(roomId, path);
        toast.success(t("folders.deleted"));
        refetch();
      } catch (e) {
        toast.error(e instanceof Error ? e.message : t("folders.deleteFailed"));
      }
    },
    [roomId, t, refetch]
  );

  /** Unlinks the document from this room only — workspace library copy remains. */
  const handleDocumentRemove = useCallback(
    async (docId: string) => {
      if (!roomId) return;
      await api.removeDealRoomDocument(roomId, docId);
    },
    [roomId],
  );

  const handleDocumentsAdd = useCallback(
    async (documentIds: string[], folderPath: string) => {
      if (!roomId) return;
      try {
        let lastOrder = (room?.documents ?? []).find((fd) => fd.folder === folderPath)?.documents.length ?? 0;
        for (const documentId of documentIds) {
          await api.addDealRoomDocument(roomId, {
            document_id: documentId,
            folder_path: folderPath,
            sort_order: lastOrder++,
          });
        }
        toast.success(t("documents.added", { count: documentIds.length }));
        refetch();
      } catch (e) {
        toast.error(e instanceof Error ? e.message : t("documents.addFailed"));
      }
    },
    [roomId, room?.documents, t, refetch]
  );

  // Only show the full loading/error placeholders on the initial load. During
  // background refetches (e.g. after a document finishes processing) we keep the
  // existing room rendered so the page doesn't flash behind the upload overlay.
  if (error && !room) {
    return (
      <div className="space-y-6">
        <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border p-12 text-center">
          <p className="text-muted-foreground">{error}</p>
          <Button onClick={refetch}>{tc("retry")}</Button>
        </div>
      </div>
    );
  }

  if (loading && !room) {
    return <SkeletonDetail />;
  }

  if (!room) {
    return null;
  }

  // Prefer live filtered active links; navSignals can lag until linksRevision bumps.
  const activeLinkCount = Math.max(
    roomLinks.filter(isLinkActive).length,
    navSignals.activeLinkCount,
    room.activeLinkCount ?? 0,
  );
  const description = room.description?.trim() ?? "";
  const descriptionLong = description.length > 120;

  const showPageTabs = isDealRoomPageTab(tab);
  // Plain derivation (not a hook) so it can sit after loading early-returns safely.
  const pageTabs = orderDealRoomPageTabs(tab);

  return (
    <motion.div className="space-y-6" {...(reducedMotion ? {} : pageTransition)}>
      <PageHeader
        title={room.name}
        description={
          description
            ? descriptionExpanded || !descriptionLong
              ? description
              : `${description.slice(0, 120).trimEnd()}…`
            : undefined
        }
      >
        {descriptionLong ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => setDescriptionExpanded((v) => !v)}
          >
            {descriptionExpanded
              ? t("documentsHome.descriptionHide")
              : t("documentsHome.descriptionShow")}
          </Button>
        ) : null}
      </PageHeader>

      {showPageTabs && (
        <Tabs
          value={tab}
          onValueChange={(value) => setTab(value as DealRoomTab)}
          className="gap-0"
        >
          <TabsList
            variant="line"
            className="h-auto w-full justify-start gap-0 overflow-x-auto rounded-none border-b border-border bg-transparent p-0"
            data-testid="deal-room-page-tabs"
          >
            {pageTabs.map((pageTab) => (
              <TabsTrigger
                key={pageTab}
                value={pageTab}
                className="rounded-none px-3 pb-2.5"
                data-testid={`deal-room-page-tab-${pageTab}`}
              >
                {t(DEAL_ROOM_PAGE_TAB_LABEL_KEY[pageTab])}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      )}

      <AnimatePresence mode="wait">
        <motion.div
          key={tab}
          {...(reducedMotion ? {} : tabTransition)}
        >
          {tab === "documents" && (
            <DealRoomDocumentsHome
              activeLinkCount={activeLinkCount}
              failedDeliveries={navSignals.failedDeliveries}
              unreadQuestions={navSignals.unreadQuestions}
              onJumpTab={setTab}
            >
              <Card>
                <CardContent className="pt-6">
                  <DealRoomFolderTree
                    roomId={room.id}
                    folders={room.folders ?? []}
                    folderDocs={room.documents ?? []}
                    workspaceDocuments={data?.workspaceDocs ?? []}
                    roomDocuments={allRoomDocuments}
                    isAdmin={true}
                    onFolderCreate={handleFolderCreate}
                    onFolderRename={handleFolderRename}
                    onFolderDelete={handleFolderDelete}
                    onDocumentRemove={handleDocumentRemove}
                    onDocumentsAdd={handleDocumentsAdd}
                    onFolderUpload={uploadFileToFolder}
                    onChanged={refetch}
                    onDocumentOpen={(docId) =>
                      navigate(`/${workspaceSlug}/documents/${docId}`, {
                        state: {
                          returnTo: location.pathname + location.search,
                          returnLabel: t("detail.back"),
                        },
                      })
                    }
                  />
                </CardContent>
              </Card>
            </DealRoomDocumentsHome>
          )}

          {tab === "access" && (
            <DealRoomAccessControlTab
              roomId={room.id}
              initialLinkId={accessLinkId}
              onChanged={async () => {
                await refetch();
              }}
            />
          )}

          {tab === "links" && (
            <FolderPermissionsSection
              roomId={room.id}
              slug={room.slug}
              refreshKey={linksRevision}
              onLinksChanged={bumpLinksRevision}
              onManageAccess={(linkId) => {
                setAccessLinkId(linkId);
                setTab("access");
              }}
              onSetupAccess={() => {
                setAccessLinkId(undefined);
                setTab("access");
              }}
            />
          )}

          {tab === "knowledge" && <DealRoomKnowledgeTab roomId={room.id} />}

          {tab === "qa" && <DealRoomQATab roomId={room.id} />}

          {tab === "activity" && (
            <DealRoomActivityTab
              recentVisitors={room.recentVisitors}
              links={roomLinks}
              onOpenShare={() => setTab("links")}
              onOpenAnalytics={() => setTab("analytics")}
            />
          )}

          {tab === "analytics" && (
            <DealRoomAnalyticsTab roomId={room.id} links={roomLinks} />
          )}

          {tab === "settings" && (
            <DealRoomSettingsTab
              room={room}
              roomId={room.id}
              activeLinkCount={activeLinkCount}
              onMemberInvited={refetch}
            />
          )}
        </motion.div>
      </AnimatePresence>

      <DealRoomDocumentsDialog
        roomId={room.id}
        folders={room.folders ?? []}
        folderDocs={room.documents ?? []}
        workspaceDocuments={data?.workspaceDocs ?? []}
        onChanged={refetch}
        open={documentsDialogOpen}
        onOpenChange={(open) => setDocumentsDialogOpen(open)}
      />

      {/* Full-screen centered upload progress overlay.
          The deal room page is pushed into the background with a blur. */}
      <AnimatePresence>
        {uploadItems.length > 0 && (
          <motion.div
            data-testid="upload-progress-popup"
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/10 backdrop-blur-sm dark:bg-white/10"
            {...(reducedMotion
              ? {}
              : {
                  initial: { opacity: 0 },
                  animate: { opacity: 1 },
                  exit: { opacity: 0 },
                  transition: { duration: 0.3 },
                })}
          >
            <motion.div
              className="flex w-[calc(100%-2rem)] max-w-md items-center gap-3 rounded-full bg-background/70 px-6 py-4 shadow-none backdrop-blur-xl"
              {...(reducedMotion
                ? {}
                : {
                    initial: { opacity: 0, y: 12, scale: 0.96 },
                    animate: { opacity: 1, y: 0, scale: 1 },
                    exit: { opacity: 0, y: 12, scale: 0.98 },
                    transition: { duration: 0.35, ease: [0.16, 1, 0.3, 1] as const },
                  })}
            >
              <span className="text-sm font-medium tabular-nums text-foreground/80">{overallProgress}%</span>
              <div className="relative h-2 flex-1 overflow-hidden rounded-full bg-foreground/10">
                <motion.div
                  className="absolute inset-y-0 left-0 rounded-full bg-primary"
                  initial={false}
                  animate={{ width: `${overallProgress}%` }}
                  transition={reducedMotion ? { duration: 0 } : { duration: 0.3, ease: "easeOut" }}
                />
                <div className="pointer-events-none absolute inset-0 animate-[shimmer_1.5s_infinite] bg-gradient-to-r from-transparent via-background/60 to-transparent" />
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Hidden file input for toolbar upload (multi-select). */}
      <input
        type="file"
        multiple
        ref={fileInputRef}
        onChange={onFileChange}
        className="hidden"
        accept=".pdf,.docx,.pptx,.xlsx"
        disabled={isUploading}
        data-testid="deal-room-page-upload-input"
      />

      {conflictDialog}
    </motion.div>
  );
}
