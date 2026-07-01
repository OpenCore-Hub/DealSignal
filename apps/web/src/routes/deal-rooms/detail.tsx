import { useCallback, useMemo, useRef, useState } from "react";
import { useParams, useNavigate } from "react-router";
import {
  FileText,
  Users,
  Lock,
  Envelope,
  Folder,
  Check,
  UploadSimple,
  Plus,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
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

interface RecommendedFile {
  name: string;
  matchedDocId?: string;
  done: boolean;
  manual: boolean;
}

export function DealRoomDetailPage() {
  const { t, i18n } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const { workspaceSlug, roomId } = useParams<{ workspaceSlug: string; roomId: string }>();
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);

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
  const template = useMemo(
    () => (data?.templates ?? []).find((tmpl) => tmpl.scenario === room?.template),
    [data?.templates, room]
  );

  const allRoomDocuments = useMemo(
    () => (room?.documents ?? []).flatMap((fd: DealRoomFolderDocs) => fd.documents),
    [room]
  );

  const [manualDone, setManualDone] = useState<Set<string>>(new Set());

  const recommendedFiles: RecommendedFile[] = useMemo(() => {
    const list = template?.recommendedFiles ?? [];
    return list.map((name) => {
      const matched = allRoomDocuments.find((d) => matchesRecommendedFile(d.title, name));
      const manual = matched ? false : manualDone.has(name);
      return {
        name,
        matchedDocId: matched?.document_id,
        done: !!matched || manual,
        manual,
      };
    });
  }, [template, allRoomDocuments, manualDone]);

  const completion = useMemo(
    () =>
      recommendedFiles.length === 0
        ? 0
        : Math.round((recommendedFiles.filter((c) => c.done).length / recommendedFiles.length) * 100),
    [recommendedFiles]
  );

  const toggleManualDone = (name: string) => {
    setManualDone((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  };

  const handleUpload = async (file: File) => {
    if (!roomId) return;
    setUploading(true);
    try {
      const doc = await api.uploadDocument(file);
      // Try to match a recommended folder by file name.
      let targetFolder = "/";
      const roomFolders = room?.folders ?? [];
      for (const folder of roomFolders) {
        if (matchesRecommendedFile(doc.title, folder.name)) {
          targetFolder = folder.path;
          break;
        }
      }
      await api.addDealRoomDocument(roomId, { document_id: doc.id, folder_path: targetFolder });
      toast.success(t("documents.uploadedAndAdded", { title: doc.title }));
      refetch();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  const handleFolderUpload = async (file: File, folderPath: string) => {
    if (!roomId) return;
    setUploading(true);
    try {
      const doc = await api.uploadDocument(file);
      await api.addDealRoomDocument(roomId, {
        document_id: doc.id,
        folder_path: folderPath,
        sort_order: (room?.documents ?? []).find((fd) => fd.folder === folderPath)?.documents.length ?? 0,
      });
      toast.success(t("documents.uploadedAndAdded", { title: doc.title }));
      refetch();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tc("error.saveFailed"));
    } finally {
      setUploading(false);
    }
  };

  const onFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) void handleUpload(file);
  };

  const handleFolderCreate = async (name: string, parentPath?: string) => {
    if (!roomId) return;
    await api.createDealRoomFolder(roomId, { name, parent_path: parentPath });
    toast.success(t("folders.created", { name }));
    refetch();
  };

  const handleFolderRename = async (path: string, name: string) => {
    if (!roomId) return;
    await api.renameDealRoomFolder(roomId, path, { name });
    toast.success(t("folders.renamed"));
    refetch();
  };

  const handleFolderDelete = async (path: string) => {
    if (!roomId) return;
    await api.deleteDealRoomFolder(roomId, path);
    toast.success(t("folders.deleted"));
    refetch();
  };

  const handleDocumentMove = async (docId: string, folderPath: string) => {
    if (!roomId) return;
    await api.updateDealRoomDocument(roomId, docId, { folder_path: folderPath });
    toast.success(t("documents.moved"));
    refetch();
  };

  const handleDocumentReorder = async (docId: string, sortOrder: number) => {
    if (!roomId) return;
    await api.updateDealRoomDocument(roomId, docId, { sort_order: sortOrder });
    refetch();
  };

  const handleDocumentRemove = async (docId: string) => {
    if (!roomId) return;
    await api.removeDealRoomDocument(roomId, docId);
    toast.success(t("documents.removed"));
    refetch();
  };

  const handleDocumentsAdd = async (documentIds: string[], folderPath: string) => {
    if (!roomId) return;
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

          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <Check size={20} />
                {t("detail.recommendedFiles")}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">{t("detail.completion")}</span>
                <span className="font-medium">{completion}%</span>
              </div>
              <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                <div
                  className="h-full rounded-full bg-success-500 transition-[width]"
                  style={{ width: `${completion}%` }}
                />
              </div>
              <input
                type="file"
                ref={fileInputRef}
                onChange={onFileChange}
                className="hidden"
                accept=".pdf,.docx,.pptx,.xlsx"
                disabled={uploading}
              />
              <ul className="space-y-2">
                {recommendedFiles.map((item) => (
                  <li
                    key={item.name}
                    className={`flex items-center justify-between rounded-md border border-border p-3 ${
                      item.done ? "bg-muted/50" : ""
                    }`}
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <FileText
                        size={16}
                        className={item.done ? "text-success-500 shrink-0" : "text-muted-foreground shrink-0"}
                      />
                      <span
                        className={
                          item.done ? "line-through text-muted-foreground truncate" : "text-sm font-medium truncate"
                        }
                      >
                        {item.name}
                      </span>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      {item.done ? (
                        <Badge variant="outline" className="border-success-500/20 text-success-500">
                          {t("detail.uploaded")}
                        </Badge>
                      ) : (
                        <>
                          <Button
                            size="sm"
                            variant="ghost"
                            className="gap-1"
                            onClick={() => fileInputRef.current?.click()}
                            disabled={uploading}
                          >
                            <UploadSimple size={14} />
                            {t("detail.upload")}
                          </Button>
                          {!item.matchedDocId && (
                            <div className="flex items-center gap-1">
                              <Checkbox
                                id={`check-${item.name}`}
                                checked={item.manual}
                                onCheckedChange={() => toggleManualDone(item.name)}
                              />
                              <label htmlFor={`check-${item.name}`} className="sr-only">
                                {t("detail.markDone")}
                              </label>
                            </div>
                          )}
                        </>
                      )}
                    </div>
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
