// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { MetricsCards } from "./MetricsCards";
import { createTestI18n } from "@/i18n/test-utils";

async function renderCards(props = {}) {
  const i18n = await createTestI18n({
    dashboard: {
      "metrics.title": "Key metrics",
      "metrics.activeRooms": "Active rooms",
      "metrics.weeklyVisitors": "Weekly visitors",
      "metrics.pendingQuestions": "Pending Q&A",
      "metrics.highIntentContacts": "High-intent contacts",
      "metrics.aria.activeRooms": "{{count}} active rooms",
      "metrics.aria.weeklyVisitors": "{{count}} weekly visitors",
      "metrics.aria.pendingQuestions": "{{count}} pending questions",
      "metrics.aria.highIntentContacts": "{{count}} high-intent contacts",
    },
    common: {
      back: "Back",
    },
  });
  render(
    <MemoryRouter>
      <I18nextProvider i18n={i18n}>
        <MetricsCards
          workspaceSlug="acme"
          activeRooms={3}
          weeklyVisitors={12}
          pendingQuestions={2}
          highIntentContacts={1500}
          {...props}
        />
      </I18nextProvider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("MetricsCards", () => {
  it("renders all metric labels", async () => {
    await renderCards();
    expect(screen.getByText("Active rooms")).toBeInTheDocument();
    expect(screen.getByText("Weekly visitors")).toBeInTheDocument();
    expect(screen.getByText("Pending Q&A")).toBeInTheDocument();
    expect(screen.getByText("High-intent contacts")).toBeInTheDocument();
  });

  it("formats large numbers compactly", async () => {
    await renderCards();
    expect(screen.getByText("1.5K")).toBeInTheDocument();
  });

  it("links active rooms card to deal rooms", async () => {
    await renderCards();
    const link = screen.getByRole("link", { name: /active rooms/i });
    expect(link).toHaveAttribute("href", "/acme/deal-rooms");
  });

  it("links high-intent contacts card to contacts", async () => {
    await renderCards();
    const link = screen.getByRole("link", { name: /high-intent contacts/i });
    expect(link).toHaveAttribute("href", "/acme/contacts");
  });
});
