import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import {
  Folder,
  FolderOpen,
  Check,
  Minus,
} from "@phosphor-icons/react";
import { api } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { DealRoomFolder } from "@/types";
import { cn } from "@/lib/utils";
import { buildFolderTree, type FolderTreeNode } from "@/lib/folderTree";

interface DocumentsTabProps {
  roomId: string;
  selectedPaths: string[];
  onChange: (paths: string[]) => void;
}


function collectDescendantPaths(node: FolderTreeNode<DealRoomFolder>): string[] {
  const paths: string[] = [];
  for (const child of node.children) {
    paths.push(child.folder.path);
    paths.push(...collectDescendantPaths(child));
  }
  return paths;
}

function folderSelectionState(
  node: FolderTreeNode<DealRoomFolder>,
  selected: Set<string>
): "all" | "some" | "none" {
  const selfSelected = selected.has(node.folder.path);
  const descendantSelected = collectDescendantPaths(node).some((p) => selected.has(p));
  if (selfSelected) return "all";
  if (descendantSelected) return "some";
  return "none";
}

export function DocumentsTab({ roomId, selectedPaths, onChange }: DocumentsTabProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");

  const { data, loading, error } = useAsyncData(
    async () => {
      const [foldersRes, docsRes] = await Promise.all([
        api.getDealRoomFolders(roomId),
        api.getDealRoomDocuments(roomId),
      ]);
      return { folders: foldersRes.data, folderDocs: docsRes.data };
    },
    [roomId]
  );

  const { roots, totalDocCount } = useMemo(() => {
    return {
      roots: buildFolderTree(data?.folders ?? []),
      totalDocCount: (data?.folderDocs ?? []).reduce((sum, fd) => sum + fd.documents.length, 0),
    };
  }, [data]);

  const selectedSet = useMemo(() => new Set(selectedPaths), [selectedPaths]);

  const toggleFolder = (node: FolderTreeNode<DealRoomFolder>) => {
    const path = node.folder.path;
    const descendants = collectDescendantPaths(node);
    const next = new Set(selectedSet);

    if (next.has(path)) {
      next.delete(path);
      for (const p of descendants) next.delete(p);
    } else {
      next.add(path);
      for (const p of descendants) next.add(p);
    }
    onChange(Array.from(next));
  };

  const selectAll = () => {
    const paths: string[] = [];
    const collect = (n: FolderTreeNode<DealRoomFolder>) => {
      paths.push(n.folder.path);
      for (const child of n.children) collect(child);
    };
    for (const root of roots) collect(root);
    onChange(paths);
  };
  const clearAll = () => onChange([]);

  const rootState = useMemo(() => {
    if (roots.length === 0) return "none";
    const rootPaths = roots.map((r) => r.folder.path);
    const allSelected = rootPaths.every((p) => selectedSet.has(p));
    const someSelected = rootPaths.some((p) => selectedSet.has(p)) ||
      roots.some((r) => collectDescendantPaths(r).some((p) => selectedSet.has(p)));
    if (allSelected) return "all";
    if (someSelected) return "some";
    return "none";
  }, [roots, selectedSet]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <div className="relative h-8 w-8">
          <span className="absolute inset-0 animate-spin rounded-full border-2 border-muted border-t-primary" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-xl border border-destructive/20 bg-destructive/5 px-4 py-8 text-center text-sm text-destructive">
        {tc("error.loadFailed")}: {error}
      </div>
    );
  }

  if (roots.length === 0) {
    return (
      <div className="rounded-2xl border border-dashed border-border bg-muted/20 p-8 text-center">
        <FolderOpen size={40} className="mx-auto mb-3 text-muted-foreground/60" />
        <p className="text-sm font-medium text-foreground">{t("documentList.emptyTitle")}</p>
        <p className="mt-1 text-xs text-muted-foreground">{t("documentList.emptyDescription")}</p>
      </div>
    );
  }

  const allSelected = rootState === "all";
  const someSelected = rootState === "some";

  return (
    <div className="rounded-2xl border border-border/60 bg-background p-4 shadow-[0_1px_2px_rgba(0,0,0,0.04)]">
      <div className="flex items-center justify-end border-b border-border/60 pb-3">
        <button
          type="button"
          onClick={() => (allSelected ? clearAll() : selectAll())}
          className="flex items-center gap-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          <IndeterminateCheckbox
            state={allSelected ? "all" : someSelected ? "some" : "none"}
            onClick={() => (allSelected ? clearAll() : selectAll())}
            label={t("share.documents.selectDeselectAll")}
          />
          <span>{t("share.documents.selectDeselectAll")}</span>
        </button>
      </div>

      {totalDocCount === 0 && (
        <div className="rounded-lg border border-dashed border-border bg-muted/20 px-4 py-3 text-center text-xs text-muted-foreground">
          {t("share.documents.noDocumentsInFolders")}
        </div>
      )}

      <div className="max-h-[45vh] overflow-y-auto pt-2">
        <div className="space-y-0.5">
          {roots.map((root) => (
            <FolderNode
              key={root.folder.path}
              node={root}
              depth={0}
              selectedSet={selectedSet}
              onToggleFolder={toggleFolder}
              t={t}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

interface FolderNodeProps {
  node: FolderTreeNode<DealRoomFolder>;
  depth: number;
  selectedSet: Set<string>;
  onToggleFolder: (node: FolderTreeNode<DealRoomFolder>) => void;
  t: (key: string, options?: Record<string, unknown>) => string;
}

function FolderNode({
  node,
  depth,
  selectedSet,
  onToggleFolder,
  t,
}: FolderNodeProps) {
  const state = folderSelectionState(node, selectedSet);

  return (
    <div className="select-none">
      <div
        onClick={() => onToggleFolder(node)}
        className={cn(
          "group flex items-center gap-3 rounded-lg px-2 py-2 transition-all duration-200 ease-[cubic-bezier(0.32,0.72,0,1)] cursor-pointer hover:bg-muted/40",
          state === "all" && "bg-primary/[0.06]"
        )}
        style={{ paddingLeft: `${depth * 20 + 8}px` }}
      >
        <IndeterminateCheckbox
          state={state}
          onClick={() => onToggleFolder(node)}
          label={node.folder.name}
        />

        <Folder
          size={18}
          className={cn(
            "shrink-0 transition-colors duration-200",
            state === "all" ? "text-primary" : "text-muted-foreground"
          )}
        />

        <span className="truncate text-sm text-foreground">{node.folder.name}</span>
      </div>

      {node.children.length > 0 && (
        <div className="space-y-0.5 pt-0.5">
          {node.children.map((child) => (
            <FolderNode
              key={child.folder.path}
              node={child}
              depth={depth + 1}
              selectedSet={selectedSet}
              onToggleFolder={onToggleFolder}
              t={t}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function IndeterminateCheckbox({
  state,
  onClick,
  label,
  disabled = false,
}: {
  state: "all" | "some" | "none";
  onClick?: () => void;
  label: string;
  disabled?: boolean;
}) {
  return (
    <span
      role="checkbox"
      aria-checked={state === "all" ? true : state === "some" ? "mixed" : false}
      aria-label={label}
      aria-disabled={disabled}
      onClick={
        disabled || !onClick
          ? undefined
          : (e) => {
              e.stopPropagation();
              onClick();
            }
      }
      className={cn(
        "flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-all duration-200 ease-[cubic-bezier(0.32,0.72,0,1)]",
        disabled
          ? "cursor-not-allowed border-border/50 bg-muted text-transparent"
          : "cursor-pointer hover:border-primary/50",
        !disabled && state === "all"
          ? "border-primary bg-primary text-primary-foreground"
          : !disabled && state === "some"
            ? "border-primary bg-primary/10 text-primary"
            : !disabled && "border-border/80 bg-background text-transparent"
      )}
    >
      {state === "all" && <Check size={10} weight="bold" />}
      {state === "some" && <Minus size={10} weight="bold" />}
    </span>
  );
}
