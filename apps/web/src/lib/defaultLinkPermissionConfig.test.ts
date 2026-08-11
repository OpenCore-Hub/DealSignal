import { describe, expect, it } from "vitest";
import { createDefaultLinkPermissionConfig } from "./defaultLinkPermissionConfig";

describe("createDefaultLinkPermissionConfig", () => {
  it("returns an independent copy each call", () => {
    const a = createDefaultLinkPermissionConfig();
    const b = createDefaultLinkPermissionConfig();
    expect(a).toEqual(b);
    a.whitelist.push("x@example.com");
    a.watermarkEnabled = false;
    expect(b.whitelist).toEqual([]);
    expect(b.watermarkEnabled).toBe(true);
  });

  it("matches library/bundle share defaults", () => {
    const cfg = createDefaultLinkPermissionConfig();
    expect(cfg.level).toBe("customized");
    expect(cfg.watermarkEnabled).toBe(true);
    expect(cfg.expiryDays).toBe(30);
    expect(cfg.allowDownload).toBe(true);
    expect(cfg.ndaEnabled).toBe(false);
  });
});
