import { useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { Plus, Lock, Folder, MagnifyingGlass, Tag } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import { PageHeader } from "@/components/common/PageHeader";
import { EmptyState } from "@/components/common/EmptyState";
import { api } from "@/lib/api";
import { formatRelativeTime } from "@/lib/formatters";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useTranslation } from "react-i18next";
import type { DealRoom } from "@/types";

export type { DealRoom };

function normalizeSearch(value: string): string {
  return value.toLowerCase().trim();
}

export function DealRoomsPage() {
  const { t, i18n } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [search, setSearch] = useState("");
  const [selectedTag, setSelectedTag] = useState<string>("all");

  const { data: rooms, loading, error, refetch } = useAsyncData(
    async () => {
      const res = await api.getDealRooms();
      return res.data;
    },
    []
  );

  const allTags = useMemo(() => {
    const tags = new Set<string>();
    rooms?.forEach((room) => {
      room.tags?.forEach((tag) => tags.add(tag));
    });
    return Array.from(tags).sort();
  }, [rooms]);

  const filteredRooms = useMemo(() => {
    if (!rooms) return [];
    const query = normalizeSearch(search);
    return rooms.filter((room) => {
      const matchesSearch =
        query.length === 0 ||
        normalizeSearch(room.name).includes(query) ||
        normalizeSearch(room.description).includes(query);
      const matchesTag =
        selectedTag === "all" || room.tags?.includes(selectedTag);
      return matchesSearch && matchesTag;
    });
  }, [rooms, search, selectedTag]);

  const handleAddDocuments = (roomId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    navigate(`/${workspaceSlug}/deal-rooms/${roomId}?addDocuments=1`);
  };

  const handleCardClick = (roomId: string) => {
    navigate(`/${workspaceSlug}/deal-rooms/${roomId}`);
  };

  const isActive = (room: DealRoom) => room.status === "active";

  return (
    <div className="flex h-full flex-col gap-6">
      <PageHeader title={t("page.title")} description={t("page.description")}>
        <Button className="gap-1.5" onClick={() => navigate(`/${workspaceSlug}/deal-rooms/new`)}>
          <Plus size={16} weight="bold" />
          {t("page.create")}
        </Button>
      </PageHeader>

      {error ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
          <p className="text-body text-muted-foreground">{error}</p>
          <Button onClick={refetch}>{tc("retry")}</Button>
        </div>
      ) : loading ? (
        <div className="flex flex-1 flex-col gap-4 overflow-y-auto">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <Skeleton className="h-10 w-full sm:w-80" />
            <Skeleton className="h-10 w-full sm:w-44" />
          </div>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
            <Skeleton className="h-64" />
            <Skeleton className="h-64" />
            <Skeleton className="h-64" />
            <Skeleton className="h-64" />
            <Skeleton className="h-64" />
            <Skeleton className="h-64" />
          </div>
        </div>
      ) : rooms?.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center">
          <EmptyState
            icon={<Folder size={48} />}
            title={t("empty.title")}
            description={t("empty.description")}
            action={{ label: t("empty.action"), onClick: () => navigate(`/${workspaceSlug}/deal-rooms/new`) }}
            size="large"
          />
        </div>
      ) : (
        <>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-end">
            <div className="relative w-full sm:w-44">
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
            <Select value={selectedTag} onValueChange={(value) => setSelectedTag(value ?? "all")}>
              <SelectTrigger
                className="w-full gap-1.5 pl-3 sm:w-44"
                aria-label={t("tags.label")}
              >
                <Tag size={16} className="text-muted-foreground" />
                <span className="line-clamp-1 flex-1 text-left">
                  {selectedTag === "all" ? t("tags.all") : selectedTag}
                </span>
              </SelectTrigger>
              <SelectContent
                side="bottom"
                align="start"
                alignItemWithTrigger={false}
                collisionAvoidance={{ side: "none", align: "none" }}
                className="max-h-60"
              >
                <SelectItem value="all">{t("tags.all")}</SelectItem>
                {allTags.map((tag) => (
                  <SelectItem key={tag} value={tag}>
                    {tag}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto">
            {filteredRooms.length === 0 ? (
              <div className="flex h-full flex-col items-center justify-center gap-2 rounded-lg border border-border bg-card p-12 text-center">
                <p className="text-body text-muted-foreground">{t("filter.noResults")}</p>
                <Button variant="outline" onClick={() => { setSearch(""); setSelectedTag("all"); }}>
                  {t("filter.clear")}
                </Button>
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
                {filteredRooms.map((room) => (
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

                    <div className="flex items-center justify-between border-t border-border pt-3">
                      <p className="text-caption text-muted-foreground">
                        {room.lastAccessedAt
                          ? t("lastAccessed", {
                              time: formatRelativeTime(room.lastAccessedAt, i18n.language),
                            })
                          : t("card.noViewsYet")}
                      </p>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-8"
                        onClick={(e) => handleAddDocuments(room.id, e)}
                      >
                        {t("card.addDocuments")}
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </div>
      </>)}
    </div>
  );
}
