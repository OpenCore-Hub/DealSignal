import { useContext, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  Folder,
  FolderOpen,
  FileText,
  Plus,
  UploadSimple,
  PencilSimple,
  Trash,
  X,
  DotsThreeVertical,
  Lock,
  MagnifyingGlass,
} from "@phosphor-icons/react";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
  withFolderSubtreeSelection,
  withDocumentSelection,
  type FolderTreeNode,
  type SelectionKey,
  type ResourceLockFilter,
} from "@/lib/dealRoomFolderTree";
import { DocumentPicker } from "./DocumentPicker";
import { ResourcesToolbarHostContext } from "./DealRoomDocumentsHome";
import type { DealRoomDocumentItem, DealRoomFolder, DealRoomFolderDocs, Document } from "@/types";

interface DealRoomFolderTreeProps {
  roomId: string;
  folders: DealRoomFolder[];
  folderDocs: DealRoomFolderDocs[];
  workspaceDocuments?: Document[];
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
  onDocumentsAdd?: (documentIds: string[], folderPath: string) => Promise<void>;
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

function ContextMenu({
  x,
  y,
  children,
  onClose,
}: {
  x: number;
  y: number;
  children: React.ReactNode;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) {
        onClose();
      }
    };
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("click", handleClick, true);
    window.addEventListener("scroll", onClose, true);
    window.addEventListener("resize", onClose, true);
    window.addEventListener("keydown", handleEsc);
    return () => {
      window.removeEventListener("click", handleClick, true);
      window.removeEventListener("scroll", onClose, true);
      window.removeEventListener("resize", onClose, true);
      window.removeEventListener("keydown", handleEsc);
    };
  }, [onClose]);

  return (
    <div
      ref={ref}
      className="fixed z-50 min-w-[10rem] rounded-md border border-border bg-popover p-1 shadow-md"
      style={{ left: x, top: y }}
      onContextMenu={(e) => e.preventDefault()}
    >
      {children}
    </div>
  );
}

