import { useDeferredValue, useEffect, useRef, useState, type ReactNode } from "react";
import { apiErrorMessage } from "@/lib/apiErrors";
import { filterUploadSelection, notifyUploadSelectionFiltered } from "@/lib/uploadFileFilters";
import { useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  flexRender,
  type SortingState,
} from "@tanstack/react-table";
import { CalendarBlank, FilePdf, Link as LinkIcon, MagnifyingGlass, Plus, Download } from "@phosphor-icons/react";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { DocumentFilter } from "@/types";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/common/EmptyState";
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import { SkeletonList } from "@/components/common/SkeletonLayout";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useDocumentDeleteImpact } from "@/hooks/useDocumentDeleteImpact";
import {
  UploadCancelledError,
  useDocumentUploadConflict,
} from "@/hooks/useDocumentUploadConflict";
import { api } from "@/lib/api";
import { LIBRARY_DOCUMENT_CATEGORY } from "@/lib/documentCategory";
import { ApiError } from "@/lib/apiClient";
import { LinksTable } from "@/components/links/LinksTable";
import {
  clearLibraryShareHandoff,
  readLibraryShareHandoff,
} from "@/lib/documentsLibraryShareHandoff";
import {
  documentsCreateLinkPath,
  sanitizeDocumentsLibrarySearchParams,
} from "@/lib/documentsSharePath";
import {
  isLinkCreatedWithin,
  LINK_CREATED_WITHIN_VALUES,
  parseLinkCreatedWithin,
  SHARE_CREATED_WITHIN_PARAM,
  SHARE_SEARCH_PARAM,
  type LinkCreatedWithin,
} from "@/lib/shareLinksFilter";
import { AgreementDocumentCard } from "./AgreementDocumentCard";
import { buildDocumentRows, useDocumentColumns, type DocumentRow } from "./DocumentsColumns";
import { AddToDealRoomDialog } from "./AddToDealRoomDialog";
import { DocumentShareDialog } from "./DocumentShareDialog";
import {
  DOCUMENTS_UPLOADED_EVENT,
  dispatchDocumentsUploaded,
  isDocumentReadyForLibraryShare,
  isLibraryShareableUpload,
  type DocumentsUploadedDetail,
} from "@/lib/documentsUploadedEvent";
import type { HeatLevel } from "@/types";

interface DocumentsTableProps {
  category?: string;
}

function isPdfFile(file: File): boolean {
  return file.name.toLowerCase().endsWith(".pdf") || file.type === "application/pdf";
}

function filterFromTabParam(tab: string | null): DocumentFilter {
  if (tab === "shared" || tab === "archived") return tab;
  return "all";
}

