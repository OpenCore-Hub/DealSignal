// @vitest-environment jsdom
import { describe, it, expect, vi, type Mock } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import type { RadarWorkItem } from "@/lib/radarQueue";
import { RadarRow } from "./RadarRow";

function makeItem(over: Partial<RadarWorkItem> = {}): RadarWorkItem {
  return {
    id: "act-1",
    product: "buying_window",
    headline: "Follow up with buyer",
    subtitle: "",
    verb: "email",
    priority: "high",
    slaDueAt: "2026-06-21T18:00:00Z",
    createdAt: "2026-06-20T18:00:00Z",
    dealKey: "link:l1",
    dealName: "Deck",
    actionId: "act-1",
    contactEmail: "buyer@acme.test",
    navigatePath: "/acme/links/l1",
    evidencePath: "/acme/links/l1",
    ...over,
  };
}

async function renderRow(
  item: RadarWorkItem,
  handlers: {
    onPrimary?: Mock<(item: RadarWorkItem) => void>;
    onSelect?: Mock<(item: RadarWorkItem) => void>;
    onEvidence?: Mock<(item: RadarWorkItem) => void>;
  } = {},
  opts: { hideProductLabel?: boolean } = {},
) {
  const i18n = await createTestI18n({
    dashboard: {
      "radar.products.buying_window": "Hot intent",
      "radar.products.diligence_gate": "Diligence gate",
      "radar.cta.approve": "Approve",
      "radar.cta.email": "Email",
      "radar.cta.review": "Review",
      "radar.evidence": "Evidence",
      "radar.dealFallback": "Untitled deal",
      "radar.outcome.choose": "Choose outcome",
      "radar.outcome.acted": "Acted",
      "radar.snoozeHours.24": "Snooze 24h",
      "radar.snoozeHours.72": "Snooze 72h",
      "radar.snoozeHours.168": "Snooze 1w",
      "actions.moreOptions": "More",
      "actions.ignore": "Ignore",
    },
    common: {
      complete: "Complete",
      dueDate: "Due",
      "overdue.days_one": "{{count}} day overdue",
      "overdue.days_other": "{{count}} days overdue",
    },
  });

  const onPrimary = handlers.onPrimary ?? vi.fn<(item: RadarWorkItem) => void>();
  const onSelect = handlers.onSelect ?? vi.fn<(item: RadarWorkItem) => void>();
  const onEvidence = handlers.onEvidence ?? vi.fn<(item: RadarWorkItem) => void>();

  render(
    <I18nextProvider i18n={i18n}>
      <RadarRow
        item={item}
        hideProductLabel={opts.hideProductLabel}
        onPrimary={onPrimary}
        onSelect={onSelect}
        onEvidence={onEvidence}
      />
    </I18nextProvider>,
  );

  return { onPrimary, onSelect, onEvidence };
}

describe("RadarRow", () => {
  it("hides Email primary CTA for Hot intent and selects on row click", async () => {
    const { onPrimary, onSelect } = await renderRow(makeItem());

    expect(
      screen.queryByRole("button", { name: /^Email$/i }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("Follow up with buyer"));
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onPrimary).not.toHaveBeenCalled();
  });

  it("keeps Evidence affordance when onEvidence is provided", async () => {
    const { onEvidence } = await renderRow(makeItem());

    fireEvent.click(screen.getByTestId("radar-evidence-link"));
    expect(onEvidence).toHaveBeenCalledTimes(1);
  });

  it("shows Approve primary CTA for non-email verbs", async () => {
    const { onPrimary } = await renderRow(
      makeItem({
        product: "diligence_gate",
        verb: "approve",
        headline: "Approve access",
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: /^Approve$/i }));
    expect(onPrimary).toHaveBeenCalledTimes(1);
  });

  it("hides product label visually when filter is active", async () => {
    await renderRow(makeItem(), {}, { hideProductLabel: true });
    const label = screen.getByTestId("radar-product-label");
    expect(label).toHaveAttribute("aria-hidden", "true");
    expect(label.className).toMatch(/invisible/);
  });
});
