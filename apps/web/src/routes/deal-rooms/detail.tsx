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
import { useDealRoomTab } from "@/hooks/useDealRoomTab";
import { DealRoomShareButton } from "@/components/deal-rooms/DealRoomShareButton";
import { FolderPermissionsSection } from "@/components/deal-rooms/FolderPermissionsSection";
import { DealRoomAccessRequestsPanel } from "@/components/deal-rooms/DealRoomAccessRequestsPanel";
import { DealRoomAnalyticsTab } from "@/components/deal-rooms/DealRoomAnalyticsTab";
import { DealRoomQATab } from "@/components/deal-rooms/DealRoomQATab";
import { DealRoomDocumentsHome } from "@/components/deal-rooms/DealRoomDocumentsHome";
import { DealRoomActivityTab } from "@/components/deal-rooms/DealRoomActivityTab";
import { DealRoomSettingsTab } from "@/components/deal-rooms/DealRoomSettingsTab";
import { useDealRoomNavSignals, fetchDealRoomLinks } from "@/hooks/useDealRoomNavSignals";
import { useDealRoomNavStore } from "@/stores/dealRoomNavStore";
import { useUIStore, type BreadcrumbItem } from "@/stores/uiStore";
import { matchesRecommendedFile } from "@/lib/dealRoomReadiness";
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
  const bumpLinksRevision = useCallback(() => {
    setLinksRevision((n) => n + 1);
  }, []);
  const { tab, setTab } = useDealRoomTab();
  const reducedMotion = useReducedMotion();
  const currentWorkspace = useUIStore((state) => state.currentWorkspace);
  const setBreadcrumbs = useUIStore((state) => state.setBreadcrumbs);
  const navSignals = useDealRoomNavStore();
  useDealRoomNavSignals(roomId, linksRevision);

  const workspaceName = currentWorkspace?.name || workspaceSlug;

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

  // Sync page breadcrumb to the global header.
  useEffect(() => {
    if (!workspaceSlug) return;
    const items: BreadcrumbItem[] = [
      { label: workspaceName ?? workspaceSlug, to: `/${workspaceSlug}/dashboard` },
      { label: t("page.title"), to: `/${workspaceSlug}/deal-rooms` },
    ];
    if (room?.name) {
      items.push({ label: room.name });
    }
    setBreadcrumbs(items);
    return () => setBreadcrumbs([]);
  }, [workspaceSlug, workspaceName, room?.name, t, setBreadcrumbs]);

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
    async (file: File, folderPath: string) => {
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
        const doc = await api.uploadDocument(file, undefined, { skipEmbedding: true });
        clearInterval(interval);
        activeIntervalsRef.current.delete(interval);

        const sortOrder = (room?.documents ?? []).find((fd) => fd.folder === folderPath)?.documents.length ?? 0;
        await api.addDealRoomDocument(roomId, {
          document_id: doc.id,
          folder_path: folderPath,
          sort_order: sortOrder,
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
      } catch (e) {
        clearInterval(interval);
        activeIntervalsRef.current.delete(interval);
        setUploadItems((prev) =>
          prev.map((item) =>
            item.id === id
              ? { ...item, status: "error", progress: 0, error: e instanceof Error ? e.message : tc("error.saveFailed") }
              : item
          )
        );
        toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
      } finally {
        if (fileInputRef.current) fileInputRef.current.value = "";
      }
    },
    [roomId, folderByPath, room?.documents, tc, pollDocumentStatus]
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

  const handleUpload = useCallback(
    async (file: File) => {
      const override = uploadTargetFolderRef.current;
      uploadTargetFolderRef.current = null;
      const path = override ?? resolveTargetFolder(file.name).path;
      await uploadFileToFolder(file, path);
    },
    [resolveTargetFolder, uploadFileToFolder]
  );

  const onFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) void handleUpload(file);
    },
    [handleUpload]
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

  const activeLinkCount = navSignals.activeLinkCount || room.activeLinkCount || roomLinks.length;
  const viewCount = navSignals.viewCount || room.viewCount || 0;
  const description = room.description?.trim() ?? "";
  const descriptionLong = description.length > 120;

  return (
    <motion.div className="space-y-6" {...(reducedMotion ? {} : pageTransition)}>
      {tab === "participants" ? (
        <div className="flex flex-wrap items-center justify-end gap-2">
          <DealRoomShareButton
            roomId={room.id}
            slug={room.slug}
            onChanged={bumpLinksRevision}
          />
        </div>
      ) : (
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
          <div className="flex flex-wrap items-center gap-2">
            {descriptionLong && (
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
            )}
            <DealRoomShareButton
              roomId={room.id}
              slug={room.slug}
              onChanged={bumpLinksRevision}
            />
          </div>
        </PageHeader>
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
                    onDocumentsAdd={handleDocumentsAdd}
                    onFolderUpload={uploadFileToFolder}
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

          {tab === "participants" && (
            <div className="grid grid-cols-1 gap-4">
              <DealRoomAccessRequestsPanel roomId={room.id} onChanged={refetch} />
              <FolderPermissionsSection roomId={room.id} refreshKey={linksRevision} />
            </div>
          )}

          {tab === "qa" && <DealRoomQATab />}

          {tab === "activity" && (
            <DealRoomActivityTab
              recentVisitors={room.recentVisitors}
              links={roomLinks}
              onOpenShare={() => setTab("participants")}
              onOpenAnalytics={() => setTab("analytics")}
            />
          )}

          {tab === "analytics" && (
            <DealRoomAnalyticsTab
              roomId={room.id}
              documentCount={room.documentCount}
              viewCount={viewCount}
              activeLinkCount={activeLinkCount}
              recentVisitors={room.recentVisitors}
              links={roomLinks}
            />
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

      {/* Hidden file input for toolbar upload. */}
      <input
        type="file"
        ref={fileInputRef}
        onChange={onFileChange}
        className="hidden"
        accept=".pdf,.docx,.pptx,.xlsx"
        disabled={isUploading}
      />
    </motion.div>
  );
}
