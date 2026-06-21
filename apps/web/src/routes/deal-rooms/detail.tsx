import { useEffect, useMemo, useState } from "react";
import { useParams } from "react-router";
import { FileText, Users, Lock, Envelope, Folder, Check, UploadSimple } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { PageHeader } from "@/components/common/PageHeader";
import { BackButton } from "@/components/common/BackButton";
import { DetailLayout } from "@/components/common/DetailLayout";
import { StatCard } from "@/components/common/StatCard";
import { SkeletonDetail } from "@/components/common/SkeletonLayout";
import { api } from "@/lib/api";
import { formatRelativeTime } from "@/lib/formatters";
import { useTranslation } from "react-i18next";
import type { DealRoom, DealRoomTemplate } from "@/types";

export function DealRoomDetailPage() {
  const { t, i18n } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const { workspaceSlug, roomId } = useParams<{ workspaceSlug: string; roomId: string }>();
  const [room, setRoom] = useState<DealRoom | null>(null);
  const [templates, setTemplates] = useState<DealRoomTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryKey, setRetryKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const id = roomId;
    if (!id) return;
    async function load() {
      try {
        setLoading(true);
        setError(null);
        const [r, t] = await Promise.all([api.getDealRoomById(id!), api.getDealRoomTemplates()]);
        if (!cancelled) {
          setRoom(r);
          setTemplates(t.data);
        }
      } catch (e) {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : tc("error.loadFailed"));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [roomId, retryKey, tc]);

  const template = useMemo(
    () => templates.find((t) => t.scenario === room?.template),
    [templates, room]
  );

  const uploaded = useMemo(() => new Set(room?.uploadedFiles ?? []), [room]);
  const checklist = useMemo(
    () =>
      template?.recommendedFiles.map((name) => ({
        name,
        done: uploaded.has(name),
      })) ?? [],
    [template, uploaded]
  );

  const completion = useMemo(
    () => (checklist.length === 0 ? 0 : Math.round((checklist.filter((c) => c.done).length / checklist.length) * 100)),
    [checklist]
  );

  if (error) {
    return (
      <div className="space-y-6">
        <BackButton to={`/${workspaceSlug}/deal-rooms`} label={t("detail.back")} />
        <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border p-12 text-center">
          <p className="text-muted-foreground">{error}</p>
          <Button onClick={() => setRetryKey((k) => k + 1)}>{tc("retry")}</Button>
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
        <Button variant="outline" className="gap-1.5" disabled title={t("detail.inviteDisabled")}>
          <Envelope size={16} />
          {t("detail.invite")}
        </Button>
        <Button className="gap-1.5" disabled title={t("detail.manageDocsDisabled")}>
          <FileText size={16} />
          {t("detail.manageDocs")}
        </Button>
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
              {template ? (
                <ul className="space-y-2">
                  {template.folderStructure.map((folder, idx) => (
                    <li key={idx} className="flex items-start gap-2 rounded-md border border-border p-3">
                      <Folder size={18} className="mt-0.5 text-muted-foreground" />
                      <div>
                        <p className="text-sm font-medium">{folder.name}</p>
                        {folder.description && (
                          <p className="text-caption text-muted-foreground">{folder.description}</p>
                        )}
                      </div>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-body text-muted-foreground">{t("detail.noTemplate")}</p>
              )}
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
              <ul className="space-y-2">
                {checklist.map((item, idx) => (
                  <li
                    key={idx}
                    className={`flex items-center justify-between rounded-md border border-border p-3 ${
                      item.done ? "bg-muted/50" : ""
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <FileText size={16} className={item.done ? "text-success-500" : "text-muted-foreground"} />
                      <span className={item.done ? "line-through text-muted-foreground" : "text-sm font-medium"}>
                        {item.name}
                      </span>
                    </div>
                    {item.done ? (
                      <Badge variant="outline" className="border-success-500/20 text-success-500">
                        {t("detail.uploaded")}
                      </Badge>
                    ) : (
                      <Button size="sm" variant="ghost" className="gap-1" disabled title={t("detail.uploadDisabled")}>
                        <UploadSimple size={14} />
                        {t("detail.upload")}
                      </Button>
                    )}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        </div>
      </DetailLayout>
    </div>
  );
}
