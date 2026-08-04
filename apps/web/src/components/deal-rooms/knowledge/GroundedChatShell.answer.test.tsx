/** @vitest-environment jsdom */
import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { describe, expect, it, vi } from "vitest";
import { createTestI18n } from "@/i18n/test-utils";
import { GroundedChatShell } from "./GroundedChatShell";
import type { KnowledgeTurn } from "@/lib/knowledge/streamEvents";

const dealRooms = {
  "knowledge.answerLabel": "Answer",
  "knowledge.answerHint": "Grounded",
  "knowledge.turnQuestion": "Q",
  "knowledge.noHits": "No hits",
  "knowledge.sourcesTitle": "Sources",
  "knowledge.sourcesCount": "{{count}}",
  "knowledge.followUpLabel": "Follow-ups",
  "knowledge.ask": "Ask",
  "knowledge.askPlaceholder": "Ask…",
  "knowledge.phaseRetrieving": "Retrieving",
  "knowledge.phaseGenerating": "Generating",
  "knowledge.sheetLabel": "Sheet",
  "knowledge.pageSingle": "p.{{page}}",
  "knowledge.pageRange": "p.{{from}}–{{to}}",
  "knowledge.pageListSep": ", ",
  "knowledge.pageList": "{{pages}}",
  "knowledge.openPage": "Open {{page}}",
  "knowledge.openDocument": "Open doc",
  "knowledge.sheetMapMissing": "No sheet map",
  "knowledge.noPageLocus": "No page",
  "knowledge.followUp.narrowScope": "Narrow?",
  "knowledge.followUp.nameClause": "Name clause?",
  "knowledge.followUp.specificClause": "Clause?",
  "knowledge.followUp.partyObligations": "Obligations?",
  "knowledge.followUp.liabilityInSource": "Liability in {{sourceName}}?",
  "knowledge.followUp.definitionsInSource": "Defs in {{sourceName}}?",
  "knowledge.followUp.exceptionsInSource": "Exceptions in {{sourceName}}?",
  "knowledge.followUp.exceptionsInSecondSource": "Exceptions in {{sourceName}}?",
  "knowledge.followUp.crossFileConsistency": "{{sourceA}} vs {{sourceB}}?",
};

describe("GroundedChatShell answer B path", () => {
  it("renders short done answers via AnswerMarkdown", async () => {
    const i18n = await createTestI18n({ dealRooms });
    const turn: KnowledgeTurn = {
      id: "t1",
      query: "first",
      phase: "done",
      answer: "ok",
      results: [],
      refused: false,
      resultStatus: "answered",
      activeCite: null,
    };
    render(
      <I18nextProvider i18n={i18n}>
        <GroundedChatShell
          query=""
          onQueryChange={vi.fn()}
          turns={[turn]}
          asking={false}
          onAsk={vi.fn()}
          onActiveCite={vi.fn()}
          onOpenViewer={vi.fn()}
        />
      </I18nextProvider>,
    );
    expect(screen.getByTestId("knowledge-answer-markdown")).toHaveTextContent("ok");
  });
});
