// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { RiskAlertList } from "./RiskAlertList";
import { createTestI18n } from "@/i18n/test-utils";
import type { RiskAlert } from "@/types";

function makeAlert(overrides: Partial<RiskAlert> = {}): RiskAlert {
  return {
    id: "risk-1",
    type: "download",
    priority: "medium",
    title: "Unidentified download",
    description: "An unrecognized visitor downloaded a document.",
    createdAt: "2026-06-18T09:00:00Z",
    ...overrides,
  };
}

async function renderList(alerts: RiskAlert[]) {
  const i18n = await createTestI18n({
    common: {
      back: "Back",
      "priority.high": "High",
      "priority.medium": "Medium",
      "priority.low": "Low",
    },
  });
  return render(
    <MemoryRouter>
      <I18nextProvider i18n={i18n}>
        <RiskAlertList alerts={alerts} workspaceSlug="acme" />
      </I18nextProvider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("RiskAlertList", () => {
  it("returns nothing when alerts are empty", async () => {
    const view = await renderList([]);
    expect(view.container.firstChild).toBeNull();
  });

  it("links to document detail when documentId is present", async () => {
    await renderList([makeAlert({ documentId: "doc-1" })]);
    const link = screen.getByRole("link", { name: /Unidentified download/i });
    expect(link).toHaveAttribute("href", "/acme/documents/doc-1");
  });

  it("links to link detail when linkId is present", async () => {
    await renderList([makeAlert({ linkId: "link-1" })]);
    const link = screen.getByRole("link", { name: /Unidentified download/i });
    expect(link).toHaveAttribute("href", "/acme/links/link-1");
  });

  it("prefers document navigation over link navigation", async () => {
    await renderList([makeAlert({ documentId: "doc-1", linkId: "link-1" })]);
    const link = screen.getByRole("link", { name: /Unidentified download/i });
    expect(link).toHaveAttribute("href", "/acme/documents/doc-1");
  });

  it("renders non-clickable item when no document or link id", async () => {
    await renderList([makeAlert()]);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText("Unidentified download")).toBeInTheDocument();
  });

  it("sorts alerts by severity high -> low", async () => {
    await renderList([
      makeAlert({ id: "low", priority: "low", title: "Low risk" }),
      makeAlert({ id: "high", priority: "high", title: "High risk" }),
      makeAlert({ id: "medium", priority: "medium", title: "Medium risk" }),
    ]);
    const titles = screen.getAllByRole("listitem").map((li) => li.textContent);
    expect(titles[0]).toContain("High risk");
    expect(titles[1]).toContain("Medium risk");
    expect(titles[2]).toContain("Low risk");
  });

  it("shows severity badge", async () => {
    await renderList([makeAlert({ priority: "high" })]);
    expect(screen.getByText("High")).toBeInTheDocument();
  });
});
