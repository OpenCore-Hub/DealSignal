import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams, useSearchParams } from "react-router";
import { motion, AnimatePresence } from "motion/react";
import {
  FileText,
  Envelope,
  UploadSimple,
  Plus,
  Warning,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { PageHeader } from "@/components/common/PageHeader";
import { BackButton } from "@/components/common/BackButton";
import { SkeletonDetail } from "@/components/common/SkeletonLayout";
import { api } from "@/lib/api";
import { useTranslation } from "react-i18next";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { toast } from "sonner";
import { InviteMemberDialog } from "@/components/deal-rooms/InviteMemberDialog";
import { MembersCard } from "@/components/deal-rooms/MembersCard";
import { DealRoomDocumentsDialog } from "@/components/deal-rooms/DealRoomDocumentsDialog";
import { DealRoomFolderTree } from "@/components/deal-rooms/DealRoomFolderTree";
import { DealRoomTabs } from "@/components/deal-rooms/DealRoomTabs";
import { useDealRoomTab } from "@/hooks/useDealRoomTab";
import { DealRoomShareButton } from "@/components/deal-rooms/DealRoomShareButton";
import { FolderPermissionsSection } from "@/components/deal-rooms/FolderPermissionsSection";
import { DealRoomAnalyticsTab } from "@/components/deal-rooms/DealRoomAnalyticsTab";
import { DealRoomQATab } from "@/components/deal-rooms/DealRoomQATab";
import type { DealRoomFolderDocs } from "@/types";

function normalizeText(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, " ");
}

function matchesRecommendedFile(documentTitle: string, recommendedName: string): boolean {
  const title = normalizeText(documentTitle);
  const rec = normalizeText(recommendedName);
  if (title.includes(rec)) return true;
  if (rec.includes(title) && title.length > 3) return true;
  const recWords = rec.split(" ").filter(Boolean);
  if (recWords.length > 1) {
    return recWords.every((word) => title.includes(word));
  }
  return false;
}

interface UploadProgressItem {
  id: string;
  fileName: string;
  folderPath: string;
  folderName: string;
  status: "pending" | "uploading" | "done" | "error";
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
  const [searchParams, setSearchParams] = useSearchParams();
  const shouldOpenDocuments = searchParams.get("addDocuments") === "1";
  const [documentsDialogOpen, setDocumentsDialogOpen] = useState(shouldOpenDocuments);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const activeUploadsRef = useRef(0);
  const activeIntervalsRef = useRef<Set<ReturnType<typeof setInterval>>>(new Set());
  const [uploading, setUploading] = useState(false);
  const [uploadItems, setUploadItems] = useState<UploadProgressItem[]>([]);
  const { tab } = useDealRoomTab();
  const reducedMotion = useReducedMotion();

  // Cleanup all progress intervals on unmount to prevent state updates on
  // unmounted component.
  useEffect(() => {
    const intervals = activeIntervalsRef.current;
    return () => {
      for (const id of intervals) {
        clearInterval(id);
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


  const resolveTargetFolder = (fileName: string): { path: string; name: string } => {
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
  };

  const uploadFileToFolder = async (file: File, folderPath: string) => {
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
    activeUploadsRef.current++;
    setUploading(true);

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
      const doc = await api.uploadDocument(file);
      const sortOrder = (room?.documents ?? []).find((fd) => fd.folder === folderPath)?.documents.length ?? 0;
      await api.addDealRoomDocument(roomId, {
        document_id: doc.id,
        folder_path: folderPath,
        sort_order: sortOrder,
      });
      clearInterval(interval);
      activeIntervalsRef.current.delete(interval);
      setUploadItems((prev) =>
        prev.map((item) => (item.id === id ? { ...item, status: "done", progress: 100 } : item))
      );
      toast.success(t("documents.uploadedAndAdded", { title: doc.title }));
      await refetch();
    } catch (e) {
      clearInterval(interval);
      activeIntervalsRef.current.delete(interval);
      setUploadItems((prev) =>
        prev.map((item) =>
          item.id === id
            ? { ...item, status: "error", error: e instanceof Error ? e.message : tc("error.saveFailed") }
            : item
        )
      );
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      activeUploadsRef.current--;
      if (activeUploadsRef.current <= 0) {
        setUploading(false);
      }
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  const handleUpload = async (file: File) => {
    const { path } = resolveTargetFolder(file.name);
    await uploadFileToFolder(file, path);
  };

  const onFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) void handleUpload(file);
  };

  const activeUploads = uploadItems.filter((item) => item.status === "uploading");
  const hasHistory = uploadItems.some((item) => item.status !== "uploading");
  const showUploadDashboard = activeUploads.length > 0 || hasHistory;
  const overallProgress =
    uploadItems.length === 0
      ? 0
      : Math.round(uploadItems.reduce((sum, item) => sum + item.progress, 0) / uploadItems.length);

  const handleFolderCreate = async (name: string, parentPath?: string) => {
    if (!roomId) return;
    try {
      await api.createDealRoomFolder(roomId, { name, parent_path: parentPath });
      toast.success(t("folders.created", { name }));
      refetch();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("folders.createFailed"));
    }
  };

  const handleFolderRename = async (path: string, name: string) => {
    if (!roomId) return;
    try {
      await api.renameDealRoomFolder(roomId, path, { name });
      toast.success(t("folders.renamed"));
      refetch();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("folders.renameFailed"));
    }
  };

  const handleFolderDelete = async (path: string) => {
    if (!roomId) return;
    try {
      await api.deleteDealRoomFolder(roomId, path);
      toast.success(t("folders.deleted"));
      refetch();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("folders.deleteFailed"));
    }
  };

