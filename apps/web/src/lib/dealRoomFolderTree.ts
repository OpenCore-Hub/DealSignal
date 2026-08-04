import type { DealRoomDocumentItem, DealRoomFolder } from "@/types";

export type ResourceTypeFilter = "all" | "folders" | "files";
export type ResourceLockFilter = "all" | "locked" | "unlocked";

export interface FolderTreeNode {
  folder: DealRoomFolder;
  children: FolderTreeNode[];
  documents: DealRoomDocumentItem[];
}

function parentPath(path: string): string | null {
  if (path === "/") return null;
  const idx = path.lastIndexOf("/");
  if (idx <= 0) return "/";
  return path.slice(0, idx);
}

export function buildFolderTree(
  folders: DealRoomFolder[],
  docsByFolder: Map<string, DealRoomDocumentItem[]>,
): FolderTreeNode[] {
  const sorted = [...folders]
    .filter((f) => f.path !== "/")
    .sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name));

  const map = new Map<string, FolderTreeNode>();
  for (const folder of sorted) {
    map.set(folder.path, {
      folder,
      children: [],
      documents: docsByFolder.get(folder.path) ?? [],
    });
  }

  const roots: FolderTreeNode[] = [];
  for (const folder of sorted) {
    const node = map.get(folder.path)!;
    const pp = parentPath(folder.path);
    if (pp === "/" || pp === null) {
      roots.push(node);
    } else {
      const parent = map.get(pp);
      if (parent) parent.children.push(node);
      else roots.push(node);
    }
  }
  return roots;
}

function matchesLock(locked: boolean | undefined, filter: ResourceLockFilter): boolean {
  if (filter === "all") return true;
  const isLocked = Boolean(locked);
  return filter === "locked" ? isLocked : !isLocked;
}

function matchesQuery(text: string, query: string): boolean {
  if (!query) return true;
  return text.toLowerCase().includes(query);
}

/**
 * Filters a folder tree by search text, type, and lock state.
 * Matching descendants keep their ancestors so the tree stays navigable.
 */
export function filterFolderTree(
  roots: FolderTreeNode[],
  opts: {
    query: string;
    type: ResourceTypeFilter;
    lock: ResourceLockFilter;
  },
): { roots: FolderTreeNode[]; expandPaths: Set<string> } {
  const query = opts.query.trim().toLowerCase();
  const expandPaths = new Set<string>();

  const visit = (node: FolderTreeNode): FolderTreeNode | null => {
    const childNodes = node.children
      .map(visit)
      .filter((n): n is FolderTreeNode => n != null);

    const folderNameMatch = matchesQuery(node.folder.name, query);
    const folderLockMatch = matchesLock(node.folder.locked, opts.lock);

    let documents = node.documents.filter((doc) => {
      if (!matchesQuery(doc.title, query)) return false;
      if (!matchesLock(doc.locked, opts.lock)) return false;
      return true;
    });

    if (opts.type === "folders") {
      documents = [];
    }

    const folderSelfVisible =
      opts.type !== "files" &&
      folderNameMatch &&
      folderLockMatch &&
      (opts.lock === "all" || matchesLock(node.folder.locked, opts.lock));

    // When type=files, keep folder shells that still have matching docs/children.
    const keepForChildren = childNodes.length > 0 || documents.length > 0;
    if (!folderSelfVisible && !keepForChildren) {
      return null;
    }

    if (keepForChildren || folderSelfVisible) {
      expandPaths.add(node.folder.path);
    }

    return {
      folder: node.folder,
      children: childNodes,
      documents,
    };
  };

  const filtered = roots.map(visit).filter((n): n is FolderTreeNode => n != null);
  return { roots: filtered, expandPaths };
}

export type SelectionKey = `folder:${string}` | `doc:${string}`;

export function folderSelectionKey(path: string): SelectionKey {
  return `folder:${path}`;
}

export function docSelectionKey(documentId: string): SelectionKey {
  return `doc:${documentId}`;
}

export function parseSelection(keys: Iterable<SelectionKey>): {
  folderPaths: string[];
  documentIds: string[];
} {
  const folderPaths: string[] = [];
  const documentIds: string[] = [];
  for (const key of keys) {
    if (key.startsWith("folder:")) folderPaths.push(key.slice("folder:".length));
    else if (key.startsWith("doc:")) documentIds.push(key.slice("doc:".length));
  }
  return { folderPaths, documentIds };
}

