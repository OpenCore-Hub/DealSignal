import { describe, expect, it } from "vitest";
import { resolveWorkspaceNavBreadcrumbs } from "./workspaceBreadcrumbs";

const labels = {
  home: "Home",
  section: (key: string) => key,
};

describe("resolveWorkspaceNavBreadcrumbs", () => {
  it("returns home >> deal rooms for the deal-rooms list", () => {
    expect(resolveWorkspaceNavBreadcrumbs("/acme/deal-rooms", "acme", labels)).toEqual([
      { label: "Home", to: "/acme/dashboard" },
      { label: "sidebar.nav.dealRooms", to: "/acme/deal-rooms" },
    ]);
  });

  it("keeps the same section crumb for nested deal-room routes", () => {
    expect(resolveWorkspaceNavBreadcrumbs("/acme/deal-rooms/room-1", "acme", labels)).toEqual([
      { label: "Home", to: "/acme/dashboard" },
      { label: "sidebar.nav.dealRooms", to: "/acme/deal-rooms" },
    ]);
  });

  it("returns empty crumbs on dashboard", () => {
    expect(resolveWorkspaceNavBreadcrumbs("/acme/dashboard", "acme", labels)).toEqual([]);
  });

  it("maps other workspace nav sections", () => {
    expect(resolveWorkspaceNavBreadcrumbs("/acme/documents", "acme", labels)[1]?.label).toBe(
      "sidebar.nav.documents"
    );
    expect(resolveWorkspaceNavBreadcrumbs("/acme/links", "acme", labels)[1]?.label).toBe(
      "sidebar.nav.links"
    );
    expect(resolveWorkspaceNavBreadcrumbs("/acme/contacts", "acme", labels)[1]?.label).toBe(
      "sidebar.nav.contacts"
    );
    expect(resolveWorkspaceNavBreadcrumbs("/acme/settings/general", "acme", labels)[1]?.label).toBe(
      "sidebar.nav.settings"
    );
  });
});
