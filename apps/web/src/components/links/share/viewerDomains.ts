import type { WorkspaceViewerDomain } from "@/types";

export interface ShareViewerDomains {
  /** Verified Brand viewer hostnames selectable in the share dropdown. */
  availableDomains: string[];
  /** Pending Brand hostname (awaiting DNS) — not selectable until verified. */
  pendingHostname: string;
}

/** Maps Brand viewer-domain state into share-dialog dropdown options. */
export function resolveShareViewerDomains(
  domain: WorkspaceViewerDomain | null | undefined,
): ShareViewerDomains {
  const hostname = domain?.hostname?.trim() ?? "";
  if (!hostname) {
    return { availableDomains: [], pendingHostname: "" };
  }
  if (domain?.status === "verified") {
    return { availableDomains: [hostname], pendingHostname: "" };
  }
  if (domain?.status === "pending") {
    return { availableDomains: [], pendingHostname: hostname };
  }
  return { availableDomains: [], pendingHostname: "" };
}
