import { describe, expect, it } from "vitest";
import {
  defaultVisitorWorkspaceTab,
  resolveShowVisitorWorkspace,
  shouldDefaultVisitorWorkspaceOpen,
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

  it("inserts FAQ tab to the right of Ask when pins exist", () => {
    expect(
      visitorWorkspaceTabs({
        documentCount: 1,
        fileRequestsEnabled: true,
        qaEnabled: true,
        faqCount: 2,
      }),
    ).toEqual(["qa", "faq", "requests"]);
  });

  it("opens workspace by default for deal-room links when sidebar is available", () => {
    expect(
      shouldDefaultVisitorWorkspaceOpen({
        dealRoomId: "room-1",
        showWorkspace: true,
      }),
    ).toBe(true);
    expect(
      shouldDefaultVisitorWorkspaceOpen({
        dealRoomId: undefined,
        showWorkspace: true,
      }),
    ).toBe(false);
    expect(
      shouldDefaultVisitorWorkspaceOpen({
        dealRoomId: "room-1",
        showWorkspace: false,
      }),
    ).toBe(false);
  });

  it("opens workspace by default for multi-file document shares", () => {
    expect(
      shouldDefaultVisitorWorkspaceOpen({
        showWorkspace: true,
        documentCount: 2,
      }),
    ).toBe(true);
    expect(
      shouldDefaultVisitorWorkspaceOpen({
        showWorkspace: true,
        documentCount: 1,
      }),
    ).toBe(false);
    expect(
      shouldDefaultVisitorWorkspaceOpen({
        showWorkspace: false,
        documentCount: 3,
      }),
    ).toBe(false);
  });

  it("resolves workspace visibility for unified deal-room links", () => {
    expect(
      resolveShowVisitorWorkspace({
        link: {
          dealRoomId: "room-1",
          qaEnabled: true,
          visitorAskUnified: true,
          fileRequestsEnabled: false,
        },
        documentCount: 1,
      }),
    ).toBe(true);
    expect(
      resolveShowVisitorWorkspace({
        link: {
          qaEnabled: false,
          fileRequestsEnabled: false,
        },
        documentCount: 1,
      }),
    ).toBe(false);
  });
});