/**
 * Selected folders whose parent path is not also selected.
 * Selecting a parent cascades to children — bulk actions should target these roots.
 */
export function topmostSelectedFolderPaths(folderPaths: string[]): string[] {
  const set = new Set(folderPaths);
  return folderPaths.filter((path) => {
    for (const other of set) {
      if (other !== path && path.startsWith(`${other}/`)) return false;
    }
    return true;
  });
}

/** Folder key + all descendant folder/document keys. */
export function collectSubtreeKeys(node: FolderTreeNode): SelectionKey[] {
  const keys: SelectionKey[] = [folderSelectionKey(node.folder.path)];
  for (const doc of node.documents) {
    keys.push(docSelectionKey(doc.document_id));
  }
  for (const child of node.children) {
    keys.push(...collectSubtreeKeys(child));
  }
  return keys;
}

export type SubtreeCheckState = "checked" | "unchecked" | "indeterminate";

export function subtreeSelectionState(
  node: FolderTreeNode,
  selection: ReadonlySet<SelectionKey>,
): SubtreeCheckState {
  const keys = collectSubtreeKeys(node);
  let selected = 0;
  for (const key of keys) {
    if (selection.has(key)) selected += 1;
  }
  if (selected === 0) return "unchecked";
  if (selected === keys.length) return "checked";
  return "indeterminate";
}

export function findNodeByPath(roots: FolderTreeNode[], path: string): FolderTreeNode | null {
  for (const node of roots) {
    if (node.folder.path === path) return node;
    const nested = findNodeByPath(node.children, path);
    if (nested) return nested;
  }
  return null;
}

function findAncestorChain(
  roots: FolderTreeNode[],
  path: string,
): FolderTreeNode[] | null {
  const walk = (nodes: FolderTreeNode[], chain: FolderTreeNode[]): FolderTreeNode[] | null => {
    for (const node of nodes) {
      const next = [...chain, node];
      if (node.folder.path === path) return next;
      const nested = walk(node.children, next);
      if (nested) return nested;
    }
    return null;
  };
  return walk(roots, []);
}

function findAncestorChainForDocument(
  roots: FolderTreeNode[],
  documentId: string,
): FolderTreeNode[] | null {
  const walk = (nodes: FolderTreeNode[], chain: FolderTreeNode[]): FolderTreeNode[] | null => {
    for (const node of nodes) {
      const next = [...chain, node];
      if (node.documents.some((doc) => doc.document_id === documentId)) return next;
      const nested = walk(node.children, next);
      if (nested) return nested;
    }
    return null;
  };
  return walk(roots, []);
}

/** Keep parent folder keys in sync with whether their whole subtree is selected. */
function syncAncestorFolderKeys(selection: Set<SelectionKey>, chain: FolderTreeNode[]) {
  for (let i = chain.length - 1; i >= 0; i -= 1) {
    const node = chain[i]!;
    const folderKey = folderSelectionKey(node.folder.path);
    const descendantKeys = collectSubtreeKeys(node).filter((key) => key !== folderKey);
    const allDescendantsSelected =
      descendantKeys.length === 0 || descendantKeys.every((key) => selection.has(key));
    if (allDescendantsSelected) selection.add(folderKey);
    else selection.delete(folderKey);
  }
}

/** Select or clear a folder and every descendant folder/file. */
export function withFolderSubtreeSelection(
  selection: ReadonlySet<SelectionKey>,
  roots: FolderTreeNode[],
  node: FolderTreeNode,
  selected: boolean,
): Set<SelectionKey> {
  const next = new Set(selection);
  for (const key of collectSubtreeKeys(node)) {
    if (selected) next.add(key);
    else next.delete(key);
  }
  const chain = findAncestorChain(roots, node.folder.path);
  if (chain && chain.length > 1) {
    // Sync parents only (current node already matches `selected`).
    syncAncestorFolderKeys(next, chain.slice(0, -1));
  }
  return next;
}

/** Toggle a document and refresh ancestor folder checked state. */
export function withDocumentSelection(
  selection: ReadonlySet<SelectionKey>,
  roots: FolderTreeNode[],
  documentId: string,
  selected: boolean,
): Set<SelectionKey> {
  const next = new Set(selection);
  const key = docSelectionKey(documentId);
  if (selected) next.add(key);
  else next.delete(key);
  const chain = findAncestorChainForDocument(roots, documentId);
  if (chain) syncAncestorFolderKeys(next, chain);
  return next;
}
