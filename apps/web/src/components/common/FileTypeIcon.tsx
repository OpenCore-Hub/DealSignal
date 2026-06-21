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
  const cfg = config[type] || {
    icon: FileText,
    key: undefined,
  };
  const Icon = cfg.icon;
  const label = cfg.key ? (t(cfg.key) as string) : type.toUpperCase();

  if (showLabel) {
    return (
      <div
        className={cn(
          "flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-border bg-muted text-muted-foreground",
          className
        )}
        aria-label={label}
      >
        <span className="text-caption font-bold">{label}</span>
      </div>
    );
  }

  return <Icon size={size} className={cn("text-muted-foreground", className)} aria-label={label} />;
}
