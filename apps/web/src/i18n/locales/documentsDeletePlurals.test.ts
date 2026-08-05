import { describe, expect, it } from "vitest";
import en from "./en/documents.json";
import zh from "./zh-CN/documents.json";

describe("documents delete plural keys", () => {
  it("keeps en/zh-CN plural forms in sync for delete impact copy", () => {
    const keys = [
      "withLinks_one",
      "withLinks_other",
      "withDealRooms_one",
      "withDealRooms_other",
    ] as const;

    for (const key of keys) {
      expect(en.delete[key], `en.delete.${key}`).toBeTypeOf("string");
      expect(zh.delete[key], `zh-CN.delete.${key}`).toBeTypeOf("string");
      expect(en.delete[key].length).toBeGreaterThan(0);
      expect(zh.delete[key].length).toBeGreaterThan(0);
    }

    // Base keys without plural suffixes must not remain (i18next would miss count forms).
    expect("withLinks" in en.delete).toBe(false);
    expect("withLinks" in zh.delete).toBe(false);
    expect("withDealRooms" in en.delete).toBe(false);
    expect("withDealRooms" in zh.delete).toBe(false);
  });
});
