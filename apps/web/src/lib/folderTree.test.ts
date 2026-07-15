import { describe, expect, it } from "vitest";
import { buildFolderTree } from "./folderTree";

interface TestFolder {
  path: string;
  name: string;
  sort_order: number;
}

function f(path: string, name: string, sort_order: number): TestFolder {
  return { path, name, sort_order };
}

describe("buildFolderTree", () => {
  it("returns empty array for empty input", () => {
    expect(buildFolderTree([])).toEqual([]);
  });

  it("filters out root path", () => {
    expect(buildFolderTree([f("/", "Root", 0)])).toEqual([]);
  });

  it("builds nested tree ordered by sort_order then name", () => {
    const folders = [
      f("/A", "A", 1),
      f("/A/1", "One", 1),
      f("/A/2", "Two", 2),
      f("/B", "B", 0),
    ];
    const roots = buildFolderTree(folders);
    expect(roots.map((r) => r.folder.path)).toEqual(["/B", "/A"]);
    expect(roots[1].children.map((c) => c.folder.path)).toEqual([
      "/A/1",
      "/A/2",
    ]);
  });

  it("orphans missing parents become roots", () => {
    const folders = [f("/X/Y", "Y", 0)];
    const roots = buildFolderTree(folders);
    expect(roots.map((r) => r.folder.path)).toEqual(["/X/Y"]);
  });
});
