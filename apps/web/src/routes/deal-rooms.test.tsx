/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DealRoomsPage } from "./deal-rooms";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";

const { getDealRoomsMock, getBillingInfoMock, getWorkspacesMock } = vi.hoisted(() => ({
  getDealRoomsMock: vi.fn(),
  getBillingInfoMock: vi.fn(),
  getWorkspacesMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRooms: getDealRoomsMock,
    getBillingInfo: getBillingInfoMock,
    getWorkspaces: getWorkspacesMock,
  },
}));

const resources = {
  en: {
    dealRooms: {
      page: {
        title: "Data Rooms",
        description: "Manage rooms",
        create: "New data room",
        roomLimitReached: "You've reached the data room limit for your plan. Upgrade to create more.",
        pagination: { prev: "Previous", next: "Next", pageOf: "Page {{page}} of {{totalPages}}" },
      },
      empty: {
        title: "No data rooms yet",
        description: "Create your first",
        action: "New data room",
      },
      search: { placeholder: "Search" },
      filter: { noResults: "No results", clear: "Clear" },
      lastAccessed: "Last accessed {{time}}",
      status: { active: "Active", inactive: "Inactive" },
      stats: { documents: "Documents", views: "Views", activeLinks: "Active links" },
      card: { noViewsYet: "No views yet", viewDocuments: "Documents" },
    },
    common: {
      retry: "Retry",
      error: { codes: { insufficient_role: "Insufficient role", forbidden: "You do not have permission to do this." } },
    },
  },
};

async function renderPage() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    resources,
    interpolation: { escapeValue: false },
  });
  return render(
    <I18nextProvider i18n={instance}>
      <MemoryRouter initialEntries={["/acme/deal-rooms"]}>
        <Routes>
          <Route path=":workspaceSlug/deal-rooms" element={<DealRoomsPage />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("DealRoomsPage plan gates", () => {
  beforeEach(() => {
    getDealRoomsMock.mockReset();
    getBillingInfoMock.mockReset();
    getWorkspacesMock.mockReset();
    vi.mocked(useWorkspaceAccess).mockReturnValue({
      role: "owner",
      loading: false,
      canRead: true,
      canWrite: true,
      canManage: true,
      isGuest: false,
    });
    getWorkspacesMock.mockResolvedValue({
      data: [{ id: "ws1", slug: "acme", name: "Acme", role: "owner" }],
    });
    getDealRoomsMock.mockResolvedValue({
      data: [
        {
          id: "room-1",
          name: "Seed Room",
          description: "",
          template: "startup-fundraising",
          documentCount: 0,
          memberCount: 1,
          pendingApprovals: 0,
          ndaEnabled: false,
          createdAt: "2026-06-24T00:00:00Z",
          status: "active",
        },
      ],
      pagination: { page: 1, page_size: 24, total: 1 },
    });
    getBillingInfoMock.mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 1,
      linksUsed: 0,
      linksLimit: 20,
      roomsUsed: 0,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });
  });

  it("allows create when under room cap", async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByTestId("deal-rooms-create")).toBeEnabled();
    });
    expect(screen.queryByTestId("deal-rooms-limit-hint")).toBeNull();
  });

  it("disables create and shows hint when room quota is exhausted", async () => {
    getBillingInfoMock.mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 1,
      linksUsed: 0,
      linksLimit: 20,
      roomsUsed: 1,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByTestId("deal-rooms-limit-hint")).toBeInTheDocument();
    });
    expect(screen.getByTestId("deal-rooms-create")).toBeDisabled();
  });

  it("lists rooms for guests without fetching billing", async () => {
    vi.mocked(useWorkspaceAccess).mockReturnValue({
      role: "guest",
      loading: false,
      canRead: true,
      canWrite: false,
      canManage: false,
      isGuest: true,
    });
    getWorkspacesMock.mockResolvedValue({
      data: [{ id: "ws1", slug: "acme", name: "Acme", role: "guest" }],
    });
    getBillingInfoMock.mockRejectedValue(new Error("forbidden"));
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText("Seed Room")).toBeInTheDocument();
    });
    expect(screen.queryByText("You do not have permission to do this.")).not.toBeInTheDocument();
    expect(screen.getByTestId("deal-rooms-create")).toBeDisabled();
    expect(getDealRoomsMock).toHaveBeenCalled();
    expect(getBillingInfoMock).not.toHaveBeenCalled();
  });
});
