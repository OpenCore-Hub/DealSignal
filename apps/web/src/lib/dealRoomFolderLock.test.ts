import { describe, expect, it } from "vitest";
import {
  isFolderPathLocked,
  lockedFolderPathSet,
  selectableRootFolderPaths,
  stripLockedFolderPaths,
} from "./dealRoomFolderLock";

describe("dealRoomFolderLock", () => {
  const folders = [
    { path: "/general", name: "General", sort_order: 0 },
    { path: "/legal", name: "Legal", sort_order: 1, locked: true },
    { path: "/legal/nda", name: "NDA", sort_order: 2 },
  ];

  it("detects locked folders and descendants", () => {
    const locked = lockedFolderPathSet(folders);
    expect(isFolderPathLocked("/legal", locked)).toBe(true);
    expect(isFolderPathLocked("/legal/nda", locked)).toBe(true);
    expect(isFolderPathLocked("/general", locked)).toBe(false);
  });

  it("returns selectable roots without locked folders", () => {
    expect(selectableRootFolderPaths(folders, ["/general", "/legal"])).toEqual(["/general"]);
  });

  it("strips locked paths from selection payloads", () => {
    const locked = lockedFolderPathSet(folders);
    expect(stripLockedFolderPaths(["/general", "/legal", "/legal/nda"], locked)).toEqual([
      "/general",
    ]);
  });
});
