// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { InsightsSuggestionsPage } from "./suggestions";
import type { Suggestion } from "@/types";

const { getSuggestionsMock } = vi.hoisted(() => ({
  getSuggestionsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getSuggestions: getSuggestionsMock,
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

const resources = {
  en: {
    insights: {
      suggestions: {
        emptyTitle: "No suggestions yet",
        emptyDescription: "No pending suggestions.",
        viewContact: "View contact",
        writeEmail: "Write follow-up email",
        emailDisabled: "Email sending requires backend support",
        score: "{{score}} pts",
      },
    },
    common: {
      retry: "Retry",
      error: { loadFailed: "Failed to load" },
      heat: { hot: "Hot", warm: "Warm", cold: "Cold" },
    },
  },
};

async function initI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["insights", "common"],
    defaultNS: "insights",
    resources,
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
      </I18nextProvider>
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("InsightsSuggestionsPage", () => {
  beforeEach(() => {
    getSuggestionsMock.mockReset();
  });

  it("renders loading skeletons", async () => {
    getSuggestionsMock.mockReturnValue(new Promise(() => {}));
    await renderPage();

    expect(document.querySelectorAll(".animate-pulse").length).toBeGreaterThan(0);
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
  });

  it("renders empty state when there are no suggestions", async () => {
    getSuggestionsMock.mockResolvedValue({ data: [] });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("No suggestions yet")).toBeInTheDocument();
    });
    expect(screen.getByText("No pending suggestions.")).toBeInTheDocument();
  });

  it("shows error and retries on failure", async () => {
    getSuggestionsMock.mockRejectedValue(new Error("network error"));
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("network error")).toBeInTheDocument();
    });

    getSuggestionsMock.mockResolvedValue({ data: mockSuggestions });
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("Follow up on terms")).toBeInTheDocument();
    });
  });
});
