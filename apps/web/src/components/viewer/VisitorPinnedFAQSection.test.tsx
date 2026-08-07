// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { VisitorPinnedFAQSection } from "./VisitorPinnedFAQSection";
import enDocuments from "@/i18n/locales/en/documents.json";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: { en: { documents: enDocuments } },
  interpolation: { escapeValue: false },
});

describe("VisitorPinnedFAQSection", () => {
  it("shows link labels when FAQs come from multiple links", () => {
    render(
      <I18nextProvider i18n={i18nInstance}>
        <VisitorPinnedFAQSection
          faqs={[
            {
              id: "faq1",
              question: "What is ARR?",
              answer: "Annual recurring revenue.",
              source: "ai",
              link_id: "link-a",
              link_name: "Teaser",
              pinned_at: "2026-01-01T00:00:00Z",
            },
            {
              id: "faq2",
              question: "Where is the cap table?",
              answer: "Legal folder.",
              source: "host",
              link_id: "link-b",
              link_name: "Full deck",
              pinned_at: "2026-01-02T00:00:00Z",
            },
          ]}
        />
      </I18nextProvider>,
    );
    expect(screen.getByText(/From link: Teaser/i)).toBeInTheDocument();
    expect(screen.getByText(/From link: Full deck/i)).toBeInTheDocument();
  });
});
