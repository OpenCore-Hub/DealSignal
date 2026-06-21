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
import { useTranslation } from "react-i18next";
import type { DealRoom } from "@/types";

export type { DealRoom };

export function DealRoomsPage() {
  const { t, i18n } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
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
      <PageHeader title={t("page.title")} description={t("page.description")}>
        <Button className="gap-1.5" onClick={() => navigate(`/${workspaceSlug}/deal-rooms/new`)}>
          <Plus size={16} weight="bold" />
          {t("page.create")}
        </Button>
      </PageHeader>

      {error ? (
        <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
          <p className="text-body text-muted-foreground">{error}</p>
          <Button onClick={refetch}>{tc("retry")}</Button>
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
          title={t("empty.title")}
          description={t("empty.description")}
          action={{ label: t("empty.action"), onClick: () => navigate(`/${workspaceSlug}/deal-rooms/new`) }}
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
                  <Badge variant="secondary">{t("documentCount", { count: room.documentCount })}</Badge>
                  <Badge variant="secondary">{t("memberCount", { count: room.memberCount })}</Badge>
                  {room.pendingApprovals > 0 && (
                    <Badge variant="destructive">{t("pendingApprovals", { count: room.pendingApprovals })}</Badge>
                  )}
                </div>
                <p className="text-caption text-muted-foreground">
                  {t("lastAccessed", { time: room.lastAccessedAt ? formatRelativeTime(room.lastAccessedAt, i18n.language) : "-" })}
                </p>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
