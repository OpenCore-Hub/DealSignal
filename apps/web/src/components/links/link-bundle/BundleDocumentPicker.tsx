import { useMemo } from "react";
import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import {
  MagnifyingGlassIcon,
  CaretUpIcon,
  CaretDownIcon,
  XIcon,
  PlusIcon,
  CheckIcon,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { Document } from "@/types";

const SOURCE_TYPE_META: Record<string, { label: string }> = {
  pdf: { label: "PDF" },
  docx: { label: "DOCX" },
  pptx: { label: "PPTX" },
  xlsx: { label: "XLSX" },
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

  const typeLabel = (sourceType: string) =>
    SOURCE_TYPE_META[sourceType]?.label ?? sourceType.toUpperCase();

  if (loading) {
    return (
      <div className="space-y-8 px-6 py-5 sm:px-7 sm:py-6">
        <div className="space-y-3">
          <Skeleton className="h-3 w-28" />
          <Skeleton className="h-9 w-full" />
          <Skeleton className="h-36 w-full" />
        </div>
        <div className="space-y-3">
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-36 w-full" />
        </div>
      </div>
    );
  }

  if (allDocuments.length === 0) {
    return (
      <div className="px-6 py-12 text-center sm:px-7">
        <p className="font-mono text-[10px] font-medium uppercase tracking-[0.22em] text-muted-foreground/70">
          01
        </p>
        <h3 className="mt-3 text-[1.05rem] font-semibold tracking-[-0.02em]">
          {t("creator.noDocuments")}
        </h3>
        <p className="mt-1.5 text-[13px] text-muted-foreground">
          {t("bundle.documents.selectPrompt")}
        </p>
        <Button
          className="mt-6 h-11 rounded-xl px-6 text-[13px] font-medium"
          onClick={() => navigate(`/${workspaceSlug}/documents/upload`)}
        >
          <PlusIcon size={15} weight="light" className="mr-1.5" />
          {tc("upload")}
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-8 px-6 py-5 sm:px-7 sm:py-6">
      <section className="space-y-3">
        <div className="flex items-baseline justify-between gap-3">
          <h3 className="flex items-baseline gap-2.5">
            <span className="font-mono text-[10px] tracking-[0.16em] text-muted-foreground/45">
              01
            </span>
            <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/75">
              {t("bundle.documents.availableLabel")}
            </span>
            <span className="font-mono text-[10px] tabular-nums text-muted-foreground/50">
              {filtered.length}
            </span>
          </h3>
          {filteredUnselected.length > 0 ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-[11px] font-medium tracking-tight text-muted-foreground"
              onClick={handleSelectFiltered}
            >
              <CheckIcon size={13} weight="light" className="mr-1" />
              {t("bundle.documents.selectAll")}
            </Button>
          ) : null}
        </div>

        <div className="relative">
          <MagnifyingGlassIcon
            size={15}
            weight="light"
            className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
          />
          <Input
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder={t("bundle.documents.searchPlaceholder")}
            className="h-9 rounded-xl border-foreground/10 bg-transparent pl-9 text-[13px] shadow-none"
          />
        </div>

        <div className="h-[12rem] overflow-y-auto">
          {filtered.length === 0 ? (
            <div className="flex h-full items-center justify-center text-[13px] text-muted-foreground">
              {searchQuery ? t("bundle.documents.empty") : t("creator.noDocuments")}
            </div>
          ) : (
            <div className="divide-y divide-foreground/[0.06]">
              {filtered.map((doc) => {
                const isSelected = selectedIds.has(doc.id);
                return (
                  <label
                    key={doc.id}
                    data-testid={`bundle-doc-label-${doc.id}`}
                    className={cn(
                      "group flex cursor-pointer items-center gap-3 px-1 py-2.5 transition-colors",
                      isSelected ? "bg-transparent" : "hover:bg-muted/35",
                    )}
                  >
                    <Checkbox
                      data-testid={`bundle-doc-checkbox-${doc.id}`}
                      checked={isSelected}
                      onCheckedChange={() => onToggle(doc)}
                    />
                    <span className="shrink-0 rounded-md px-1.5 py-0.5 font-mono text-[10px] tracking-[0.14em] text-muted-foreground ring-1 ring-foreground/[0.08]">
                      {typeLabel(doc.sourceType)}
                    </span>
                    <p className="min-w-0 flex-1 truncate text-[13px] font-medium leading-tight">
                      {doc.fileName}
                    </p>
                  </label>
                );
              })}
            </div>
          )}
        </div>
      </section>

      <section className="space-y-3">
        <div className="flex items-baseline justify-between gap-3">
          <h3 className="flex items-baseline gap-2.5">
            <span className="font-mono text-[10px] tracking-[0.16em] text-muted-foreground/45">
              02
            </span>
            <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/75">
              {t("bundle.documents.label")}
            </span>
            <span className="font-mono text-[10px] tabular-nums text-muted-foreground/50">
              {t("bundle.documents.selectedCount", {
                count: selectedDocuments.length,
              })}
            </span>
          </h3>
          {selectedDocuments.length > 0 ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-[11px] font-medium tracking-tight text-muted-foreground hover:text-destructive"
              onClick={handleClearSelected}
            >
              <XIcon size={13} weight="light" className="mr-1" />
              {t("bundle.documents.clearAll")}
            </Button>
          ) : null}
        </div>

        <div className="h-[12rem] overflow-y-auto">
          {selectedDocuments.length === 0 ? (
            <div className="flex h-full flex-col items-center justify-center text-center">
              <p className="text-[13px] font-medium text-muted-foreground">
                {t("bundle.documents.emptySelected")}
              </p>
              <p className="mt-1 text-[12px] text-muted-foreground/80">
                {t("bundle.documents.emptyHint")}
              </p>
            </div>
          ) : (
            <div className="divide-y divide-foreground/[0.06]">
              {selectedDocuments.map((doc, idx) => (
                <div
                  key={doc.id}
                  className="group flex items-center gap-2.5 px-1 py-2.5"
                >
                  <span className="w-5 shrink-0 font-mono text-[10px] tabular-nums text-muted-foreground/50">
                    {String(idx + 1).padStart(2, "0")}
                  </span>
                  <span className="shrink-0 rounded-md px-1.5 py-0.5 font-mono text-[10px] tracking-[0.14em] text-muted-foreground ring-1 ring-foreground/[0.08]">
                    {typeLabel(doc.sourceType)}
                  </span>
                  <p className="min-w-0 flex-1 truncate text-[13px] font-medium leading-tight">
                    {doc.fileName}
                  </p>
                  <div className="flex shrink-0 items-center gap-0.5 opacity-70 transition-opacity group-hover:opacity-100">
                    <button
                      type="button"
                      disabled={idx === 0}
                      onClick={() => onMoveUp(doc.id)}
                      className={cn(
                        "flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
                        idx === 0 && "cursor-not-allowed opacity-30",
                      )}
                      aria-label={t("bundle.documents.moveUp")}
                    >
                      <CaretUpIcon size={14} weight="light" />
                    </button>
                    <button
                      type="button"
                      disabled={idx === selectedDocuments.length - 1}
                      onClick={() => onMoveDown(doc.id)}
                      className={cn(
                        "flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
                        idx === selectedDocuments.length - 1 &&
                          "cursor-not-allowed opacity-30",
                      )}
                      aria-label={t("bundle.documents.moveDown")}
                    >
                      <CaretDownIcon size={14} weight="light" />
                    </button>
                    <button
                      type="button"
                      onClick={() => onRemove(doc.id)}
                      className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
                      aria-label={t("bundle.documents.remove")}
                    >
                      <XIcon size={14} weight="light" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
