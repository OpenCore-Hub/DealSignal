/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { downloadEvalSeedExport } from "./downloadEvalSeeds";
import type { DealRoomKnowledgeEvalSeedExport } from "@/types";

describe("downloadEvalSeedExport", () => {
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
    vi.spyOn(document.body, "appendChild").mockImplementation((node) => node);

    const createElement = vi.spyOn(document, "createElement").mockImplementation(() => {
      return {
        href: "",
        download: "",
        rel: "",
        click,
        remove,
      } as unknown as HTMLAnchorElement;
    });

    const pack: DealRoomKnowledgeEvalSeedExport = {
      description: "Accepted gold",
      seeds: [
        {
          id: "s1",
          kind: "wrong_citation",
          question: "price?",
          expect: "reject_or_rebind",
        },
      ],
    };

    downloadEvalSeedExport(pack, "gold-seeds.json");

    expect(createElement).toHaveBeenCalledWith("a");
    expect(createObjectURL).toHaveBeenCalled();
    expect(click).toHaveBeenCalled();
    expect(remove).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:mock");
  });
});
