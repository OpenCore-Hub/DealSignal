/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DealRoomsPage } from "./deal-rooms";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";
import { toast } from "sonner";

const { getDealRoomsMock, getBillingInfoMock, getWorkspacesMock, deleteDealRoomMock } = vi.hoisted(() => ({
  getDealRoomsMock: vi.fn(),
  getBillingInfoMock: vi.fn(),
  getWorkspacesMock: vi.fn(),
  deleteDealRoomMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRooms: getDealRoomsMock,
    getBillingInfo: getBillingInfoMock,
    getWorkspaces: getWorkspacesMock,
    deleteDealRoom: deleteDealRoomMock,
  },
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
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
      card: {
        noViewsYet: "No views yet",
        viewDocuments: "Documents",
        deleteTitle: "Delete data room",
        deleteDescription: "Delete “{{name}}”? Documents return to the library. Share links, knowledge, and analytics for this room will be removed. This cannot be undone.",
        deleted: "Data room deleted",
        deleteFailed: "Failed to delete data room",
      },
    },
    common: {
      retry: "Retry",
      cancel: "Cancel",
      delete: "Delete",
      moreActions: "More actions",
      error: {
        deleteFailed: "Failed to delete",
        codes: { insufficient_role: "Insufficient role", forbidden: "You do not have permission to do this." },
      },
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
    deleteDealRoomMock.mockReset();
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.error).mockClear();
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
    expect(screen.queryByTestId("deal-room-menu-room-1")).not.toBeInTheDocument();
  });

  it("hides the delete menu when the caller is not a room admin", async () => {
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
          isAdmin: false,
          createdAt: "2026-06-24T00:00:00Z",
          status: "active",
        },
      ],
      pagination: { page: 1, page_size: 24, total: 1 },
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText("Seed Room")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("deal-room-menu-room-1")).not.toBeInTheDocument();
  });

  it("deletes a data room from the card menu after confirmation", async () => {
    deleteDealRoomMock.mockResolvedValue(undefined);
    getDealRoomsMock
      .mockResolvedValueOnce({
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
            isAdmin: true,
            createdAt: "2026-06-24T00:00:00Z",
            status: "active",
          },
        ],
        pagination: { page: 1, page_size: 24, total: 1 },
      })
      .mockResolvedValueOnce({
        data: [],
        pagination: { page: 1, page_size: 24, total: 0 },
      });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText("Seed Room")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("deal-room-menu-room-1"));
    const menu = await screen.findByRole("menu");
    fireEvent.click(within(menu).getByRole("menuitem", { name: /^Delete$/i }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Delete data room")).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: /^Delete$/i }));

    await waitFor(() => {
      expect(deleteDealRoomMock).toHaveBeenCalledWith("room-1");
    });
    expect(toast.success).toHaveBeenCalledWith("Data room deleted");
    await waitFor(() => {
      expect(screen.queryByText("Seed Room")).not.toBeInTheDocument();
    });
  });
});
