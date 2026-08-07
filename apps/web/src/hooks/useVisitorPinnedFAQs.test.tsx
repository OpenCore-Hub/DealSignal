// @vitest-environment jsdom
import type { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import {
  faqListFingerprint,
  useVisitorPinnedFAQs,
  VISITOR_ASK_FAQ_POLL_MS,
} from "./useVisitorPinnedFAQs";
import enDocuments from "@/i18n/locales/en/documents.json";
import type { PublicAskFAQ } from "@/types";

const listPublicAskFAQsMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", () => ({
  api: {
    listPublicAskFAQs: listPublicAskFAQsMock,
  },
}));

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: { en: { documents: enDocuments } },
  interpolation: { escapeValue: false },
});

function wrapper({ children }: { children: ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

const sampleFaq: PublicAskFAQ = {
  id: "faq1",
  question: "What is ARR?",
  answer: "Annual recurring revenue.",
  source: "ai",
  pinned_at: "2026-01-01T00:00:00Z",
};

function mockDocumentHidden(hidden: boolean) {
  Object.defineProperty(document, "hidden", {
    configurable: true,
    get: () => hidden,
  });
}

describe("faqListFingerprint", () => {
  it("changes when FAQ content or order changes", () => {
    const base = faqListFingerprint([sampleFaq]);
    expect(faqListFingerprint([{ ...sampleFaq, pinned_faq_sort: 1 }])).not.toBe(base);
    expect(faqListFingerprint([{ ...sampleFaq, answer: "Updated." }])).not.toBe(base);
  });
});

describe("useVisitorPinnedFAQs", () => {
  beforeEach(() => {
    listPublicAskFAQsMock.mockReset();
    listPublicAskFAQsMock.mockResolvedValue({ data: [] });
    mockDocumentHidden(false);
  });

  it("loads FAQs on mount", async () => {
    listPublicAskFAQsMock.mockResolvedValue({ data: [sampleFaq] });
    const { result } = renderHook(
      () => useVisitorPinnedFAQs({ token: "tok", sessionToken: "sess" }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(listPublicAskFAQsMock).toHaveBeenCalledWith("tok", { sessionToken: "sess" });
    expect(result.current.faqs).toEqual([sampleFaq]);
  });

  describe("polling", () => {
    it("polls in the background without toggling loading", async () => {
      listPublicAskFAQsMock.mockResolvedValueOnce({ data: [sampleFaq] });
      const { result } = renderHook(
        () =>
          useVisitorPinnedFAQs({
            token: "tok",
            pollIntervalMs: 50,
          }),
        { wrapper },
      );

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      listPublicAskFAQsMock.mockResolvedValueOnce({
        data: [{ ...sampleFaq, answer: "Updated answer." }],
      });

      await waitFor(
        () => {
          expect(result.current.faqs[0]?.answer).toBe("Updated answer.");
        },
        { timeout: 2_000 },
      );
      expect(result.current.loading).toBe(false);
      expect(listPublicAskFAQsMock.mock.calls.length).toBeGreaterThanOrEqual(2);
    });

    it("refetches when the tab becomes visible again", async () => {
      listPublicAskFAQsMock.mockResolvedValue({ data: [sampleFaq] });
      renderHook(
        () =>
          useVisitorPinnedFAQs({
            token: "tok",
            pollIntervalMs: VISITOR_ASK_FAQ_POLL_MS,
          }),
        { wrapper },
      );

      await waitFor(() => {
        expect(listPublicAskFAQsMock).toHaveBeenCalledTimes(1);
      });

      mockDocumentHidden(true);
      await act(async () => {
        document.dispatchEvent(new Event("visibilitychange"));
      });
      expect(listPublicAskFAQsMock).toHaveBeenCalledTimes(1);

      mockDocumentHidden(false);
      await act(async () => {
        document.dispatchEvent(new Event("visibilitychange"));
      });

      await waitFor(() => {
        expect(listPublicAskFAQsMock.mock.calls.length).toBeGreaterThanOrEqual(2);
      });
    });

    it("skips polling while the tab is hidden", async () => {
      mockDocumentHidden(true);
      renderHook(
        () =>
          useVisitorPinnedFAQs({
            token: "tok",
            pollIntervalMs: 50,
          }),
        { wrapper },
      );

      await waitFor(() => {
        expect(listPublicAskFAQsMock).toHaveBeenCalledTimes(1);
      });

      await new Promise((resolve) => setTimeout(resolve, 150));

      expect(listPublicAskFAQsMock).toHaveBeenCalledTimes(1);
    });
  });
});
