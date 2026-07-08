import { Link as LinkIcon } from "@phosphor-icons/react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useTranslation } from "react-i18next";
import { CreateLinkSheet } from "./CreateLinkSheet";

export function FolderPermissionsSection() {
  const { t } = useTranslation("dealRooms");

  return (
    <Card>
      <CardContent>
        <div className="overflow-hidden rounded-lg bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-muted-foreground">{t("permissions.table.name")}</TableHead>
                <TableHead className="text-muted-foreground">{t("permissions.table.link")}</TableHead>
                <TableHead className="text-muted-foreground">{t("permissions.table.views")}</TableHead>
                <TableHead className="text-muted-foreground">{t("permissions.table.lastViewed")}</TableHead>
                <TableHead className="text-right text-muted-foreground">{t("permissions.table.active")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow className="border-0 hover:bg-transparent">
                <TableCell colSpan={5} className="p-0">
                  <div className="flex flex-col items-center justify-center rounded-b-lg bg-muted/30 px-6 py-10 text-center">
                    <LinkIcon size={40} className="mb-3 text-muted-foreground" />
                    <p className="text-body text-muted-foreground">{t("permissions.emptyLinksTitle")}</p>
                    <CreateLinkSheet>
                      <Button className="mt-4">{t("permissions.createLink")}</Button>
                    </CreateLinkSheet>
                  </div>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  );
}
