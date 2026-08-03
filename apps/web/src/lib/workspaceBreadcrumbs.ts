import type { BreadcrumbItem } from "@/stores/uiStore";

/** First path segment after workspace slug → layout sidebar i18n key. */
export const WORKSPACE_NAV_SEGMENT_KEYS: Record<string, string> = {
  "deal-rooms": "sidebar.nav.dealRooms",
  documents: "sidebar.nav.documents",
  links: "sidebar.nav.links",
  contacts: "sidebar.nav.contacts",
  insights: "sidebar.nav.insights",
  "agreement-documents": "sidebar.nav.agreementDocuments",
  settings: "sidebar.nav.settings",
};

/**
 * Build workspace nav breadcrumbs: Home >> current primary section.
 * Dashboard returns [] (welcome header owns that surface).
 */
export function resolveWorkspaceNavBreadcrumbs(
  pathname: string,
  workspaceSlug: string,
  labels: { home: string; section: (navKey: string) => string }
): BreadcrumbItem[] {
  const prefix = `/${workspaceSlug}/`;
  if (!pathname.startsWith(prefix) && pathname !== `/${workspaceSlug}`) {
    return [];
  }

  const rest = pathname.slice(prefix.length);
  const segment = rest.split("/").filter(Boolean)[0];
  if (!segment || segment === "dashboard") {
    return [];
  }

  const navKey = WORKSPACE_NAV_SEGMENT_KEYS[segment];
  if (!navKey) {
    return [];
  }

  return [
    { label: labels.home, to: `/${workspaceSlug}/dashboard` },
    {
      label: labels.section(navKey),
      to: `/${workspaceSlug}/${segment}`,
    },
  ];
}
