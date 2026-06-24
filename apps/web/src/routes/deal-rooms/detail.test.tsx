// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { DealRoomDetailPage } from "./detail";
import type { DealRoom, DealRoomTemplate } from "@/types";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getDealRoomByIdMock, getDealRoomTemplatesMock } = vi.hoisted(() => ({
  getDealRoomByIdMock: vi.fn(),
  getDealRoomTemplatesMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomById: getDealRoomByIdMock,
    getDealRoomTemplates: getDealRoomTemplatesMock,
  },
}));

const mockRoom: DealRoom = {
  id: "room-1",
  name: "Series A Data Room",
  description: "Due diligence materials",
  template: "series-a",
  documentCount: 3,
  memberCount: 2,
  pendingApprovals: 1,
  ndaEnabled: true,
  createdAt: "2026-06-20T10:00:00Z",
  status: "active",
  uploadedFiles: ["Pitch deck", "Financial model"],
};

const mockTemplates: DealRoomTemplate[] = [
  {
    id: "tpl-series-a",
    name: "Series A",
    description: "Growth-stage data room",
    scenario: "series-a",
    folderStructure: [{ name: "Financials" }],
    recommendedFiles: ["Pitch deck", "Financial model", "Cap table"],
    defaultPermissionLevel: "medium",
    ndaEnabled: true,
  },
];

async function initI18n() {
  const instance = i18n.createInstance();
  const dealRoomsJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/dealRooms.json"), "utf-8"));
  const commonJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"));
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["dealRooms", "common"],
    defaultNS: "dealRooms",
    resources: { en: { dealRooms: dealRoomsJson, common: commonJson } },
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderPage() {
  const i18nInstance = await initI18n();
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={["/acme/deal-rooms/room-1"]}>
          <Routes>
            <Route path=":workspaceSlug/deal-rooms/:roomId" element={<DealRoomDetailPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("DealRoomDetailPage", () => {
  beforeEach(() => {
    getDealRoomByIdMock.mockReset();
    getDealRoomTemplatesMock.mockReset();
    getDealRoomTemplatesMock.mockResolvedValue({ data: mockTemplates });
  });

  it("renders loading skeleton", async () => {
    getDealRoomByIdMock.mockReturnValue(new Promise(() => {}));
    await renderPage();
    expect(document.querySelector("[aria-busy='true']")).toBeInTheDocument();
  });

  it("renders deal room details and checklist", async () => {
    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Series A Data Room")).toBeInTheDocument();
    });

    expect(screen.getByText("Due diligence materials")).toBeInTheDocument();
    expect(screen.getByText("Financials")).toBeInTheDocument();
    expect(screen.getByText("Pitch deck")).toBeInTheDocument();
  });

  it("shows error and retries on failure", async () => {
    getDealRoomByIdMock.mockRejectedValue(new Error("network error"));
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("network error")).toBeInTheDocument();
    });

    getDealRoomByIdMock.mockResolvedValue(mockRoom);
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("Series A Data Room")).toBeInTheDocument();
    });
  });
});
