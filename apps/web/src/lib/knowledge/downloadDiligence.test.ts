/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { downloadDiligencePack } from "./downloadDiligence";
import type { DealRoomKnowledgeDiligencePack } from "@/types";

describe("downloadDiligencePack", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("creates an object URL and clicks a download anchor", () => {
    const createObjectURL = vi.fn(() => "blob:mock");
    const revokeObjectURL = vi.fn();
    vi.stubGlobal("URL", {
      ...URL,
      createObjectURL,
      revokeObjectURL,
    });

    const click = vi.fn();
    const remove = vi.fn();
    const appendChild = vi
      .spyOn(document.body, "appendChild")
      .mockImplementation((node) => node);

    const createElement = vi.spyOn(document, "createElement").mockImplementation(() => {
      return {
        href: "",
        download: "",
        rel: "",
        click,
        remove,
      } as unknown as HTMLAnchorElement;
    });

    const pack: DealRoomKnowledgeDiligencePack = {
      schemaVersion: "knowledge_qa_diligence_v1",
      exportedAt: "2026-08-04T00:00:00Z",
      workspaceId: "ws",
      roomId: "room",
      sessionId: "sess-9",
      session: {
        id: "sess-9",
        roomId: "room",
        status: "active",
        createdAt: "2026-08-04T00:00:00Z",
        updatedAt: "2026-08-04T00:00:00Z",
      },
      turns: [],
    };

    downloadDiligencePack(pack);

    expect(createElement).toHaveBeenCalledWith("a");
    expect(createObjectURL).toHaveBeenCalled();
    expect(appendChild).toHaveBeenCalled();
    expect(click).toHaveBeenCalled();
    expect(remove).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:mock");
  });
});
