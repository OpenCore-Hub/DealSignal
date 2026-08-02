import { useCallback, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { ChartPie, Plus, Trash } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { EmptyState } from "@/components/common/EmptyState";
import { PageHeader } from "@/components/common/PageHeader";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import type { DDPortfolioViewDetail, DealRoom } from "@/types";

const PACK_FINANCING = "financing_dd_v1";
const PACK_MA = "ma_redflag_v1";

export function DealRoomPortfolioPage() {
  const { t } = useTranslation("dealRooms");
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const navigate = useNavigate();
  const [selectedViewId, setSelectedViewId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [packId, setPackId] = useState(PACK_FINANCING);
  const [selectedRooms, setSelectedRooms] = useState<Set<string>>(new Set());
  const [saving, setSaving] = useState(false);

  const { data, loading, error, refetch } = useAsyncData(async () => {
    let disabled = false;
    let views: Awaited<ReturnType<typeof api.listDDPortfolioViews>>["data"] = [];
    try {
      const viewsRes = await api.listDDPortfolioViews();
      views = viewsRes.data ?? [];
    } catch (e) {
      if (e instanceof ApiError && e.code === "portfolio_disabled") {
        disabled = true;
      } else {
        throw e;
      }
    }
    const roomsRes = await api.getDealRooms();
    return {
      views,
      rooms: roomsRes.data ?? [],
      disabled,
    };
  }, []);

  const detailLoader = useCallback(async () => {
    if (!selectedViewId) return null;
    try {
      return await api.getDDPortfolioView(selectedViewId);
    } catch (e) {
      if (e instanceof ApiError && (e.status === 404 || e.code === "not_found")) {
        return null;
      }
      throw e;
    }
  }, [selectedViewId]);

  const {
    data: detail,
    loading: detailLoading,
    refetch: refetchDetail,
  } = useAsyncData(detailLoader, [selectedViewId]);

  const views = data?.views ?? [];
  const rooms = useMemo(() => data?.rooms ?? [], [data?.rooms]);
  const disabled = data?.disabled ?? false;

  const toggleRoom = (roomId: string) => {
    setSelectedRooms((prev) => {
      const next = new Set(prev);
      if (next.has(roomId)) next.delete(roomId);
      else next.add(roomId);
      return next;
    });
  };

  const handleCreate = async () => {
    if (!name.trim() || selectedRooms.size === 0) {
      toast.error(t("portfolio.createRequired"));
      return;
    }
    setSaving(true);
    try {
      const view = await api.createDDPortfolioView({
        name: name.trim(),
        pack_id: packId,
        room_ids: Array.from(selectedRooms),
      });
      toast.success(t("portfolio.created"));
      setCreating(false);
      setName("");
      setSelectedRooms(new Set());
      setSelectedViewId(view.id);
      await refetch();
      await refetchDetail();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("portfolio.createFailed"));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (viewId: string) => {
    try {
      await api.deleteDDPortfolioView(viewId);
      toast.success(t("portfolio.deleted"));
      if (selectedViewId === viewId) setSelectedViewId(null);
      await refetch();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("portfolio.deleteFailed"));
    }
  };

  const openRoomDiligence = (roomId: string) => {
    navigate(`/${workspaceSlug}/deal-rooms/${roomId}?tab=diligence`);
  };

  if (disabled) {
    return (
      <div className="space-y-4" data-testid="deal-room-portfolio-page">
        <PageHeader title={t("portfolio.title")} description={t("portfolio.disabled")} />
      </div>
    );
  }

  return (
    <div className="space-y-4" data-testid="deal-room-portfolio-page">
      <PageHeader title={t("portfolio.title")} description={t("portfolio.description")}>
        <Button
          type="button"
          onClick={() => setCreating((v) => !v)}
          data-testid="portfolio-create-toggle"
        >
          <Plus size={16} className="mr-1" />
          {creating ? t("portfolio.cancelCreate") : t("portfolio.create")}
        </Button>
      </PageHeader>

      {creating ? (
        <Card data-testid="portfolio-create-form">
          <CardHeader>
            <CardTitle className="text-h3">{t("portfolio.createTitle")}</CardTitle>
            <CardDescription>{t("portfolio.createHint")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="portfolio-name">{t("portfolio.nameLabel")}</Label>
              <Input
                id="portfolio-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t("portfolio.namePlaceholder")}
              />
            </div>
            <div className="space-y-2">
              <Label>{t("portfolio.packLabel")}</Label>
              <Select value={packId} onValueChange={(v) => v && setPackId(v)}>
                <SelectTrigger className="w-[240px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={PACK_FINANCING}>
                    {t("diligence.packs.financing_dd_v1")}
                  </SelectItem>
                  <SelectItem value={PACK_MA}>{t("diligence.packs.ma_redflag_v1")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>{t("portfolio.roomsLabel")}</Label>
              <div className="max-h-56 space-y-2 overflow-y-auto rounded-lg border border-border p-3">
                {rooms.map((room: DealRoom) => (
                  <label
                    key={room.id}
                    className="flex cursor-pointer items-center gap-2 text-sm"
                  >
                    <Checkbox
                      checked={selectedRooms.has(room.id)}
                      onCheckedChange={() => toggleRoom(room.id)}
                    />
                    <span>{room.name}</span>
                  </label>
                ))}
                {rooms.length === 0 ? (
                  <p className="text-sm text-muted-foreground">{t("portfolio.noRooms")}</p>
                ) : null}
              </div>
            </div>
            <Button
              type="button"
              onClick={() => void handleCreate()}
              disabled={saving}
              data-testid="portfolio-create-submit"
            >
              {saving ? t("portfolio.saving") : t("portfolio.createSubmit")}
            </Button>
          </CardContent>
        </Card>
      ) : null}

      {loading ? (
        <p className="text-sm text-muted-foreground">{t("portfolio.loading")}</p>
      ) : error ? (
        <p className="text-sm text-destructive">{error}</p>
      ) : views.length === 0 && !creating ? (
        <EmptyState
          icon={<ChartPie size={40} />}
          title={t("portfolio.emptyTitle")}
          description={t("portfolio.emptyDescription")}
        />
      ) : (
        <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-h3">{t("portfolio.viewsTitle")}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-1">
              {views.map((view) => (
                <div
                  key={view.id}
                  className={`flex items-center justify-between rounded-md px-2 py-2 text-sm ${
                    selectedViewId === view.id ? "bg-muted" : "hover:bg-muted/50"
                  }`}
                >
                  <button
                    type="button"
                    className="flex-1 text-left"
                    onClick={() => setSelectedViewId(view.id)}
                    data-testid={`portfolio-view-${view.id}`}
                  >
                    <p className="font-medium">{view.name}</p>
                    <p className="text-xs text-muted-foreground">
                      {t("portfolio.roomCount", { count: view.room_count })}
                    </p>
                  </button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label={t("portfolio.delete")}
                    onClick={() => void handleDelete(view.id)}
                  >
                    <Trash size={14} />
                  </Button>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card data-testid="portfolio-detail">
            <CardHeader className="pb-2">
              <CardTitle className="text-h3">{t("portfolio.detailTitle")}</CardTitle>
              <CardDescription>{t("portfolio.detailDescription")}</CardDescription>
            </CardHeader>
            <CardContent>
              {!selectedViewId ? (
                <p className="text-sm text-muted-foreground">{t("portfolio.selectView")}</p>
              ) : detailLoading ? (
                <p className="text-sm text-muted-foreground">{t("portfolio.loading")}</p>
              ) : !detail ? (
                <p className="text-sm text-muted-foreground">{t("portfolio.viewMissing")}</p>
              ) : (
                <PortfolioDetail
                  detail={detail}
                  onOpenRoom={openRoomDiligence}
                  t={t}
                />
              )}
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}

function PortfolioDetail({
  detail,
  onOpenRoom,
  t,
}: {
  detail: DDPortfolioViewDetail;
  onOpenRoom: (roomId: string) => void;
  t: (key: string, opts?: Record<string, unknown>) => string;
}) {
  return (
    <div className="space-y-3">
      <p className="text-sm text-muted-foreground">
        {t("portfolio.packLine", {
          pack: t(`diligence.packs.${detail.pack_id}`, { defaultValue: detail.pack_id }),
        })}
      </p>
      {detail.rooms.map((room) => (
        <div
          key={room.deal_room_id}
          className="rounded-lg border border-border p-3"
          data-testid={`portfolio-room-${room.deal_room_id}`}
        >
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div>
              <p className="text-sm font-medium">{room.deal_room_name}</p>
              {room.has_snapshot ? (
                <p className="mt-1 text-xs text-muted-foreground">
                  {t("portfolio.roomSummary", {
                    supported: room.supported,
                    absent: room.absent,
                    insufficient: room.insufficient,
                    total: room.total,
                  })}
                </p>
              ) : (
                <p className="mt-1 text-xs text-muted-foreground">{t("portfolio.noSnapshot")}</p>
              )}
            </div>
            <div className="flex items-center gap-2">
              {room.stale ? <Badge variant="secondary">{t("portfolio.stale")}</Badge> : null}
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => onOpenRoom(room.deal_room_id)}
                data-testid={`portfolio-drilldown-${room.deal_room_id}`}
              >
                {t("portfolio.drillDown")}
              </Button>
            </div>
          </div>
          {room.top_absent && room.top_absent.length > 0 ? (
            <ul className="mt-2 space-y-1">
              {room.top_absent.map((item) => (
                <li key={item.item_id} className="text-xs text-muted-foreground">
                  · {item.label || item.item_id}
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ))}
    </div>
  );
}
