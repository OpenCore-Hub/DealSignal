// @vitest-environment jsdom
/**
 * Unit tests for useSecurityOptions hook.
 *
 * Covers update — patch + cross-option constraint enforcement.
 */
import { describe, it, expect, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useSecurityOptions } from "./useSecurityOptions";
import type { PermissionConfig } from "@/types";
import { buildConfigFromPreset } from "../link-bundle/pipelineUtils";

describe("useSecurityOptions", () => {
  const makeConfig = (overrides?: Partial<PermissionConfig>): PermissionConfig => ({
    ...buildConfigFromPreset("customized"),
    ...overrides,
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
  });
});
