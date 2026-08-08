// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { InsightsSuggestionsPage } from "./suggestions";
import type { Suggestion } from "@/types";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getSuggestionsMock, dismissSuggestionMock, snoozeSuggestionMock } = vi.hoisted(() => ({
  getSuggestionsMock: vi.fn(),
  dismissSuggestionMock: vi.fn(),
  snoozeSuggestionMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getSuggestions: getSuggestionsMock,
    dismissSuggestion: dismissSuggestionMock,
    snoozeSuggestion: snoozeSuggestionMock,
  },
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const mockSuggestions: Suggestion[] = [
  {
    id: "sg-1",
    contactId: "c-1",
    contactEmail: "sarah@example.com",
    documentTitle: "Q3 Pitch",
    linkId: "link-1",
    heatLevel: "hot",
    score: 92,
    reason: "Viewed financial slide twice",
    action: "Follow up on terms",
    lastActivityAt: "2026-06-24T00:00:00Z",
  },
  {
    id: "sg-2",
    contactId: "c-2",
    contactEmail: "marcus@example.com",
    documentTitle: "Series A Deck",
    linkId: "link-2",
    heatLevel: "warm",
    score: 74,
    reason: "Revisited team slide",
    action: "Send founder bios",
    lastActivityAt: "2026-06-23T00:00:00Z",
  },
];

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
        <MemoryRouter initialEntries={["/acme/insights/suggestions"]}>
          <Routes>
            <Route path=":workspaceSlug/insights/suggestions" element={<InsightsSuggestionsPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("InsightsSuggestionsPage", () => {
  beforeEach(() => {
    getSuggestionsMock.mockReset();
    dismissSuggestionMock.mockReset();
    snoozeSuggestionMock.mockReset();
    dismissSuggestionMock.mockResolvedValue(undefined);
    snoozeSuggestionMock.mockResolvedValue({ id: "sg-1", snoozed_until: "2026-06-25T00:00:00Z" });
    vi.spyOn(window, "open").mockImplementation(() => null);
  });

  it("renders loading skeletons", async () => {
    getSuggestionsMock.mockReturnValue(new Promise(() => {}));
    await renderPage();

    expect(document.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0);
  });

  it("renders suggestions after loading", async () => {
    getSuggestionsMock.mockResolvedValue({ data: mockSuggestions });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Follow up on terms")).toBeInTheDocument();
    });

    expect(screen.getByText("sarah@example.com")).toBeInTheDocument();
    expect(screen.getByText("Q3 Pitch")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /view contact/i })).toHaveLength(2);
    expect(screen.getAllByRole("button", { name: /write follow-up email/i })).toHaveLength(2);
    expect(screen.getAllByRole("button", { name: /dismiss/i })).toHaveLength(2);
  });

  it("opens a mailto follow-up when Write follow-up email is clicked", async () => {
    getSuggestionsMock.mockResolvedValue({ data: mockSuggestions });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Follow up on terms")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByRole("button", { name: /write follow-up email/i })[0]!);

    expect(window.open).toHaveBeenCalled();
    const href = String(vi.mocked(window.open).mock.calls[0]?.[0] ?? "");
    expect(href.startsWith("mailto:sarah@example.com?")).toBe(true);
    expect(href).toContain("Follow-up");
    expect(href).toContain("Q3");
  });

  it("dismisses a suggestion via the API and hides the card", async () => {
    getSuggestionsMock.mockResolvedValue({ data: mockSuggestions });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Follow up on terms")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByRole("button", { name: /dismiss/i })[0]!);

    await waitFor(() => {
      expect(dismissSuggestionMock).toHaveBeenCalledWith("link-1", "sg-1");
    });
    await waitFor(() => {
      expect(screen.queryByText("Follow up on terms")).not.toBeInTheDocument();
    });
    expect(screen.getByText("Send founder bios")).toBeInTheDocument();
  });

  it("snoozes a suggestion via the API and hides the card", async () => {
    getSuggestionsMock.mockResolvedValue({ data: mockSuggestions });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Follow up on terms")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByRole("button", { name: /^snooze$/i })[0]!);
    await waitFor(() => {
      expect(screen.getByText(/snooze 1 day/i)).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText(/snooze 1 day/i));

    await waitFor(() => {
      expect(snoozeSuggestionMock).toHaveBeenCalledWith("sg-1", 24);
    });
    await waitFor(() => {
      expect(screen.queryByText("Follow up on terms")).not.toBeInTheDocument();
    });
    expect(screen.getByText("Send founder bios")).toBeInTheDocument();
  });

  it("renders empty state when there are no suggestions", async () => {
    getSuggestionsMock.mockResolvedValue({ data: [] });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("No suggestions yet")).toBeInTheDocument();
    });
  });

  it("shows error and retries on failure", async () => {
    getSuggestionsMock.mockRejectedValue(new Error("network error"));
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText(/Failed to load/i)).toBeInTheDocument();
    });

    getSuggestionsMock.mockResolvedValue({ data: mockSuggestions });
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("Follow up on terms")).toBeInTheDocument();
    });
  });

  it("opens Formal Ask CTA on deal-room path when dealRoomId is present", async () => {
    getSuggestionsMock.mockResolvedValue({
      data: [
        {
          id: "sg-formal",
          contactId: "c-3",
          contactEmail: "visitor@example.com",
          documentTitle: "Room Deck",
          linkId: "link-room",
          dealRoomId: "room-9",
          heatLevel: "warm",
          score: 60,
          reason: "Formal Q&A awaiting review",
          action: "Review formal answer",
          kind: "formal_ask",
          lastActivityAt: "2026-06-24T00:00:00Z",
        } satisfies Suggestion,
      ],
    });

    const i18nInstance = await initI18n();
    await act(async () => {
      render(
        <I18nextProvider i18n={i18nInstance}>
          <MemoryRouter initialEntries={["/acme/insights/suggestions"]}>
            <Routes>
              <Route path=":workspaceSlug/insights/suggestions" element={<InsightsSuggestionsPage />} />
              <Route
                path=":workspaceSlug/deal-rooms/:roomId"
                element={<div data-testid="deal-room-ask">deal-room-ask</div>}
              />
            </Routes>
          </MemoryRouter>
        </I18nextProvider>,
      );
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /open formal q&a/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /open formal q&a/i }));
    await waitFor(() => {
      expect(screen.getByTestId("deal-room-ask")).toBeInTheDocument();
    });
  });
});
