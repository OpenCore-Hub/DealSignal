// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DealRoomPortfolioPage } from "./portfolio";
import { ApiError } from "@/lib/apiClient";

const listDDPortfolioViews = vi.fn();
const getDDPortfolioView = vi.fn();
const createDDPortfolioView = vi.fn();
const deleteDDPortfolioView = vi.fn();
const getDealRooms = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    listDDPortfolioViews: (...args: unknown[]) => listDDPortfolioViews(...args),
    getDDPortfolioView: (...args: unknown[]) => getDDPortfolioView(...args),
    createDDPortfolioView: (...args: unknown[]) => createDDPortfolioView(...args),
    deleteDDPortfolioView: (...args: unknown[]) => deleteDDPortfolioView(...args),
    getDealRooms: (...args: unknown[]) => getDealRooms(...args),
  },
}));

async function renderPage() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    resources: {
      en: {
        dealRooms: {
          diligence: {
            packs: {
              financing_dd_v1: "Financing DD",
              ma_redflag_v1: "M&A red flags",
            },
          },
          portfolio: {
            title: "Diligence portfolio",
            description: "Aggregate snapshots",
            disabled: "Diligence portfolio is not enabled for this environment.",
            create: "New view",
            cancelCreate: "Cancel",
            createTitle: "Create portfolio view",
            createHint: "Select rooms",
            nameLabel: "View name",
            namePlaceholder: "Name",
            packLabel: "Checklist pack",
            roomsLabel: "Deal rooms",
            noRooms: "No deal rooms yet.",
            createSubmit: "Create view",
            saving: "Creating…",
            createRequired: "Enter a name and select at least one room",
            created: "Portfolio view created",
            createFailed: "Failed to create portfolio view",
            deleted: "Portfolio view deleted",
            deleteFailed: "Failed to delete portfolio view",
            delete: "Delete view",
            loading: "Loading portfolio…",
            emptyTitle: "No portfolio views yet",
            emptyDescription: "Create a view",
            viewsTitle: "Views",
            detailTitle: "Coverage summary",
            detailDescription: "Read-only snapshot counts",
            selectView: "Select a view",
            viewMissing: "View not found.",
            roomCount: "{{count}} rooms",
            packLine: "Pack: {{pack}}",
            roomSummary:
              "{{supported}} supported · {{absent}} absent · {{insufficient}} insufficient ({{total}})",
            noSnapshot: "No coverage snapshot yet",
            stale: "Stale",
            drillDown: "Open diligence",
          },
        },
      },
    },
  });

  return render(
    <I18nextProvider i18n={instance}>
      <MemoryRouter initialEntries={["/acme/deal-rooms/portfolio"]}>
        <Routes>
          <Route path="/:workspaceSlug/deal-rooms/portfolio" element={<DealRoomPortfolioPage />} />
          <Route path="/:workspaceSlug/deal-rooms/:roomId" element={<div>room</div>} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("DealRoomPortfolioPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getDealRooms.mockResolvedValue({
      data: [
        { id: "room-1", name: "Room A", description: "", tags: [] },
        { id: "room-2", name: "Room B", description: "", tags: [] },
      ],
    });
    listDDPortfolioViews.mockResolvedValue({ data: [] });
  });

  it("shows empty state then creates a view with room summary", async () => {
    createDDPortfolioView.mockResolvedValue({
      id: "view-1",
      name: "Series A",
      pack_id: "financing_dd_v1",
      created_by: "u1",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      rooms: [
        {
          deal_room_id: "room-1",
          deal_room_name: "Room A",
          has_snapshot: true,
          supported: 1,
          absent: 2,
          insufficient: 1,
          total: 4,
          top_absent: [{ item_id: "option_pool", label: "Option pool" }],
        },
      ],
    });
    listDDPortfolioViews
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValue({
        data: [
          {
            id: "view-1",
            name: "Series A",
            pack_id: "financing_dd_v1",
            room_count: 1,
            created_by: "u1",
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
        ],
      });
    getDDPortfolioView.mockResolvedValue({
      id: "view-1",
      name: "Series A",
      pack_id: "financing_dd_v1",
      created_by: "u1",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      rooms: [
        {
          deal_room_id: "room-1",
          deal_room_name: "Room A",
          has_snapshot: true,
          supported: 1,
          absent: 2,
          insufficient: 1,
          total: 4,
          top_absent: [{ item_id: "option_pool", label: "Option pool" }],
        },
      ],
    });

    await renderPage();
    expect(await screen.findByText("No portfolio views yet")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("portfolio-create-toggle"));
    fireEvent.change(screen.getByLabelText("View name"), { target: { value: "Series A" } });
    fireEvent.click(screen.getByText("Room A"));
    fireEvent.click(screen.getByTestId("portfolio-create-submit"));
    await waitFor(() =>
      expect(createDDPortfolioView).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Series A",
          room_ids: ["room-1"],
        }),
      ),
    );
    expect(await screen.findByTestId("portfolio-room-room-1")).toHaveTextContent("Option pool");
    expect(screen.getByTestId("portfolio-drilldown-room-1")).toHaveTextContent("Open diligence");
  });

  it("shows disabled state", async () => {
    listDDPortfolioViews.mockRejectedValue(
      new ApiError({
        status: 404,
        code: "portfolio_disabled",
        message: "disabled",
        requestId: "r1",
      }),
    );
    await renderPage();
    expect(await screen.findByText(/not enabled/i)).toBeInTheDocument();
  });
});
