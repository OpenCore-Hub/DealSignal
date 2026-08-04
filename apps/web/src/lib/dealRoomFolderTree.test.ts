import { describe, expect, it } from "vitest";
import {
  buildFolderTree,
  filterFolderTree,
  parseSelection,
  topmostSelectedFolderPaths,
  collectSubtreeKeys,
  subtreeSelectionState,
  withFolderSubtreeSelection,
  withDocumentSelection,
  type FolderTreeNode,
} from "./dealRoomFolderTree";
import type { DealRoomDocumentItem, DealRoomFolder } from "@/types";

function folder(path: string, name: string, locked = false): DealRoomFolder {
  return { path, name, sort_order: 0, locked };
}

function doc(id: string, title: string, folderPath: string, locked = false): DealRoomDocumentItem {
  return {
    id: `rd-${id}`,
    document_id: id,
    title,
    folder_path: folderPath,
    sort_order: 0,
    source_type: "pdf",
    status: "ready",
    created_at: "2026-01-01T00:00:00Z",
    locked,
  };
}

describe("filterFolderTree", () => {
  const folders = [
    folder("/legal", "Legal"),
    folder("/legal/nda", "NDA", true),
    folder("/finance", "Finance"),
  ];
  const docsByFolder = new Map<string, DealRoomDocumentItem[]>([
    ["/legal", [doc("a", "Board Minutes", "/legal")]],
    ["/legal/nda", [doc("b", "Standard NDA", "/legal/nda", true)]],
    ["/finance", [doc("c", "Budget 2026", "/finance")]],
  ]);
  const roots = buildFolderTree(folders, docsByFolder);

  it("filters by search and keeps ancestors", () => {
    const { roots: filtered, expandPaths } = filterFolderTree(roots, {
      query: "nda",
      type: "all",
      lock: "all",
    });
    expect(filtered.map((n) => n.folder.path)).toEqual(["/legal"]);
    expect(filtered[0]?.children.map((n) => n.folder.path)).toEqual(["/legal/nda"]);
    expect(expandPaths.has("/legal")).toBe(true);
    expect(expandPaths.has("/legal/nda")).toBe(true);
  });

  it("filters locked resources", () => {
    const { roots: filtered } = filterFolderTree(roots, {
      query: "",
      type: "all",
      lock: "locked",
    });
    const paths: string[] = [];
    const walk = (nodes: FolderTreeNode[]) => {
      for (const n of nodes) {
        paths.push(n.folder.path);
        walk(n.children);
      }
    };
    walk(filtered);
    expect(paths).toContain("/legal/nda");
    expect(paths).not.toContain("/finance");
  });

  it("type=files hides empty folders without matching docs", () => {
    const { roots: filtered } = filterFolderTree(roots, {
      query: "Budget",
      type: "files",
      lock: "all",
    });
    expect(filtered.map((n) => n.folder.path)).toEqual(["/finance"]);
    expect(filtered[0]?.documents.map((d) => d.document_id)).toEqual(["c"]);
  });
});

describe("parseSelection", () => {
  it("splits folder and document keys", () => {
    expect(
      parseSelection(["folder:/legal", "doc:abc", "folder:/finance"] as const),
    ).toEqual({
      folderPaths: ["/legal", "/finance"],
      documentIds: ["abc"],
    });
  });
});

describe("topmostSelectedFolderPaths", () => {
  it("keeps only roots when parents and children are both selected", () => {
    expect(
      topmostSelectedFolderPaths(["/legal", "/legal/nda", "/finance"]).sort(),
    ).toEqual(["/finance", "/legal"].sort());
  });
});

describe("recursive folder selection", () => {
  const folders = [
    folder("/legal", "Legal"),
    folder("/legal/nda", "NDA"),
    folder("/finance", "Finance"),
  ];
  const docsByFolder = new Map<string, DealRoomDocumentItem[]>([
    ["/legal", [doc("a", "Board Minutes", "/legal")]],
    ["/legal/nda", [doc("b", "Standard NDA", "/legal/nda")]],
    ["/finance", [doc("c", "Budget 2026", "/finance")]],
  ]);
  const roots = buildFolderTree(folders, docsByFolder);
  const legal = roots.find((n) => n.folder.path === "/legal")!;

  it("collects folder, nested folders, and documents", () => {
    expect(collectSubtreeKeys(legal).sort()).toEqual(
      ["doc:a", "doc:b", "folder:/legal", "folder:/legal/nda"].sort(),
    );
  });

  it("selecting a folder selects all descendants", () => {
    const next = withFolderSubtreeSelection(new Set(), roots, legal, true);
    expect(subtreeSelectionState(legal, next)).toBe("checked");
    expect(next.has("doc:a")).toBe(true);
    expect(next.has("doc:b")).toBe(true);
    expect(next.has("folder:/legal/nda")).toBe(true);
    expect(next.has("doc:c")).toBe(false);
  });

  it("clearing a folder clears all descendants", () => {
    const selected = withFolderSubtreeSelection(new Set(), roots, legal, true);
    const next = withFolderSubtreeSelection(selected, roots, legal, false);
    expect(subtreeSelectionState(legal, next)).toBe("unchecked");
    expect(next.size).toBe(0);
  });

  it("partial document selection marks parent indeterminate; completing subtree checks parent", () => {
    let next = withDocumentSelection(new Set(), roots, "a", true);
    expect(subtreeSelectionState(legal, next)).toBe("indeterminate");
    expect(next.has("folder:/legal")).toBe(false);

    const nda = legal.children[0]!;
    next = withFolderSubtreeSelection(next, roots, nda, true);
    expect(next.has("folder:/legal/nda")).toBe(true);
    expect(next.has("doc:b")).toBe(true);
    expect(subtreeSelectionState(legal, next)).toBe("checked");
    expect(next.has("folder:/legal")).toBe(true);
  });
});
