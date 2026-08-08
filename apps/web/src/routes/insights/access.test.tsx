// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { InsightsAccessPage } from "./access";
import type { AccessAudit } from "@/lib/api";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getAccessAuditMock } = vi.hoisted(() => ({
  getAccessAuditMock: vi.fn(),
}));

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    api: {
      ...actual.api,
      getAccessAudit: getAccessAuditMock,
    },
  };
});

const mockAudit: AccessAudit = {
  rangeDays: 30,
  generatedAt: "2026-08-08T04:00:00Z",
  totalEvents: 2,
  byType: [
    { eventType: "invalid_password", count: 1 },
    { eventType: "blocked_email", count: 1 },
  ],
  byDealRoom: [
    { dealRoomId: "room-1", dealRoomName: "Series A", count: 1 },
    { dealRoomId: null, dealRoomName: "", count: 1, scope: "library" },
  ],
  byMember: [{ memberId: "member-1", memberEmail: "owner@example.com", count: 2 }],
  byFolder: [
    {
      folderPath: "Finance",
      dealRoomId: "room-1",
      dealRoomName: "Series A",
      count: 1,
    },
  ],
  events: [
    {
      id: "ev-1",
      eventType: "invalid_password",
      createdAt: "2026-08-07T12:00:00Z",
      documentTitle: "Pitch Deck",
      dealRoomName: "Series A",
      dealRoomId: "room-1",
      email: "buyer@example.com",
      reason: "bad password",
      memberId: "member-1",
      memberEmail: "owner@example.com",
      folderPath: "Finance",
    },
  ],
  hasMore: false,
  limit: 25,
  offset: 0,
};

async function initI18n() {
  const instance = i18n.createInstance();
  const insightsJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/insights.json"), "utf-8"),
  );
  const commonJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"),
  );
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["insights", "common"],
    defaultNS: "insights",
    resources: { en: { insights: insightsJson, common: commonJson } },
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
        <MemoryRouter initialEntries={["/acme/insights/access"]}>
          <Routes>
            <Route path=":workspaceSlug/insights/access" element={<InsightsAccessPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );
    await new Promise((r) => setTimeout(r, 0));
  });
  return result;
}

describe("InsightsAccessPage", () => {
  beforeEach(() => {
    getAccessAuditMock.mockReset();
    getAccessAuditMock.mockResolvedValue(mockAudit);
  });

  it("renders type buckets and event rows", async () => {
    await renderPage();
    await waitFor(() => expect(getAccessAuditMock).toHaveBeenCalled());
    expect(screen.queryByText("By data room")).not.toBeInTheDocument();
    expect(screen.queryByText("By folder")).not.toBeInTheDocument();
    expect(screen.getByText("Denied attempts")).toBeInTheDocument();
    expect(screen.getByText("buyer@example.com")).toBeInTheDocument();
    expect(screen.getByText("Pitch Deck")).toBeInTheDocument();
    expect(screen.getAllByText("owner@example.com").length).toBeGreaterThan(0);
  });

  it("filters by event type when a bucket is clicked", async () => {
    await renderPage();
    await waitFor(() => expect(getAccessAuditMock).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: /Invalid password/i }));
    await waitFor(() =>
      expect(getAccessAuditMock).toHaveBeenLastCalledWith(
        expect.objectContaining({ eventType: "invalid_password", days: 30 }),
      ),
    );
  });

  it("filters by link owner bucket", async () => {
    await renderPage();
    await waitFor(() => expect(getAccessAuditMock).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: /owner@example.com/i }));
    await waitFor(() =>
      expect(getAccessAuditMock).toHaveBeenLastCalledWith(
        expect.objectContaining({ memberId: "member-1", days: 30 }),
      ),
    );
  });
});
