import type { DealRoomFolder } from "@/types";

function normalizeFolderPath(path: string): string {
  const trimmed = path.trim();
  if (!trimmed) return "/";
  const withSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
  const noTrailing = withSlash.replace(/\/+$/, "");
  return noTrailing || "/";
}

/** Locked folder paths from deal-room folder metadata. */
export function lockedFolderPathSet(folders: DealRoomFolder[]): Set<string> {
  const out = new Set<string>();
  for (const folder of folders) {
    if (!folder.locked) continue;
    const path = normalizeFolderPath(folder.path);
    if (path !== "/") out.add(path);
  }
  return out;
}

/** Whether a folder path is locked or under a locked ancestor. */
export function isFolderPathLocked(path: string, lockedFolders: Set<string>): boolean {
  if (lockedFolders.size === 0) return false;
  const normalized = normalizeFolderPath(path);
  if (lockedFolders.has(normalized)) return true;
  for (const locked of lockedFolders) {
    if (locked === "/") continue;
    if (normalized.startsWith(`${locked}/`)) return true;
  }
  return false;
}

export function isFolderSelectable(folder: DealRoomFolder, lockedFolders: Set<string>): boolean {
  return !isFolderPathLocked(folder.path, lockedFolders);
}

/** Root folder paths that can be included in a share-link allowlist. */
export function selectableRootFolderPaths(
  folders: DealRoomFolder[],
  treeRoots: string[],
): string[] {
  const locked = lockedFolderPathSet(folders);
  const roots = treeRoots.length > 0 ? treeRoots : folders.map((f) => f.path);
  return roots.filter((path) => !isFolderPathLocked(path, locked));
}

export function stripLockedFolderPaths(paths: string[], lockedFolders: Set<string>): string[] {
  return paths.filter((path) => !isFolderPathLocked(path, lockedFolders));
}
