import type { DraftLink } from "@/components/links/share";

const STORAGE_PREFIX = "dealsignal:deal-room-access-defaults:";

function storageKey(roomId: string): string {
  return `${STORAGE_PREFIX}${roomId}`;
}

/** Persist room-level access defaults when no share link exists yet. */
export function saveRoomAccessDefaults(roomId: string, draft: DraftLink): void {
  try {
    localStorage.setItem(storageKey(roomId), JSON.stringify(draft));
  } catch {
    // Ignore quota / private-mode failures; UI still shows the form.
  }
}

/** Load previously saved room-level access defaults, if any. */
export function loadRoomAccessDefaults(roomId: string): DraftLink | null {
  try {
    const raw = localStorage.getItem(storageKey(roomId));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as DraftLink;
    if (!parsed || typeof parsed !== "object") return null;
    return parsed;
  } catch {
    return null;
  }
}

/** True when the room has saved access-control defaults (local persistence). */
export function hasRoomAccessDefaults(roomId: string): boolean {
  return loadRoomAccessDefaults(roomId) != null;
}

/** Clear room defaults after they have been applied to a newly created link. */
export function clearRoomAccessDefaults(roomId: string): void {
  try {
    localStorage.removeItem(storageKey(roomId));
  } catch {
    // ignore
  }
}
