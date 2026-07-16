// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Routes, Route } from "react-router";
import { SignalCard } from "./SignalCard";
import { createTestI18n } from "@/i18n/test-utils";
import type { Signal, ActionItem } from "@/types";

function makeSignal(overrides: Partial<Signal> = {}): Signal {
  return {
    id: "sig-1",
    type: "hot_signal",
    subtype: "hot",
    title: "High-intent signal",
    description: "Heat score is high.",
    explanation: "The contact viewed key pages.",
    suggestion: "Send follow-up email",
    context: {
      opens: 3,
      uniqueVisitors: 2,
      durationSeconds: 252,
      keyPageCount: 2,
      keyPageTitles: ["Financial Projections", "Team"],
      contactName: "Sarah Chen",
      contactEmail: "sarah@example.com",
      documentTitle: "Seed Round Deck",
    },
    documentId: "doc-1",
    contactId: "contact-1",
    linkId: "link-1",
    createdAt: "2026-06-20T18:42:00Z",
    priority: "high",
    ...overrides,
  };
}

const action: ActionItem = {
  id: "act-1",
  signalId: "sig-1",
  title: "Send follow-up email",
  impact: "high",
  dueAt: "2026-06-21T18:00:00Z",
  status: "pending",
  actionType: "email",
};

async function renderCard(signal: Signal, act?: ActionItem) {
  const i18n = await createTestI18n({
    common: {
      back: "Back",
      "priority.high": "High",
      "priority.medium": "Medium",
      "priority.low": "Low",
      viewDetails: "View details",
      dueDate: "Due",
      "status.done": "Done",
    },
    dashboard: {
      "signal.expand": "Expand",
      "signal.collapse": "Collapse",
      "signal.aiExplanation": "AI insight",
      "signal.suggestedAction": "Suggested action",
      "signal.markDone": "Mark done",
      "signal.summary.unknownContact": "Unknown visitor",
      "signal.summary.hot_signal": "{{contact}} opened {{opens}} times, spent {{duration}}, viewed {{keyPages}}",
      "signal.summary.follow_up": "{{contact}} · {{opens}} opens · {{duration}} · {{keyPages}}",
      "signal.summary.risk_alert": "{{opens}} opens · {{duration}} · {{keyPageCount}} key pages",
      "signal.summary.question": "{{actor}} asked about {{intent}}: {{question}}",
    },
  });
  return render(
    <MemoryRouter initialEntries={["/acme/dashboard"]}>
      <Routes>
        <Route
          path="/acme/dashboard"
          element={
            <I18nextProvider i18n={i18n}>
              <SignalCard signal={signal} action={act} />
            </I18nextProvider>
          }
        />
      </Routes>
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("SignalCard", () => {
  it("renders the context summary from signal context", async () => {
    await renderCard(makeSignal());
    expect(screen.getByText(/Sarah Chen opened 3 times/)).toBeInTheDocument();
    expect(screen.getByText(/Financial Projections/)).toBeInTheDocument();
  });

  it("does not render context summary when context is absent", async () => {
    await renderCard(makeSignal({ context: undefined }));
    expect(screen.queryByText(/Sarah Chen/)).not.toBeInTheDocument();
    expect(screen.getByText("High-intent signal")).toBeInTheDocument();
  });

  it("renders question summary for question subtype", async () => {
    await renderCard(
      makeSignal({
        type: "follow_up",
        subtype: "question",
        context: {
          opens: 0,
          uniqueVisitors: 0,
          durationSeconds: 0,
          keyPageCount: 0,
          keyPageTitles: [],
          question: "What is your pricing?",
          intent: "pricing",
          actor: "visitor@example.com",
        },
      }),
      action
    );
    expect(screen.getByText(/visitor@example.com asked about pricing/)).toBeInTheDocument();
  });
});
