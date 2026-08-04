import {
  FilePdf,
  FileDoc,
  FilePpt,
  FileXls,
  FileText,
} from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import type { Document } from "@/types";
import { useTranslation } from "react-i18next";

type FileTypeKey = "fileType.pdf" | "fileType.docx" | "fileType.pptx" | "fileType.xlsx";

const config: Record<
  Document["fileType"],
  { icon: typeof FilePdf; key: FileTypeKey }
> = {
  pdf: { icon: FilePdf, key: "fileType.pdf" },
  docx: { icon: FileDoc, key: "fileType.docx" },
  pptx: { icon: FilePpt, key: "fileType.pptx" },
  xlsx: { icon: FileXls, key: "fileType.xlsx" },
};

const tileTone: Record<Document["fileType"], string> = {
  pdf: "bg-rose-50 text-rose-700 ring-rose-200/70 dark:bg-rose-950/50 dark:text-rose-300 dark:ring-rose-800/60",
  docx: "bg-sky-50 text-sky-700 ring-sky-200/70 dark:bg-sky-950/50 dark:text-sky-300 dark:ring-sky-800/60",
  pptx: "bg-amber-50 text-amber-800 ring-amber-200/70 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-800/60",
  xlsx: "bg-emerald-50 text-emerald-700 ring-emerald-200/70 dark:bg-emerald-950/50 dark:text-emerald-300 dark:ring-emerald-800/60",
};

interface FileTypeIconProps {
  type: Document["fileType"];
  size?: number;
  showLabel?: boolean;
  className?: string;
}

export function FileTypeIcon({
  type,
  size = 20,
  showLabel = false,
  className,
}: FileTypeIconProps) {
  const { t } = useTranslation("common");
  const cfg = config[type as Document["fileType"]] || {
    icon: FileText,
    key: undefined,
  };
  const Icon = cfg.icon;
  const label = cfg.key ? (t(cfg.key) as string) : (type || "file").toUpperCase();
  const tone =
    tileTone[type as Document["fileType"]] ??
    "bg-muted text-muted-foreground ring-border/60";

  if (showLabel) {
    return (
      <div
        className={cn(
          "relative flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-xl ring-1",
          "shadow-[inset_0_1px_0_rgba(255,255,255,0.65)] transition-transform duration-200",
          "dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.06)]",
          tone,
          className,
        )}
        aria-label={label}
      >
        <span className="relative z-10 text-[10px] font-bold tracking-[0.04em]">
          {label}
        </span>
        <span
          aria-hidden
          className="absolute inset-0 bg-[radial-gradient(120%_80%_at_20%_15%,rgba(255,255,255,0.45),transparent_55%)] dark:bg-[radial-gradient(120%_80%_at_20%_15%,rgba(255,255,255,0.08),transparent_55%)]"
        />
      </div>
    );
  }

  return <Icon size={size} className={cn("text-muted-foreground", className)} aria-label={label} />;
}
