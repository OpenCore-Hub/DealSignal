// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { AttentionZone } from "./AttentionZone";
import { createTestI18n } from "@/i18n/test-utils";
import type { ActionItem, Signal, RiskAlert } from "@/types";

function makeAction(status: ActionItem["status"] = "pending"): ActionItem {
  return {
    id: "act-1",
    signalId: "sig-1",
    title: "Follow up",
    impact: "high",
    dueAt: "2026-06-25T00:00:00Z",
    status,
    actionType: "email",
  };
}

function makeSignal(type: Signal["type"] = "hot_signal"): Signal {
  return {
    id: "sig-1",
    type,
    title: "High intent",
    description: "Investor opened deck 5 times",
    explanation: "Multiple opens in 24 hours",
    suggestion: "Send follow-up email",
    priority: "high",
    createdAt: "2026-06-20T00:00:00Z",
  };
}

function makeRisk(): RiskAlert {
  return {
    id: "risk-1",
    type: "download",
    priority: "high",
    title: "Unidentified download",
    description: "Unknown visitor downloaded",
    createdAt: "2026-06-18T09:00:00Z",
  };
}

async function renderZone(props = {}) {
  const i18n = await createTestI18n({
    dashboard: {
      "sections.attention": "Attention",
      "attention.actions": "Actions",
      "attention.signals": "High-intent signals",
      "attention.risks": "Risks",
      "empty.signals.title": "No signals",
      "empty.signals.description": "No signals yet",
      "empty.actions.title": "No pending actions",
      "empty.actions.description": "All done",
      "riskAlerts.title": "Risk alerts",
      "empty.risks.description": "No risks",
      "actions.completedWithCount": "Completed ({{count}})",
      "actions.moreOptions": "More options",
      "actions.postpone": "Postpone",
      "actions.ignore": "Ignore",
      "actions.hiddenWithCount": "Hidden ({{count}})",
      "actions.reactivate": "Reactivate",
      "signal.expand": "Expand",
      "signal.collapse": "Collapse",
      "signal.aiExplanation": "AI insight",
      "signal.suggestedAction": "Suggested action",
      "signal.markDone": "Mark done",
      "signal.types.hot_signal": "High-intent signal",
      "signal.types.follow_up": "Follow-up suggestion",
      "signal.types.risk_alert": "Risk alert",
    },
    common: {
      complete: "Complete",
      dueDate: "Due",
      "impact.high": "High",
      "impact.medium": "Medium",
      "impact.low": "Low",
      "overdue.days_one": "{{count}} day overdue",
      "overdue.days_other": "{{count}} days overdue",
      "status.snoozed": "Snoozed",
      "status.ignored": "Ignored",
      back: "Back",
    },
  });
  render(
    <MemoryRouter>
      <I18nextProvider i18n={i18n}>
        <AttentionZone
          actions={[makeAction()]}
          signals={[makeSignal()]}
          riskAlerts={[makeRisk()]}
          workspaceSlug="acme"
          onActionStatusChange={vi.fn()}
          {...props}
        />
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

describe("AttentionZone", () => {
  it("renders the attention title and tabs", async () => {
    await renderZone();
    expect(screen.getByRole("tab", { name: /Actions/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /High-intent signals/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Risks/i })).toBeInTheDocument();
  });

  it("displays pending actions by default", async () => {
    await renderZone();
    expect(screen.getByText("Follow up")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Complete/i })).toBeInTheDocument();
  });

  it("switches tabs when clicked", async () => {
    await renderZone();
    const signalsTab = screen.getByRole("tab", { name: /High-intent signals/i });
    fireEvent.click(signalsTab);
    await waitFor(() => {
      expect(signalsTab).toHaveAttribute("aria-selected", "true");
    });
  });

  it("shows empty state for actions", async () => {
    await renderZone({ actions: [] });
    fireEvent.click(screen.getByRole("tab", { name: /Actions/i }));
    await waitFor(() => {
      expect(screen.getByText("No pending actions")).toBeInTheDocument();
    });
  });

  it("calls onActionStatusChange when completing an action", async () => {
    const onChange = vi.fn();
    await renderZone({ onActionStatusChange: onChange });
    fireEvent.click(screen.getByRole("button", { name: /Complete/i }));
    expect(onChange).toHaveBeenCalledWith("act-1", "done");
  });
});
