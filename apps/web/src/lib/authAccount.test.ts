// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import {
  clearCachedAccountEmail,
  getCachedAccountEmail,
  setCachedAccountEmail,
} from "./authAccount";

describe("authAccount cache", () => {
  afterEach(() => {
    clearCachedAccountEmail();
  });

  it("stores and reads trimmed login email", () => {
    setCachedAccountEmail("  owner@example.com ");
    expect(getCachedAccountEmail()).toBe("owner@example.com");
  });

  it("clears on empty set or explicit clear", () => {
    setCachedAccountEmail("owner@example.com");
    setCachedAccountEmail("");
    expect(getCachedAccountEmail()).toBeUndefined();
    setCachedAccountEmail("owner@example.com");
    clearCachedAccountEmail();
    expect(getCachedAccountEmail()).toBeUndefined();
  });
});
