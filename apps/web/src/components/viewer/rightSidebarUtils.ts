interface DocSummary {
  id: string;
  title: string;
  pageCount: number;
  folderPath?: string;
}

export interface FolderGroup {
  path: string;
  name: string;
  items: { doc: DocSummary; index: number }[];
}

export function normalizeFolderPath(path?: string): string {
  if (!path || path === "") return "/";
  return path.length > 1 && path.endsWith("/") ? path.replace(/\/+$/, "") : path;
}

function folderDisplayName(path: string, rootLabel: string): string {
  const segments = path.split("/").filter(Boolean);
  return segments.length === 0 ? rootLabel : (segments[segments.length - 1] ?? path);
}

export function shouldGroupDocumentsByFolder(documents: DocSummary[]): boolean {
  const paths = new Set(documents.map((d) => normalizeFolderPath(d.folderPath)));
  if (paths.size > 1) return true;
  const only = [...paths][0];
  return only !== undefined && only !== "/";
}

export function groupDocumentsByFolder(
  documents: DocSummary[],
  rootLabel: string
): FolderGroup[] {
  const map = new Map<string, FolderGroup>();
  documents.forEach((doc, index) => {
    const path = normalizeFolderPath(doc.folderPath);
    let group = map.get(path);
    if (!group) {
      group = { path, name: folderDisplayName(path, rootLabel), items: [] };
      map.set(path, group);
    }
    group.items.push({ doc, index });
  });
  return Array.from(map.values()).sort((a, b) => a.path.localeCompare(b.path));
}
