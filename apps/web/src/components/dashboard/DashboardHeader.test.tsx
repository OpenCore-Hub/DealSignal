// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { DashboardHeader } from "./DashboardHeader";
import { createTestI18n } from "@/i18n/test-utils";

const navigate = vi.fn();

vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return { ...actual, useNavigate: () => navigate };
});

async function renderHeader() {
  const i18n = await createTestI18n({
    dashboard: {
      title: "Dashboard",
      "quickActions.createRoom": "New deal room",
      "quickActions.upload": "Upload document",
      "quickActions.invite": "Invite visitor",
    },
  });
  render(
    <I18nextProvider i18n={i18n}>
      <DashboardHeader workspaceSlug="acme" />
    </I18nextProvider>
  );
}

beforeEach(() => {
  navigate.mockClear();
});

describe("DashboardHeader", () => {
  it("navigates to new deal room", async () => {
    await renderHeader();
    fireEvent.click(screen.getByRole("button", { name: /New deal room/i }));
    expect(navigate).toHaveBeenCalledWith("/acme/deal-rooms/new");
  });

  it("navigates to document upload", async () => {
    await renderHeader();
    fireEvent.click(screen.getByRole("button", { name: /Upload document/i }));
    expect(navigate).toHaveBeenCalledWith("/acme/documents/upload");
  });

  it("navigates to new contact for inviting visitors", async () => {
    await renderHeader();
    fireEvent.click(screen.getByRole("button", { name: /Invite visitor/i }));
    expect(navigate).toHaveBeenCalledWith("/acme/contacts/new");
  });
});
