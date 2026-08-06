import { useMemo, useState } from "react";
import { Check, Folder, LockSimple, Minus } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { buildFolderTree, type FolderTreeNode } from "@/lib/folderTree";
import {
  isFolderPathLocked,
  lockedFolderPathSet,
  selectableRootFolderPaths,
  stripLockedFolderPaths,
} from "@/lib/dealRoomFolderLock";
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
    (s) => s === path || (path.length > s.length && path.startsWith(`${s}/`)),
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
    return selectedPaths;
  }
  return [...selectedPaths.filter((s) => !s.startsWith(`${path}/`)), path];
}

function folderState(path: string, selectedPaths: string[]): "all" | "some" | "none" {
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
        "flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-all duration-150",
        disabled
          ? "cursor-not-allowed border-border/50 bg-muted text-transparent"
          : "cursor-pointer hover:border-primary/50",
        !disabled && state === "all"
          ? "border-primary bg-primary text-primary-foreground"
          : !disabled && state === "some"
            ? "border-primary bg-primary/10 text-primary"
            : !disabled && "border-border/80 bg-background text-transparent",
      )}
    >
      {state === "all" && <Check size={10} weight="bold" />}
      {state === "some" && <Minus size={10} weight="bold" />}
    </span>
  );
}

function countDocumentsByPath(documents: DealRoomFolderDocs[]): Map<string, number> {
  const map = new Map<string, number>();
  for (const folder of documents) {
    map.set(folder.folder, (folder.documents ?? []).length);
  }
  return map;
}

function totalDocumentsInScope(
  documents: DealRoomFolderDocs[],
  selectedPaths: string[],
  lockedFolders: Set<string>,
): number {
  let count = 0;
  for (const folder of documents) {
    const folderPath = folder.folder;
    if (isFolderPathLocked(folderPath, lockedFolders)) continue;
    if (
      selectedPaths.length === 0 ||
      selectedPaths.some(
        (s) =>
          s === folderPath ||
          (folderPath.length > s.length && folderPath.startsWith(`${s}/`)),
      )
    ) {
      count += (folder.documents ?? []).length;
    }
  }
  return count;
}

function totalDocumentsInFolder(
  node: FolderTreeNode<DealRoomFolder>,
  directCounts: Map<string, number>,
  lockedFolders: Set<string>,
): number {
  if (isFolderPathLocked(node.folder.path, lockedFolders)) return 0;
  let total = directCounts.get(node.folder.path) ?? 0;
  for (const child of node.children) {
    total += totalDocumentsInFolder(child, directCounts, lockedFolders);
  }
  return total;
}

