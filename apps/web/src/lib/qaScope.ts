/** Mirrors backend link.QaEnabledForLink — document links off; deal-room links on. */
export function qaEnabledForLink(isDealRoomLink: boolean): boolean {
  return isDealRoomLink;
}

export function qaEnabledForLinkId(dealRoomId?: string | null): boolean {
  return qaEnabledForLink(Boolean(dealRoomId?.trim()));
}
