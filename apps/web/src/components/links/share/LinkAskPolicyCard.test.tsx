// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { LinkAskPolicyCard } from "./LinkAskPolicyCard";
import { createTestI18n } from "@/i18n/test-utils";
import enLinkShare from "@/i18n/locales/en/linkShare.json";
import { api } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: {
    updateLinkAskPolicy: vi.fn(),
    getLinkAskPolicy: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

async function renderCard(
  initialAskAiEnabled = false,
  onAskAiEnabledChange?: (enabled: boolean) => void,
) {
  const i18n = await createTestI18n({
    linkShare: enLinkShare as unknown as Record<string, string>,
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <LinkAskPolicyCard
        linkId="link_room_1"
        initialAskAiEnabled={initialAskAiEnabled}
        onAskAiEnabledChange={onAskAiEnabledChange}
      />
    </I18nextProvider>,
  );
}

describe("LinkAskPolicyCard", () => {
  beforeEach(() => {
    vi.mocked(api.updateLinkAskPolicy).mockReset();
    vi.mocked(api.getLinkAskPolicy).mockReset();
    vi.mocked(api.getLinkAskPolicy).mockResolvedValue({
      data: {
        id: "link_room_1",
        askMode: "supervised",
        askAiEnabled: false,
        askAiMonthlyQuota: null,
        askAiMonthlyUsed: 12,
        askAiMonthlyLimit: 500,
        askAiQuotaExceeded: false,
      },
    });
  });

  it("renders grounded AI toggle for deal-room links", async () => {
    await renderCard(false);
    expect(screen.getByText("Grounded AI answers")).toBeInTheDocument();
    expect(screen.getByTestId("link-ask-ai-enabled")).toBeInTheDocument();
  });

  it("renders ask routing mode selector", async () => {
    await renderCard(false);
    await waitFor(() => {
      expect(screen.getByTestId("link-ask-mode")).toBeInTheDocument();
    });
    expect(screen.getByText("Ask routing mode")).toBeInTheDocument();
  });

  it("shows self-serve mode option in routing selector", async () => {
    await renderCard(false);
    await waitFor(() => {
      expect(screen.getByTestId("link-ask-mode")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("link-ask-mode"));
    expect(await screen.findByRole("option", { name: "Self-serve" })).toBeInTheDocument();
  });

  it("shows formal mode option in routing selector", async () => {
    await renderCard(false);
    await waitFor(() => {
      expect(screen.getByTestId("link-ask-mode")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("link-ask-mode"));
    expect(await screen.findByRole("option", { name: "Formal" })).toBeInTheDocument();
  });

  it("enables grounded AI via ask-policy API", async () => {
    vi.mocked(api.updateLinkAskPolicy).mockResolvedValue({
      data: {
        id: "link_room_1",
        askMode: "supervised",
        askAiEnabled: true,
        askAiMonthlyQuota: null,
        askAiMonthlyUsed: 12,
        askAiMonthlyLimit: 500,
        askAiQuotaExceeded: false,
      },
    });
    await renderCard(false);
    await waitFor(() => {
      expect(screen.getByRole("switch")).not.toBeDisabled();
    });
    const toggle = screen.getByRole("switch");
    fireEvent.click(toggle);
    await waitFor(() => {
      expect(api.updateLinkAskPolicy).toHaveBeenCalledWith("link_room_1", {
        askAiEnabled: true,
      });
    });
  });

  it("notifies parent when grounded AI policy changes", async () => {
    const onAskAiEnabledChange = vi.fn();
    vi.mocked(api.updateLinkAskPolicy).mockResolvedValue({
      data: {
        id: "link_room_1",
        askMode: "supervised",
        askAiEnabled: true,
        askAiMonthlyQuota: null,
        askAiMonthlyUsed: 12,
        askAiMonthlyLimit: 500,
        askAiQuotaExceeded: false,
      },
    });
    await renderCard(false, onAskAiEnabledChange);
    await waitFor(() => {
      expect(screen.getByRole("switch")).not.toBeDisabled();
    });
    fireEvent.click(screen.getByRole("switch"));
    await waitFor(() => {
      expect(onAskAiEnabledChange).toHaveBeenCalledWith(true);
    });
  });
});
