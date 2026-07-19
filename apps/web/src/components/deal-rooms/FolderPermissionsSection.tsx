import { useState } from "react";
import { Link as LinkIcon } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import { DealRoomShareDialog } from "./DealRoomShareDialog";
import { LinkActivityDialog } from "@/components/links/share";
import { RowActions } from "@/components/common/RowActions";
import type { Link } from "@/types";

interface FolderPermissionsSectionProps {
  roomId: string;
}

function formatLastViewed(value?: string): string {
  if (!value) return "—";
  try {
    return new Date(value).toLocaleString();
  } catch {
    return value;
  }
}

function extractLinkToken(shortUrl: string): string {
  return shortUrl.split("/").pop() ?? shortUrl;
}

export function FolderPermissionsSection({ roomId }: FolderPermissionsSectionProps) {
  const { t } = useTranslation("dealRooms");
  const {
    data: links,
    loading,
    refetch,
  } = useAsyncData(async () => {
    const res = await api.getDealRoomLinks(roomId);
    return res.data;
  }, [roomId]);

  const [viewLink, setViewLink] = useState<Link | null>(null);
  const [editLink, setEditLink] = useState<Link | null>(null);
  const [sendCodeLink, setSendCodeLink] = useState<Link | null>(null);
  const [sendCodeEmail, setSendCodeEmail] = useState("");
  const [sendCodeLoading, setSendCodeLoading] = useState(false);
  const [deleteLink, setDeleteLink] = useState<Link | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  const handleActiveChange = async (linkId: string, checked: boolean) => {
    try {
      await api.updateLink(linkId, { status: checked ? "active" : "revoked" });
      await refetch();
    } catch {
      // error toast handled by api client
    }
  };

  const handleSendCode = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!sendCodeLink || !sendCodeEmail.trim()) return;
    setSendCodeLoading(true);
    try {
      const token = extractLinkToken(sendCodeLink.shortUrl);
      await api.sendEmailVerificationCode(token, sendCodeEmail.trim());
      toast.success(t("permissions.links.sendCode.success"));
      setSendCodeLink(null);
      setSendCodeEmail("");
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t("permissions.links.sendCode.error")
      );
    } finally {
      setSendCodeLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteLink) return;
    setDeleteLoading(true);
    try {
      await api.deleteLink(deleteLink.id);
      toast.success(t("permissions.links.delete.success"));
      setDeleteLink(null);
      await refetch();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("permissions.links.delete.error"));
    } finally {
      setDeleteLoading(false);
    }
  };

  const linkList = links ?? [];

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle>{t("permissions.links.title")}</CardTitle>
        {linkList.length > 0 && (
          <DealRoomShareDialog roomId={roomId} onChanged={refetch}>
            <Button size="sm">{t("permissions.links.createLink")}</Button>
          </DealRoomShareDialog>
        )}
      </CardHeader>
      <CardContent>
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
                      <DealRoomShareDialog roomId={roomId} onChanged={refetch}>
                        <Button className="mt-4">{t("permissions.links.createLink")}</Button>
                      </DealRoomShareDialog>
                    </div>
                  </TableCell>
                </TableRow>
              ) : (
                linkList.map((link) => (
                  <TableRow key={link.id} className="cursor-pointer">
                    <TableCell>
                      <DealRoomShareDialog
                        roomId={roomId}
                        linkId={link.id}
                        onChanged={refetch}
                      >
                        <Button variant="link" className="h-auto p-0 font-medium">
                          {link.name || t("permissions.links.table.name")}
                        </Button>
                      </DealRoomShareDialog>
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
                        aria-label={t("permissions.links.table.active")}
                      />
                    </TableCell>
                    <TableCell className="text-right">
                      <RowActions
                        actions={[
                          {
                            label: t("permissions.links.actions.view"),
                            onClick: () => setViewLink(link),
                          },
                          {
                            label: t("permissions.links.actions.edit"),
                            onClick: () => setEditLink(link),
                          },
                          ...(link.requireEmailVerification
                            ? [
                                {
                                  label: t("permissions.links.actions.sendCode"),
                                  onClick: () => {
                                    setSendCodeLink(link);
                                    setSendCodeEmail("");
                                  },
                                },
                              ]
                            : []),
                          {
                            label: t("permissions.links.actions.delete"),
                            destructive: true,
                            onClick: () => setDeleteLink(link),
                          },
                        ]}
                      />
                    </TableCell>
                  </TableRow>
                ))
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
          onChanged={refetch}
          onOpenChange={(open) => !open && setEditLink(null)}
        />
      )}

      <Dialog open={!!sendCodeLink} onOpenChange={(open) => !open && setSendCodeLink(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("permissions.links.sendCode.title")}</DialogTitle>
            <DialogDescription>
              {t("permissions.links.sendCode.description")}
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleSendCode} className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="send-code-email">{t("permissions.links.sendCode.emailLabel")}</Label>
              <Input
                id="send-code-email"
                type="email"
                placeholder={t("permissions.links.sendCode.emailPlaceholder")}
                value={sendCodeEmail}
                onChange={(e) => setSendCodeEmail(e.target.value)}
                required
                disabled={sendCodeLoading}
              />
            </div>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setSendCodeLink(null)}
                disabled={sendCodeLoading}
              >
                {t("common:cancel")}
              </Button>
              <Button type="submit" disabled={sendCodeLoading || !sendCodeEmail.trim()}>
                {sendCodeLoading ? t("permissions.links.sendCode.sending") : t("permissions.links.sendCode.send")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

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