  const handleDocumentsAdd = async (documentIds: string[], folderPath: string) => {
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
  };

  if (error) {
    return (
      <div className="space-y-6">
        <BackButton to={`/${workspaceSlug}/deal-rooms`} label={t("detail.back")} />
        <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border p-12 text-center">
          <p className="text-muted-foreground">{error}</p>
          <Button onClick={refetch}>{tc("retry")}</Button>
        </div>
      </div>
    );
  }

  if (loading || !room) {
    return <SkeletonDetail />;
  }
  return (
    <motion.div className="space-y-6" {...(reducedMotion ? {} : pageTransition)}>
      <BackButton to={`/${workspaceSlug}/deal-rooms`} label={t("detail.back")} />

      <PageHeader title={room.name} description={room.description}>
        <div className="flex flex-wrap items-center gap-2">
          <DealRoomShareButton roomId={room.id} slug={room.slug} />
          <InviteMemberDialog roomId={room.id} onInvited={refetch}>
            <Button variant="outline" className="gap-1.5">
              <Envelope size={16} />
              {t("detail.invite")}
            </Button>
          </InviteMemberDialog>
        </div>
      </PageHeader>

      <DealRoomTabs />

      <AnimatePresence mode="wait">
        <motion.div
          key={tab}
          {...(reducedMotion ? {} : tabTransition)}
        >
          {tab === "documents" && (
            <div className="space-y-4">
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
                  />
                </CardContent>
              </Card>

              {showUploadDashboard && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-h2 flex items-center gap-2">
                      <UploadSimple size={20} />
                      {t("detail.uploadProgress")}
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="flex items-center justify-between text-sm">
                      <span className="text-muted-foreground">{t("detail.completion")}</span>
                      <span className="font-medium">{overallProgress}%</span>
                    </div>
                    <Progress value={overallProgress} className="h-2" />
                    <input
                      type="file"
                      ref={fileInputRef}
                      onChange={onFileChange}
                      className="hidden"
                      accept=".pdf,.docx,.pptx,.xlsx"
                      disabled={uploading}
                    />
                    <ul className="space-y-2">
                      {uploadItems.map((item) => (
                        <li
                          key={item.id}
                          className="flex flex-col gap-2 rounded-md border border-border p-3"
                          data-testid={`upload-item-${item.id}`}
                        >
                          <div className="flex items-center justify-between gap-2">
                            <div className="flex min-w-0 items-center gap-2">
                              <FileText
                                size={16}
                                className={item.status === "done" ? "text-success-500 shrink-0" : "text-muted-foreground shrink-0"}
                              />
                              <span className="truncate text-sm font-medium">{item.fileName}</span>
                            </div>
                            <div className="shrink-0">
                              {item.status === "uploading" && (
                                <Badge variant="outline" className="text-muted-foreground">
                                  {t("detail.uploading")}
                                </Badge>
                              )}
                              {item.status === "done" && (
                                <Badge variant="outline" className="border-success-500/20 text-success-500">
                                  {t("detail.uploaded")}
                                </Badge>
                              )}
                              {item.status === "error" && (
                                <Badge variant="outline" className="border-error/30 gap-1 text-error-500">
                                  <Warning size={12} />
                                  {t("detail.uploadFailed")}
                                </Badge>
                              )}
                            </div>
                          </div>
                          <div className="flex items-center justify-between gap-2 text-caption text-muted-foreground">
                            <span className="truncate">{item.folderName}</span>
                            {item.status === "uploading" && <span>{Math.round(item.progress)}%</span>}
                          </div>
                          {item.status === "uploading" && <Progress value={item.progress} className="h-1" />}
                          {item.status === "error" && item.error && (
                            <p className="text-caption text-error-500 truncate">{item.error}</p>
                          )}
                        </li>
                      ))}
                    </ul>
                    <Button
                      variant="outline"
                      className="w-full gap-1"
                      onClick={() => fileInputRef.current?.click()}
                      disabled={uploading}
                    >
                      <Plus size={16} />
                      {uploading ? t("detail.uploading") : t("detail.uploadAny")}
                    </Button>
                  </CardContent>
                </Card>
              )}
            </div>
          )}

          {tab === "permissions" && (
            <div className="grid grid-cols-1 gap-4 lg:grid-cols-[1fr_320px] lg:items-start">
              <FolderPermissionsSection roomId={room.id} />
              <div className="lg:sticky lg:top-4">
                <MembersCard roomId={room.id} members={room.members ?? []} onChanged={refetch} />
              </div>
            </div>
          )}

          {tab === "analytics" && (
            <DealRoomAnalyticsTab
              documentCount={room.documentCount}
              viewCount={room.viewCount}
              activeLinkCount={room.activeLinkCount}
              recentVisitors={room.recentVisitors}
            />
          )}

          {tab === "qa" && <DealRoomQATab />}
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

      {/* Hidden file input for toolbar upload. */}
      <input
        type="file"
        ref={fileInputRef}
        onChange={onFileChange}
        className="hidden"
        accept=".pdf,.docx,.pptx,.xlsx"
        disabled={uploading}
      />
    </motion.div>
  );
}
