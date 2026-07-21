import { useMemo, useState } from "react";
import { Check, Minus, Folder } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { buildFolderTree, type FolderTreeNode } from "@/lib/folderTree";
import type { DealRoomFolder, DealRoomFolderDocs } from "@/types";

interface DocumentScopeSectionProps {
  folders: DealRoomFolder[];
  documents: DealRoomFolderDocs[];
  selectedPaths: string[];
  scopeMode: "full" | "allowlist";
  onChange: (next: { scopeMode: "full" | "allowlist"; selectedPaths: string[] }) => void;
  disabled?: boolean;
}

function isPathIncluded(path: string, selectedPaths: string[]): boolean {
  return selectedPaths.some(
    (s) => s === path || (path.length > s.length && path.startsWith(`${s}/`))
  );
}

function isPathIndeterminate(path: string, selectedPaths: string[]): boolean {
  if (isPathIncluded(path, selectedPaths)) return false;
  return selectedPaths.some((s) => s.startsWith(`${path}/`));
}

function togglePath(path: string, selectedPaths: string[]): string[] {
  if (selectedPaths.includes(path)) {
    return selectedPaths.filter((s) => s !== path);
  }
  if (isPathIncluded(path, selectedPaths)) {
    // Already included by an ancestor; toggling the exact path would add a
    // redundant entry. Keep the current selection to avoid noisy payloads.
    return selectedPaths;
  }
  // Add this path and remove any descendants that are now redundant.
  return [...selectedPaths.filter((s) => !s.startsWith(`${path}/`)), path];
}

function folderState(
  path: string,
  selectedPaths: string[]
): "all" | "some" | "none" {
  if (isPathIncluded(path, selectedPaths)) return "all";
  if (isPathIndeterminate(path, selectedPaths)) return "some";
  return "none";
}

