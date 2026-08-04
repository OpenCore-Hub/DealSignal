// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { SessionStateRail } from "./SessionStateRail";

const dealRooms = {
  "knowledge.sessionStateTitle": "Desk state",
  "knowledge.sessionStateHint": "Audited gaps and entities from this session.",
  "knowledge.sessionStateOpenQuestions": "{{count}} open gaps",
  "knowledge.sessionStateNoGaps": "No open gaps in this session",
  "knowledge.sessionStateAskGap": "Ask this",
  "knowledge.sessionStateEntities": "Entities",
  "knowledge.sessionStateCoverage": "Recent coverage",
  "knowledge.sessionStateExpand": "Expand",
  "knowledge.sessionStateCollapse": "Collapse",
  "knowledge.pageListSep": ", ",
};

describe("SessionStateRail", () => {
  it("renders nothing when state is empty", async () => {
    const i18n = await createTestI18n({ dealRooms });
    const { container } = render(
      <I18nextProvider i18n={i18n}>
        <SessionStateRail state={{}} />
      </I18nextProvider>,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("shows open questions expanded and asks on click", async () => {
    const onAsk = vi.fn();
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <SessionStateRail
          state={{
            openQuestions: [{ text: "What is the cap?", sourceTurnId: "t1" }],
            entities: [{ name: "SAFE.pdf", type: "document", firstTurnId: "t1" }],
            coverageHints: [
              { sourceNames: ["SAFE.pdf", "SPA.pdf"], turnId: "t1" },
            ],
          }}
          onAskOpenQuestion={onAsk}
        />
      </I18nextProvider>,
    );
    expect(screen.getByTestId("knowledge-session-state-rail")).toHaveAttribute(
      "data-expanded",
      "true",
    );
    expect(screen.getByTestId("knowledge-session-state-details")).toBeInTheDocument();
    expect(screen.getByText("What is the cap?")).toBeInTheDocument();
    expect(screen.getByText("SAFE.pdf")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ask this" }));
    expect(onAsk).toHaveBeenCalledWith("What is the cap?");
  });

  it("collapses and expands details via the toggle", async () => {
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <SessionStateRail
          state={{
            openQuestions: [{ text: "What is the valuation cap?", sourceTurnId: "t1" }],
          }}
        />
      </I18nextProvider>,
    );
    fireEvent.click(screen.getByTestId("knowledge-session-state-toggle"));
    expect(screen.getByTestId("knowledge-session-state-rail")).toHaveAttribute(
      "data-expanded",
      "false",
    );
    expect(screen.queryByTestId("knowledge-session-state-details")).not.toBeInTheDocument();
    expect(screen.getByText("1 open gaps")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("knowledge-session-state-toggle"));
    expect(screen.getByTestId("knowledge-session-state-details")).toBeInTheDocument();
  });

  it("hides meta and out-of-room open questions", async () => {
    const i18n = await createTestI18n({ dealRooms });
    render(
      <I18nextProvider i18n={i18n}>
        <SessionStateRail
          state={{
            openQuestions: [
              { text: "因此，无法根据现有上下文回答该问题。", sourceTurnId: "t1" },
              {
                text: "The EBITDA multiple for this sector is typically 12x.",
                sourceTurnId: "t1",
              },
            ],
            entities: [{ name: "SAFE.pdf", type: "document", firstTurnId: "t1" }],
          }}
        />
      </I18nextProvider>,
    );
    expect(screen.getByTestId("knowledge-session-state-rail")).toBeInTheDocument();
    expect(screen.queryByTestId("knowledge-session-open-questions")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("knowledge-session-state-toggle"));
    expect(screen.getByText("SAFE.pdf")).toBeInTheDocument();
    expect(
      screen.queryByText("因此，无法根据现有上下文回答该问题。"),
    ).not.toBeInTheDocument();
  });

  it("lists every open gap so the count matches visible rows", async () => {
    const i18n = await createTestI18n({ dealRooms });
    const openQuestions = Array.from({ length: 5 }, (_, i) => ({
      text: `Gap question ${i + 1}`,
      sourceTurnId: `t${i + 1}`,
    }));
    render(
      <I18nextProvider i18n={i18n}>
        <SessionStateRail state={{ openQuestions }} />
      </I18nextProvider>,
    );
    expect(screen.getByText("5 open gaps")).toBeInTheDocument();
    expect(screen.getAllByTestId("knowledge-session-open-question")).toHaveLength(5);
    expect(screen.getByText("Gap question 5")).toBeInTheDocument();
  });

  it("keeps all open gaps in the DOM with a scrollable viewport past 5", async () => {
    const i18n = await createTestI18n({ dealRooms });
    const openQuestions = Array.from({ length: 7 }, (_, i) => ({
      text: `Gap question ${i + 1}`,
      sourceTurnId: `t${i + 1}`,
    }));
    render(
      <I18nextProvider i18n={i18n}>
        <SessionStateRail state={{ openQuestions }} />
      </I18nextProvider>,
    );
    expect(screen.getByText("7 open gaps")).toBeInTheDocument();
    expect(screen.getAllByTestId("knowledge-session-open-question")).toHaveLength(7);
    expect(screen.getByTestId("knowledge-session-open-questions-list").className).toMatch(
      /overflow-y-auto/,
    );
  });
});
