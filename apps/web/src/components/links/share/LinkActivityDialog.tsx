import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
} from "@/components/ui/dialog";
import { useAccessLogs } from "./hooks";
import { AnalyticsTab } from "./AnalyticsTab";
import type { Link } from "@/types";

interface LinkActivityDialogProps {
  link: Link;
  children: React.ReactElement;
}

export function LinkActivityDialog({ link, children }: LinkActivityDialogProps) {
  const { t } = useTranslation("linkShare");
  const [open, setOpen] = useState(false);
  const { logs, loading: logsLoading } = useAccessLogs(link.id, open);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {children && <DialogTrigger render={children} />}
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>{t("activity.title")}</DialogTitle>
        </DialogHeader>
        <div className="flex-1 overflow-y-auto py-2">
          {logsLoading ? (
            <div className="py-10 text-center text-sm text-muted-foreground">
              {t("common:loading")}
            </div>
          ) : (
            <AnalyticsTab link={link} logs={logs} />
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            {t("common:close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
