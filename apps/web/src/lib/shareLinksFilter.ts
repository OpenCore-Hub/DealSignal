import type { Link } from "@/types";

/** Share-tab create-time windows (rolling duration from now). */
export const LINK_CREATED_WITHIN_VALUES = ["all", "24h", "7d", "30d", "90d"] as const;
export type LinkCreatedWithin = (typeof LINK_CREATED_WITHIN_VALUES)[number];

const CREATED_WITHIN_MS: Record<Exclude<LinkCreatedWithin, "all">, number> = {
  "24h": 24 * 60 * 60 * 1000,
  "7d": 7 * 24 * 60 * 60 * 1000,
  "30d": 30 * 24 * 60 * 60 * 1000,
  "90d": 90 * 24 * 60 * 60 * 1000,
};

/** URL query key for Share-tab text search (scoped; stripped off other tabs). */
export const SHARE_SEARCH_PARAM = "shareQ";
/** URL query key for Share-tab create-time window. */
export const SHARE_CREATED_WITHIN_PARAM = "createdWithin";

export function isLinkCreatedWithin(value: string | null | undefined): value is LinkCreatedWithin {
  return (
    value === "all" ||
    value === "24h" ||
    value === "7d" ||
    value === "30d" ||
    value === "90d"
  );
}

export function parseLinkCreatedWithin(
  value: string | null | undefined,
): LinkCreatedWithin {
  return isLinkCreatedWithin(value) ? value : "all";
}

export function hasActiveShareListFilters(opts: {
  searchQuery?: string;
  createdWithin?: LinkCreatedWithin;
}): boolean {
  const q = opts.searchQuery?.trim() ?? "";
  const within = opts.createdWithin ?? "all";
  return q.length > 0 || within !== "all";
}

/** Pure filter used by the Share tab search + create-time controls. */
export function filterLinksForShareView(
  links: Link[],
  opts: { searchQuery?: string; createdWithin?: LinkCreatedWithin; nowMs?: number },
): Link[] {
  const q = opts.searchQuery?.trim().toLowerCase() ?? "";
  const within = opts.createdWithin ?? "all";
  const now = opts.nowMs ?? Date.now();
  const cutoffMs = within === "all" ? null : now - CREATED_WITHIN_MS[within];

  return links.filter((link) => {
    if (cutoffMs != null) {
      const created = new Date(link.createdAt).getTime();
      if (Number.isNaN(created) || created < cutoffMs) return false;
    }
    if (!q) return true;
    const haystack = [link.shortUrl, link.documentTitle, link.name]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();
    return haystack.includes(q);
  });
}
