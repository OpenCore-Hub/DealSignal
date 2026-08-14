export type CreateFolderPayload = {
  name: string;
  path: string;
  description?: string;
};

export type CreateFolderDraft = {
  id: string;
  name: string;
  path: string;
  description?: string;
  selected: boolean;
  parentId: string | null;
};

function slugify(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function uniquePath(path: string, used: Set<string>): string {
  let candidate = path;
  let n = 2;
  while (used.has(candidate)) {
    candidate = `${path}-${n}`;
    n += 1;
  }
  return candidate;
}

function pathSegments(path: string): string[] {
  return path.split("/").filter(Boolean);
}

export function pathFromFolderName(name: string, used: Set<string>): string {
  const base = slugify(name) || "folder";
  return uniquePath(`/${base}`, used);
}

export function seedCreateFolderDrafts(
  folders: { name: string; path?: string; description?: string }[] | null | undefined,
): CreateFolderDraft[] {
  const raw = (folders ?? []).map((folder, index) => {
    const path = folder.path && folder.path !== "/" ? folder.path : "";
    return {
      id: `folder-${index}-${path || folder.name}`,
      name: folder.name,
      path,
      description: folder.description?.trim() || undefined,
      selected: true,
      parentId: null as string | null,
      segments: pathSegments(path),
    };
  });

  const byTop = new Map<string, string>();
  for (const item of raw) {
    if (item.segments.length === 1) {
      byTop.set(item.segments[0] ?? "", item.id);
    }
  }

  return raw.map((item) => {
    const parentKey = item.segments.length >= 2 ? item.segments[0] : "";
    const parentId = item.segments.length >= 2 ? (byTop.get(parentKey) ?? null) : null;
    return {
      id: item.id,
      name: item.name,
      path: item.segments.length > 2 ? `/${item.segments.slice(0, 2).join("/")}` : item.path,
      description: item.description,
      selected: item.selected,
      parentId,
    };
  });
}

export function groupCreateFolderDrafts(drafts: CreateFolderDraft[]): {
  root: CreateFolderDraft;
  children: CreateFolderDraft[];
}[] {
  return drafts
    .filter((draft) => !draft.parentId)
    .map((root) => ({
      root,
      children: drafts.filter((draft) => draft.parentId === root.id),
    }));
}

function folderPayload(name: string, path: string, description?: string): CreateFolderPayload {
  const desc = description?.trim();
  return desc ? { name, path, description: desc } : { name, path };
}

export function toCreateFolderPayload(drafts: CreateFolderDraft[]): CreateFolderPayload[] {
  const used = new Set<string>();
  const out: CreateFolderPayload[] = [];

  for (const group of groupCreateFolderDrafts(drafts)) {
    if (!group.root.selected) continue;
    const name = group.root.name.trim();
    if (!name) continue;
    const raw = group.root.path.trim();
    const parentPath = raw && raw !== "/" && pathSegments(raw).length === 1
      ? uniquePath(raw.startsWith("/") ? raw : `/${raw}`, used)
      : pathFromFolderName(name, used);
    used.add(parentPath);
    out.push(folderPayload(name, parentPath, group.root.description));

    for (const child of group.children) {
      if (!child.selected) continue;
      const childName = child.name.trim();
      if (!childName) continue;
      const childSlug = slugify(childName) || "folder";
      const childPath = uniquePath(`${parentPath}/${childSlug}`, used);
      used.add(childPath);
      out.push(folderPayload(childName, childPath, child.description));
    }
  }

  return out;
}