export function DocumentsTable({ category }: DocumentsTableProps) {
  "use no memo";
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const { t } = useTranslation(["documents", "common", "links", "agreementDocuments"]);
  const { canWrite } = useWorkspaceAccess(workspaceSlug);
  const isAgreement = category === "agreement";
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { uploadDocument, conflictDialog } = useDocumentUploadConflict();
  const [isUploading, setIsUploading] = useState(false);

  const openDocument = (documentId: string) => {
    navigate(`/${workspaceSlug}/documents/${documentId}`, {
      state: {
        returnTo: location.pathname + location.search,
        returnLabel: t("documents:detail.back"),
      },
    });
  };
  const [sorting, setSorting] = useState<SortingState>([{ id: "totalViews", desc: true }]);
  const [globalFilter, setGlobalFilter] = useState("");
  const filter = category ? ("all" as DocumentFilter) : filterFromTabParam(searchParams.get("tab"));
  const shareDocumentId = searchParams.get("documentId") ?? undefined;
  const shareDocumentTitle = searchParams.get("documentTitle") ?? undefined;
  const showShareTab = !category && filter === "shared";
  const shareQFromUrl = searchParams.get(SHARE_SEARCH_PARAM) ?? "";
  const shareCreatedWithin = parseLinkCreatedWithin(
    searchParams.get(SHARE_CREATED_WITHIN_PARAM),
  );
  const [shareSearchInput, setShareSearchInput] = useState(shareQFromUrl);
  const deferredShareSearch = useDeferredValue(shareSearchInput);
  // Prevent URL echo from clobbering in-flight keystrokes after we write shareQ.
  const lastWrittenShareQRef = useRef(shareQFromUrl);

  // External URL changes only (back/forward / sanitize) — skip our own writes.
  useEffect(() => {
    if (shareQFromUrl === lastWrittenShareQRef.current) return;
    lastWrittenShareQRef.current = shareQFromUrl;
    setShareSearchInput(shareQFromUrl);
  }, [shareQFromUrl]);

  // Persist settled search to the URL without depending on searchParams identity.
  useEffect(() => {
    if (!showShareTab) return;
    const nextQ = deferredShareSearch.trim();
    if (nextQ === lastWrittenShareQRef.current) return;
    lastWrittenShareQRef.current = nextQ;
    setSearchParams((prev) => {
      const currentQ = prev.get(SHARE_SEARCH_PARAM) ?? "";
      if (nextQ === currentQ) return prev;
      const next = new URLSearchParams(prev);
      if (nextQ) next.set(SHARE_SEARCH_PARAM, nextQ);
      else next.delete(SHARE_SEARCH_PARAM);
      return next;
    }, { replace: true });
  }, [deferredShareSearch, showShareTab, setSearchParams]);

  const patchShareListParams = (
    patch: { shareQ?: string | null; createdWithin?: LinkCreatedWithin | null },
  ) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      if ("shareQ" in patch) {
        const q = patch.shareQ?.trim() ?? "";
        if (q) next.set(SHARE_SEARCH_PARAM, q);
        else next.delete(SHARE_SEARCH_PARAM);
      }
      if ("createdWithin" in patch) {
        const within = patch.createdWithin ?? "all";
        if (within === "all") next.delete(SHARE_CREATED_WITHIN_PARAM);
        else next.set(SHARE_CREATED_WITHIN_PARAM, within);
      }
      return next;
    }, { replace: true });
  };

  const clearShareListFilters = () => {
    setShareSearchInput("");
    lastWrittenShareQRef.current = "";
    patchShareListParams({ shareQ: null, createdWithin: "all" });
  };

  const openUpload = () => {
    if (isAgreement) {
      fileInputRef.current?.click();
      return;
    }
    navigate(
      `/${workspaceSlug}/documents/upload` +
        (category ? `?category=${encodeURIComponent(category)}` : ""),
    );
  };

  const handleAgreementFiles = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const selected = Array.from(e.target.files ?? []);
    const selection = filterUploadSelection(selected);
    e.target.value = "";
    if (!notifyUploadSelectionFiltered(selection, t("documents:upload.blockedFilesSkipped"), toast)) {
      return;
    }
    const picked = selection.files;

    const pdfs = picked.filter(isPdfFile);
    if (pdfs.length < picked.length) {
      toast.error(t("agreementDocuments:page.pdfOnly"));
    }
    if (pdfs.length === 0) return;

    setIsUploading(true);
    try {
      for (const file of pdfs) {
        try {
          await uploadDocument(file, "agreement");
        } catch (err) {
          if (err instanceof UploadCancelledError) return;
          if (err instanceof ApiError && err.code === "agreement_pdf_required") {
            toast.error(t("agreementDocuments:page.pdfOnly"));
            return;
          }
          toast.error(t("common:error.saveFailed"));
          return;
        }
      }
      dispatchDocumentsUploaded();
      toast.success(t("agreementDocuments:page.uploadSuccess"));
    } finally {
      setIsUploading(false);
    }
  };

  const setFilter = (next: DocumentFilter) => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        if (next === "all") {
          params.delete("tab");
        } else {
          params.set("tab", next);
        }
        if (next !== "shared") {
          params.delete("documentId");
          params.delete("documentTitle");
        }
        return params;
      },
      { replace: true },
    );
    if (next !== "all") setGlobalFilter("");
  };

  // Drop unknown ?tab= values and share-only filters on non-Share tabs.
  useEffect(() => {
    if (category) return;
    const sanitized = sanitizeDocumentsLibrarySearchParams(searchParams);
    if (!sanitized) return;
    setSearchParams(sanitized, { replace: true });
  }, [category, searchParams, setSearchParams]);

  const [docToAddToRoom, setDocToAddToRoom] = useState<DocumentRow | null>(null);
  const [docToShare, setDocToShare] = useState<DocumentRow | null>(null);
  /** Upload handoff: open Share only after this document becomes ready. */
  const [pendingShareHandoff, setPendingShareHandoff] = useState<{
    documentId: string;
    documentTitle: string;
  } | null>(null);
  const [docToArchive, setDocToArchive] = useState<DocumentRow | null>(null);
  const archiveImpact = useDocumentDeleteImpact(docToArchive);
  const [isArchiving, setIsArchiving] = useState(false);
  const [docToDelete, setDocToDelete] = useState<DocumentRow | null>(null);
  const deleteImpact = useDocumentDeleteImpact(docToDelete);
  const [isDeleting, setIsDeleting] = useState(false);
  const filters: DocumentFilter[] = ["all", "shared", "archived"];

  const {
    data,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    if (showShareTab) return [] as DocumentRow[];
    // Partition by category truth: library → general; agreements page → agreement.
    const listCategory = category ?? LIBRARY_DOCUMENT_CATEGORY;
    const [docsRes, linksRes] = await Promise.all([
      api.getDocuments(filter, listCategory),
      api.getLinks(),
    ]);
    const rows = buildDocumentRows(docsRes.data, linksRes.data);
    // Defense in depth: Archived tab never shows non-archived rows even if API is stale.
    if (filter === "archived") {
      return rows.filter((row) => row.status === "archived");
    }
    // Library Documents tab has a dedicated Archived tab; agreements keep archived in-list.
    if (!category && filter === "all") {
      return rows.filter((row) => row.status !== "archived");
    }
    return rows;
  }, [filter, category, showShareTab]);

  // Poll for status updates while any document is still being processed.
  useEffect(() => {
    const hasProcessing = data?.some((row) => row.status === "processing" || row.status === "uploading");
    if (!hasProcessing) return;

    const interval = setInterval(() => {
      refetch();
    }, 3000);
    return () => clearInterval(interval);
  }, [data, refetch]);

  const buildShareRow = (detail: DocumentsUploadedDetail): DocumentRow => {
    const category =
      detail.category === "general" ||
      detail.category === "agreement" ||
      detail.category === "deal_room"
        ? detail.category
        : "general";
    return {
      id: detail.documentId,
      title: detail.documentTitle,
      sourceType: "pdf",
      fileName: detail.documentTitle,
      fileType: "pdf",
      fileSize: 0,
      pageCount: 0,
      status: (detail.status as DocumentRow["status"]) || "processing",
      category,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      links: [],
      totalViews: 0,
      heatLevel: "cold" as HeatLevel,
    };
  };

  // Upload page handoff: never open Share while status is still processing/uploading.
  // Root cause: POST /documents returns before ingestion is ready; old code opened the
  // dialog immediately with notReady, right after "Upload now".
  useEffect(() => {
    if (isAgreement || category) return;
    const handoff = readLibraryShareHandoff(searchParams);
    if (!handoff) return;
    setSearchParams(
      (prev) => clearLibraryShareHandoff(prev) ?? prev,
      { replace: true },
    );
    if (isDocumentReadyForLibraryShare(handoff.documentStatus)) {
      setDocToShare(
        buildShareRow({
          documentId: handoff.documentId,
          documentTitle: handoff.documentTitle,
          status: "ready",
          category: "general",
        }),
      );
      return;
    }
    setPendingShareHandoff({
      documentId: handoff.documentId,
      documentTitle: handoff.documentTitle,
    });
  }, [category, isAgreement, searchParams, setSearchParams]);

  // Open deferred Share once list polling marks the uploaded doc ready.
  useEffect(() => {
    if (!pendingShareHandoff || !data?.length) return;
    const row = data.find((doc) => doc.id === pendingShareHandoff.documentId);
    if (!row) return;
    if (isDocumentReadyForLibraryShare(row.status)) {
      setDocToShare(row);
      setPendingShareHandoff(null);
      return;
    }
    if (row.status === "failed" || row.status === "archived") {
      setPendingShareHandoff(null);
    }
  }, [data, pendingShareHandoff]);

  // Refresh after in-page upload; toast Share action for general library uploads.
  useEffect(() => {
    const handleUploaded = (event: Event) => {
      void refetch();
      if (isAgreement) return;
      const detail = (event as CustomEvent<DocumentsUploadedDetail | undefined>).detail;
      if (!isLibraryShareableUpload(detail)) return;
      toast.success(t("documents:share.uploadSuccess"), {
        action: {
          label: t("documents:share.uploadShareAction"),
          onClick: () => {
            if (isDocumentReadyForLibraryShare(detail!.status)) {
              setDocToShare(buildShareRow(detail!));
              return;
            }
            setPendingShareHandoff({
              documentId: detail!.documentId,
              documentTitle: detail!.documentTitle,
            });
          },
        },
      });
    };
    window.addEventListener(DOCUMENTS_UPLOADED_EVENT, handleUploaded);
    return () => window.removeEventListener(DOCUMENTS_UPLOADED_EVENT, handleUploaded);
  }, [refetch, isAgreement, t]);

  const columns = useDocumentColumns({
    workspaceSlug,
    navigate,
    refetch,
    canWrite,
    onAddToDealRoom: canWrite ? setDocToAddToRoom : undefined,
    onArchive: canWrite ? setDocToArchive : undefined,
    onShare: canWrite && !isAgreement ? setDocToShare : undefined,
    onDelete: canWrite ? setDocToDelete : undefined,
    returnTo: location.pathname + location.search,
    returnLabel: t("documents:detail.back"),
  });

  const table = useReactTable({
    data: data ?? [],
    columns,
    state: { sorting, globalFilter },
    onSortingChange: setSorting,
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
  });

  const filterTabs =
    category !== "agreement" ? (
      <div className="mb-10 flex justify-center sm:mb-12">
        <Tabs
          value={filter}
          onValueChange={(value) => setFilter(value as DocumentFilter)}
          className="min-w-0"
        >
          <TabsList variant="line" className="justify-center gap-12 overflow-x-auto">
            {filters.map((f) => (
              <TabsTrigger
                key={f}
                value={f}
                className={cn(
                  "flex-none px-6 text-[15px] tracking-wide",
                  "transition-[color,transform] duration-300 ease-out",
                  "hover:-translate-y-px hover:text-foreground",
                  "data-active:font-semibold",
                  "after:h-0.5 after:origin-center after:scale-x-0 after:rounded-full",
                  "after:transition-[transform,opacity] after:duration-300 after:ease-out",
                  "data-active:after:scale-x-100 data-active:after:opacity-100"
                )}
              >
                {t(`documents:filters.${f}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>
    ) : null;

  if (showShareTab) {
    return (
      <div>
        {filterTabs}
        <LinksTable
          embedded
          documentId={shareDocumentId}
          documentTitle={shareDocumentTitle}
          searchQuery={deferredShareSearch}
          createdWithin={shareCreatedWithin}
          onClearListFilters={clearShareListFilters}
          toolbar={
            <div className="flex flex-wrap items-center justify-end gap-3">
              <div className="relative w-full min-w-[12rem] max-w-xs sm:w-56">
                <MagnifyingGlass
                  size={16}
                  className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
                />
                <Input
                  placeholder={t("links:table.searchPlaceholder")}
                  value={shareSearchInput}
                  onChange={(e) => setShareSearchInput(e.target.value)}
                  className="h-9 border-border/70 bg-background/80 pl-9 text-sm shadow-none backdrop-blur-sm"
                  data-testid="share-links-search"
                  aria-label={t("links:table.searchPlaceholder")}
                />
              </div>
              <Select
                value={shareCreatedWithin}
                onValueChange={(value) => {
                  if (!isLinkCreatedWithin(value)) return;
                  patchShareListParams({ createdWithin: value });
                }}
              >
                <SelectTrigger
                  className="h-9 min-w-[11.5rem] border-border/70 bg-background/80 shadow-none"
                  data-testid="share-created-within"
                  aria-label={t("links:table.createdWithinLabel")}
                >
                  <span className="flex min-w-0 items-center gap-1.5">
                    <CalendarBlank size={14} className="shrink-0 text-muted-foreground" />
                    <span className="truncate">
                      {t(`links:table.createdWithin.${shareCreatedWithin}`)}
                    </span>
                  </span>
                </SelectTrigger>
                <SelectContent align="end" alignItemWithTrigger={false}>
                  {LINK_CREATED_WITHIN_VALUES.map((value) => (
                    <SelectItem key={value} value={value}>
                      {t(`links:table.createdWithin.${value}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                onClick={() =>
                  navigate(
                    documentsCreateLinkPath(workspaceSlug!, {
                      documentId: shareDocumentId,
                    }),
                  )
                }
                disabled={!canWrite}
                className="gap-1.5 shrink-0"
                data-testid="share-create-link"
                title={canWrite ? undefined : t("common:error.codes.insufficient_role")}
              >
                <Plus size={16} weight="bold" />
                {t("links:page.createLink")}
              </Button>
            </div>
          }
        />
      </div>
    );
  }

  const agreementSearch = (
    <div className="relative w-full min-w-[11rem] max-w-xs sm:w-52">
      <MagnifyingGlass
        size={16}
        className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
      />
      <Input
        placeholder={t("documents:table.searchPlaceholder")}
        value={globalFilter}
        onChange={(e) => setGlobalFilter(e.target.value)}
        className="h-9 border-border/80 bg-background pl-9 text-sm shadow-none"
      />
    </div>
  );

  const agreementToolbar = (opts?: { showSearch?: boolean }) => (
    <>
      {opts?.showSearch ? agreementSearch : null}
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button variant="outline" className="gap-1.5 border-border/80 shadow-none">
              <Download size={15} weight="bold" />
              {t("documents:table.downloadTemplate")}
            </Button>
          }
        />
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={() => window.open("/templates/nda-cn.txt", "_blank")}>
            {t("documents:table.templates.ndaCN")}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => window.open("/templates/nda-en.txt", "_blank")}>
            {t("documents:table.templates.ndaEN")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <Button
        onClick={openUpload}
        disabled={isUploading || !canWrite}
        className="gap-1.5 shrink-0"
        data-testid="agreement-upload-button"
        title={canWrite ? undefined : t("common:error.codes.insufficient_role")}
      >
        <Plus size={15} weight="bold" />
        {t("documents:table.upload")}
      </Button>
    </>
  );

  const agreementPageHeader = (actions?: ReactNode, meta?: ReactNode) =>
    isAgreement ? (
      <div className="space-y-3" data-testid="agreement-page-header">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <h1 className="text-h1 tracking-tight">
            {t("agreementDocuments:page.documentsTitle")}
          </h1>
          {actions ? (
            <div className="flex flex-wrap items-center justify-end gap-2 shrink-0">
              {actions}
            </div>
          ) : null}
        </div>
        <p className="max-w-xl text-body text-muted-foreground">
          {t("agreementDocuments:page.documentsHint")}
        </p>
        {meta}
      </div>
    ) : null;

  const agreementCardSkeleton = (
    <div
      className="grid grid-cols-2 gap-x-5 gap-y-8 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-5"
      data-testid="agreement-doc-skeleton"
    >
      {Array.from({ length: 4 }).map((_, i) => (
        <div key={i} className="space-y-3">
          <Skeleton className="aspect-[3/4] w-full rounded-xl" />
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-3 w-1/2" />
        </div>
      ))}
    </div>
  );

  if (error) {
    return (
      <div className="space-y-8">
        {filterTabs}
        {agreementPageHeader(isAgreement ? agreementToolbar() : undefined)}
        <div className="flex flex-col items-center justify-center gap-4 rounded-xl border border-border/80 bg-card p-12 text-center">
          <p className="text-body text-muted-foreground">{error}</p>
          <Button onClick={refetch}>{t("common:retry")}</Button>
        </div>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="space-y-8">
        {filterTabs}
        {agreementPageHeader(isAgreement ? agreementToolbar() : undefined)}
        {isAgreement ? agreementCardSkeleton : <SkeletonList rows={6} />}
      </div>
    );
  }

  const hasDocuments = data && data.length > 0;
  /** Search + upload belong to the Documents tab only (not Shared / Archived). */
  const showDocumentsActions = hasDocuments && filter === "all" && !isAgreement;

  const documentsActions = showDocumentsActions ? (
    <>
      <div className="relative w-full min-w-[12rem] max-w-xs sm:w-56">
        <MagnifyingGlass
          size={16}
          className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
        />
        <Input
          placeholder={t("documents:table.searchPlaceholder")}
          value={globalFilter}
          onChange={(e) => setGlobalFilter(e.target.value)}
          className="h-9 border-border/70 bg-background/80 pl-9 text-sm shadow-none backdrop-blur-sm"
        />
      </div>
      <Button
        onClick={openUpload}
        disabled={isUploading || !canWrite}
        className="gap-1.5 shrink-0 shadow-[0_1px_0_rgba(15,23,42,0.06)]"
        data-testid="documents-upload-button"
        title={canWrite ? undefined : t("common:error.codes.insufficient_role")}
      >
        <Plus size={16} weight="bold" />
        {t("documents:table.upload")}
      </Button>
    </>
  ) : null;

  const agreementMeta =
    isAgreement && hasDocuments ? (
      <p className="text-caption text-muted-foreground">
        {t("agreementDocuments:page.fileCount", { count: data.length })}
      </p>
    ) : null;

  return (
    <div>
      {filterTabs}

      <div className={cn(isAgreement ? "space-y-8" : "space-y-4")}>
      {isAgreement
        ? agreementPageHeader(agreementToolbar({ showSearch: Boolean(hasDocuments) }), agreementMeta)
        : null}
      {!data || data.length === 0 ? (
        filter !== "all" ? (
          <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
            <p className="text-body text-muted-foreground">{t("documents:table.emptyFilter")}</p>
            <Button variant="outline" onClick={() => setFilter("all")}>
              {t("documents:table.clearFilter")}
            </Button>
          </div>
        ) : (
          <EmptyState
            icon={isAgreement ? <FilePdf size={40} weight="duotone" /> : <LinkIcon size={64} />}
            title={
              isAgreement
                ? t("agreementDocuments:page.emptyTitle")
                : t("documents:table.emptyTitle")
            }
            description={
              isAgreement
                ? t("agreementDocuments:page.emptyDescription")
                : t("documents:table.emptyDescription")
            }
            action={
              isAgreement || !canWrite
                ? undefined
                : {
                    label: t("documents:table.upload"),
                    onClick: openUpload,
                  }
            }
            className={
              isAgreement
                ? "border border-dashed border-border/80 bg-muted/15 py-16"
                : undefined
            }
          />
        )
      ) : (
        <>
          {documentsActions ? (
            <div className="flex flex-wrap items-center justify-end gap-3">
              {documentsActions}
            </div>
          ) : null}

          {isAgreement ? (
            table.getRowModel().rows.length === 0 ? (
              <p className="py-16 text-center text-sm text-muted-foreground">
                {t("documents:table.noMatches")}
              </p>
            ) : (
              <div
                className="grid grid-cols-2 gap-x-5 gap-y-8 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-5"
                data-testid="agreement-doc-cards"
              >
                {table.getRowModel().rows.map((row) => (
                  <AgreementDocumentCard
                    key={row.id}
                    doc={row.original}
                    onOpen={() => openDocument(row.original.id)}
                    onDelete={canWrite ? () => setDocToDelete(row.original) : undefined}
                  />
                ))}
              </div>
            )
          ) : (
            <div
              className={cn(
                "relative overflow-auto rounded-2xl",
                // Header (h-11) + 6 document rows (~4.25rem each: py-3.5*2 + h-10 icon).
                "max-h-[calc(2.75rem+6*4.25rem)]",
                "border border-border/45 bg-gradient-to-b from-muted/35 via-background to-background",
                "shadow-[0_1px_0_rgba(15,23,42,0.03)]",
              )}
              data-testid="documents-library-list"
            >
              <div
                aria-hidden
                className="pointer-events-none absolute inset-x-0 top-0 z-20 h-px bg-gradient-to-r from-transparent via-foreground/10 to-transparent"
              />
              {/* Native table so sticky header scrolls with this container (Table wraps overflow-x). */}
              <table className="w-full caption-bottom text-sm">
                <TableHeader className="sticky top-0 z-10 [&_tr]:border-border/40">
                  {table.getHeaderGroups().map((headerGroup) => (
                    <TableRow
                      key={headerGroup.id}
                      className="border-border/40 bg-background/95 hover:bg-background/95 backdrop-blur-sm"
                    >
                      {headerGroup.headers.map((header) => (
                        <TableHead
                          key={header.id}
                          className={cn(
                            "h-11 px-4 first:pl-5 last:pr-5",
                            header.id === "actions" ? "w-[96px] text-right" : "",
                            header.id === "totalViews" ? "w-[88px]" : "",
                            header.id === "shareLinks" ? "w-[96px]" : "",
                          )}
                        >
                          {header.isPlaceholder
                            ? null
                            : flexRender(header.column.columnDef.header, header.getContext())}
                        </TableHead>
                      ))}
                    </TableRow>
                  ))}
                </TableHeader>
                <TableBody>
                  {table.getRowModel().rows.length === 0 ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell
                        colSpan={columns.length}
                        className="h-36 text-center text-muted-foreground"
                      >
                        {t("documents:table.noMatches")}
                      </TableCell>
                    </TableRow>
                  ) : (
                    table.getRowModel().rows.map((row, index) => (
                      <TableRow
                        key={row.id}
                        className={cn(
                          "group/doc-row cursor-pointer border-border/35",
                          "transition-[background-color,box-shadow] duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
                          "hover:bg-foreground/[0.03]",
                          "focus-visible:bg-foreground/[0.04] focus-visible:outline-none",
                          "animate-in fade-in-0 slide-in-from-bottom-1 fill-mode-both",
                        )}
                        style={{
                          animationDuration: "320ms",
                          animationDelay: `${Math.min(index, 10) * 28}ms`,
                        }}
                        role="link"
                        tabIndex={0}
                        onClick={() => openDocument(row.original.id)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            openDocument(row.original.id);
                          }
                        }}
                      >
                        {row.getVisibleCells().map((cell) => (
                          <TableCell
                            key={cell.id}
                            className={cn(
                              "h-[4.25rem] px-4 py-3.5 first:pl-5 last:pr-5",
                              cell.column.id === "actions" ? "text-right" : "",
                            )}
                          >
                            {flexRender(cell.column.columnDef.cell, cell.getContext())}
                          </TableCell>
                        ))}
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </table>
            </div>
          )}
        </>
      )}
      </div>

      {isAgreement && (
        <input
          ref={fileInputRef}
          type="file"
          accept=".pdf,application/pdf"
          multiple
          className="hidden"
          data-testid="agreement-file-input"
          onChange={(e) => void handleAgreementFiles(e)}
        />
      )}
      {isAgreement ? conflictDialog : null}

      {docToAddToRoom && (
        <AddToDealRoomDialog
          documentId={docToAddToRoom.id}
          documentTitle={docToAddToRoom.title}
          open={!!docToAddToRoom}
          onOpenChange={(open) => !open && setDocToAddToRoom(null)}
          onAdded={() => setDocToAddToRoom(null)}
        />
      )}

      {docToShare && workspaceSlug ? (
        <DocumentShareDialog
          open={!!docToShare}
          onOpenChange={(open) => !open && setDocToShare(null)}
          documentId={docToShare.id}
          documentTitle={docToShare.title}
          workspaceSlug={workspaceSlug}
          documentStatus={docToShare.status}
          onCreated={() => {
            setDocToShare(null);
            void refetch();
          }}
        />
      ) : null}

      <Dialog
        open={!!docToArchive}
        onOpenChange={(open) => !open && !isArchiving && setDocToArchive(null)}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("documents:archive.title")}</DialogTitle>
            <DialogDescription className="space-y-2 break-words">
              <span className="block">
                {t("documents:archive.description", { name: docToArchive?.title ?? "" })}
              </span>
              <span className="block">{t("documents:archive.visitorRevoke")}</span>
              <span className="block">{t("documents:archive.unarchiveDoesNotRestore")}</span>
              {(archiveImpact?.activeLinkCount ?? 0) > 0 ? (
                <span className="block text-destructive">
                  {t("documents:archive.withLinks", {
                    count: archiveImpact?.activeLinkCount ?? 0,
                  })}
                </span>
              ) : null}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDocToArchive(null)}
              disabled={isArchiving}
            >
              {t("common:cancel")}
            </Button>
            <Button
              disabled={isArchiving}
              onClick={async () => {
                if (!docToArchive) return;
                setIsArchiving(true);
                try {
                  await api.archiveDocument(docToArchive.id);
                  toast.success(t("documents:columns.archived"));
                  setDocToArchive(null);
                  void refetch();
                } catch (e) {
                  toast.error(
                    apiErrorMessage(e, { messageKey: "documents:columns.archiveFailed" }),
                  );
                } finally {
                  setIsArchiving(false);
                }
              }}
            >
              {isArchiving ? t("documents:archive.confirmLoading") : t("common:archive")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!docToDelete} onOpenChange={(open) => !open && !isDeleting && setDocToDelete(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("documents:delete.title")}</DialogTitle>
            <DialogDescription className="space-y-2 break-words">
              <span className="block">
                {t("documents:delete.description", { name: docToDelete?.title ?? "" })}
              </span>
              {(deleteImpact?.activeLinkCount ?? 0) > 0 ? (
                <span className="block text-destructive">
                  {t("documents:delete.withLinks", {
                    count: deleteImpact?.activeLinkCount ?? 0,
                  })}
                </span>
              ) : null}
              {(deleteImpact?.dealRoomCount ?? 0) > 0 ? (
                <span className="block text-destructive">
                  {t("documents:delete.withDealRooms", {
                    count: deleteImpact?.dealRoomCount ?? 0,
                  })}
                </span>
              ) : null}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDocToDelete(null)}
              disabled={isDeleting}
            >
              {t("common:cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={isDeleting}
              onClick={async () => {
                if (!docToDelete) return;
                setIsDeleting(true);
                try {
                  await api.deleteDocument(docToDelete.id);
                  toast.success(t("documents:delete.success"));
                  setDocToDelete(null);
                  void refetch();
                } catch (e) {
                  toast.error(apiErrorMessage(e, { messageKey: "documents:delete.failed" }));
                } finally {
                  setIsDeleting(false);
                }
              }}
            >
              {isDeleting ? t("documents:delete.confirmLoading") : t("common:delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
