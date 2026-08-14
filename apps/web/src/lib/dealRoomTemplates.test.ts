import { describe, expect, it } from "vitest";
import type { DealRoomTemplate } from "@/types";
import {
  CUSTOM_DEAL_ROOM_TEMPLATE,
  CUSTOM_DEAL_ROOM_TEMPLATE_ID,
  ensureDealRoomTemplates,
  isCustomDealRoomTemplate,
  templateFolderCount,
} from "./dealRoomTemplates";

const seed: DealRoomTemplate = {
  id: "tpl-seed",
  name: "Seed Round",
  description: "Early-stage",
  scenario: "startup-fundraising",
  folderStructure: [{ name: "Pitch" }],
  recommendedFiles: ["Deck"],
  defaultPermissionLevel: "public",
  ndaEnabled: true,
};

describe("ensureDealRoomTemplates", () => {
  it("appends the custom scenario when the catalog omits it", () => {
    const result = ensureDealRoomTemplates([seed]);
    expect(result).toHaveLength(2);
    expect(result[1]).toEqual(CUSTOM_DEAL_ROOM_TEMPLATE);
  });

  it("does not duplicate custom when the API already returns it", () => {
    const result = ensureDealRoomTemplates([seed, CUSTOM_DEAL_ROOM_TEMPLATE]);
    expect(result.filter(isCustomDealRoomTemplate)).toHaveLength(1);
  });

  it("normalizes a missing folder list so the custom tile can render", () => {
    const broken = {
      ...CUSTOM_DEAL_ROOM_TEMPLATE,
      folderStructure: undefined as unknown as DealRoomTemplate["folderStructure"],
    };
    const result = ensureDealRoomTemplates([seed, broken]);
    expect(templateFolderCount(result[1])).toBe(0);
    expect(result[1].id).toBe(CUSTOM_DEAL_ROOM_TEMPLATE_ID);
  });
});
