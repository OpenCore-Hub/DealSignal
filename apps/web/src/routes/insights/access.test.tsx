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

const { getAccessAuditMock, getPendingLinkAccessRequestsMock } = vi.hoisted(() => ({
  getAccessAuditMock: vi.fn(),
  getPendingLinkAccessRequestsMock: vi.fn(),
}));

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    api: {
      ...actual.api,
      getAccessAudit: getAccessAuditMock,
      getPendingLinkAccessRequests: getPendingLinkAccessRequestsMock,
    },
  };
});

const mockAudit: AccessAudit = {
  rangeDays: 30,
  generatedAt: "2026-08-08T04:00:00Z",
  totalEvents: 2,
  byType: [
    { eventType: "security_gate_failed", count: 1 },
    { eventType: "invalid_password", count: 1 },
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
      eventType: "security_gate_failed",
      createdAt: "2026-08-07T12:00:00Z",
      documentTitle: "Pitch Deck",
      dealRoomName: "Series A",
      dealRoomId: "room-1",
      email: "buyer@example.com",
      reason: "email_code_required",
      memberId: "member-1",
      memberEmail: "owner@example.com",
      folderPath: "Finance",
    },
    {
      id: "ev-2",
      eventType: "invalid_password",
      createdAt: "2026-08-07T11:00:00Z",
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
    getPendingLinkAccessRequestsMock.mockReset();
    getAccessAuditMock.mockResolvedValue(mockAudit);
    getPendingLinkAccessRequestsMock.mockResolvedValue({ data: [] });
  });

  it("renders type buckets and reason-first event rows", async () => {
    await renderPage();
    await waitFor(() => expect(getAccessAuditMock).toHaveBeenCalled());
    expect(screen.queryByText("By data room")).not.toBeInTheDocument();
    expect(screen.getByText("Held at gate")).toBeInTheDocument();
    expect(screen.getByTestId("access-scope-hint")).toBeInTheDocument();
    expect(screen.getByText("Pending: verify email")).toBeInTheDocument();
    expect(screen.getAllByText("Unclassified gate hold").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Invalid password").length).toBeGreaterThan(0);
    expect(screen.getAllByText("buyer@example.com").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Pitch Deck").length).toBeGreaterThan(0);
    expect(screen.getAllByText("owner@example.com").length).toBeGreaterThan(0);
  });

  it("bridges pending Share access requests without changing denial KPIs", async () => {
    getPendingLinkAccessRequestsMock.mockResolvedValue({
      data: [
        {
          id: "lar_1",
          link_id: "link_1",
          email: "visitor@example.com",
          status: "pending",
          created_at: "2026-08-07T12:00:00Z",
          updated_at: "2026-08-07T12:00:00Z",
        },
      ],
    });
    await renderPage();
    await waitFor(() =>
      expect(screen.getByTestId("access-pending-requests-bridge")).toBeInTheDocument(),
    );
    expect(getPendingLinkAccessRequestsMock).toHaveBeenCalledWith({ scope: "document" });
    const cta = screen.getByRole("link", { name: /Open Share/i });
    expect(cta).toHaveAttribute("href", "/acme/documents?tab=shared");
    // Denied KPI stays audit totalEvents (2), not pending request count (1).
    expect(screen.getByText("Held at gate").closest("[data-slot='card']")).toHaveTextContent(
      /^[\s\S]*2[\s\S]*Gate holds only/,
    );
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