function IndeterminateCheckbox({
  state,
  onClick,
  disabled = false,
}: {
  state: "all" | "some" | "none";
  onClick?: () => void;
  disabled?: boolean;
}) {
  return (
    <span
      role="checkbox"
      aria-checked={state === "all" ? true : state === "some" ? "mixed" : false}
      onClick={
        disabled || !onClick
          ? undefined
          : (e) => {
              e.stopPropagation();
              onClick();
            }
      }
      className={cn(
        "flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-all duration-150",
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

function countDocumentsByPath(
  documents: DealRoomFolderDocs[]
): Map<string, number> {
  const map = new Map<string, number>();
  for (const folder of documents) {
    map.set(folder.folder, (folder.documents ?? []).length);
  }
  return map;
}

function totalDocumentsInScope(
  documents: DealRoomFolderDocs[],
  selectedPaths: string[]
): number {
  let count = 0;
  for (const folder of documents) {
    const folderPath = folder.folder;
    if (
      selectedPaths.length === 0 ||
      selectedPaths.some(
        (s) =>
          s === folderPath ||
          (folderPath.length > s.length && folderPath.startsWith(`${s}/`))
      )
    ) {
      count += (folder.documents ?? []).length;
    }
  }
  return count;
}

function totalDocumentsInFolder(
  node: FolderTreeNode<DealRoomFolder>,
  directCounts: Map<string, number>
): number {
  let total = directCounts.get(node.folder.path) ?? 0;
  for (const child of node.children) {
    total += totalDocumentsInFolder(child, directCounts);
  }
  return total;
}

function FolderNode({
  node,
  depth,
  selectedPaths,
  directCounts,
  onToggle,
}: {
  node: FolderTreeNode<DealRoomFolder>;
  depth: number;
  selectedPaths: string[];
  directCounts: Map<string, number>;
  onToggle: (path: string) => void;
}) {
  const [expanded, setExpanded] = useState(true);
  const state = folderState(node.folder.path, selectedPaths);
  const totalDocs = totalDocumentsInFolder(node, directCounts);

  return (
    <div className="select-none">
      <div
        data-testid={`folder-row-${node.folder.path}`}
        className={cn(
          "flex items-center gap-2 rounded-md py-1.5 pr-2 hover:bg-muted/40",
          node.children.length > 0 && "cursor-pointer"
        )}
        style={{ paddingLeft: `${depth * 16}px` }}
        onClick={() => {
          if (node.children.length > 0) {
            setExpanded((v) => !v);
          }
        }}
      >
        <IndeterminateCheckbox
          state={state}
          onClick={() => onToggle(node.folder.path)}
        />
        <div className="flex min-w-0 items-center gap-2">
          <Folder
            size={16}
            className={cn(
              "shrink-0",
              state === "all" || state === "some"
                ? "text-primary"
                : "text-muted-foreground"
            )}
          />
          <span className="truncate text-sm font-medium">{node.folder.name}</span>
          {totalDocs > 0 && (
            <span className="text-xs text-muted-foreground">
              ({totalDocs})
            </span>
          )}
        </div>
      </div>
      {expanded && node.children.length > 0 && (
        <div className="space-y-0.5">
          {node.children.map((child) => (
            <FolderNode
              key={child.folder.path}
              node={child}
              depth={depth + 1}
              selectedPaths={selectedPaths}
              directCounts={directCounts}
              onToggle={onToggle}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function DocumentScopeSection({
  folders,
  documents,
  selectedPaths,
  scopeMode,
  onChange,
  disabled,
}: DocumentScopeSectionProps) {
  const { t } = useTranslation("linkShare");

  const tree = useMemo(() => buildFolderTree(folders), [folders]);
  const allRootPaths = useMemo(
    () => (tree.length > 0 ? tree.map((node) => node.folder.path) : folders.map((f) => f.path)),
    [tree, folders]
  );
  // Legacy full mode is displayed as all roots checked until the owner edits scope.
  const effectivePaths = scopeMode === "full" ? allRootPaths : selectedPaths;
  const directCounts = useMemo(
    () => countDocumentsByPath(documents),
    [documents]
  );
  const totalDocs = useMemo(
    () => totalDocumentsInScope(documents, effectivePaths),
    [documents, effectivePaths]
  );
  const selectedCount = effectivePaths.length;

  const emitAllowlist = (paths: string[]) => {
    onChange({ scopeMode: "allowlist", selectedPaths: paths });
  };

  const handleToggle = (path: string) => {
    if (disabled) return;
    const base = scopeMode === "full" ? allRootPaths : selectedPaths;
    emitAllowlist(togglePath(path, base));
  };

  const handleClearAll = () => {
    if (disabled) return;
    // Empty allowlist denies all visitor document access.
    emitAllowlist([]);
  };

  const handleSelectAll = () => {
    if (disabled) return;
    emitAllowlist(allRootPaths);
  };

  const isAllowlist = scopeMode === "allowlist";
  const hasSelection = effectivePaths.length > 0;

  return (
    <div className="flex h-full flex-col space-y-3 rounded-lg border border-border bg-muted/30 p-4">
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">
          {scopeMode === "full"
            ? t("share.documentScope.legacyAllDocuments")
            : hasSelection
              ? t("share.documentScope.selectedDocuments", {
                  folders: selectedCount,
                  documents: totalDocs,
                })
              : t("share.documentScope.noneAuthorized")}
        </p>
        <button
          type="button"
          disabled={disabled}
          onClick={isAllowlist && hasSelection ? handleClearAll : handleSelectAll}
          className="shrink-0 text-xs text-primary hover:underline disabled:pointer-events-none disabled:text-muted-foreground"
        >
          {isAllowlist && hasSelection
            ? t("share.documentScope.deselectAll")
            : t("share.documentScope.selectAll")}
        </button>
      </div>

      {folders.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("share.documentScope.empty")}</p>
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto rounded-md border border-border bg-background p-2">
          {tree.length === 0 ? (
            folders.map((folder) => (
              <FolderNode
                key={folder.path}
                node={{ folder, children: [] }}
                depth={0}
                selectedPaths={effectivePaths}
                directCounts={directCounts}
                onToggle={handleToggle}
              />
            ))
          ) : (
            tree.map((node) => (
              <FolderNode
                key={node.folder.path}
                node={node}
                depth={0}
                selectedPaths={effectivePaths}
                directCounts={directCounts}
                onToggle={handleToggle}
              />
            ))
          )}
        </div>
      )}
    </div>
  );
}
