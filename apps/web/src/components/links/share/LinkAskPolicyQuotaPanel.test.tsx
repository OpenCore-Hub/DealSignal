// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { LinkAskPolicyQuotaPanel } from "./LinkAskPolicyQuotaPanel";
import { api } from "@/lib/api";
import enLinkShare from "@/i18n/locales/en/linkShare.json";

vi.mock("@/lib/api", () => ({
  api: {
    getLinkAskPolicy: vi.fn(),
  },
}));

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: { en: { linkShare: enLinkShare } },
  interpolation: { escapeValue: false },
});

function renderPanel(
  experience: "host_only" | "ai_supervised" | "ai_self_serve" | "formal" = "ai_supervised",
) {
  return render(
    <I18nextProvider i18n={i18nInstance}>
      <LinkAskPolicyQuotaPanel linkId="link-1" experience={experience} />
    </I18nextProvider>,
  );
}

describe("LinkAskPolicyQuotaPanel", () => {
  beforeEach(() => {
    vi.mocked(api.getLinkAskPolicy).mockReset();
  });

  it("does not fetch or render when experience is host-only", () => {
    const { container } = renderPanel("host_only");
    expect(api.getLinkAskPolicy).not.toHaveBeenCalled();
    expect(container).toBeEmptyDOMElement();
  });

  it("shows quota usage when AI lane is enabled", async () => {
    vi.mocked(api.getLinkAskPolicy).mockResolvedValue({
      data: {
        id: "link-1",
        askMode: "supervised",
        askAiEnabled: true,
        askAiMonthlyQuota: 100,
        askAiMonthlyUsed: 12,
        askAiMonthlyLimit: 100,
        askAiQuotaExceeded: false,
        askAiEntitled: true,
      },
    });

    renderPanel("ai_supervised");

    await waitFor(() => {
      expect(screen.getByTestId("link-ask-policy-quota")).toBeInTheDocument();
    });
    expect(screen.getByText("12 / 100 AI answers this month")).toBeInTheDocument();
    expect(
      screen.queryByText(/Monthly AI quota reached/i),
    ).not.toBeInTheDocument();
  });

  it("shows paywall copy when quota is exceeded", async () => {
    vi.mocked(api.getLinkAskPolicy).mockResolvedValue({
      data: {
        id: "link-1",
        askMode: "supervised",
        askAiEnabled: true,
        askAiMonthlyQuota: 50,
        askAiMonthlyUsed: 50,
        askAiMonthlyLimit: 50,
        askAiQuotaExceeded: true,
        askAiEntitled: true,
      },
    });

    renderPanel("ai_self_serve");

    await waitFor(() => {
      expect(screen.getByText(/Monthly AI quota reached/i)).toBeInTheDocument();
    });
  });

  it("shows not-entitled message when corpus is missing", async () => {
    vi.mocked(api.getLinkAskPolicy).mockResolvedValue({
      data: {
        id: "link-1",
        askMode: "supervised",
        askAiEnabled: true,
        askAiMonthlyQuota: null,
        askAiMonthlyUsed: 0,
        askAiMonthlyLimit: 0,
        askAiQuotaExceeded: false,
        askAiEntitled: false,
      },
    });

    renderPanel("ai_supervised");

    await waitFor(() => {
      expect(
        screen.getByText(/requires a synced deal-room knowledge corpus/i),
      ).toBeInTheDocument();
    });
  });
});
