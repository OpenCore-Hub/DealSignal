// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { act, render, screen } from "@testing-library/react";
import i18next from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { WatermarkOverlay } from "./WatermarkOverlay";

async function renderOverlay(
  watermark: Parameters<typeof WatermarkOverlay>[0]["watermark"],
) {
  const instance = i18next.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["documents"],
    defaultNS: "documents",
    resources: {
      en: {
        documents: {
          viewer: {
            watermarkFallback: "CONFIDENTIAL",
            watermarkPreviewIp: "preview",
          },
        },
      },
    },
  });

  return render(
    <I18nextProvider i18n={instance}>
      <div style={{ width: 400, height: 300, position: "relative" }}>
        <WatermarkOverlay watermark={watermark} />
      </div>
    </I18nextProvider>,
  );
}

describe("WatermarkOverlay", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-08-08T12:00:00.000Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders nothing when watermark is omitted", async () => {
    await renderOverlay(undefined);
    expect(screen.queryByTestId("viewer-watermark-overlay")).not.toBeInTheDocument();
  });

  it("keeps server UTC stamp frozen (no per-second tick)", async () => {
    await renderOverlay({
      watermarkText: "visitor@example.com | 2026-08-08 01:00:00 UTC | IP:abcd1234",
    });
    const el = screen.getByTestId("viewer-watermark-overlay");
    expect(el).toHaveAttribute(
      "data-watermark-text",
      "visitor@example.com | 2026-08-08 01:00:00 UTC | IP:abcd1234",
    );

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(el).toHaveAttribute(
      "data-watermark-text",
      "visitor@example.com | 2026-08-08 01:00:00 UTC | IP:abcd1234",
    );
  });

  it("uses login account email for owner viewer preview", async () => {
    await renderOverlay({ email: "owner@example.com" });
    expect(screen.getByTestId("viewer-watermark-overlay")).toHaveAttribute(
      "data-watermark-text",
      "owner@example.com | 2026-08-08 12:00:00 UTC | IP:preview",
    );
  });

  it("uses static fallback when login email is unavailable (no forged identity)", async () => {
    await renderOverlay({});
    expect(screen.getByTestId("viewer-watermark-overlay")).toHaveAttribute(
      "data-watermark-text",
      "CONFIDENTIAL",
    );
  });
});
