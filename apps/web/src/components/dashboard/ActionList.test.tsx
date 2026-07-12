// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { ActionList } from "./ActionList";
import { createTestI18n } from "@/i18n/test-utils";
import type { ActionItem } from "@/types";

function makeAction(status: ActionItem["status"] = "pending"): ActionItem {
  return {
    id: "act_1",
    signalId: "sig_1",
    title: "Follow up",
    impact: "high",
    dueAt: "2026-06-25T00:00:00Z",
    status,
    actionType: "email",
  };
}

async function renderList(actions: ActionItem[], onChange = vi.fn()) {
  const i18n = await createTestI18n({
    dashboard: {
      "empty.actions.title": "No pending actions",
      "empty.actions.description": "All done",
      "actions.completedWithCount": "Completed ({{count}})",
      "actions.hiddenWithCount": "Hidden ({{count}})",
      "actions.moreOptions": "More options",
      "actions.postpone": "Postpone",
      "actions.ignore": "Ignore",
      "actions.reactivate": "Reactivate",
    },
    common: {
      complete: "Complete",
      "dueDate": "Due",
      "impact.high": "High impact",
      "impact.medium": "Medium impact",
      "impact.low": "Low impact",
      "overdue.days": "{{count}} days overdue",
      "status.snoozed": "Snoozed",
      "status.ignored": "Ignored",
    },
  });
  render(
    <I18nextProvider i18n={i18n}>
      <ActionList actions={actions} onStatusChange={onChange} />
    </I18nextProvider>
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

describe("ActionList", () => {
  it("calls onStatusChange with done when complete is clicked", async () => {
    const onChange = vi.fn();
    await renderList([makeAction()], onChange);
    fireEvent.click(screen.getByRole("button", { name: /Complete/i }));
    expect(onChange).toHaveBeenCalledWith("act_1", "done");
  });

  it("supports postponing and ignoring via the dropdown", async () => {
    const onChange = vi.fn();
    await renderList([makeAction()], onChange);
    fireEvent.click(screen.getByRole("button", { name: /More options/i }));
    await waitFor(() => {
      expect(screen.getByRole("menuitem", { name: /Postpone/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("menuitem", { name: /Postpone/i }));
    expect(onChange).toHaveBeenCalledWith("act_1", "snoozed");

    fireEvent.click(screen.getByRole("button", { name: /More options/i }));
    await waitFor(() => {
      expect(screen.getByRole("menuitem", { name: /Ignore/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("menuitem", { name: /Ignore/i }));
    expect(onChange).toHaveBeenCalledWith("act_1", "ignored");
  });

  it("shows hidden actions and allows reactivation", async () => {
    const onChange = vi.fn();
    await renderList([makeAction("snoozed")], onChange);
    fireEvent.click(screen.getByRole("button", { name: /Hidden \(1\)/i }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Reactivate/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /Reactivate/i }));
    expect(onChange).toHaveBeenCalledWith("act_1", "pending");
  });
});
