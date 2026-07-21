import { useMemo, useState } from "react";
import {
  ChartLine,
  EnvelopeSimple,
  Link as LinkIcon,
  PencilSimple,
  Trash,
  UserPlus,
} from "@phosphor-icons/react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import { DealRoomShareDialog } from "./DealRoomShareDialog";
import { SendVerificationCodeDialog } from "./SendVerificationCodeDialog";
import { LinkActivityDialog } from "@/components/links/share";
import { RowActions } from "@/components/common/RowActions";
import type { Link } from "@/types";

interface FolderPermissionsSectionProps {
  roomId: string;
  /** Bump to force-reload links after creates from outside this section (e.g. toolbar). */
  refreshKey?: number;
}

function formatLastViewed(value?: string): string {
  if (!value) return "—";
  try {
    return new Date(value).toLocaleString();
  } catch {
    return value;
  }
}

export function FolderPermissionsSection({
  roomId,
  refreshKey = 0,
}: FolderPermissionsSectionProps) {
  const { t } = useTranslation("dealRooms");
  const {
    data: links,
    loading,
    refetch,
  } = useAsyncData(async () => {
    const res = await api.getDealRoomLinks(roomId);
    return res.data;
  }, [roomId, refreshKey]);

  const linkList = links ?? [];
  const linkIdsKey = linkList.map((l) => l.id).join(",");

  const { data: pendingByLinkId, error: pendingError, refetch: refetchPending } = useAsyncData(async () => {
    if (linkList.length === 0) return {} as Record<string, number>;
    const entries = await Promise.all(
      linkList.map(async (link) => {
        const res = await api.getLinkAccessRequests(link.id);
        const pending = (res.data ?? []).filter((r) => r.status === "pending").length;
        return [link.id, pending] as const;
      })
    );
    return Object.fromEntries(entries) as Record<string, number>;
  }, [roomId, refreshKey, linkIdsKey]);

  const [viewLink, setViewLink] = useState<Link | null>(null);
  const [editLink, setEditLink] = useState<Link | null>(null);
  const [approveLink, setApproveLink] = useState<Link | null>(null);
  const [sendCodeLink, setSendCodeLink] = useState<Link | null>(null);
  const [deleteLink, setDeleteLink] = useState<Link | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  const totalPending = useMemo(() => {
    if (!pendingByLinkId) return 0;
    return Object.values(pendingByLinkId).reduce((sum, n) => sum + n, 0);
  }, [pendingByLinkId]);

  const refreshAll = async () => {
    await refetch();
    await refetchPending();
  };

  const handleActiveChange = async (linkId: string, checked: boolean) => {
    try {
      await api.updateLink(linkId, { status: checked ? "active" : "revoked" });
      await refetch();
    } catch {
      // error toast handled by api client
    }
  };

  const handleDelete = async () => {
    if (!deleteLink) return;
    setDeleteLoading(true);
    try {
      await api.deleteLink(deleteLink.id);
      toast.success(t("permissions.links.delete.success"));
      setDeleteLink(null);
      await refreshAll();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("permissions.links.delete.error"));
    } finally {
      setDeleteLoading(false);
    }
  };

  return (
    <Card>
      <CardContent className="pt-6">
        {pendingError ? (
          <div
            className="mb-4 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm"
            role="alert"
            data-testid="deal-room-pending-access-requests-error"
          >
            <p className="text-destructive">{t("permissions.links.pendingRequestsLoadFailed")}</p>
            <Button
              size="sm"
              variant="outline"
              className="mt-2"
              onClick={() => { void refetchPending(); }}
            >
              {t("common:retry")}
            </Button>
          </div>
        ) : totalPending > 0 ? (
          <div
            className="mb-4 rounded-lg border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-sm"
            role="status"
            data-testid="deal-room-pending-access-requests"
          >
            {t("permissions.links.pendingRequestsBanner", { count: totalPending })}
          </div>
        ) : null}
        <div className="overflow-hidden rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-muted-foreground">
                  {t("permissions.links.table.name")}
                </TableHead>
                <TableHead className="text-muted-foreground">
                  {t("permissions.links.table.link")}
                </TableHead>
                <TableHead className="text-muted-foreground">
                  {t("permissions.links.table.views")}
                </TableHead>
                <TableHead className="text-muted-foreground">
                  {t("permissions.links.table.lastViewed")}
                </TableHead>
                <TableHead className="text-right text-muted-foreground">
                  {t("permissions.links.table.active")}
                </TableHead>
                <TableHead className="text-right text-muted-foreground">
                  {t("permissions.links.table.actions")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow className="border-0 hover:bg-transparent">
                  <TableCell colSpan={6} className="py-8 text-center text-sm text-muted-foreground">
                    {t("common:loading")}
                  </TableCell>
                </TableRow>
              ) : linkList.length === 0 ? (
                <TableRow className="border-0 hover:bg-transparent">
                  <TableCell colSpan={6} className="p-0">
                    <div className="flex flex-col items-center justify-center rounded-b-lg bg-muted/30 px-6 py-10 text-center">
                      <LinkIcon size={40} className="mb-3 text-muted-foreground" />
                      <p className="text-body text-muted-foreground">
                        {t("permissions.links.emptyTitle")}
                      </p>
                      <DealRoomShareDialog roomId={roomId} onChanged={refreshAll}>
                        <Button className="mt-4">{t("permissions.links.createLink")}</Button>
                      </DealRoomShareDialog>
                    </div>
                  </TableCell>
                </TableRow>
              ) : (
                linkList.map((link) => {
                  const pendingCount = pendingByLinkId?.[link.id] ?? 0;
                  return (
                  <TableRow
                    key={link.id}
                    className="cursor-pointer"
                    onClick={() => setViewLink(link)}
                    data-testid={`deal-room-link-row-${link.id}`}
                  >
                    <TableCell className="font-medium">
                      <div className="flex items-center gap-2">
                        <span>{link.name || t("permissions.links.table.name")}</span>
                        {pendingCount > 0 ? (
                          <Badge variant="warm">
                            {t("permissions.links.pendingRequestsBadge", { count: pendingCount })}
                          </Badge>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {link.shortUrl.split("/").pop()}
                    </TableCell>
                    <TableCell>{link.accessCount ?? 0}</TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatLastViewed(link.lastViewedAt)}
                    </TableCell>
                    <TableCell className="text-right">
                      <Switch
                        checked={link.isActive ?? false}
                        onCheckedChange={(checked) => handleActiveChange(link.id, checked)}
                        onClick={(e) => e.stopPropagation()}
                        aria-label={t("permissions.links.table.active")}
                      />
                    </TableCell>
                    <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                      <RowActions
                        actions={[
                          {
                            label: t("permissions.links.actions.view"),
                            icon: <ChartLine size={16} />,
                            onClick: () => setViewLink(link),
                          },
                          {
                            label: t("permissions.links.actions.edit"),
                            icon: <PencilSimple size={16} />,
                            onClick: () => setEditLink(link),
                          },
                          ...(pendingCount > 0
                            ? [
                                {
                                  label: t("permissions.links.actions.approveRequests"),
                                  icon: <UserPlus size={16} />,
                                  onClick: () => setApproveLink(link),
                                },
                              ]
                            : []),
                          ...(link.requireEmailVerification
                            ? [
                                {
                                  label: t("permissions.links.actions.sendCode"),
                                  icon: <EnvelopeSimple size={16} />,
                                  onClick: () => setSendCodeLink(link),
                                },
                              ]
                            : []),
                          {
                            label: t("permissions.links.actions.delete"),
                            icon: <Trash size={16} />,
                            destructive: true,
                            onClick: () => setDeleteLink(link),
                          },
                        ]}
                      />
                    </TableCell>
                  </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </div>
      </CardContent>

      {viewLink && (
        <LinkActivityDialog
          link={viewLink}
          open
          onOpenChange={(open) => !open && setViewLink(null)}
        />
      )}

      {editLink && (
        <DealRoomShareDialog
          roomId={roomId}
          linkId={editLink.id}
          open
          onChanged={refreshAll}
          onOpenChange={(open) => !open && setEditLink(null)}
        />
      )}

      {approveLink && (
        <DealRoomShareDialog
          roomId={roomId}
          linkId={approveLink.id}
          defaultTab="access"
          open
          onChanged={refreshAll}
          onOpenChange={(open) => !open && setApproveLink(null)}
        />
      )}

      <SendVerificationCodeDialog
        link={sendCodeLink}
        open={!!sendCodeLink}
        onOpenChange={(open) => !open && setSendCodeLink(null)}
      />

      <Dialog open={!!deleteLink} onOpenChange={(open) => !open && setDeleteLink(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("permissions.links.delete.title")}</DialogTitle>
            <DialogDescription className="break-words">
              {t("permissions.links.delete.description", {
                name: deleteLink?.name || deleteLink?.shortUrl.split("/").pop(),
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteLink(null)}
              disabled={deleteLoading}
            >
              {t("common:cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={deleteLoading}
              onClick={handleDelete}
            >
              {deleteLoading ? t("permissions.links.delete.loading") : t("common:delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
