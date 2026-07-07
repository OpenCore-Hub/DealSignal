import { useCallback, useEffect, useRef } from "react";
import type { PermissionConfig } from "@/types";
import { enforceCrossOptionConstraints } from "./levelConfig";

/**
 * Shared hook for security options toggle logic.
 *
 * Bundles:
 *  - update (patch + cross-option constraint enforcement)
 *
 * Uses a ref for config to avoid stale closures when rapid toggles occur.
 * Each update call reads the latest config value from the ref, preventing
 * lost updates when React hasn't re-rendered between consecutive toggles.
 */
export function useSecurityOptions(
  config: PermissionConfig,
  onChange: (config: PermissionConfig) => void,
) {
  const configRef = useRef(config);
  const onChangeRef = useRef(onChange);

  useEffect(() => {
    configRef.current = config;
    onChangeRef.current = onChange;
  }, [config, onChange]);

  const update = useCallback(
    (patch: Partial<PermissionConfig>) => {
      const merged = { ...configRef.current, ...patch };
      const constrained = enforceCrossOptionConstraints(merged);
      onChangeRef.current(constrained);
    },
    [], // stable: reads latest values from refs
  );

  return { update } as const;
}