function FolderNode({
  node,
  depth,
  selectedPaths,
  directCounts,
  lockedFolders,
  onToggle,
}: {
  node: FolderTreeNode<DealRoomFolder>;
  depth: number;
  selectedPaths: string[];
  directCounts: Map<string, number>;
  lockedFolders: Set<string>;
  onToggle: (path: string) => void;
}) {
  const { t } = useTranslation("dealRooms");
  const [expanded, setExpanded] = useState(true);
  const folderLocked = isFolderPathLocked(node.folder.path, lockedFolders);
  const state = folderLocked ? "none" : folderState(node.folder.path, selectedPaths);
  const totalDocs = totalDocumentsInFolder(node, directCounts, lockedFolders);

  return (
    <div className="select-none">
      <div
        data-testid={`folder-row-${node.folder.path}`}
        className={cn(
          "flex items-center gap-2 rounded-md py-1.5 pr-2 hover:bg-muted/40",
          node.children.length > 0 && "cursor-pointer",
          folderLocked && "opacity-60",
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
          disabled={folderLocked}
          onClick={() => onToggle(node.folder.path)}
        />
        <div className="flex min-w-0 items-center gap-2">
          <Folder
            size={16}
            className={cn(
              "shrink-0",
              state === "all" || state === "some"
                ? "text-primary"
                : "text-muted-foreground",
            )}
          />
          <span className="truncate text-sm font-medium">{node.folder.name}</span>
          {folderLocked ? (
            <span
              className="inline-flex items-center gap-1 text-xs text-muted-foreground"
              title={t("folders.lockedBadge")}
            >
              <LockSimple size={12} aria-hidden />
              {t("folders.lockedBadge")}
            </span>
          ) : null}
          {totalDocs > 0 && (
            <span className="text-xs text-muted-foreground">({totalDocs})</span>
          )}
        </div>
      </div>
      {expanded && node.children.length > 0 ? (
        <div className="space-y-0.5">
          {node.children.map((child) => (
            <FolderNode
              key={child.folder.path}
              node={child}
              depth={depth + 1}
              selectedPaths={selectedPaths}
              directCounts={directCounts}
              lockedFolders={lockedFolders}
              onToggle={onToggle}
            />
          ))}
        </div>
      ) : null}
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
    [tree, folders],
  );
  const lockedFolders = useMemo(() => lockedFolderPathSet(folders), [folders]);
  const selectableRootPaths = useMemo(
    () => selectableRootFolderPaths(folders, allRootPaths),
    [folders, allRootPaths],
  );
  const shareableSelectedPaths = useMemo(
    () => stripLockedFolderPaths(selectedPaths, lockedFolders),
    [selectedPaths, lockedFolders],
  );
  const effectivePaths =
    scopeMode === "full" ? selectableRootPaths : shareableSelectedPaths;
  const directCounts = useMemo(() => countDocumentsByPath(documents), [documents]);
  const totalDocs = useMemo(
    () => totalDocumentsInScope(documents, effectivePaths, lockedFolders),
    [documents, effectivePaths, lockedFolders],
  );
  const selectedCount = effectivePaths.length;
  const hasLockedFolders = lockedFolders.size > 0;

  const emitAllowlist = (paths: string[]) => {
    onChange({
      scopeMode: "allowlist",
      selectedPaths: stripLockedFolderPaths(paths, lockedFolders),
    });
  };

  const handleModeChange = (mode: "full" | "allowlist") => {
    if (disabled || mode === scopeMode) return;
    if (mode === "full") {
      onChange({ scopeMode: "full", selectedPaths: [] });
      return;
    }
    emitAllowlist(selectableRootPaths);
  };

  const handleToggle = (path: string) => {
    if (disabled || isFolderPathLocked(path, lockedFolders)) return;
    const base = scopeMode === "full" ? selectableRootPaths : shareableSelectedPaths;
    emitAllowlist(togglePath(path, base));
  };

  const handleClearAll = () => {
    if (disabled) return;
    emitAllowlist([]);
  };

  const handleSelectAll = () => {
    if (disabled) return;
    emitAllowlist(selectableRootPaths);
  };

  const isAllowlist = scopeMode === "allowlist";
  const hasSelection = effectivePaths.length > 0;

  return (
    <div
      className="flex h-full flex-col space-y-3 rounded-lg border border-border bg-muted/30 p-4"
      data-testid="document-scope-section"
    >
      <div
        role="radiogroup"
        aria-label={t("share.documentScope.modeLabel")}
        className="grid grid-cols-2 gap-2"
        data-testid="document-scope-mode"
      >
        {(
          [
            { mode: "full" as const, label: t("share.documentScope.modeAll") },
            { mode: "allowlist" as const, label: t("share.documentScope.modeCustom") },
          ] as const
        ).map(({ mode, label }) => {
          const selected = scopeMode === mode;
          return (
            <button
              key={mode}
              type="button"
              role="radio"
              aria-checked={selected}
              disabled={disabled}
              data-testid={`document-scope-mode-${mode}`}
              onClick={() => handleModeChange(mode)}
              className={cn(
                "rounded-md border px-3 py-2 text-xs transition-colors",
                selected
                  ? "border-primary/40 bg-background text-foreground shadow-sm"
                  : "border-transparent bg-transparent text-muted-foreground hover:bg-background/60 hover:text-foreground",
                disabled && "pointer-events-none opacity-50",
              )}
            >
              {label}
            </button>
          );
        })}
      </div>

      <div className="flex items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">
          {scopeMode === "full"
            ? hasLockedFolders
              ? t("share.documentScope.allDocumentsExceptLocked")
              : t("share.documentScope.allDocuments")
            : hasSelection
              ? t("share.documentScope.selectedDocuments", {
                  folders: selectedCount,
                  documents: totalDocs,
                })
              : t("share.documentScope.noneAuthorized")}
        </p>
        {isAllowlist ? (
          <button
            type="button"
            disabled={disabled}
            onClick={hasSelection ? handleClearAll : handleSelectAll}
            className="shrink-0 text-xs text-primary hover:underline disabled:pointer-events-none disabled:text-muted-foreground"
          >
            {hasSelection
              ? t("share.documentScope.deselectAll")
              : t("share.documentScope.selectAll")}
          </button>
        ) : null}
      </div>

      {isAllowlist ? (
        folders.length === 0 ? (
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
                  lockedFolders={lockedFolders}
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
                  lockedFolders={lockedFolders}
                  onToggle={handleToggle}
                />
              ))
            )}
          </div>
        )
      ) : null}
    </div>
  );
}
