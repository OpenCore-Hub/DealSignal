import { useTranslation } from "react-i18next";
import { SpinnerGap, CheckCircle, XCircle, Clock } from "@phosphor-icons/react";
import { Badge } from "@/components/ui/badge";
import type { Document } from "@/types";

interface DocumentStatusBadgeProps {
  status: Document["status"];
  progress?: number;
  errorMessage?: string | null;
}

export function DocumentStatusBadge({ status, progress, errorMessage }: DocumentStatusBadgeProps) {
  const { t } = useTranslation(["documents", "common"]);

  switch (status) {
    case "uploading":
      return (
        <Badge variant="secondary" className="gap-1">
          <SpinnerGap className="size-3 animate-spin" />
          {t("documents:status.uploading")}
        </Badge>
      );
    case "processing":
      return (
        <Badge variant="secondary" className="gap-1">
          <SpinnerGap className="size-3 animate-spin" />
          {t("documents:status.processing")}
          {progress !== undefined && progress > 0 && progress < 100 ? ` ${progress}%` : null}
        </Badge>
      );
    case "ready":
      return (
        <Badge variant="outline" className="gap-1 text-emerald-600 border-emerald-200 bg-emerald-50 dark:bg-emerald-950 dark:border-emerald-800 dark:text-emerald-400">
          <CheckCircle className="size-3" />
          {t("documents:status.ready")}
        </Badge>
      );
    case "failed":
      return (
        <Badge variant="destructive" className="gap-1" title={errorMessage ?? undefined}>
          <XCircle className="size-3" />
          {t("documents:status.failed")}
        </Badge>
      );
    default:
      return (
        <Badge variant="secondary" className="gap-1">
          <Clock className="size-3" />
          {t("documents:status.pending")}
        </Badge>
      );
  }
}
