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
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import { DealRoomShareDialog } from "./DealRoomShareDialog";
import { LinkActivityDialog } from "@/components/links/share";

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

  const handleActiveChange = async (linkId: string, checked: boolean) => {
    try {
      await api.updateLink(linkId, { status: checked ? "active" : "revoked" });
      await refetch();
    } catch {
      // error toast handled by api client
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
                  {t("permissions.links.table.activity")}
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
                      <LinkActivityDialog link={link}>
                        <Button variant="ghost" size="sm">
                          {t("permissions.links.table.activity")}
                        </Button>
                      </LinkActivityDialog>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  );
}