export function DealRoomFolderTree({
  roomId,
  folders,
  folderDocs,
  workspaceDocuments,
  roomDocuments,
  isAdmin = true,
  selectedFolderPath,
  onSelectFolder,
  onFolderCreate,
  onFolderRename,
  onFolderDelete,
  onDocumentsAdd,
  onFolderUpload,
  onDocumentOpen,
  onChanged,
}: DealRoomFolderTreeProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");
  const { t: td } = useTranslation("documents");

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
  const [lockFilter, setLockFilter] = useState<ResourceLockFilter>("all");
  const [selection, setSelection] = useState<Set<SelectionKey>>(() => new Set());
  const [bulkLoading, setBulkLoading] = useState(false);

  const filtersActive = searchQuery.trim() !== "" || lockFilter !== "all";

  const { roots, expandPaths } = useMemo(() => {
    if (!filtersActive) {
      return { roots: allRoots, expandPaths: new Set<string>() };
    }
    return filterFolderTree(allRoots, {
      query: searchQuery,
      type: "all",
      lock: lockFilter,
    });
  }, [allRoots, searchQuery, lockFilter, filtersActive]);

  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(folders.map((f) => f.path)));
  const [creatingParent, setCreatingParent] = useState<string | null>(null);
  const [newFolderName, setNewFolderName] = useState("");
  const [creating, setCreating] = useState(false);
  const [renamingFolder, setRenamingFolder] = useState<DealRoomFolder | null>(null);
  const [renameValue, setRenameValue] = useState("");
  const [renaming, setRenaming] = useState(false);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; folder: DealRoomFolder } | null>(null);
  const [addToFolder, setAddToFolder] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const fileInputRefs = useRef<Map<string, HTMLInputElement>>(new Map());

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
    setExpanded((prev) => {
      if (prev.has(parentPath)) return prev;
      const next = new Set(prev);
      next.add(parentPath);
      return next;
    });
  };

  const handleCreate = async () => {
    if (!newFolderName.trim()) return;
    setCreating(true);
    try {
      await onFolderCreate(newFolderName.trim(), creatingParent ?? undefined);
      setNewFolderName("");
      setCreatingParent(null);
    } finally {
      setCreating(false);
    }
  };

  const startRename = (folder: DealRoomFolder) => {
    setRenamingFolder(folder);
    setRenameValue(folder.name);
    setContextMenu(null);
  };

  const handleRename = async () => {
    if (!renamingFolder || !renameValue.trim() || renameValue.trim() === renamingFolder.name) {
      setRenamingFolder(null);
      return;
    }
    setRenaming(true);
    try {
      await onFolderRename(renamingFolder.path, renameValue.trim());
      setRenamingFolder(null);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("folders.renameFailed"));
    } finally {
      setRenaming(false);
    }
  };

  const handleDelete = async (folder: DealRoomFolder) => {
    const docs = docsByFolder.get(folder.path) ?? [];
    if (docs.length > 0) {
      alert(t("folders.deleteNotEmpty"));
      return;
    }
    if (!confirm(t("folders.deleteConfirm", { name: folder.name }))) return;
    try {
      await onFolderDelete(folder.path);
      setContextMenu(null);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("folders.deleteFailed"));
    }
  };

  const handleAddDocuments = async (documentIds: string[], folderPath: string) => {
    if (!onDocumentsAdd) return;
    setAdding(true);
    try {
      await onDocumentsAdd(documentIds, folderPath);
      setAddToFolder(null);
    } finally {
      setAdding(false);
    }
  };

  const handleFolderUploadChange = async (
    e: React.ChangeEvent<HTMLInputElement>,
    folderPath: string
  ) => {
    const files = Array.from(e.target.files ?? []);
    if (files.length === 0 || !onFolderUpload) return;
    // Clear immediately so the same selection can be re-picked after upload.
    e.target.value = "";
    const baseSort =
      folderDocs.find((fd) => fd.folder === folderPath)?.documents.length ?? 0;
    // Kick off in parallel; stable sort_order via base+index (avoids duplicate ranks).
    await Promise.all(
      files.map((file, index) => onFolderUpload(file, folderPath, baseSort + index)),
    );
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
          onContextMenu={(e) => handleContextMenu(e, node.folder)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              handleFolderClick(node.folder.path);
            }
          }}
          className={cn(
            "group flex w-full items-center justify-between gap-3 rounded-lg border border-transparent p-2.5 text-left transition-colors duration-150 ease-out hover:bg-muted/50 hover:border-border/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
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
          <div className="flex shrink-0 items-center gap-0.5">
            {isAdmin && (
              <>
                {onFolderUpload && (
                  <input
                    ref={(el) => {
                      if (el) fileInputRefs.current.set(node.folder.path, el);
                    }}
                    type="file"
                    multiple
                    accept={td("upload.supportedTypes")}
                    data-testid={`folder-upload-input-${node.folder.path}`}
                    tabIndex={-1}
                    aria-hidden
                    className="sr-only"
                    onChange={(e) => void handleFolderUploadChange(e, node.folder.path)}
                  />
                )}
                <DropdownMenu>
                  <DropdownMenuTrigger
                    className={cn(
                      buttonVariants({ variant: "ghost", size: "icon-sm" }),
                      "rounded-full text-muted-foreground opacity-70 transition-all duration-200",
                      "hover:bg-muted hover:text-foreground hover:opacity-100",
                      "data-[popup-open]:bg-muted data-[popup-open]:text-foreground data-[popup-open]:opacity-100",
                      "focus-visible:opacity-100",
                    )}
                    onClick={(e) => e.stopPropagation()}
                    aria-label={t("folders.actions", { name: node.folder.name })}
                  >
                    <DotsThreeVertical size={18} weight="bold" />
                  </DropdownMenuTrigger>
                  <DropdownMenuContent
                    align="end"
                    side="bottom"
                    sideOffset={8}
                    className="min-w-52 origin-top-right p-1.5 shadow-dropdown"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <DropdownMenuItem
                      className={cn(
                        "gap-2.5 px-2.5 py-2 animate-in fade-in-0 slide-in-from-top-1 fill-mode-both",
                        folderLocked && "opacity-50",
                      )}
                      style={{ animationDelay: "20ms", animationDuration: "180ms" }}
                      onClick={() =>
                        guardLockedFolderAction(node.folder, () => startCreate(node.folder.path))
                      }
                    >
                      <span className="flex size-4 shrink-0 items-center justify-center text-muted-foreground group-focus/dropdown-menu-item:text-accent-foreground">
                        <Plus size={16} weight="bold" />
                      </span>
                      <span className="font-medium tracking-tight">
                        {t("folders.newSubfolder")}
                      </span>
                    </DropdownMenuItem>

                    {onFolderUpload && (
                      <DropdownMenuItem
                        className={cn(
                          "gap-2.5 px-2.5 py-2 animate-in fade-in-0 slide-in-from-top-1 fill-mode-both",
                          folderLocked && "opacity-50",
                        )}
                        style={{ animationDelay: "50ms", animationDuration: "180ms" }}
                        onClick={() =>
                          guardLockedFolderAction(node.folder, () => {
                            fileInputRefs.current.get(node.folder.path)?.click();
                          })
                        }
                      >
                        <span className="flex size-4 shrink-0 items-center justify-center text-muted-foreground group-focus/dropdown-menu-item:text-accent-foreground">
                          <UploadSimple size={16} />
                        </span>
                        <span className="font-medium tracking-tight">
                          {t("folders.addFiles")}
                        </span>
                      </DropdownMenuItem>
                    )}

                    <DropdownMenuItem
                      className={cn(
                        "gap-2.5 px-2.5 py-2 animate-in fade-in-0 slide-in-from-top-1 fill-mode-both",
                        folderLocked && "opacity-50",
                      )}
                      style={{ animationDelay: "80ms", animationDuration: "180ms" }}
                      onClick={() =>
                        guardLockedFolderAction(node.folder, () => startRename(node.folder))
                      }
                    >
                      <span className="flex size-4 shrink-0 items-center justify-center text-muted-foreground group-focus/dropdown-menu-item:text-accent-foreground">
                        <PencilSimple size={16} />
                      </span>
                      <span className="font-medium tracking-tight">
                        {t("folders.rename")}
                      </span>
                    </DropdownMenuItem>

                    <DropdownMenuSeparator className="my-1.5" />

                    <DropdownMenuItem
                      variant="destructive"
                      className={cn(
                        "gap-2.5 px-2.5 py-2 animate-in fade-in-0 slide-in-from-top-1 fill-mode-both",
                        folderLocked && "opacity-50",
                      )}
                      style={{ animationDelay: "110ms", animationDuration: "180ms" }}
                      onClick={() =>
                        guardLockedFolderAction(node.folder, () => void handleDelete(node.folder))
                      }
                    >
                      <span className="flex size-4 shrink-0 items-center justify-center text-destructive">
                        <Trash size={16} />
                      </span>
                      <span className="font-medium tracking-tight">{tc("delete")}</span>
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </>
            )}
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

  const handleContextMenu = (e: React.MouseEvent, folder: DealRoomFolder) => {
    e.preventDefault();
    e.stopPropagation();
    setContextMenu({ x: e.clientX, y: e.clientY, folder });
  };

  const showNoMatches = folders.length > 0 && roots.length === 0 && filtersActive;
  const toolbarHost = useContext(ResourcesToolbarHostContext);

  const toolbar =
    folders.length > 0 ? (
      <div className="flex flex-wrap items-center justify-end gap-2" data-testid="folder-tree-toolbar">
        {selection.size === 0 ? (
          <>
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
            <Select
              value={lockFilter}
              onValueChange={(value) => {
                if (value) setLockFilter(value as ResourceLockFilter);
              }}
            >
              <SelectTrigger className="w-[130px]" aria-label={t("folders.toolbar.lockLabel")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("folders.toolbar.lockAll")}</SelectItem>
                <SelectItem value="locked">{t("folders.toolbar.lockLocked")}</SelectItem>
                <SelectItem value="unlocked">{t("folders.toolbar.lockUnlocked")}</SelectItem>
              </SelectContent>
            </Select>
          </>
        ) : (
          <>
            <span className="text-sm font-medium text-foreground">
              {t("folders.toolbar.selected", { count: selection.size })}
            </span>
            <Button
              size="sm"
              variant="outline"
              onClick={() => void handleBulkLock()}
              disabled={bulkLoading}
            >
              <Lock size={14} className="mr-1.5" />
              {t("folders.toolbar.bulkLock")}
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => void handleBulkUnlock()}
              disabled={bulkLoading}
            >
              {t("folders.toolbar.bulkUnlock")}
            </Button>
            <Button size="sm" variant="ghost" onClick={clearSelection} disabled={bulkLoading}>
              {t("folders.toolbar.clearSelection")}
            </Button>
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
        <div className="space-y-1">{roots.map((root) => renderFolder(root, 0))}</div>
      )}

      {contextMenu && (
        <ContextMenu x={contextMenu.x} y={contextMenu.y} onClose={() => setContextMenu(null)}>
          {isAdmin && (
            <div className="min-w-48 p-1 animate-in fade-in-0 zoom-in-95 duration-150">
              <button
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm font-medium tracking-tight transition-colors hover:bg-accent hover:text-accent-foreground active:scale-[0.98]",
                  contextMenu.folder.locked && "opacity-50",
                )}
                onClick={() =>
                  guardLockedFolderAction(contextMenu.folder, () => {
                    startCreate(contextMenu.folder.path);
                    setContextMenu(null);
                  })
                }
              >
                <Plus size={16} weight="bold" className="text-muted-foreground" />
                {t("folders.newSubfolder")}
              </button>
              {onDocumentsAdd && workspaceDocuments && workspaceDocuments.length > 0 && !isNavigator && (
                <button
                  className={cn(
                    "flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm font-medium tracking-tight transition-colors hover:bg-accent hover:text-accent-foreground active:scale-[0.98]",
                    contextMenu.folder.locked && "opacity-50",
                  )}
                  onClick={() =>
                    guardLockedFolderAction(contextMenu.folder, () => {
                      setAddToFolder(contextMenu.folder.path);
                      setContextMenu(null);
                    })
                  }
                >
                  <FileText size={16} className="text-muted-foreground" />
                  {t("folders.addFile")}
                </button>
              )}
              <button
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm font-medium tracking-tight transition-colors hover:bg-accent hover:text-accent-foreground active:scale-[0.98]",
                  contextMenu.folder.locked && "opacity-50",
                )}
                onClick={() =>
                  guardLockedFolderAction(contextMenu.folder, () => startRename(contextMenu.folder))
                }
              >
                <PencilSimple size={16} className="text-muted-foreground" />
                {t("folders.rename")}
              </button>
              <div className="my-1.5 h-px bg-border" />
              <button
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm font-medium tracking-tight text-destructive transition-colors hover:bg-destructive/10 active:scale-[0.98]",
                  contextMenu.folder.locked && "opacity-50",
                )}
                onClick={() =>
                  guardLockedFolderAction(contextMenu.folder, () => void handleDelete(contextMenu.folder))
                }
              >
                <Trash size={16} />
                {tc("delete")}
              </button>
            </div>
          )}
          {!isAdmin && <p className="px-2 py-1 text-sm text-muted-foreground">{t("folders.readOnly")}</p>}
        </ContextMenu>
      )}

      <Dialog open={!!renamingFolder} onOpenChange={(open) => !open && setRenamingFolder(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("folders.renameTitle")}</DialogTitle>
          </DialogHeader>
          <Input
            value={renameValue}
            onChange={(e) => setRenameValue(e.target.value)}
            placeholder={t("folders.namePlaceholder")}
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleRename();
            }}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setRenamingFolder(null)}>
              {tc("cancel")}
            </Button>
            <Button onClick={() => void handleRename()} disabled={!renameValue.trim() || renaming}>
              {tc("save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={addToFolder !== null} onOpenChange={(open) => !open && setAddToFolder(null)}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("folders.addFile")}</DialogTitle>
            <DialogDescription>{t("documents.addFromWorkspace")}</DialogDescription>
          </DialogHeader>
          {addToFolder && workspaceDocuments && roomDocuments && (
            <DocumentPicker
              workspaceDocuments={workspaceDocuments}
              roomDocuments={roomDocuments}
              folders={folders}
              onAdd={handleAddDocuments}
              initialFolderPath={addToFolder}
              allowFolderChange={false}
              disabled={adding}
            />
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
