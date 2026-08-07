/** Whether the unified visitor Ask workspace is enabled for this link. */
export function visitorAskUnifiedEnabled(link: {
  qaEnabled?: boolean;
  visitorAskUnified?: boolean;
  dealRoomId?: string;
}): boolean {
  if (!link.qaEnabled) return false;
  if (link.visitorAskUnified !== undefined) return link.visitorAskUnified;
  return Boolean(link.dealRoomId);
}
