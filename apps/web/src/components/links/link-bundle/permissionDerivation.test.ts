import { describe, it, expect } from "vitest";
import type { Link } from "@/types";

/**
 * Tests for the permission_type → boolean flag derivation
 * used in edit mode INIT_FOR_EDIT in BundlePipelinePage.
 */

interface SecurityBooleans {
  requireEmailVerification: boolean;
  whitelistEnabled: boolean;
  passwordEnabled: boolean;
  ndaEnabled: boolean;
}

function deriveBooleans(permissionType: Link["permissionType"]): SecurityBooleans {
  return {
    requireEmailVerification:
      permissionType === "email" ||
      permissionType === "nda" ||
      permissionType === "whitelist",
    whitelistEnabled: permissionType === "whitelist",
    passwordEnabled: permissionType === "password",
    ndaEnabled: permissionType === "nda",
  };
}

describe("permission_type → boolean flag derivation (edit mode)", () => {
  it('maps "public" correctly', () => {
    const result = deriveBooleans("public");
    expect(result).toEqual({
      requireEmailVerification: false,
      whitelistEnabled: false,
      passwordEnabled: false,
      ndaEnabled: false,
    });
  });

  it('maps "email" correctly', () => {
    const result = deriveBooleans("email");
    expect(result).toEqual({
      requireEmailVerification: true,
      whitelistEnabled: false,
      passwordEnabled: false,
      ndaEnabled: false,
    });
  });

  it('maps "whitelist" correctly', () => {
    const result = deriveBooleans("whitelist");
    expect(result).toEqual({
      requireEmailVerification: true,
      whitelistEnabled: true,
      passwordEnabled: false,
      ndaEnabled: false,
    });
  });

  it('maps "password" correctly', () => {
    const result = deriveBooleans("password");
    expect(result).toEqual({
      requireEmailVerification: false,
      whitelistEnabled: false,
      passwordEnabled: true,
      ndaEnabled: false,
    });
  });

  it('maps "nda" correctly — NDA implies email verification', () => {
    const result = deriveBooleans("nda");
    expect(result).toEqual({
      requireEmailVerification: true,
      whitelistEnabled: false,
      passwordEnabled: false,
      ndaEnabled: true,
    });
  });

  it("handles undefined permissionType gracefully", () => {
    const result = deriveBooleans(undefined);
    // All comparisons against undefined return false
    expect(result.requireEmailVerification).toBe(false);
    expect(result.whitelistEnabled).toBe(false);
    expect(result.passwordEnabled).toBe(false);
    expect(result.ndaEnabled).toBe(false);
  });
});
