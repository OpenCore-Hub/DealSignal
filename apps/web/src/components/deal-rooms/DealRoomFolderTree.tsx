import { useContext, useEffect, useMemo, useRef, useState } from "react";
import { apiErrorMessage } from "@/lib/apiErrors";
import { createPortal } from "react-dom";
import {
  Folder,
  FolderOpen,
  FileText,
  FileX,
  Plus,
  UploadSimple,
  Trash,
  X,
  Lock,
  LockOpen,
  MagnifyingGlass,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";
import {
  buildFolderTree,
  filterFolderTree,
  docSelectionKey,
  findNodeByPath,
  parseSelection,
  subtreeSelectionState,
  topmostSelectedFolderPaths,
  withFolderSubtreeSelection,
  withDocumentSelection,
  type FolderTreeNode,
  type SelectionKey,
} from "@/lib/dealRoomFolderTree";
import { ResourcesToolbarHostContext } from "./DealRoomDocumentsHome";
import { UploadCancelledError } from "@/hooks/useDocumentUploadConflict";
import type { DealRoomDocumentItem, DealRoomFolder, DealRoomFolderDocs } from "@/types";

/** Backend treats parent_path "/" as a top-level (root) folder create. */
const ROOT_PARENT = "/";

interface DealRoomFolderTreeProps {
  roomId: string;
  folders: DealRoomFolder[];
  folderDocs: DealRoomFolderDocs[];
  roomDocuments?: DealRoomDocumentItem[];
  isAdmin?: boolean;
  /** When provided, the tree works as a pure folder navigator without inline documents. */
  selectedFolderPath?: string | null;
  onSelectFolder?: (path: string | null) => void;
  onFolderCreate: (name: string, parentPath?: string) => Promise<void>;
  onFolderRename: (path: string, name: string) => Promise<void>;
  onFolderDelete: (path: string) => Promise<void>;
  onDocumentMove?: (docId: string, folderPath: string) => Promise<void>;
  onDocumentReorder?: (docId: string, sortOrder: number) => Promise<void>;
  onDocumentRemove?: (docId: string) => Promise<void>;
  onDocumentOpen?: (docId: string) => void;
  /** Upload one or more local files into a folder (multi-select supported). */
  onFolderUpload?: (file: File, folderPath: string, sortOrder?: number) => Promise<void>;
  onChanged?: () => void | Promise<void>;
}

/** Stops row-level click handlers. Base UI Checkbox re-dispatches a bubbling
 * click on a sibling hidden <input>, so stopPropagation must wrap both nodes. */
function stopRowActivation(e: React.SyntheticEvent) {
  e.stopPropagation();
}

function SelectionCheckbox({
  checked,
  indeterminate = false,
  onCheckedChange,
  "aria-label": ariaLabel,
}: {
  checked: boolean;
  indeterminate?: boolean;
  onCheckedChange: (checked: boolean | "indeterminate") => void;
  "aria-label": string;
}) {
  return (
    <div
      className="flex shrink-0 items-center"
      onClick={stopRowActivation}
      onPointerDown={stopRowActivation}
      onKeyDown={stopRowActivation}
    >
      <Checkbox
        checked={checked}
        indeterminate={indeterminate}
        onCheckedChange={onCheckedChange}
        aria-label={ariaLabel}
      />
    </div>
  );
}

export function DealRoomFolderTree({
  roomId,
  folders,
  folderDocs,
  isAdmin = true,
  selectedFolderPath,
  onSelectFolder,
  onFolderCreate,
  onFolderRename: _onFolderRename,
  onFolderDelete,
  onDocumentRemove,
  onFolderUpload,
  onDocumentOpen,
  onChanged,
}: DealRoomFolderTreeProps) {
  const { t } = useTranslation("dealRooms");
  const { t: td } = useTranslation("documents");
  void _onFolderRename; // retained for caller compatibility; rename moved off row menus

  const isNavigator = typeof onSelectFolder === "function";

  const docsByFolder = useMemo(() => {
    const map = new Map<string, DealRoomDocumentItem[]>();
    for (const fd of folderDocs) {
      const sorted = [...fd.documents].sort((a, b) => a.sort_order - b.sort_order);
      map.set(fd.folder, sorted);
    }
    for (const folder of folders) {
      if (!map.has(folder.path)) map.set(folder.path, []);
    }
    return map;
  }, [folderDocs, folders]);

  const allRoots = useMemo(
    () => buildFolderTree(folders, docsByFolder),
    [folders, docsByFolder],
  );

  const [searchQuery, setSearchQuery] = useState("");
  const [selection, setSelection] = useState<Set<SelectionKey>>(() => new Set());
  const [bulkLoading, setBulkLoading] = useState(false);

  const filtersActive = searchQuery.trim() !== "";

  const { roots, expandPaths } = useMemo(() => {
    if (!filtersActive) {
      return { roots: allRoots, expandPaths: new Set<string>() };
    }
    return filterFolderTree(allRoots, {
      query: searchQuery,
      type: "all",
      lock: "all",
    });
  }, [allRoots, searchQuery, filtersActive]);

  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(folders.map((f) => f.path)));
  const [creatingParent, setCreatingParent] = useState<string | null>(null);
  const [newFolderName, setNewFolderName] = useState("");
  const [creating, setCreating] = useState(false);
  const bulkUploadTargetRef = useRef<string | null>(null);
  const bulkUploadInputRef = useRef<HTMLInputElement | null>(null);

  const selectionParsed = useMemo(() => parseSelection(selection), [selection]);
  const topmostFolders = useMemo(
    () => topmostSelectedFolderPaths(selectionParsed.folderPaths),
    [selectionParsed.folderPaths],
  );
  const singleFolderTarget = topmostFolders.length === 1 ? topmostFolders[0]! : null;
  const singleFolderLocked = Boolean(
    singleFolderTarget && folders.find((f) => f.path === singleFolderTarget)?.locked,
  );

  useEffect(() => {
    if (expandPaths.size === 0) return;
    setExpanded((prev) => {
      const next = new Set(prev);
      for (const path of expandPaths) next.add(path);
      return next;
    });
  }, [expandPaths]);

  const toggleFolderSelection = (
    node: FolderTreeNode,
    checked: boolean | "indeterminate",
  ) => {
    setSelection((prev) => {
      // Always cascade against the full tree so filters don't hide descendants from selection.
      const fullNode = findNodeByPath(allRoots, node.folder.path) ?? node;
      return withFolderSubtreeSelection(prev, allRoots, fullNode, checked === true);
    });
  };

  const toggleDocumentSelection = (
    documentId: string,
    checked: boolean | "indeterminate",
  ) => {
    setSelection((prev) => withDocumentSelection(prev, allRoots, documentId, checked === true));
  };

  const clearSelection = () => setSelection(new Set());

  const guardLockedFolderAction = (folder: DealRoomFolder, action: () => void) => {
    if (folder.locked) {
      toast.error(t("folders.lockedActionBlocked"));
      return;
    }
    action();
  };

  const handleBulkLock = async () => {
    if (selection.size === 0) {
      toast.error(t("folders.toolbar.needItemSelection"));
      return;
    }
    const { folderPaths, documentIds } = parseSelection(selection);
    if (!confirm(t("folders.toolbar.lockConfirm", { count: selection.size }))) return;
    setBulkLoading(true);
    try {
      await api.lockDealRoomResources(roomId, {
        folder_paths: folderPaths,
        document_ids: documentIds,
      });
      toast.success(t("folders.toolbar.lockSuccess"));
      clearSelection();
      await onChanged?.();
    } catch {
      toast.error(t("folders.toolbar.lockFailed"));
    } finally {
      setBulkLoading(false);
    }
  };

  const handleBulkUnlock = async () => {
    if (selection.size === 0) {
      toast.error(t("folders.toolbar.needItemSelection"));
      return;
    }
    const { folderPaths, documentIds } = parseSelection(selection);
    if (!confirm(t("folders.toolbar.unlockConfirm", { count: selection.size }))) return;
    setBulkLoading(true);
    try {
      await api.unlockDealRoomResources(roomId, {
        folder_paths: folderPaths,
        document_ids: documentIds,
      });
      toast.success(t("folders.toolbar.unlockSuccess"));
      clearSelection();
      await onChanged?.();
    } catch {
      toast.error(t("folders.toolbar.lockFailed"));
    } finally {
      setBulkLoading(false);
    }
  };

  const handleBulkCreateSubfolder = () => {
    if (!singleFolderTarget) {
      toast.error(t("folders.toolbar.needOneFolder"));
      return;
    }
    const folder = folders.find((f) => f.path === singleFolderTarget);
    if (!folder) return;
    guardLockedFolderAction(folder, () => {
      clearSelection();
      startCreate(singleFolderTarget);
    });
  };

  const handleBulkDeleteFolders = async () => {
    if (topmostFolders.length === 0) {
      toast.error(t("folders.toolbar.needFolderSelection"));
      return;
    }
    if (
      !confirm(
        t("folders.toolbar.deleteDirectoriesConfirm", { count: topmostFolders.length }),
      )
    ) {
      return;
    }
    setBulkLoading(true);
    try {
      for (const path of topmostFolders) {
        const folder = folders.find((f) => f.path === path);
        if (folder?.locked) {
          toast.error(t("folders.lockedActionBlocked"));
          continue;
        }
        const docs = docsByFolder.get(path) ?? [];
        if (docs.length > 0) {
          toast.error(t("folders.deleteNotEmpty"));
          continue;
        }
        await onFolderDelete(path);
      }
      clearSelection();
      await onChanged?.();
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "dealRooms:folders.deleteFailed" }));
    } finally {
      setBulkLoading(false);
    }
  };

  const handleBulkUpload = () => {
    if (!onFolderUpload) return;
    if (!singleFolderTarget) {
      toast.error(t("folders.toolbar.needOneFolder"));
      return;
    }
    const folder = folders.find((f) => f.path === singleFolderTarget);
    if (!folder) return;
    guardLockedFolderAction(folder, () => {
      // Ref (not state) so the change handler cannot race a pending re-render.
      bulkUploadTargetRef.current = singleFolderTarget;
      window.setTimeout(() => bulkUploadInputRef.current?.click(), 0);
    });
  };

  const handleBulkUploadChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const target = bulkUploadTargetRef.current;
    const files = Array.from(e.target.files ?? []);
    e.target.value = "";
    bulkUploadTargetRef.current = null;
    if (!target || files.length === 0 || !onFolderUpload) return;
    setBulkLoading(true);
    let uploaded = 0;
    try {
      const baseSort = folderDocs.find((fd) => fd.folder === target)?.documents.length ?? 0;
      // Sequential so shared replace/cancel dialogs never race.
      for (let index = 0; index < files.length; index++) {
        await onFolderUpload(files[index]!, target, baseSort + index);
        uploaded += 1;
      }
      toast.success(t("folders.toolbar.batchUploadSuccess", { count: files.length }));
      clearSelection();
    } catch (err) {
      if (!(err instanceof UploadCancelledError)) {
        toast.error(apiErrorMessage(err, { fallback: "uploadFailed", messageKey: "dealRooms:folders.toolbar.batchUploadFailed" }));
      }
    } finally {
      // Refresh even when a later file is cancelled/fails so earlier successes appear.
      if (uploaded > 0) {
        await onChanged?.();
      }
      setBulkLoading(false);
    }
  };

  const handleBulkRemoveDocuments = async () => {
    if (!onDocumentRemove) return;
    const { documentIds } = parseSelection(selection);
    if (documentIds.length === 0) {
      toast.error(t("folders.toolbar.needFileSelection"));
      return;
    }
    if (
      !confirm(t("folders.toolbar.removeFilesConfirm", { count: documentIds.length }))
    ) {
      return;
    }
    setBulkLoading(true);
    try {
      for (const id of documentIds) {
        await onDocumentRemove(id);
      }
      toast.success(t("folders.toolbar.removeFilesSuccess", { count: documentIds.length }));
      clearSelection();
      await onChanged?.();
    } catch (e) {
      toast.error(apiErrorMessage(e, { fallback: "deleteFailed", messageKey: "dealRooms:documents.removeFailed" }));
    } finally {
      setBulkLoading(false);
    }
  };

  const toggleFolder = (path: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  const startCreate = (parentPath: string) => {
    setCreatingParent(parentPath);
    setNewFolderName("");
    if (parentPath === ROOT_PARENT) return;
    setExpanded((prev) => {
      if (prev.has(parentPath)) return prev;
      const next = new Set(prev);
      next.add(parentPath);
      return next;
    });
  };

  const handleCreate = async () => {
    if (!newFolderName.trim() || creatingParent === null) return;
    setCreating(true);
    try {
      await onFolderCreate(newFolderName.trim(), creatingParent);
      setNewFolderName("");
      setCreatingParent(null);
    } finally {
      setCreating(false);
    }
  };

  const handleFolderClick = (path: string) => {
    if (isNavigator) {
      onSelectFolder?.(path);
    } else {
      toggleFolder(path);
    }
  };

  const handleAllDocumentsClick = () => {
    onSelectFolder?.(null);
  };

  const renderCreateRow = (parentPath: string) => (
    <div
      key={`create-${parentPath}`}
      className="flex items-center gap-3 rounded-lg bg-muted/30 p-2.5"
    >
      <Folder size={16} className="text-muted-foreground" />
      <Input
        value={newFolderName}
        onChange={(e) => setNewFolderName(e.target.value)}
        placeholder={t("folders.namePlaceholder")}
        className="h-7 flex-1"
        autoFocus
        onKeyDown={(e) => {
          if (e.key === "Enter") void handleCreate();
          if (e.key === "Escape") setCreatingParent(null);
        }}
      />
      <Button size="sm" className="h-7" onClick={() => void handleCreate()} disabled={!newFolderName.trim() || creating}>
        {t("folders.create")}
      </Button>
      <Button size="sm" variant="ghost" className="h-7" onClick={() => setCreatingParent(null)}>
        <X size={14} />
      </Button>
    </div>
  );

  const renderFolder = (node: FolderTreeNode, depth: number) => {
    const isExpanded = expanded.has(node.folder.path);
    const docs = node.documents;
    const isSelected = isNavigator && selectedFolderPath === node.folder.path;
    const folderLocked = Boolean(node.folder.locked);

    const documentCount = docs.length;
    const subfolderCount = node.children.length;
    const metadata: string[] = [];
    if (documentCount > 0 || subfolderCount === 0) {
      metadata.push(t("folders.documentsCount", { count: documentCount }));
    }
    if (subfolderCount > 0) {
      metadata.push(t("folders.foldersCount", { count: subfolderCount }));
    }
    const fullNode = findNodeByPath(allRoots, node.folder.path) ?? node;
    const folderCheckState = subtreeSelectionState(fullNode, selection);

    return (
      <div key={node.folder.path} className="select-none">
        <div
          role="button"
          tabIndex={0}
          aria-expanded={isExpanded}
          onClick={() => handleFolderClick(node.folder.path)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              handleFolderClick(node.folder.path);
            }
          }}
          className={cn(
            "group flex w-full items-center gap-3 rounded-lg border border-transparent p-2.5 text-left transition-colors duration-150 ease-out hover:bg-muted/50 hover:border-border/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
            isSelected && "bg-primary/[0.04] border-primary/20 hover:border-primary/30 hover:bg-primary/[0.06]"
          )}
        >
          <div className="flex min-w-0 items-center gap-3">
            {isAdmin && (
              <SelectionCheckbox
                checked={folderCheckState === "checked"}
                indeterminate={folderCheckState === "indeterminate"}
                onCheckedChange={(checked) => toggleFolderSelection(node, checked)}
                aria-label={node.folder.name}
              />
            )}
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-muted/40">
              {isSelected || isExpanded ? (
                <FolderOpen size={18} className={cn("text-primary", !isSelected && "text-foreground")} />
              ) : (
                <Folder size={18} className="text-muted-foreground" />
              )}
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
                <span className="truncate text-sm font-medium text-foreground">
                  {node.folder.name}
                </span>
                {folderLocked && (
                  <Lock
                    size={14}
                    weight="fill"
                    className="shrink-0 text-muted-foreground"
                    aria-label={t("folders.lockedBadge")}
                  />
                )}
                {node.folder.description && (
                  <span className="hidden text-xs text-muted-foreground/80 sm:inline">
                    {node.folder.description}
                  </span>
                )}
              </div>
              {metadata.length > 0 && (
                <div className="mt-0.5 text-xs text-muted-foreground/70">
                  {metadata.join(" • ")}
                </div>
              )}
            </div>
          </div>
        </div>

        {!isNavigator && isExpanded && (
          <div className="relative mt-0.5 ml-3 border-l border-border/30 pl-3">
            {creatingParent === node.folder.path && renderCreateRow(node.folder.path)}
            {docs.length > 0 && (
              <ul className="py-1">
                {docs.map((doc) => (
                  <li
                    key={doc.id}
                    className="group flex items-center gap-3 rounded-md px-2.5 py-1.5 text-sm text-foreground hover:bg-muted/40"
                  >
                    {isAdmin && (
                      <SelectionCheckbox
                        checked={selection.has(docSelectionKey(doc.document_id))}
                        onCheckedChange={(checked) =>
                          toggleDocumentSelection(doc.document_id, checked)
                        }
                        aria-label={doc.title}
                      />
                    )}
                    <button
                      type="button"
                      className="flex min-w-0 flex-1 cursor-pointer items-center gap-3 rounded-sm text-left hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      onClick={() => onDocumentOpen?.(doc.document_id)}
                      title={t("documents.clickToOpen")}
                    >
                      <div className="flex h-5 w-8 shrink-0 items-center justify-center">
                        <FileText size={15} className="text-muted-foreground/80" />
                      </div>
                      <span className="truncate">{doc.title}</span>
                      {doc.locked && (
                        <Lock
                          size={13}
                          weight="fill"
                          className="shrink-0 text-muted-foreground"
                          aria-label={t("folders.lockedBadge")}
                        />
                      )}
                    </button>
                  </li>
                ))}
              </ul>
            )}
            <div className="space-y-0.5">
              {node.children.map((child) => renderFolder(child, depth + 1))}
            </div>
          </div>
        )}

        {isNavigator && isExpanded && (
          <div className="relative mt-0.5 ml-3 border-l border-border/30 pl-3">
            {creatingParent === node.folder.path && renderCreateRow(node.folder.path)}
            <div className="space-y-0.5">
              {node.children.map((child) => renderFolder(child, depth + 1))}
            </div>
          </div>
        )}
      </div>
    );
  };

  const showNoMatches = folders.length > 0 && roots.length === 0 && filtersActive;
  const toolbarHost = useContext(ResourcesToolbarHostContext);
  const showToolbar = isAdmin || folders.length > 0;

  const toolbar = showToolbar ? (
      <div className="flex flex-wrap items-center gap-2" data-testid="folder-tree-toolbar">
        {selection.size === 0 ? (
          <>
            {folders.length > 0 ? (
              <div className="relative w-full max-w-xs sm:w-64">
                <MagnifyingGlass
                  size={16}
                  className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground"
                  aria-hidden
                />
                <Input
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder={t("folders.toolbar.searchPlaceholder")}
                  aria-label={t("folders.toolbar.searchAria")}
                  className="h-9 pl-8"
                />
              </div>
            ) : null}
            {isAdmin ? (
              <>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  className="h-9"
                  data-testid="folder-tree-create-directory"
                  onClick={() => startCreate(ROOT_PARENT)}
                >
                  <Plus size={14} weight="bold" className="mr-1.5" />
                  {t("folders.toolbar.createDirectory")}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  className="h-9"
                  data-testid="folder-tree-bulk-lock"
                  onClick={() => void handleBulkLock()}
                  disabled={bulkLoading}
                >
                  <Lock size={14} className="mr-1.5" />
                  {t("folders.toolbar.bulkLock")}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  className="h-9"
                  data-testid="folder-tree-bulk-unlock"
                  onClick={() => void handleBulkUnlock()}
                  disabled={bulkLoading}
                >
                  <LockOpen size={14} className="mr-1.5" />
                  {t("folders.toolbar.bulkUnlock")}
                </Button>
              </>
            ) : null}
          </>
        ) : (
          <>
            <span className="text-sm font-medium text-foreground">
              {t("folders.toolbar.selected", { count: selection.size })}
            </span>
            {isAdmin ? (
              <>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleBulkCreateSubfolder}
                  disabled={bulkLoading || !singleFolderTarget || singleFolderLocked}
                  data-testid="folder-tree-bulk-create-subfolder"
                >
                  <Plus size={14} weight="bold" className="mr-1.5" />
                  {t("folders.toolbar.createSubdirectory")}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => void handleBulkDeleteFolders()}
                  disabled={bulkLoading || topmostFolders.length === 0}
                  data-testid="folder-tree-bulk-delete-directory"
                >
                  <Trash size={14} className="mr-1.5" />
                  {t("folders.toolbar.deleteDirectory")}
                </Button>
                {onFolderUpload ? (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={handleBulkUpload}
                    disabled={bulkLoading || !singleFolderTarget || singleFolderLocked}
                    data-testid="folder-tree-bulk-upload"
                  >
                    <UploadSimple size={14} className="mr-1.5" />
                    {t("folders.toolbar.batchUpload")}
                  </Button>
                ) : null}
                {onDocumentRemove ? (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void handleBulkRemoveDocuments()}
                    disabled={
                      bulkLoading || selectionParsed.documentIds.length === 0
                    }
                    data-testid="folder-tree-bulk-remove-files"
                  >
                    <FileX size={14} className="mr-1.5" />
                    {t("folders.toolbar.removeFiles")}
                  </Button>
                ) : null}
                <Button
                  size="sm"
                  variant="outline"
                  className="h-9"
                  data-testid="folder-tree-bulk-lock"
                  onClick={() => void handleBulkLock()}
                  disabled={bulkLoading}
                >
                  <Lock size={14} className="mr-1.5" />
                  {t("folders.toolbar.bulkLock")}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="h-9"
                  data-testid="folder-tree-bulk-unlock"
                  onClick={() => void handleBulkUnlock()}
                  disabled={bulkLoading}
                >
                  <LockOpen size={14} className="mr-1.5" />
                  {t("folders.toolbar.bulkUnlock")}
                </Button>
              </>
            ) : null}
            {onFolderUpload ? (
              <input
                ref={bulkUploadInputRef}
                type="file"
                multiple
                accept={td("upload.supportedTypes")}
                data-testid="folder-tree-bulk-upload-input"
                tabIndex={-1}
                aria-hidden
                className="sr-only"
                onChange={(e) => void handleBulkUploadChange(e)}
              />
            ) : null}
          </>
        )}
      </div>
  ) : null;

  return (
    <div className="space-y-1" data-testid="folder-tree">
      {toolbarHost && toolbar ? createPortal(toolbar, toolbarHost) : toolbar ? (
        <div className="mb-3">{toolbar}</div>
      ) : null}

      {folders.length === 0 && <p className="text-sm text-muted-foreground">{t("folders.empty")}</p>}

      {isNavigator && (
        <button
          type="button"
          onClick={handleAllDocumentsClick}
          className={`
            flex w-full items-center gap-2 rounded-md p-2 text-left text-sm font-medium outline-none
            focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background
            ${selectedFolderPath === null ? "bg-primary/10 text-primary" : "hover:bg-muted/50"}
          `}
        >
          <FileText size={18} className={selectedFolderPath === null ? "text-primary" : "text-muted-foreground"} />
          {t("documentList.allDocuments")}
        </button>
      )}

      {showNoMatches ? (
        <p className="py-4 text-center text-sm text-muted-foreground">
          {t("folders.toolbar.noMatches")}
        </p>
      ) : (
        <div className="space-y-1">
          {creatingParent === ROOT_PARENT ? renderCreateRow(ROOT_PARENT) : null}
          {roots.map((root) => renderFolder(root, 0))}
        </div>
      )}

    </div>
  );
}
