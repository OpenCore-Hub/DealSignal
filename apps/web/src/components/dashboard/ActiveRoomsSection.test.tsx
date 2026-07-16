// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { ActiveRoomsSection } from "./ActiveRoomsSection";
import { createTestI18n } from "@/i18n/test-utils";
import type { DealRoom } from "@/types";

const navigate = vi.fn();

vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return { ...actual, useNavigate: () => navigate, useLocation: () => ({ pathname: "/acme", search: "" }) };
});

vi.mock("@/components/deal-rooms/DealRoomShareDialog", () => ({
  DealRoomShareDialog: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

function makeRoom(overrides: Partial<DealRoom> = {}): DealRoom {
  return {
    id: "room-1",
    slug: "seed-round",
    name: "Seed Round",
    description: "Due diligence materials",
    template: "startup-fundraising",
    status: "active",
    documentCount: 5,
    memberCount: 3,
    pendingApprovals: 0,
    ndaEnabled: false,
    createdAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

async function renderSection(rooms: DealRoom[]) {
  const i18n = await createTestI18n({
    dashboard: {
      "sections.activeRooms": "Active deal rooms",
      "empty.rooms.title": "No active rooms",
      "empty.rooms.description": "Create one",
      "empty.rooms.action": "Create",
      "empty.rooms.noDescription": "No description",
      "room.enter": "Enter {{name}}",
      "room.pendingApprovals_one": "{{count}} pending",
      "room.pendingApprovals_other": "{{count}} pending",
      "room.viewAllWithCount_one": "View all {{count}} room",
      "room.viewAllWithCount_other": "View all {{count}} rooms",
    },
    common: {
      viewDetails: "View details",
      viewAll: "View all",
      createLink: "Create link",
      back: "Back",
    },
  });
  render(
    <I18nextProvider i18n={i18n}>
      <ActiveRoomsSection rooms={rooms} workspaceSlug="acme" />
    </I18nextProvider>
  );
}

beforeEach(() => {
  navigate.mockClear();
});

describe("ActiveRoomsSection", () => {
  it("renders empty state when no active rooms", async () => {
    await renderSection([]);
    expect(screen.getByText("No active rooms")).toBeInTheDocument();
  });

  it("navigates to room detail when card is clicked", async () => {
    await renderSection([makeRoom()]);
    fireEvent.click(screen.getByRole("link", { name: /Enter Seed Round/i }));
    expect(navigate).toHaveBeenCalledWith("/acme/deal-rooms/room-1", expect.any(Object));
  });

  it("supports keyboard activation", async () => {
    await renderSection([makeRoom()]);
    const card = screen.getByRole("link", { name: /Enter Seed Round/i });
    fireEvent.keyDown(card, { key: "Enter" });
    expect(navigate).toHaveBeenCalledWith("/acme/deal-rooms/room-1", expect.any(Object));
  });

  it("shows pending approval badge when present", async () => {
    await renderSection([makeRoom({ pendingApprovals: 2 })]);
    expect(screen.getByText("2 pending")).toBeInTheDocument();
  });

  it("limits to four rooms and shows view all link", async () => {
    const rooms = Array.from({ length: 5 }, (_, i) => makeRoom({ id: `room-${i}`, name: `Room ${i}` }));
    await renderSection(rooms);
    expect(screen.getAllByRole("link").length).toBeGreaterThanOrEqual(4);
    expect(screen.getByRole("button", { name: /View all 5 rooms/i })).toBeInTheDocument();
  });
});
