import {
  FilePdf,
  FileDoc,
  FilePpt,
  FileXls,
  FileText,
} from "@phosphor-icons/react";
import type { Document } from "@/types";

const config: Record<
  Document["fileType"],
  { icon: typeof FilePdf; color: string; label: string }
> = {
  pdf: { icon: FilePdf, color: "text-red-500", label: "PDF" },
  docx: { icon: FileDoc, color: "text-blue-500", label: "Word" },
  pptx: { icon: FilePpt, color: "text-orange-500", label: "PPT" },
  xlsx: { icon: FileXls, color: "text-green-500", label: "Excel" },
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
    color: "text-muted-foreground",
    label: type.toUpperCase(),
  };
  const Icon = cfg.icon;

  if (showLabel) {
    return (
      <div
        className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-border bg-muted ${cfg.color} ${className || ""}`}
        aria-label={cfg.label}
      >
        <span className="text-caption font-bold">{cfg.label}</span>
      </div>
    );
  }

  return <Icon size={size} className={cfg.color} aria-label={cfg.label} />;
}
