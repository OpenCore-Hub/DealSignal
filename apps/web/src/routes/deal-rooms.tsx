import { useEffect, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { Plus, Lock, Folder, MagnifyingGlass } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { PageHeader } from "@/components/common/PageHeader";
import { EmptyState } from "@/components/common/EmptyState";
import { api } from "@/lib/api";
import { formatRelativeTime } from "@/lib/formatters";
import { usageAtCap } from "@/lib/planQuota";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useTranslation } from "react-i18next";
import type { DealRoom } from "@/types";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";

export type { DealRoom };

const PAGE_SIZE = 24;

export function DealRoomsPage() {
  const { t, i18n } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const location = useLocation();
  const { canWrite, loading: accessLoading } = useWorkspaceAccess(workspaceSlug);
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const searchQuery = search.trim();

  useEffect(() => {
    setPage(1);
  }, [search]);

  const { data, loading, error, refetch } = useAsyncData(
    async () => {
      const res = await api.getDealRooms({
        page,
        page_size: PAGE_SIZE,
        q: searchQuery || undefined,
      });
      return res;
    },
    [page, searchQuery],
  );
  const { data: billing } = useAsyncData(() => {
    if (accessLoading || !canWrite) return Promise.resolve(null);
    return api.getBillingInfo().catch(() => null);
  }, [accessLoading, canWrite]);
  const roomsAtCap = billing ? usageAtCap(billing.roomsUsed, billing.roomsLimit) : false;
  const canCreateRoom = canWrite && !roomsAtCap;

  const rooms = data?.data;
  const pagination = data?.pagination;
  const totalPages = Math.max(
    1,
    Math.ceil((pagination?.total ?? rooms?.length ?? 0) / (pagination?.page_size ?? PAGE_SIZE)),
  );

  const navigateToRoom = (roomId: string, opts?: { tab?: string }) => {
    const params = new URLSearchParams();
    if (opts?.tab) params.set("tab", opts.tab);
    const qs = params.toString();
    navigate(`/${workspaceSlug}/deal-rooms/${roomId}${qs ? `?${qs}` : ""}`, {
      state: {
        returnTo: location.pathname + location.search,
        returnLabel: t("detail.back"),
      },
    });
  };

  const handleViewDocuments = (roomId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    navigateToRoom(roomId, { tab: "documents" });
  };

  const handleCardClick = (roomId: string) => {
    navigateToRoom(roomId);
  };

  const isActive = (room: DealRoom) => room.status === "active";

  return (
    <div className="flex h-full flex-col gap-6">
      <PageHeader title={t("page.title")} description={t("page.description")}>
        <div
          className="flex w-full flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-end"
          data-testid="deal-rooms-toolbar"
        >
          <div className="relative min-w-0 flex-1 sm:max-w-xs sm:flex-none sm:w-56">
            <MagnifyingGlass
              size={16}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              type="search"
              placeholder={t("search.placeholder")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9"
              aria-label={t("search.placeholder")}
            />
          </div>
          <div className="flex w-full flex-col items-stretch gap-1 sm:w-auto sm:items-end">
            <Button
              className="w-full shrink-0 gap-1.5 sm:w-auto"
              onClick={() => navigate(`/${workspaceSlug}/deal-rooms/new`)}
              disabled={!canCreateRoom}
              title={
                !canWrite
                  ? tc("error.codes.insufficient_role")
                  : roomsAtCap
                    ? t("page.roomLimitReached")
                    : undefined
              }
              data-testid="deal-rooms-create"
            >
              <Plus size={16} weight="bold" />
              {t("page.create")}
            </Button>
            {roomsAtCap ? (
              <p className="text-caption text-muted-foreground" data-testid="deal-rooms-limit-hint">
                {t("page.roomLimitReached")}
              </p>
            ) : null}
          </div>
        </div>
      </PageHeader>

      {error ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
          <p className="text-body text-muted-foreground">{error}</p>
          <Button onClick={refetch}>{tc("retry")}</Button>
        </div>
      ) : loading ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          <Skeleton className="h-64" />
          <Skeleton className="h-64" />
          <Skeleton className="h-64" />
          <Skeleton className="h-64" />
          <Skeleton className="h-64" />
          <Skeleton className="h-64" />
        </div>
      ) : rooms?.length === 0 && !searchQuery ? (
        <div className="flex min-h-0 flex-1 flex-col">
          <EmptyState
            icon={<Folder size={48} />}
            title={t("empty.title")}
            description={t("empty.description")}
            action={
              canCreateRoom
                ? { label: t("empty.action"), onClick: () => navigate(`/${workspaceSlug}/deal-rooms/new`) }
                : undefined
            }
            size="large"
            className="h-full min-h-[20rem] w-full justify-center"
          />
        </div>
      ) : (
        <>
          <div className="min-h-0 flex-1 overflow-y-auto">
            {(rooms?.length ?? 0) === 0 ? (
              <div className="flex h-full flex-col items-center justify-center gap-2 rounded-lg border border-border bg-card p-12 text-center">
                <p className="text-body text-muted-foreground">{t("filter.noResults")}</p>
                <Button variant="outline" onClick={() => setSearch("")}>
                  {t("filter.clear")}
                </Button>
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
                {(rooms ?? []).map((room) => (
                  <Card
                    key={room.id}
                    role="link"
                    tabIndex={0}
                    className="cursor-pointer transition-colors hover:bg-muted/50"
                    onClick={() => handleCardClick(room.id)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        handleCardClick(room.id);
                      }
                    }}
                  >
                  <CardHeader className="pb-2">
                    <div className="flex items-start justify-between gap-3">
                      <CardTitle className="text-h3 line-clamp-1">{room.name}</CardTitle>
                      <div className="flex shrink-0 items-center gap-1.5">
                        <span
                          className={`h-2 w-2 rounded-full ${
                            isActive(room) ? "bg-emerald-500" : "bg-slate-400"
                          }`}
                          aria-hidden="true"
                        />
                        <span
                          className={`text-caption font-medium ${
                            isActive(room) ? "text-emerald-600" : "text-muted-foreground"
                          }`}
                        >
                          {isActive(room) ? t("status.active") : t("status.inactive")}
                        </span>
                        {room.ndaEnabled && (
                          <Lock size={14} className="ml-1 text-muted-foreground" />
                        )}
                      </div>
                    </div>
                    {(room.tags?.length ?? 0) > 0 && (
                      <div className="mt-2 flex flex-wrap gap-1.5">
                        {room.tags?.map((tag) => (
                          <Badge
                            key={tag}
                            variant="secondary"
                            className="bg-blue-50 text-blue-700 hover:bg-blue-100 dark:bg-blue-950 dark:text-blue-300"
                          >
                            {tag}
                          </Badge>
                        ))}
                      </div>
                    )}
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="space-y-2">
                      <div className="flex items-center justify-between text-body">
                        <span className="text-muted-foreground">{t("stats.documents")}</span>
                        <span className="font-medium tabular-nums">{room.documentCount}</span>
                      </div>
                      <div className="flex items-center justify-between text-body">
                        <span className="text-muted-foreground">{t("stats.views")}</span>
                        <span className="font-medium tabular-nums">{room.viewCount ?? 0}</span>
                      </div>
                      <div className="flex items-center justify-between text-body">
                        <span className="text-muted-foreground">{t("stats.activeLinks")}</span>
                        <span className="font-medium tabular-nums">{room.activeLinkCount ?? 0}</span>
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border pt-3">
                      <p className="text-caption text-muted-foreground">
                        {room.lastAccessedAt
                          ? t("lastAccessed", {
                              time: formatRelativeTime(room.lastAccessedAt, i18n.language),
                            })
                          : t("card.noViewsYet")}
                      </p>
                      <div className="flex shrink-0 flex-wrap items-center gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-8"
                          onClick={(e) => handleViewDocuments(room.id, e)}
                        >
                          {t("card.viewDocuments")}
                        </Button>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
          {pagination && totalPages > 1 && (
            <div className="mt-4 flex items-center justify-between gap-3">
              <p className="text-caption text-muted-foreground">
                {t("page.pagination.pageOf", {
                  page: pagination.page,
                  totalPages,
                })}
              </p>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  {t("page.pagination.prev")}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={!pagination.has_more}
                  onClick={() => setPage((p) => p + 1)}
                >
                  {t("page.pagination.next")}
                </Button>
              </div>
            </div>
          )}
        </div>
      </>)}
    </div>
  );
}
