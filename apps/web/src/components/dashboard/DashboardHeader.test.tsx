// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { DashboardHeader } from "./DashboardHeader";
import { createTestI18n } from "@/i18n/test-utils";
import { useRadarStore } from "@/stores/radarStore";

vi.mock("@/stores/uiStore", () => ({
  useUIStore: (selector: (s: { currentWorkspace: { name: string } | null }) => unknown) =>
    selector({ currentWorkspace: { name: "Acme Capital" } }),
}));

async function renderHeader() {
  const i18n = await createTestI18n({
    dashboard: {
      "radar.title": "Deal Radar",
      "radar.openCount_one": "{{count}} open",
      "radar.openCount_other": "{{count}} open",
    },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <DashboardHeader workspaceSlug="acme" />
    </I18nextProvider>,
  );
}

describe("DashboardHeader", () => {
  beforeEach(() => {
    useRadarStore.setState({ openCount: 3 });
  });

  it("shows Deal Radar identity and open count from radar store", async () => {
    await renderHeader();
    expect(screen.getByTestId("dashboard-welcome-header")).toBeInTheDocument();
    expect(screen.getByText("Deal Radar")).toBeInTheDocument();
    expect(screen.getByText(/3 open/)).toBeInTheDocument();
    expect(screen.getByText("Acme Capital")).toBeInTheDocument();
  });
});
