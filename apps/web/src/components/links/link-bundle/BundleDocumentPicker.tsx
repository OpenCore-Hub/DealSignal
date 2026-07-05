import { useMemo } from "react";
import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import {
  MagnifyingGlassIcon,
  FileTextIcon,
  FilePdfIcon,
  FileDocIcon,
  FilePptIcon,
  FileXlsIcon,
  CaretUpIcon,
  CaretDownIcon,
  XIcon,
  PlusIcon,
  CheckIcon,
  ListDashesIcon,
  TrayArrowDownIcon,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { Document } from "@/types";

const SOURCE_TYPE_META: Record<
  string,
  { label: string; icon: typeof FileTextIcon; color: string; bg: string }
> = {
  pdf: {
    label: "PDF",
    icon: FilePdfIcon,
    color: "text-hot-500",
    bg: "bg-hot-100",
  },
  docx: {
    label: "DOCX",
    icon: FileDocIcon,
    color: "text-info-500",
    bg: "bg-info-100",
  },
  pptx: {
    label: "PPTX",
    icon: FilePptIcon,
    color: "text-warning-500",
    bg: "bg-warning-100",
  },
  xlsx: {
    label: "XLSX",
    icon: FileXlsIcon,
    color: "text-success-500",
    bg: "bg-success-100",
  },
};

interface BundleDocumentPickerProps {
  allDocuments: Document[];
  loading: boolean;
  selectedDocuments: Document[];
  selectedIds: Set<string>;
  searchQuery: string;
  onSearchChange: (query: string) => void;
  onToggle: (doc: Document) => void;
  onRemove: (id: string) => void;
  onMoveUp: (id: string) => void;
  onMoveDown: (id: string) => void;
}

export function BundleDocumentPicker({
  allDocuments,
  loading,
  selectedDocuments,
  selectedIds,
  searchQuery,
  onSearchChange,
  onToggle,
  onRemove,
  onMoveUp,
  onMoveDown,
}: BundleDocumentPickerProps) {
  const { t } = useTranslation("links");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();

  const filtered = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return allDocuments;
    return allDocuments.filter(
      (d) =>
        d.title.toLowerCase().includes(q) ||
        d.fileName.toLowerCase().includes(q)
    );
  }, [allDocuments, searchQuery]);

  const filteredUnselected = useMemo(
    () => filtered.filter((d) => !selectedIds.has(d.id)),
    [filtered, selectedIds]
  );

  const handleSelectFiltered = () => {
    filteredUnselected.forEach((doc) => onToggle(doc));
  };

  const handleClearSelected = () => {
    selectedDocuments.forEach((doc) => onRemove(doc.id));
  };

  const getMeta = (sourceType: string) =>
    SOURCE_TYPE_META[sourceType] ?? {
      label: sourceType.toUpperCase(),
      icon: FileTextIcon,
      color: "text-muted-foreground",
      bg: "bg-muted",
    };

  if (loading) {
    return (
      <div className="flex flex-col gap-5">
        <div className="space-y-3 rounded-xl border border-border bg-card p-4 shadow-card">
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-40 w-full" />
        </div>
        <div className="space-y-3 rounded-xl border border-border bg-card p-4 shadow-card">
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-40 w-full" />
        </div>
      </div>
    );
  }

  if (allDocuments.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-border bg-card p-10 text-center shadow-card">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-muted">
          <TrayArrowDownIcon size={24} className="text-muted-foreground" />
        </div>
        <h3 className="mt-4 text-base font-medium">{t("creator.noDocuments")}</h3>
        <p className="mt-1 text-sm text-muted-foreground">
          {t("bundle.documents.selectPrompt")}
        </p>
        <Button
          className="mt-5"
          size="sm"
          onClick={() => navigate(`/${workspaceSlug}/documents/upload`)}
        >
          <PlusIcon size={16} className="mr-1" />
          {tc("upload")}
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-5">
      {/* Available documents */}
      <div className="flex flex-col rounded-xl border border-border bg-card shadow-card">
        <div className="flex items-center justify-between border-b border-border px-4 py-3">
          <div className="flex items-center gap-2">
            <ListDashesIcon size={18} className="text-muted-foreground" />
            <span className="text-sm font-semibold">
              {t("bundle.documents.availableLabel")}
            </span>
            <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
              {filtered.length}
            </span>
          </div>
          {filteredUnselected.length > 0 && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 text-xs"
              onClick={handleSelectFiltered}
            >
              <CheckIcon size={14} className="mr-1" />
              {t("bundle.documents.selectAll")}
            </Button>
          )}
        </div>

        <div className="p-3">
          <div className="relative">
            <MagnifyingGlassIcon
              size={16}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              value={searchQuery}
              onChange={(e) => onSearchChange(e.target.value)}
              placeholder={t("bundle.documents.searchPlaceholder")}
              className="h-9 rounded-lg border-border pl-9 text-sm"
            />
          </div>
        </div>

        <div className="h-[12rem] overflow-y-auto px-3 pb-3">
          {filtered.length === 0 ? (
            <div className="flex h-full flex-col items-center justify-center rounded-lg border border-dashed border-border text-center">
              <p className="text-sm text-muted-foreground">
                {searchQuery ? t("bundle.documents.empty") : t("creator.noDocuments")}
              </p>
            </div>
          ) : (
            <div className="space-y-1">
              {filtered.map((doc) => {
                const isSelected = selectedIds.has(doc.id);
                const meta = getMeta(doc.sourceType);
                const Icon = meta.icon;
                return (
                  <label
                    key={doc.id}
                    data-testid={`bundle-doc-label-${doc.id}`}
                    className={cn(
                      "group flex cursor-pointer items-center gap-3 rounded-lg border-l-2 px-3 py-2 transition-all",
                      isSelected
                        ? "border-l-primary bg-transparent"
                        : "border-l-transparent hover:bg-muted/40"
                    )}
                  >
                    <Checkbox
                      data-testid={`bundle-doc-checkbox-${doc.id}`}
                      checked={isSelected}
                      onCheckedChange={() => onToggle(doc)}
                    />
                    <Icon
                      size={20}
                      className={cn("shrink-0", meta.color)}
                      weight="fill"
                    />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium leading-tight">
                        {doc.fileName}
                      </p>
                    </div>
                  </label>
                );
              })}
            </div>
          )}
        </div>
      </div>

      {/* Selected documents */}
      <div className="flex flex-col rounded-xl border border-border bg-card shadow-card">
        <div className="flex items-center justify-between border-b border-border px-4 py-3">
          <div className="flex items-center gap-2">
            <TrayArrowDownIcon size={18} className="text-muted-foreground" />
            <span className="text-sm font-semibold">
              {t("bundle.documents.label")}
            </span>
            <span
              className={cn(
                "rounded-full px-2 py-0.5 text-xs",
                selectedDocuments.length > 0
                  ? "bg-primary/10 text-primary"
                  : "bg-muted text-muted-foreground"
              )}
            >
              {t("bundle.documents.selectedCount", {
                count: selectedDocuments.length,
              })}
            </span>
          </div>
          {selectedDocuments.length > 0 && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 text-xs text-destructive hover:text-destructive"
              onClick={handleClearSelected}
            >
              <XIcon size={14} className="mr-1" />
              {t("bundle.documents.clearAll")}
            </Button>
          )}
        </div>

        <div className="h-[12rem] overflow-y-auto p-3">
          {selectedDocuments.length === 0 ? (
            <div className="flex h-full flex-col items-center justify-center rounded-lg border border-dashed border-border text-center">
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                <TrayArrowDownIcon size={20} className="text-muted-foreground" />
              </div>
              <p className="mt-3 text-sm font-medium text-muted-foreground">
                {t("bundle.documents.emptySelected")}
              </p>
              <p className="mt-0.5 text-xs text-muted-foreground">
                {t("bundle.documents.emptyHint")}
              </p>
            </div>
          ) : (
            <div className="space-y-1">
              {selectedDocuments.map((doc, idx) => {
                const meta = getMeta(doc.sourceType);
                const Icon = meta.icon;
                return (
                  <div
                    key={doc.id}
                    className="group flex items-center gap-2 rounded-lg px-3 py-2 transition-colors hover:bg-muted/40"
                  >
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-semibold text-muted-foreground">
                      {idx + 1}
                    </span>

                    <Icon
                      size={20}
                      className={cn("shrink-0", meta.color)}
                      weight="fill"
                    />

                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium leading-tight">
                        {doc.fileName}
                      </p>
                    </div>

                    <div className="flex shrink-0 items-center gap-1 opacity-70 transition-opacity group-hover:opacity-100">
                      <button
                        type="button"
                        disabled={idx === 0}
                        onClick={() => onMoveUp(doc.id)}
                        className={cn(
                          "flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
                          idx === 0 && "cursor-not-allowed opacity-30"
                        )}
                        aria-label={t("bundle.documents.moveUp")}
                      >
                        <CaretUpIcon size={14} weight="bold" />
                      </button>
                      <button
                        type="button"
                        disabled={idx === selectedDocuments.length - 1}
                        onClick={() => onMoveDown(doc.id)}
                        className={cn(
                          "flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
                          idx === selectedDocuments.length - 1 &&
                            "cursor-not-allowed opacity-30"
                        )}
                        aria-label={t("bundle.documents.moveDown")}
                      >
                        <CaretDownIcon size={14} weight="bold" />
                      </button>
                      <button
                        type="button"
                        onClick={() => onRemove(doc.id)}
                        className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
                        aria-label={t("bundle.documents.remove")}
                      >
                        <XIcon size={14} weight="bold" />
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
