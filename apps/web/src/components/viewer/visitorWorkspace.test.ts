import { describe, expect, it } from "vitest";
import {
  defaultVisitorWorkspaceTab,
  shouldShowVisitorWorkspace,
  visitorWorkspaceTabs,
} from "./visitorWorkspace";

describe("visitorWorkspace", () => {
  it("shows workspace for multi-doc, file requests, or deal-room Ask Host", () => {
    expect(
      shouldShowVisitorWorkspace({ documentCount: 1, fileRequestsEnabled: false, qaEnabled: false }),
    ).toBe(false);
    expect(
      shouldShowVisitorWorkspace({ documentCount: 2, fileRequestsEnabled: false, qaEnabled: false }),
    ).toBe(true);
    expect(
      shouldShowVisitorWorkspace({ documentCount: 1, fileRequestsEnabled: true, qaEnabled: false }),
    ).toBe(true);
    expect(
      shouldShowVisitorWorkspace({ documentCount: 1, fileRequestsEnabled: false, qaEnabled: true }),
    ).toBe(true);
  });

  it("defaults to Ask when single-doc deal-room link has qa only", () => {
    expect(
      defaultVisitorWorkspaceTab({
        hasMultipleDocuments: false,
        fileRequestsEnabled: false,
        qaEnabled: true,
      }),
    ).toBe("qa");
  });

  it("lists qa tab for deal-room links without multi-doc or file requests", () => {
    expect(
      visitorWorkspaceTabs({ documentCount: 1, fileRequestsEnabled: false, qaEnabled: true }),
    ).toEqual(["qa"]);
  });
});
