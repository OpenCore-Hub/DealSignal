// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { DashboardHeader } from "./DashboardHeader";
import { createTestI18n } from "@/i18n/test-utils";

async function renderHeader() {
  const i18n = await createTestI18n({
    dashboard: {
      "welcome.title": "Welcome back",
    },
  });
  render(
    <I18nextProvider i18n={i18n}>
      <DashboardHeader workspaceSlug="acme" />
    </I18nextProvider>
  );
}

describe("DashboardHeader", () => {
  it("renders workspace name and title", async () => {
    await renderHeader();
    expect(screen.getByText("Welcome back")).toBeInTheDocument();
    expect(screen.getByText("Acme")).toBeInTheDocument();
  });
});
