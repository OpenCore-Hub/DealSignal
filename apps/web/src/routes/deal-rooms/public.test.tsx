// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { PublicDealRoomPage } from "./public";
import type { PublicDealRoomView } from "@/lib/api";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getPublicDealRoomMock, requestDealRoomAccessMock, signDealRoomNDAMock } = vi.hoisted(() => ({
  getPublicDealRoomMock: vi.fn(),
  requestDealRoomAccessMock: vi.fn(),
  signDealRoomNDAMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getPublicDealRoom: getPublicDealRoomMock,
    requestDealRoomAccess: requestDealRoomAccessMock,
    signDealRoomNDA: signDealRoomNDAMock,
  },
}));

const mockView: PublicDealRoomView = {
  room: {
    id: "room_1",
    name: "Seed Round Due Diligence",
    description: "Due diligence materials",
    ndaEnabled: true,
    requiresApproval: true,
  },
  member: {
    id: "rm_1",
    email: "sarah@horizon.vc",
    role: "viewer",
    ndaStatus: "signed",
    status: "active",
  },
  folders: [
    { path: "/pitch", name: "01 Pitch Deck", sort_order: 0 },
    { path: "/financials", name: "02 Financials", sort_order: 1 },
  ],
  documents: [
    {
      folder: "/pitch",
      permission: "view",
      documents: [
        {
          id: "rd_1",
          document_id: "doc_1",
          title: "Acme Seed Round Pitch Deck",
          folder_path: "/pitch",
          sort_order: 0,
          source_type: "pdf",
          status: "ready",
          created_at: "2026-06-18T09:30:00Z",
        },
      ],
    },
  ],
};

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
        <MemoryRouter initialEntries={["/r/seed-round-due-diligence"]}>
          <Routes>
            <Route path="/r/:slug" element={<PublicDealRoomPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("PublicDealRoomPage", () => {
  beforeEach(() => {
    getPublicDealRoomMock.mockReset();
    requestDealRoomAccessMock.mockReset();
    signDealRoomNDAMock.mockReset();
  });

  it("asks for email then shows the deal room", async () => {
    getPublicDealRoomMock.mockResolvedValue(mockView);
    await renderPage();

    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: "sarah@horizon.vc" } });
    fireEvent.click(screen.getByRole("button", { name: /continue/i }));

    await waitFor(() => {
      expect(screen.getByText("Seed Round Due Diligence")).toBeInTheDocument();
    });

    expect(screen.getByText("Acme Seed Round Pitch Deck")).toBeInTheDocument();
  });

  it("shows access request form when not a member", async () => {
    getPublicDealRoomMock.mockResolvedValue({
      ...mockView,
      member: null,
    });
    requestDealRoomAccessMock.mockResolvedValue({ request_id: "ra_new" });
    await renderPage();

    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: "new@example.com" } });
    fireEvent.click(screen.getByRole("button", { name: /continue/i }));

    await waitFor(() => {
      expect(screen.getByText(/request access/i)).toBeInTheDocument();
    });
  });

  it("shows NDA signing when NDA is required and not signed", async () => {
    getPublicDealRoomMock.mockReset();
    // First lookup: member exists but NDA not signed yet.
    getPublicDealRoomMock.mockResolvedValueOnce({
      ...mockView,
      member: { ...mockView.member!, ndaStatus: "pending" },
    });
    // After signing NDA, re-check returns signed member.
    getPublicDealRoomMock.mockResolvedValueOnce(mockView);
    signDealRoomNDAMock.mockResolvedValue(undefined);
    await renderPage();

    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: "sarah@horizon.vc" } });
    fireEvent.click(screen.getByRole("button", { name: /continue/i }));

    await waitFor(() => {
      expect(screen.getByText(/sign nda/i)).toBeInTheDocument();
    });
  });
});
