/** Canonical path for Deal Room → Ask Inbox (QA tab). */
export function dealRoomAskPath(
  workspaceSlug: string,
  roomId: string,
  opts?: { linkId?: string },
): string {
  const params = new URLSearchParams();
  params.set("tab", "qa");
  if (opts?.linkId) {
    params.set("linkId", opts.linkId);
  }
  return `/${workspaceSlug}/deal-rooms/${roomId}?${params.toString()}`;
}

/** Parse dashboard target_id for deal_room_link_question (room or room/link). */
export function parseDealRoomAskTarget(targetId: string): {
  roomId: string;
  linkId?: string;
} {
  const slash = targetId.indexOf("/");
  if (slash === -1) {
    return { roomId: targetId };
  }
  const roomId = targetId.slice(0, slash);
  const linkId = targetId.slice(slash + 1);
  return linkId ? { roomId, linkId } : { roomId };
}
