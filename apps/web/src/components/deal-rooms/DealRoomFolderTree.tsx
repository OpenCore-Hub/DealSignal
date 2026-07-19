import { useEffect, useMemo, useRef, useState } from "react";
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
} from "@phosphor-icons/react";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { DocumentPicker } from "./DocumentPicker";
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
  onFolderUpload?: (file: File, folderPath: string) => Promise<void>;
}

interface TreeNode {
  folder: DealRoomFolder;
  children: TreeNode[];
  documents: DealRoomDocumentItem[];
}

function parentPath(path: string): string | null {
  if (path === "/") return null;
  const idx = path.lastIndexOf("/");
  if (idx <= 0) return "/";
  return path.slice(0, idx);
}

function buildTree(folders: DealRoomFolder[], docsByFolder: Map<string, DealRoomDocumentItem[]>): TreeNode[] {
  const sorted = [...folders]
    .filter((f) => f.path !== "/")
    .sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name));

  const map = new Map<string, TreeNode>();
  for (const folder of sorted) {
    map.set(folder.path, { folder, children: [], documents: docsByFolder.get(folder.path) ?? [] });
  }

  const roots: TreeNode[] = [];
  for (const folder of sorted) {
    const node = map.get(folder.path)!;
    const pp = parentPath(folder.path);
    if (pp === "/" || pp === null) {
      roots.push(node);
    } else {
      const parent = map.get(pp);
      if (parent) {
        parent.children.push(node);
      } else {
        roots.push(node);
      }
    }
  }

  return roots;
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

  const roots = useMemo(() => buildTree(folders, docsByFolder), [folders, docsByFolder]);

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
    const file = e.target.files?.[0];
    if (!file || !onFolderUpload) return;
    try {
      await onFolderUpload(file, folderPath);
    } finally {
      e.target.value = "";
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

  const renderFolder = (node: TreeNode, depth: number) => {
    const isExpanded = expanded.has(node.folder.path);
    const docs = node.documents;
    const isSelected = isNavigator && selectedFolderPath === node.folder.path;

    const documentCount = docs.length;
    const subfolderCount = node.children.length;
    const metadata: string[] = [];
    if (documentCount > 0 || subfolderCount === 0) {
      metadata.push(t("folders.documentsCount", { count: documentCount }));
    }
    if (subfolderCount > 0) {
      metadata.push(t("folders.foldersCount", { count: subfolderCount }));
    }

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
                    accept={td("upload.supportedTypes")}
                    data-testid={`folder-upload-input-${node.folder.path}`}
                    tabIndex={-1}
                    aria-hidden
                    className="sr-only"
                    onChange={(e) => void handleFolderUploadChange(e, node.folder.path)}
                  />
                )}
                <Popover>
                  <PopoverTrigger
                    className={buttonVariants({ variant: "ghost", size: "icon-sm" })}
                    onClick={(e) => e.stopPropagation()}
                    aria-label={t("folders.actions", { name: node.folder.name })}
                  >
                    <DotsThreeVertical size={18} />
                  </PopoverTrigger>
                  <PopoverContent align="end" side="bottom" sideOffset={4} className="w-auto min-w-[3.5rem] p-2">
                    <TooltipProvider delay={100}>
                      <div className="flex flex-col gap-1">
                        <Tooltip>
                          <TooltipTrigger
                            className={buttonVariants({ variant: "ghost", size: "icon-sm" })}
                            onClick={(e) => {
                              e.stopPropagation();
                              startCreate(node.folder.path);
                            }}
                            aria-label={t("folders.newSubfolder")}
                          >
                            <Plus size={18} />
                          </TooltipTrigger>
                          <TooltipContent side="right">
                            {t("folders.newSubfolder")}
                          </TooltipContent>
                        </Tooltip>

                        {onFolderUpload && (
                          <Tooltip>
                            <TooltipTrigger
                              className={buttonVariants({ variant: "ghost", size: "icon-sm" })}
                              onClick={(e) => {
                                e.stopPropagation();
                                fileInputRefs.current.get(node.folder.path)?.click();
                              }}
                              aria-label={t("folders.addFile")}
                            >
                              <UploadSimple size={18} />
                            </TooltipTrigger>
                            <TooltipContent side="right">
                              {t("folders.addFile")}
                            </TooltipContent>
                          </Tooltip>
                        )}

                        <Tooltip>
                          <TooltipTrigger
                            className={buttonVariants({ variant: "ghost", size: "icon-sm" })}
                            onClick={(e) => {
                              e.stopPropagation();
                              startRename(node.folder);
                            }}
                            aria-label={t("folders.rename")}
                          >
                            <PencilSimple size={18} />
                          </TooltipTrigger>
                          <TooltipContent side="right">
                            {t("folders.rename")}
                          </TooltipContent>
                        </Tooltip>

                        <Tooltip>
                          <TooltipTrigger
                            className={buttonVariants({ variant: "ghost", size: "icon-sm" })}
                            onClick={(e) => {
                              e.stopPropagation();
                              void handleDelete(node.folder);
                            }}
                            aria-label={tc("delete")}
                          >
                            <Trash size={18} className="text-destructive" />
                          </TooltipTrigger>
                          <TooltipContent side="right">
                            {tc("delete")}
                          </TooltipContent>
                        </Tooltip>
                      </div>
                    </TooltipProvider>
                  </PopoverContent>
                </Popover>
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
                    className="group flex cursor-pointer items-center gap-3 rounded-md px-2.5 py-1.5 text-sm text-foreground hover:bg-muted/40"
                    onClick={() => onDocumentOpen?.(doc.document_id)}
                    title={t("documents.clickToOpen")}
                  >
                    <div className="flex h-5 w-8 shrink-0 items-center justify-center">
                      <FileText size={15} className="text-muted-foreground/80" />
                    </div>
                    <span className="truncate hover:text-foreground">{doc.title}</span>
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

  return (
    <div className="space-y-1" data-testid="folder-tree">
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

      <div className="space-y-1">{roots.map((root) => renderFolder(root, 0))}</div>

      {contextMenu && (
        <ContextMenu x={contextMenu.x} y={contextMenu.y} onClose={() => setContextMenu(null)}>
          {isAdmin && (
            <>
              <button
                className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent hover:text-accent-foreground"
                onClick={() => {
                  startCreate(contextMenu.folder.path);
                  setContextMenu(null);
                }}
              >
                <Plus size={14} />
                {t("folders.newSubfolder")}
              </button>
              {onDocumentsAdd && workspaceDocuments && workspaceDocuments.length > 0 && !isNavigator && (
                <button
                  className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent hover:text-accent-foreground"
                  onClick={() => {
                    setAddToFolder(contextMenu.folder.path);
                    setContextMenu(null);
                  }}
                >
                  <FileText size={14} />
                  {t("folders.addFile")}
                </button>
              )}
              <button
                className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent hover:text-accent-foreground"
                onClick={() => startRename(contextMenu.folder)}
              >
                <PencilSimple size={14} />
                {t("folders.rename")}
              </button>
              <div className="my-1 h-px bg-border" />
              <button
                className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-destructive hover:bg-destructive/10"
                onClick={() => void handleDelete(contextMenu.folder)}
              >
                <Trash size={14} />
                {tc("delete")}
              </button>
            </>
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
