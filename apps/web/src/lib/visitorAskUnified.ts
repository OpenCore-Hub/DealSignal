/** Whether the unified visitor Ask workspace is enabled for this link. */
export function visitorAskUnifiedEnabled(link: {
  qaEnabled?: boolean;
  visitorAskUnified?: boolean;
}): boolean {
  return Boolean(link.qaEnabled && link.visitorAskUnified);
}
