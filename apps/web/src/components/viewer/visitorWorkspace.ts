import { visitorAskUnifiedEnabled } from "@/lib/visitorAskUnified";

export type VisitorWorkspaceTab = "documents" | "qa" | "requests";

export interface VisitorWorkspaceVisibility {
  documentCount: number;
  fileRequestsEnabled: boolean;
  qaEnabled: boolean;
}

export interface VisitorWorkspaceLink {
  qaEnabled?: boolean;
  fileRequestsEnabled?: boolean;
  dealRoomId?: string;
  visitorAskUnified?: boolean;
}

/** Whether the public viewer should expose the workspace sidebar at all. */
export function shouldShowVisitorWorkspace(args: VisitorWorkspaceVisibility): boolean {
  return args.documentCount > 1 || args.fileRequestsEnabled || args.qaEnabled;
}

/** Resolve workspace sidebar visibility for the public viewer link payload. */
export function resolveShowVisitorWorkspace(args: {
  link: VisitorWorkspaceLink;
  documentCount: number;
}): boolean {
  const unifiedAsk = visitorAskUnifiedEnabled(args.link);
  if (unifiedAsk) {
    return shouldShowVisitorWorkspace({
      documentCount: args.documentCount,
      fileRequestsEnabled: Boolean(args.link.fileRequestsEnabled),
      qaEnabled: Boolean(args.link.qaEnabled),
    });
  }
  return Boolean(
    args.link.qaEnabled ||
      args.link.fileRequestsEnabled ||
      args.documentCount > 1,
  );
}

/** Deal-room share links open the workspace panel by default on first entry. */
export function shouldDefaultVisitorWorkspaceOpen(args: {
  dealRoomId?: string;
  showWorkspace: boolean;
}): boolean {
  return Boolean(args.dealRoomId && args.showWorkspace);
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
