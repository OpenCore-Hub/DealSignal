// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { HeatBreakdownDialog } from "./HeatBreakdownDialog";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getLinkHeatScoreMock } = vi.hoisted(() => ({
  getLinkHeatScoreMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getLinkHeatScore: getLinkHeatScoreMock,
  },
}));

async function initI18n() {
  const instance = i18n.createInstance();
  const insightsJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/insights.json"), "utf-8"),
  );
  const commonJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"),
  );
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["insights", "common"],
    defaultNS: "insights",
    resources: { en: { insights: insightsJson, common: commonJson } },
    interpolation: { escapeValue: false },
  });
  return instance;
}

describe("HeatBreakdownDialog", () => {
  beforeEach(() => {
    getLinkHeatScoreMock.mockReset();
    getLinkHeatScoreMock.mockResolvedValue({
      linkId: "link-1",
      score: 88,
      level: "hot",
      trend: "rising",
      breakdown: {
        opens: 30,
        revisits: 18,
        avgDurationMinutes: 12,
        keyPageViews: 25,
        forwardSignals: 15,
        downloads: 0,
        bouncePenalty: 0,
      },
      updatedAt: "2026-06-20T00:00:00Z",
    });
  });

  it("loads and renders heat.Compute factors when open", async () => {
    const i18nInstance = await initI18n();
    await act(async () => {
      render(
        <I18nextProvider i18n={i18nInstance}>
          <HeatBreakdownDialog
            open
            onOpenChange={() => {}}
            linkId="link-1"
            linkLabel="Q3 Pitch"
          />
        </I18nextProvider>,
      );
      await new Promise((r) => setTimeout(r, 0));
    });

    await waitFor(() => {
      expect(screen.getByText("Why this heat level")).toBeInTheDocument();
    });
    expect(getLinkHeatScoreMock).toHaveBeenCalledWith("link-1");
    expect(screen.getByText("Key pages")).toBeInTheDocument();
    expect(screen.getByText(/88 pts/)).toBeInTheDocument();
    expect(screen.getByText("Rising")).toBeInTheDocument();
  });
});
