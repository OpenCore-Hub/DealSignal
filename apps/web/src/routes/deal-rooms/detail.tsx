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
import { api, formatRelativeTime } from "@/lib/api";
import type { DealRoom, DealRoomTemplate } from "@/types";

export function DealRoomDetailPage() {
  const { workspaceSlug, roomId } = useParams<{ workspaceSlug: string; roomId: string }>();
  const [room, setRoom] = useState<DealRoom | null>(null);
  const [templates, setTemplates] = useState<DealRoomTemplate[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    const id = roomId;
    if (!id) return;
    async function load() {
      try {
        setLoading(true);
        const [r, t] = await Promise.all([api.getDealRoomById(id!), api.getDealRoomTemplates()]);
        if (!cancelled) {
          setRoom(r);
          setTemplates(t.data);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [roomId]);

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

  if (loading || !room) {
    return <SkeletonDetail />;
  }

  return (
    <div className="space-y-6">
      <BackButton to={`/${workspaceSlug}/deal-rooms`} label="返回 Deal Rooms" />

      <PageHeader title={room.name} description={room.description}>
        <Button variant="outline" className="gap-1.5" onClick={() => {}}>
          <Envelope size={16} />
          邀请成员
        </Button>
        <Button className="gap-1.5" onClick={() => {}}>
          <FileText size={16} />
          管理文档
        </Button>
      </PageHeader>

      <DetailLayout
        sidebar={
          <div className="space-y-4">
            <StatCard label="文档" value={room.documentCount} icon={<FileText size={18} />} />
            <StatCard label="成员" value={room.memberCount} icon={<Users size={18} />} />
            <Card>
              <CardHeader>
                <CardTitle className="text-h3">安全</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap gap-2">
                  {room.ndaEnabled ? (
                    <Badge variant="destructive" className="gap-1">
                      <Lock size={12} />
                      NDA 已启用
                    </Badge>
                  ) : (
                    <Badge variant="secondary">无需 NDA</Badge>
                  )}
                </div>
                <p className="mt-3 text-caption text-muted-foreground">
                  创建于 {formatRelativeTime(room.createdAt)}
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
                文件夹结构
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
                <p className="text-body text-muted-foreground">未匹配到模板结构。</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-h2 flex items-center gap-2">
                <Check size={20} />
                推荐文件清单
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">完成度</span>
                <span className="font-medium">{completion}%</span>
              </div>
              <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                <div
                  className="h-full rounded-full bg-success-500 transition-all"
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
                        已上传
                      </Badge>
                    ) : (
                      <Button size="sm" variant="ghost" className="gap-1" onClick={() => {}}>
                        <UploadSimple size={14} />
                        上传
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
