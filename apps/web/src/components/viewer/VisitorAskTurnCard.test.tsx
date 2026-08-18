// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { VisitorAskTurnCard } from "./VisitorAskTurnCard";
import { formatDate } from "@/lib/formatters";
import enDocuments from "@/i18n/locales/en/documents.json";
import type { PublicAskTurn } from "@/types";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: { en: { documents: enDocuments } },
  interpolation: { escapeValue: false },
});

function scheduledTurn(publishAt?: string): PublicAskTurn {
  return {
    id: "turn_scheduled",
    session_id: "sess1",
    question: "GMV增长率",
    lane: "host",
    status: "host_pending",
    route_reason: "policy_formal",
    formal_status: "scheduled",
    formal_publish_at: publishAt,
    created_at: "2026-08-18T10:00:00.000Z",
    updated_at: "2026-08-18T10:05:00.000Z",
  };
}

describe("VisitorAskTurnCard scheduled Formal copy", () => {
  it("includes the local publish time in the scheduled notice", () => {
    const publishAt = "2026-08-18T14:40:00.000Z";
    render(
      <I18nextProvider i18n={i18nInstance}>
        <VisitorAskTurnCard turn={scheduledTurn(publishAt)} onRefresh={vi.fn()} />
      </I18nextProvider>,
    );
    const time = formatDate(publishAt, "en");
    expect(screen.getByTestId("visitor-ask-refresh")).toHaveTextContent(
      `The answer will be published at ${time}.`,
    );
  });

  it("falls back to the scheduled copy when no time is present", () => {
    render(
      <I18nextProvider i18n={i18nInstance}>
        <VisitorAskTurnCard turn={scheduledTurn()} onRefresh={vi.fn()} />
      </I18nextProvider>,
    );
    expect(screen.getByTestId("visitor-ask-refresh")).toHaveTextContent(
      /The answer is scheduled and will appear here soon/i,
    );
  });
});
