// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
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

  it("renders unified Q&A strategy card radios", async () => {
    await renderCard(false);
    expect(screen.getByText("Grounded AI answers")).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByTestId("link-ask-experience")).toBeInTheDocument();
    });
    expect(screen.getByText("Q&A strategy")).toBeInTheDocument();
    expect(screen.getByTestId("link-ask-experience-formal")).toBeInTheDocument();
    expect(screen.getByTestId("link-ask-experience-ai_supervised")).toBeInTheDocument();
  });
});
