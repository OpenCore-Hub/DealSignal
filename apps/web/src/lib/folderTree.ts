export interface FolderTreeNode<T> {
  folder: T;
  children: FolderTreeNode<T>[];
}

export interface FolderTreeItem {
  path: string;
  name: string;
  sort_order: number;
}

function parentPath(path: string): string | null {
  if (path === "/") return null;
  const idx = path.lastIndexOf("/");
  if (idx <= 0) return "/";
  return path.slice(0, idx);
}

export function buildFolderTree<T extends FolderTreeItem>(
  folders: T[]
): FolderTreeNode<T>[] {
  const sorted = [...folders]
    .filter((f) => f.path !== "/")
    .sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name));

  const map = new Map<string, FolderTreeNode<T>>();
  for (const folder of sorted) {
    map.set(folder.path, {
      folder,
      children: [],
    });
  }

  const roots: FolderTreeNode<T>[] = [];
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
