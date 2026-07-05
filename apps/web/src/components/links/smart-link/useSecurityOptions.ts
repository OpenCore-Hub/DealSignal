import { useCallback, useMemo, useRef } from "react";
import type { PermissionConfig } from "@/types";
import { enforceCrossOptionConstraints } from "./levelConfig";

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function isValidEmail(value: string) {
  return EMAIL_RE.test(value.trim());
}

/**
 * Shared hook for security options toggle logic.
 *
 * Bundles:
 *  - invalidWhitelistEmails (derived validation)
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
  configRef.current = config;

  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const invalidWhitelistEmails = useMemo(() => {
    return config.whitelist.filter((entry) => !isValidEmail(entry));
  }, [config.whitelist]);

  const update = useCallback(
    (patch: Partial<PermissionConfig>) => {
      const merged = { ...configRef.current, ...patch };
      const constrained = enforceCrossOptionConstraints(merged);
      onChangeRef.current(constrained);
    },
    [], // stable: reads latest values from refs
  );

  return { invalidWhitelistEmails, update } as const;
}
