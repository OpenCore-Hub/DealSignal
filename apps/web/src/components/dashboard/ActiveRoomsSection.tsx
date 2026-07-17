import { useLocation, useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import {
  FolderOpen,
  Users,
  FileText,
  ArrowRight,
  ChatTeardropText,
} from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/common/EmptyState";
import { formatRelativeTime } from "@/lib/formatters";
import type { DealRoom } from "@/types";

interface ActiveRoomsSectionProps {
  rooms: DealRoom[];
  workspaceSlug: string;
}

export function ActiveRoomsSection({
  rooms,
  workspaceSlug,
}: ActiveRoomsSectionProps) {
  const { t } = useTranslation("dashboard");
  const { t: tCommon } = useTranslation("common");
  const navigate = useNavigate();
  const location = useLocation();

  const activeRooms = rooms
    .filter((room) => room.status === "active")
    .sort((a, b) => {
      const aTime = a.lastAccessedAt ? new Date(a.lastAccessedAt).getTime() : 0;
      const bTime = b.lastAccessedAt ? new Date(b.lastAccessedAt).getTime() : 0;
      return bTime - aTime;
    });

  const RECENTLY_ACTIVE_LIMIT = 3;

  const openRoom = (roomId: string) =>
    navigate(`/${workspaceSlug}/deal-rooms/${roomId}`, {
      state: {
        returnTo: location.pathname + location.search,
        returnLabel: tCommon("back"),
      },
    });

  const heatBarColor = (score: number) => {
    if (score >= 70) return "bg-hot-500";
    if (score >= 40) return "bg-warm-500";
    return "bg-cold-500";
  };

  const statusMeta = (room: DealRoom) => {
    if (room.status === "active") {
      return {
        label: t("room.status.active"),
        dotColor: "bg-emerald-500",
        textColor: "text-emerald-600",
      };
    }
    return {
      label: t("room.status.inactive"),
      dotColor: "bg-slate-400",
      textColor: "text-muted-foreground",
    };
  };

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-body flex items-center gap-2 font-medium text-muted-foreground">
          <FolderOpen size={16} className="text-hot-500" />
          {t("sections.activeRooms")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {activeRooms.length === 0 ? (
          <EmptyState
            size="compact"
            icon={<FolderOpen size={32} />}
            title={t("empty.rooms.title")}
            description={t("empty.rooms.description")}
            action={{
              label: t("empty.rooms.action"),
              onClick: () => navigate(`/${workspaceSlug}/deal-rooms/new`),
            }}
          />
        ) : (
          <>
            <div className="grid grid-cols-1 gap-4">
              {activeRooms.slice(0, RECENTLY_ACTIVE_LIMIT).map((room) => (
                <div
                  key={room.id}
                  role="link"
                  tabIndex={0}
                  aria-label={t("room.enter", { name: room.name })}
                  className="group/card spotlight relative flex cursor-pointer flex-col overflow-hidden rounded-xl border border-border bg-card p-4 shadow-card transition-all hover:-translate-y-0.5 hover:shadow-card-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background pressable"
                  onClick={() => openRoom(room.id)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      openRoom(room.id);
                    }
                  }}
                >
                  <div className="relative z-10 mb-2 flex items-start justify-between gap-3">
                    <h3 className="text-h3 line-clamp-1">{room.name}</h3>
                    {(() => {
                      const { label, dotColor, textColor } = statusMeta(room);
                      return (
                        <div className="flex shrink-0 items-center gap-1.5">
                          <span
                            className={`h-2 w-2 rounded-full ${dotColor}`}
                            aria-hidden="true"
                          />
                          <span className={`text-caption font-medium ${textColor}`}>
                            {label}
                          </span>
                        </div>
                      );
                    })()}
                  </div>

                  {room.pendingApprovals > 0 && (
                    <div className="relative z-10 mb-3 flex flex-wrap items-center gap-2">
                      <Badge variant="outline" className="text-xs text-risk-500 border-risk-500/20 bg-risk-500/5">
                        {t("room.pendingApprovals", { count: room.pendingApprovals })}
                      </Badge>
                    </div>
                  )}

                  <div className="relative z-10 mb-3 flex flex-wrap items-center gap-3 text-caption text-muted-foreground">
                    <span className="flex items-center gap-1">
                      <FileText size={12} />
                      {room.documentCount}
                    </span>
                    <span className="flex items-center gap-1">
                      <Users size={12} />
                      {room.memberCount}
                    </span>
                    {room.visitorCount ? (
                      <span className="flex items-center gap-1">
                        <Users size={12} />
                        {room.visitorCount}
                      </span>
                    ) : null}
                    {room.unreadQuestions !== undefined ? (
                      <span className="flex items-center gap-1">
                        <ChatTeardropText size={12} />
                        {room.unreadQuestions}
                      </span>
                    ) : null}
                    {room.lastAccessedAt && (
                      <span className="ml-auto flex items-center gap-1">
                        {t("room.lastAccessed")}: {formatRelativeTime(room.lastAccessedAt)}
                      </span>
                    )}
                  </div>

                  {room.heatScore !== undefined && (
                    <div className="relative z-10 mt-auto">
                      <div className="h-1 w-full overflow-hidden rounded-full bg-muted">
                        <div
                          className={`h-full rounded-full ${heatBarColor(room.heatScore)}`}
                          style={{ width: `${Math.min(100, Math.max(0, room.heatScore))}%` }}
                        />
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
            {activeRooms.length > RECENTLY_ACTIVE_LIMIT && (
              <div className="mt-4 flex justify-end">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => navigate(`/${workspaceSlug}/deal-rooms`)}
                >
                  {t("room.viewAllWithCount", { count: activeRooms.length })}
                  <ArrowRight size={16} className="ml-1" />
                </Button>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
