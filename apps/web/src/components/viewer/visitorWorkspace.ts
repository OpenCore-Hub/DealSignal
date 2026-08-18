import { visitorAskUnifiedEnabled } from "@/lib/visitorAskUnified";

export type VisitorWorkspaceTab = "documents" | "qa" | "faq" | "requests";

export interface VisitorWorkspaceVisibility {
  documentCount: number;
  fileRequestsEnabled: boolean;
  qaEnabled: boolean;
  faqCount?: number;
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

/** Deal-room links and multi-file shares open the workspace panel on first entry. */
export function shouldDefaultVisitorWorkspaceOpen(args: {
  dealRoomId?: string;
  showWorkspace: boolean;
  documentCount?: number;
}): boolean {
  if (!args.showWorkspace) return false;
  if (args.dealRoomId) return true;
  return (args.documentCount ?? 0) > 1;
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
  if (args.qaEnabled && (args.faqCount ?? 0) > 0) tabs.push("faq");
  if (args.fileRequestsEnabled) tabs.push("requests");
  return tabs;
}
