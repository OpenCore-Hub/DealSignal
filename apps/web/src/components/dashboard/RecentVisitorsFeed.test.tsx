// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { RecentVisitorsFeed } from "./RecentVisitorsFeed";
import { createTestI18n } from "@/i18n/test-utils";
import type { InsightsOverview } from "@/lib/api";

const validId = "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11";
const emailFallback = "visitor@example.com";

async function renderFeed(insights: InsightsOverview | null) {
  const i18n = await createTestI18n({
    dashboard: {
      "sections.recentVisitors": "Recent visitors",
      "empty.visitors.description": "No visitors yet",
      "visitor.score": "Score {{score}}",
      "visitor.viewProfile": "View profile of {{email}}",
    },
    common: {
      "heat.hot": "Hot",
      "heat.warm": "Warm",
      "heat.cold": "Cold",
    },
  });
  return render(
    <MemoryRouter>
      <I18nextProvider i18n={i18n}>
        <RecentVisitorsFeed insights={insights} workspaceSlug="acme" />
      </I18nextProvider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
});

describe("RecentVisitorsFeed", () => {
  it("makes the row a link when a real contact id is present", async () => {
    const insights: InsightsOverview = {
      tierCounts: { hot: 1, warm: 0, cold: 0 },
      topDocuments: [],
      topLinks: [],
      topContacts: [
        { id: validId, email: "lead@example.com", score: 95, heatLevel: "hot" },
      ],
    };
    await renderFeed(insights);
    const row = screen.getByRole("link", { name: /lead@example.com/i });
    expect(row).toHaveAttribute("tabIndex", "0");
  });

  it("does not make the row clickable when the backend returns an email fallback id", async () => {
    const insights: InsightsOverview = {
      tierCounts: { hot: 0, warm: 0, cold: 0 },
      topDocuments: [],
      topLinks: [],
      topContacts: [
        { id: emailFallback, email: emailFallback, score: 12, heatLevel: "cold" },
      ],
    };
    await renderFeed(insights);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    const row = screen.getByText(emailFallback).closest("div[class*='rounded-lg']");
    expect(row).not.toHaveAttribute("role");
    expect(row).not.toHaveAttribute("tabIndex");
  });

  it("renders empty state when no visitors", async () => {
    await renderFeed(null);
    expect(screen.getByText("No visitors yet")).toBeInTheDocument();
  });
});
