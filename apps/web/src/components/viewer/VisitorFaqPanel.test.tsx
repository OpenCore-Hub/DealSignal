// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { VisitorFaqPanel } from "./VisitorFaqPanel";
import enDocuments from "@/i18n/locales/en/documents.json";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: { en: { documents: enDocuments } },
  interpolation: { escapeValue: false },
});

describe("VisitorFaqPanel", () => {
  it("filters FAQs and offers Ask when nothing matches", () => {
    const onAskQuestion = vi.fn();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <VisitorFaqPanel
          faqs={[
            {
              id: "faq1",
              question: "What is ARR?",
              answer: "Twelve million.",
              source: "host",
              pinned_at: "2026-01-01T00:00:00Z",
            },
          ]}
          onAskQuestion={onAskQuestion}
        />
      </I18nextProvider>,
    );

    fireEvent.change(screen.getByPlaceholderText(/Search common questions/i), {
      target: { value: "cap table" },
    });
    expect(screen.getByText(/No matching answers/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Ask this question/i }));
    expect(onAskQuestion).toHaveBeenCalledWith("cap table");
  });

  it("Ask this uses the submit callback, empty search uses prefill only", () => {
    const onAskQuestion = vi.fn();
    const onAskThis = vi.fn();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <VisitorFaqPanel
          faqs={[
            {
              id: "faq1",
              question: "What is ARR?",
              answer: "Twelve million.",
              source: "host",
              pinned_at: "2026-01-01T00:00:00Z",
            },
          ]}
          onAskQuestion={onAskQuestion}
          onAskThis={onAskThis}
        />
      </I18nextProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: /^Ask this$/i }));
    expect(onAskThis).toHaveBeenCalledWith("What is ARR?");
    expect(onAskQuestion).not.toHaveBeenCalled();
  });
});
