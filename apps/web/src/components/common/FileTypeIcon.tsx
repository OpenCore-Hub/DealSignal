import {
  FilePdf,
  FileDoc,
  FilePpt,
  FileXls,
  FileText,
} from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import type { Document } from "@/types";

const config: Record<
  Document["fileType"],
  { icon: typeof FilePdf; label: string }
> = {
  pdf: { icon: FilePdf, label: "PDF" },
  docx: { icon: FileDoc, label: "Word" },
  pptx: { icon: FilePpt, label: "PPT" },
  xlsx: { icon: FileXls, label: "Excel" },
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
  const cfg = config[type] || {
    icon: FileText,
    label: type.toUpperCase(),
  };
  const Icon = cfg.icon;

  if (showLabel) {
    return (
      <div
        className={cn(
          "flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-border bg-muted text-muted-foreground",
          className
        )}
        aria-label={cfg.label}
      >
        <span className="text-caption font-bold">{cfg.label}</span>
      </div>
    );
  }

  return <Icon size={size} className={cn("text-muted-foreground", className)} aria-label={cfg.label} />;
}
