import { useNavigate, useParams } from "react-router";
import { Plus, Lock, Folder } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { PageHeader } from "@/components/common/PageHeader";
import { EmptyState } from "@/components/common/EmptyState";
import { api } from "@/lib/api";
import { formatRelativeTime } from "@/lib/formatters";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { DealRoom } from "@/types";
// DealRoom 类型用于组件内的类型推断
export type { DealRoom };

export function DealRoomsPage() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { data: rooms, loading, error, refetch } = useAsyncData(
    async () => {
      const res = await api.getDealRooms();
      return res.data;
    },
    []
  );

  return (
    <div className="space-y-6">
      <PageHeader title="数据室" description="集中管理尽调资料、LP 报告与销售提案的数据室。">
        <Button className="gap-1.5" onClick={() => navigate(`/${workspaceSlug}/deal-rooms/new`)}>
          <Plus size={16} weight="bold" />
          新建数据室
        </Button>
      </PageHeader>

      {error ? (
        <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
          <p className="text-body text-muted-foreground">{error}</p>
          <Button onClick={refetch}>重试</Button>
        </div>
      ) : loading ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          <Skeleton className="h-40" />
          <Skeleton className="h-40" />
          <Skeleton className="h-40" />
        </div>
      ) : rooms?.length === 0 ? (
        <EmptyState
          icon={<Folder size={48} />}
          title="暂无数据室"
          description="创建第一个数据室，安全地向投资人、LP 或客户展示尽调材料。"
          action={{ label: "新建数据室", onClick: () => navigate(`/${workspaceSlug}/deal-rooms/new`) }}
          size="large"
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {rooms?.map((room) => (
            <Card
              key={room.id}
              role="link"
              tabIndex={0}
              className="cursor-pointer transition-colors hover:bg-muted/50"
              onClick={() => navigate(`/${workspaceSlug}/deal-rooms/${room.id}`)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  navigate(`/${workspaceSlug}/deal-rooms/${room.id}`);
                }
              }}
            >
              <CardHeader className="pb-2">
                <div className="flex items-start justify-between">
                  <CardTitle className="text-h3">{room.name}</CardTitle>
                  {room.ndaEnabled && <Lock size={16} className="text-muted-foreground" />}
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-body text-muted-foreground">{room.description}</p>
                <div className="flex flex-wrap gap-2">
                  <Badge variant="secondary">{room.documentCount} 文档</Badge>
                  <Badge variant="secondary">{room.memberCount} 成员</Badge>
                  {room.pendingApprovals > 0 && (
                    <Badge variant="destructive">{room.pendingApprovals} 待审批</Badge>
                  )}
                </div>
                <p className="text-caption text-muted-foreground">
                  最近访问 {room.lastAccessedAt ? formatRelativeTime(room.lastAccessedAt) : "-"}
                </p>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
