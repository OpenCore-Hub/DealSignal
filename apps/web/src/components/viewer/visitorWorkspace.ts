export type VisitorWorkspaceTab = "documents" | "qa" | "requests";

export interface VisitorWorkspaceVisibility {
  documentCount: number;
  fileRequestsEnabled: boolean;
  qaEnabled: boolean;
}

/** Whether the public viewer should expose the workspace sidebar at all. */
export function shouldShowVisitorWorkspace(args: VisitorWorkspaceVisibility): boolean {
  return args.documentCount > 1 || args.fileRequestsEnabled || args.qaEnabled;
}

export function defaultVisitorWorkspaceTab(args: {
  fileRequestsEnabled: boolean;
  hasMultipleDocuments: boolean;
  qaEnabled: boolean;
}): VisitorWorkspaceTab {
  if (args.hasMultipleDocuments) return "documents";
  if (args.qaEnabled) return "qa";
  if (args.fileRequestsEnabled) return "requests";
  return "documents";
}

export function visitorWorkspaceTabs(args: VisitorWorkspaceVisibility): VisitorWorkspaceTab[] {
  const tabs: VisitorWorkspaceTab[] = [];
  if (args.documentCount > 1) tabs.push("documents");
  if (args.qaEnabled) tabs.push("qa");
  if (args.fileRequestsEnabled) tabs.push("requests");
  return tabs;
}
