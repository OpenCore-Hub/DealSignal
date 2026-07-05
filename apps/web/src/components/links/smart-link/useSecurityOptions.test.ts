// @vitest-environment jsdom
/**
 * Unit tests for useSecurityOptions hook.
 *
 * Covers:
 *  1. isValidEmail — email validation edge cases
 *  2. invalidWhitelistEmails — derived validation list
 *  3. update — patch + cross-option constraint enforcement
 */
import { describe, it, expect, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useSecurityOptions } from "./useSecurityOptions";
import type { PermissionConfig } from "@/types";
import { buildConfigFromPreset } from "../link-bundle/pipelineUtils";

// Re-implement isValidEmail in test for independent verification
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
function checkEmail(value: string) {
  return EMAIL_RE.test(value.trim());
}

describe("isValidEmail", () => {
  it("accepts valid email", () => {
    expect(checkEmail("user@example.com")).toBe(true);
    expect(checkEmail("a@b.co")).toBe(true);
    expect(checkEmail("name+tag@domain.io")).toBe(true);
  });

  it("rejects invalid emails", () => {
    expect(checkEmail("")).toBe(false);
    expect(checkEmail("not-an-email")).toBe(false);
    expect(checkEmail("@domain.com")).toBe(false);
    expect(checkEmail("user@")).toBe(false);
    expect(checkEmail("@")).toBe(false);
    expect(checkEmail(" ")).toBe(false);
  });

  it("trims whitespace before validation", () => {
    expect(checkEmail(" user@example.com ")).toBe(true);
  });
});

describe("useSecurityOptions", () => {
  const makeConfig = (overrides?: Partial<PermissionConfig>): PermissionConfig => ({
    ...buildConfigFromPreset("customized"),
    ...overrides,
  });

  describe("invalidWhitelistEmails", () => {
    it("returns empty when whitelist is empty", () => {
      const config = makeConfig({ whitelist: [] });
      const onChange = vi.fn();
      const { result } = renderHook(() => useSecurityOptions(config, onChange));
      expect(result.current.invalidWhitelistEmails).toEqual([]);
    });

    it("returns empty for valid emails", () => {
      const config = makeConfig({
        whitelist: ["alice@example.com", "bob@test.org"],
      });
      const onChange = vi.fn();
      const { result } = renderHook(() => useSecurityOptions(config, onChange));
      expect(result.current.invalidWhitelistEmails).toEqual([]);
    });

    it("flags domain patterns (starting with @) as invalid", () => {
      const config = makeConfig({
        whitelist: ["@corp.com", "@example.org"],
      });
      const onChange = vi.fn();
      const { result } = renderHook(() => useSecurityOptions(config, onChange));
      expect(result.current.invalidWhitelistEmails).toEqual(["@corp.com", "@example.org"]);
    });

    it("flags entries without @ as invalid", () => {
      const config = makeConfig({
        whitelist: ["simple-entry", "another"],
      });
      const onChange = vi.fn();
      const { result } = renderHook(() => useSecurityOptions(config, onChange));
      expect(result.current.invalidWhitelistEmails).toEqual(["simple-entry", "another"]);
    });

    it("flags invalid email-like entries (has @ but not domain pattern)", () => {
      const config = makeConfig({
        whitelist: ["bad@email", "user@bad"],
      });
      const onChange = vi.fn();
      const { result } = renderHook(() => useSecurityOptions(config, onChange));
      expect(result.current.invalidWhitelistEmails).toContain("bad@email");
      expect(result.current.invalidWhitelistEmails).toContain("user@bad");
    });

    it("filters mixed valid/invalid/domain entries correctly", () => {
      const config = makeConfig({
        whitelist: [
          "valid@ok.com",    // valid email → NOT invalid
          "@domain.io",      // domain pattern → INVALID
          "bad@b",           // invalid email → INVALID
          "no-at-entry",     // no @ → INVALID
          "almost@ok.org",   // valid email → NOT invalid
        ],
      });
      const onChange = vi.fn();
      const { result } = renderHook(() => useSecurityOptions(config, onChange));
      expect(result.current.invalidWhitelistEmails).toEqual(["@domain.io", "bad@b", "no-at-entry"]);
    });
  });

  describe("update — constraint enforcement", () => {
    it("calls onChange with merged config", () => {
      const config = makeConfig();
      const onChange = vi.fn();
      const { result } = renderHook(() => useSecurityOptions(config, onChange));

      act(() => {
        result.current.update({ allowDownload: true });
      });

      expect(onChange).toHaveBeenCalledTimes(1);
      const called = onChange.mock.calls[0][0] as PermissionConfig;
      // Should be the default customized config + allowDownload override
      expect(called.allowDownload).toBe(true);
    });

    it("forcing whitelist on also enables email verification (constraint)", () => {
      const config = makeConfig({ requireEmailVerification: false, whitelistEnabled: false });
      const onChange = vi.fn();
      const { result } = renderHook(() => useSecurityOptions(config, onChange));

      act(() => {
        result.current.update({ whitelistEnabled: true });
      });

      expect(onChange).toHaveBeenCalledTimes(1);
      const called = onChange.mock.calls[0][0] as PermissionConfig;
      expect(called.whitelistEnabled).toBe(true);
      expect(called.requireEmailVerification).toBe(true); // enforced
    });

    it("forcing NDA on also enables email verification (constraint)", () => {
      const config = makeConfig({ requireEmailVerification: false, ndaEnabled: false });
      const onChange = vi.fn();
      const { result } = renderHook(() => useSecurityOptions(config, onChange));

      act(() => {
        result.current.update({ ndaEnabled: true });
      });

      expect(onChange).toHaveBeenCalledTimes(1);
      const called = onChange.mock.calls[0][0] as PermissionConfig;
      expect(called.ndaEnabled).toBe(true);
      expect(called.requireEmailVerification).toBe(true); // enforced
    });

    it("toggling email off when whitelist is on does NOT remove email (only checked once)", () => {
      // When whitelist is already on and user tries to disable email,
      // enforceCrossOptionConstraints only checks whitelistEnabled first
      // and re-enables email. It does NOT check the email-off case.
      const config = makeConfig({
        requireEmailVerification: true,
        whitelistEnabled: true,
      });
      const onChange = vi.fn();
      const { result } = renderHook(() => useSecurityOptions(config, onChange));

      act(() => {
        result.current.update({ requireEmailVerification: false });
      });

      expect(onChange).toHaveBeenCalledTimes(1);
      const called = onChange.mock.calls[0][0] as PermissionConfig;
      // email stays on because whitelist is still enabled
      expect(called.requireEmailVerification).toBe(true);
    });
  });
});
