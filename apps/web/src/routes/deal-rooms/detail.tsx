import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams, useNavigate } from "react-router";
import {
  FileText,
  Users,
  Lock,
  Envelope,
  Folder,
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
import { DetailLayout } from "@/components/common/DetailLayout";
import { StatCard } from "@/components/common/StatCard";
import { SkeletonDetail } from "@/components/common/SkeletonLayout";
import { api } from "@/lib/api";
import { formatRelativeTime } from "@/lib/formatters";
import { useTranslation } from "react-i18next";
import { useAsyncData } from "@/hooks/useAsyncData";
import { toast } from "sonner";
import { InviteMemberDialog } from "@/components/deal-rooms/InviteMemberDialog";
import { MembersCard } from "@/components/deal-rooms/MembersCard";
import { AccessRequestsCard } from "@/components/deal-rooms/AccessRequestsCard";
import { DealRoomDocumentsDialog } from "@/components/deal-rooms/DealRoomDocumentsDialog";
import { DealRoomFolderTree } from "@/components/deal-rooms/DealRoomFolderTree";
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

export function DealRoomDetailPage() {
  const { t, i18n } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const { workspaceSlug, roomId } = useParams<{ workspaceSlug: string; roomId: string }>();
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const activeUploadsRef = useRef(0);
  const activeIntervalsRef = useRef<Set<ReturnType<typeof setInterval>>>(new Set());
  const [uploading, setUploading] = useState(false);
  const [uploadItems, setUploadItems] = useState<UploadProgressItem[]>([]);

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
    map.set("/", t("folders.rootName", "Root"));
    return map;
  }, [room?.folders, t]);

  const resolveTargetFolder = (fileName: string): { path: string; name: string } => {
    const roomFolders = room?.folders ?? [];
    for (const folder of roomFolders) {
      if (matchesRecommendedFile(fileName, folder.name)) {
        return { path: folder.path, name: folder.name };
      }
    }
    return { path: "/", name: folderByPath.get("/") ?? "Root" };
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

    // Track the interval so we can clear it on unmount.
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

  const handleFolderUpload = async (file: File, folderPath: string) => {
    await uploadFileToFolder(file, folderPath);
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

  const handleDocumentMove = async (docId: string, folderPath: string) => {
    if (!roomId) return;
    try {
      await api.updateDealRoomDocument(roomId, docId, { folder_path: folderPath });
      toast.success(t("documents.moved"));
      refetch();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("documents.moveFailed"));
    }
  };

  const handleDocumentReorder = async (docId: string, sortOrder: number) => {
    if (!roomId) return;
    try {
      await api.updateDealRoomDocument(roomId, docId, { sort_order: sortOrder });
      refetch();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("documents.reorderFailed"));
    }
  };

  const handleDocumentRemove = async (docId: string) => {
    if (!roomId) return;
    try {
      await api.removeDealRoomDocument(roomId, docId);
      toast.success(t("documents.removed"));
      refetch();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("documents.removeFailed"));
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

  const handleDocumentOpen = (documentId: string) => {
    if (workspaceSlug) {
      navigate(`/${workspaceSlug}/documents/${documentId}`);
    } else {
      window.open(`/viewer/${documentId}`, "_blank", "noopener,noreferrer");
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
    <div className="space-y-6">
      <BackButton to={`/${workspaceSlug}/deal-rooms`} label={t("detail.back")} />

      <PageHeader title={room.name} description={room.description}>
        <InviteMemberDialog roomId={room.id} onInvited={refetch}>
          <Button variant="outline" className="gap-1.5">
            <Envelope size={16} />
            {t("detail.invite")}
          </Button>
        </InviteMemberDialog>
        <DealRoomDocumentsDialog
          roomId={room.id}
          folders={room.folders ?? []}
          folderDocs={room.documents ?? []}
          workspaceDocuments={data?.workspaceDocs ?? []}
          onChanged={refetch}
        >
          <Button className="gap-1.5">
            <FileText size={16} />
            {t("detail.manageDocs")}
          </Button>
        </DealRoomDocumentsDialog>
      </PageHeader>

      <DetailLayout
        sidebar={
          <div className="space-y-4">
            <StatCard label={t("detail.documents")} value={room.documentCount} icon={<FileText size={18} />} />
            <StatCard label={t("detail.members")} value={room.memberCount} icon={<Users size={18} />} />
            <Card>
              <CardHeader>
                <CardTitle className="text-h3">{t("detail.security")}</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap gap-2">
                  {room.ndaEnabled ? (
                    <Badge variant="destructive" className="gap-1">
                      <Lock size={12} />
                      {t("ndaEnabled")}
                    </Badge>
                  ) : (
                    <Badge variant="secondary">{t("noNda")}</Badge>
                  )}
                  {room.requiresApproval && <Badge variant="secondary">{t("approvalRequired")}</Badge>}
                </div>
                <p className="mt-3 text-caption text-muted-foreground">
                  {t("createdAt", { time: formatRelativeTime(room.createdAt, i18n.language) })}
                </p>
              </CardContent>
            </Card>
          </div>
        }
      >
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <Folder size={20} />
                {t("detail.folders")}
              </CardTitle>
            </CardHeader>
            <CardContent>
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
                onDocumentMove={handleDocumentMove}
                onDocumentReorder={handleDocumentReorder}
                onDocumentRemove={handleDocumentRemove}
                onDocumentsAdd={handleDocumentsAdd}
                onDocumentOpen={handleDocumentOpen}
                onFolderUpload={handleFolderUpload}
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
                        <div className="flex items-center gap-2 min-w-0">
                          <FileText
                            size={16}
                            className={item.status === "done" ? "text-success-500 shrink-0" : "text-muted-foreground shrink-0"}
                          />
                          <span className="text-sm font-medium truncate">{item.fileName}</span>
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
                            <Badge variant="outline" className="border-error/30 text-error-500 gap-1">
                              <Warning size={12} />
                              {t("detail.uploadFailed")}
                            </Badge>
                          )}
                        </div>
                      </div>
                      <div className="flex items-center justify-between gap-2 text-caption text-muted-foreground">
                        <span className="truncate">{item.folderName}</span>
                        {item.status === "uploading" && (
                          <span>{Math.round(item.progress)}%</span>
                        )}
                      </div>
                      {item.status === "uploading" && (
                        <Progress value={item.progress} className="h-1" />
                      )}
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

          <MembersCard
            roomId={room.id}
            members={room.members ?? []}
            onChanged={refetch}
          />

          <AccessRequestsCard
            roomId={room.id}
            requests={room.accessRequests ?? []}
            onChanged={refetch}
          />
        </div>
      </DetailLayout>
    </div>
  );
}
