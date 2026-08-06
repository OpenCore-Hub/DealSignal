/** Canonical path for Deal Room → Access (membership + share-link applicants). */
export function dealRoomAccessPath(
  workspaceSlug: string,
  roomId: string,
  opts?: { linkId?: string },
): string {
  const params = new URLSearchParams();
  params.set("tab", "access");
  if (opts?.linkId) {
    params.set("linkId", opts.linkId);
  }
  return `/${workspaceSlug}/deal-rooms/${roomId}?${params.toString()}`;
}
