import { useEffect, useState, type Dispatch, type SetStateAction } from "react";
import { api } from "@/lib/api";
import type { Contact } from "@/types";

export type UseWorkspaceContactsResult = {
  contacts: Contact[];
  loading: boolean;
  /** Replace the loaded list (e.g. after creating a contact in-place). */
  setContacts: Dispatch<SetStateAction<Contact[]>>;
};

/**
 * Load contacts for a single workspace. Clears immediately on slug change so
 * prior-workspace PII never remains selectable across workspace switches.
 */
export function useWorkspaceContacts(
  workspaceSlug: string | undefined,
  options?: { enabled?: boolean },
): UseWorkspaceContactsResult {
  const enabled = options?.enabled ?? true;
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [loading, setLoading] = useState(() => Boolean(enabled && workspaceSlug));

  useEffect(() => {
    if (!enabled || !workspaceSlug) {
      setContacts([]);
      setLoading(false);
      return;
    }

    let cancelled = false;
    // Fail-closed: clear before fetch so a prior workspace never lingers.
    setContacts([]);
    setLoading(true);

    api
      .getContacts(workspaceSlug)
      .then((res) => {
        if (!cancelled) setContacts(res.data);
      })
      .catch(() => {
        if (!cancelled) setContacts([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [workspaceSlug, enabled]);

  return { contacts, loading, setContacts };
}
