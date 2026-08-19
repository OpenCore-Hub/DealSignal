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
      "radar.products.access_decay": "Access expiring",
      "radar.cta.approve": "Approve",
      "radar.cta.email": "Email",
      "radar.cta.renew": "Renew",
      "radar.suggestion.email": "Suggested: email {{contact}}",
      "radar.suggestion.roomExpiry": "Room expiry can't be changed in this app",
      "radar.ctaByProduct.buying_window.confirmRecipient": "Confirm who to email",
      "radar.cta.review": "Review",
      "radar.ctaByProduct.diligence_gate.review": "See this hold",
      "radar.ctaByProduct.leak_watch.review": "Review sharing",
      "radar.evidence": "Evidence",
      "radar.dealFallback": "Untitled deal",
      "radar.shareContact": "Share contact {{name}}",
      "radar.products.leak_watch": "Check sharing",
      "radar.confidence.low": "Thin evidence",
      "radar.outcome.choose": "Choose outcome",
      "radar.outcome.acted": "Acted",
      "radar.outcome.false_positive": "No action needed",
      "radar.outcomeByProduct.diligence_gate.review.acted": "Reviewed this hold",
      "radar.outcomeByProduct.diligence_gate.review.false_positive":
        "Hold stands — no action",
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
  it("names the email suggestion after the known contact and does not offer a write-email ACT", async () => {
    const { onPrimary, onSelect } = await renderRow(makeItem());

    expect(screen.getByTestId("radar-email-suggestion")).toHaveTextContent(
      "Suggested: email buyer@acme.test",
    );
    expect(
      screen.queryByRole("button", { name: /email/i }),
    ).not.toBeInTheDocument();
    expect(onPrimary).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText("Follow up with buyer"));
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it("hides Email when the warm card has no contact address", async () => {
    await renderRow(makeItem({ contactEmail: undefined }));
    expect(
      screen.queryByRole("button", { name: /Email/i }),
    ).not.toBeInTheDocument();
  });

  it("asks the host to confirm the recipient when the warm card has no email", async () => {
    const { onPrimary } = await renderRow(
      makeItem({
        verb: "open",
        contactEmail: undefined,
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: /^Confirm who to email$/i }),
    );
    expect(onPrimary).toHaveBeenCalledTimes(1);
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

  it("does not treat a share contact name as the person held at the gate", async () => {
    await renderRow(
      makeItem({
        product: "diligence_gate",
        verb: "review",
        actor: "张姐",
        headline: "An investor is still waiting to enter",
        headlineCode: undefined,
      }),
    );
    expect(screen.queryByText("张姐")).not.toBeInTheDocument();
    expect(screen.getByTestId("radar-share-contact")).toHaveTextContent(
      "Share contact 张姐",
    );
  });

  it("uses the gated visitor email as the waiting-to-enter protagonist", async () => {
    await renderRow(
      makeItem({
        product: "diligence_gate",
        verb: "review",
        actor: "yqx-401@126.com",
        contactEmail: "yqx-401@126.com",
        headline: "An investor is still waiting to enter",
        headlineCode: undefined,
      }),
    );
    expect(screen.getByText("yqx-401@126.com")).toBeInTheDocument();
    expect(screen.queryByTestId("radar-share-contact")).not.toBeInTheDocument();
  });

  it("names the share contact beside the gated visitor email", async () => {
    await renderRow(
      makeItem({
        product: "diligence_gate",
        verb: "review",
        actor: "yqx-401@126.com",
        contactEmail: "zhang@share.example",
        headline: "An investor is still waiting to enter",
        headlineCode: undefined,
      }),
    );
    expect(screen.getByText("yqx-401@126.com")).toBeInTheDocument();
    expect(screen.getByTestId("radar-share-contact")).toHaveTextContent(
      "Share contact zhang@share.example",
    );
  });

  it("labels a gate-hold primary CTA as seeing the hold, not a generic check", async () => {
    await renderRow(
      makeItem({
        product: "diligence_gate",
        verb: "review",
        headline: "An investor is still waiting to enter",
      }),
    );
    expect(
      screen.getByRole("button", { name: /^See this hold$/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Review$/i }),
    ).not.toBeInTheDocument();
  });

  it("does not offer Renew for a deal-room expiry with no editor", async () => {
    const { onPrimary } = await renderRow(
      makeItem({
        product: "access_decay",
        verb: "renew",
        headline: "Room access expiring",
        dealRoomId: "room-1",
        linkId: undefined,
        navigatePath: "/acme/deal-rooms/room-1",
        evidencePath: "/acme/deal-rooms/room-1",
      }),
    );
    expect(screen.getByTestId("radar-room-expiry-suggestion")).toHaveTextContent(
      "Room expiry can't be changed in this app",
    );
    expect(
      screen.queryByRole("button", { name: /^Renew$/i }),
    ).not.toBeInTheDocument();
    expect(onPrimary).not.toHaveBeenCalled();
  });

  it("still offers Renew for a library share expiry", async () => {
    const { onPrimary } = await renderRow(
      makeItem({
        product: "access_decay",
        verb: "renew",
        headline: "Link expires soon",
        linkId: "l1",
        dealRoomId: undefined,
        navigatePath: "/acme/links/l1/edit?focus=expiry",
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: /^Renew$/i }));
    expect(onPrimary).toHaveBeenCalledTimes(1);
  });
});
