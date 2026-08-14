import { describe, expect, it } from "vitest";
import {
  pathFromFolderName,
  seedCreateFolderDrafts,
  toCreateFolderPayload,
} from "./createRoomFolders";

describe("createRoomFolders", () => {
  it("seeds every template folder as selected", () => {
    const drafts = seedCreateFolderDrafts([
      { name: "Pitch", path: "/pitch-deck" },
      { name: "Legal" },
    ]);
    expect(drafts).toHaveLength(2);
    expect(drafts.every((item) => item.selected && item.parentId === null)).toBe(true);
    expect(drafts[0]?.path).toBe("/pitch-deck");
  });

  it("nests a second-level path under its parent", () => {
    const drafts = seedCreateFolderDrafts([
      { name: "Legal", path: "/legal" },
      { name: "NDAs", path: "/legal/ndas" },
    ]);
    expect(drafts[1]?.parentId).toBe(drafts[0]?.id);
  });

  it("omits unselected and blank names from the create payload", () => {
    const payload = toCreateFolderPayload([
      { id: "1", name: "Pitch", path: "/pitch-deck", selected: true, parentId: null },
      { id: "2", name: "Legal", path: "/legal", selected: false, parentId: null },
      { id: "3", name: "   ", path: "", selected: true, parentId: null },
    ]);
    expect(payload).toEqual([{ name: "Pitch", path: "/pitch-deck" }]);
  });

  it("writes selected children one level under the parent path", () => {
    const payload = toCreateFolderPayload([
      { id: "p", name: "Legal", path: "/legal", selected: true, parentId: null },
      { id: "c", name: "NDAs", path: "", selected: true, parentId: "p" },
      { id: "x", name: "Skipped", path: "", selected: true, parentId: "missing" },
    ]);
    expect(payload).toEqual([
      { name: "Legal", path: "/legal" },
      { name: "NDAs", path: "/legal/ndas" },
    ]);
  });

  it("drops children when the parent is not kept", () => {
    const payload = toCreateFolderPayload([
      { id: "p", name: "Legal", path: "/legal", selected: false, parentId: null },
      { id: "c", name: "NDAs", path: "", selected: true, parentId: "p" },
    ]);
    expect(payload).toEqual([]);
  });

  it("keeps the seed description when the name is unchanged", () => {
    const drafts = seedCreateFolderDrafts([
      { name: "Pitch", path: "/pitch-deck", description: "Latest deck" },
    ]);
    expect(toCreateFolderPayload(drafts)).toEqual([
      { name: "Pitch", path: "/pitch-deck", description: "Latest deck" },
    ]);
  });

  it("builds a unique path from a name when path is empty", () => {
    const used = new Set<string>(["/legal"]);
    expect(pathFromFolderName("Legal", used)).toBe("/legal-2");
  });
});
