// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { MemoryRouter, useLocation } from "react-router";
import { api } from "@/lib/api";
import { useKnowledgeQueryStore } from "@/stores/knowledgeQueryStore";
import {
  DealRoomKnowledgeTab,
  formatHitLocusLabel,
} from "./DealRoomKnowledgeTab";

function LocationDisplay() {
  const location = useLocation();
  return (
    <div data-testid="location-display">
      {location.pathname}
      {location.search}
    </div>
  );
}

const i18nInstance = i18n.createInstance();
void i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      dealRooms: {
        knowledge: {
          title: "Knowledge base",
          disabledTitle: "Knowledge base is not enabled",
          disabledDescription: "Configure DOCLING_RAG_BASE_URL.",
          loadFailed: "Failed to load knowledge base",
          sync: "Sync",
          syncing: "Syncing…",
          syncQueued: "Knowledge sync queued",
          syncFailed: "Failed to queue knowledge sync",
          unavailable: "Knowledge base is unavailable",
          lastSynced: "Last synced {{time}}",
          emptyDocuments: "No documents in the knowledge corpus yet.",
          chunkCount: "{{count}} chunks",
          queryTitle: "Ask the knowledge base",
          queryLabel: "Question",
          queryPlaceholder: "Ask…",
          ask: "Ask",
          querying: "Asking…",
          queryFailed: "Failed to query knowledge base",
          noHits: "No matching passages found.",
          openPage: "Open page {{page}}",
          openDocument: "Open document",
          sheetLabel: "Sheet",
          sheetMapMissing: "Page map not ready — open document home",
          noPageLocus: "No page locus for this format — open document",
          status: { ready: "Ready", none: "Not provisioned" },
          docStatus: { synced: "Synced", pending: "Pending" },
        },
      },
      common: { loading: "Loading...", retry: "Retry" },
    },
  },
  interpolation: { escapeValue: false },
});

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomKnowledge: vi.fn(),
    syncDealRoomKnowledge: vi.fn(),
    queryDealRoomKnowledge: vi.fn(),
  },
}));

describe("DealRoomKnowledgeTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useKnowledgeQueryStore.getState().clear();
  });

  it("shows disabled state when RAG is not configured", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: false,
      status: "none",
      documents: [],
    });
    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" />
        </I18nextProvider>
      </MemoryRouter>,
    );
    expect(
      await screen.findByText("Knowledge base is not enabled"),
    ).toBeInTheDocument();
  });

  it("renders corpus docs and runs a query", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [
        {
          documentId: "doc-1",
          title: "Memo.pdf",
          status: "synced",
          chunkCount: 4,
        },
      ],
    });
    vi.mocked(api.queryDealRoomKnowledge).mockResolvedValue({
      query: "valuation",
      mode: "hybrid",
      answer: "The cap is $10M [1]",
      results: [
        {
          chunkId: "c1",
          documentId: "doc-1",
          text: "valuation cap $10M",
          score: 0.9,
          sourceName: "Memo.pdf",
          pages: [3, 4],
          viewerPage: 3,
        },
      ],
    });

    render(
      <MemoryRouter initialEntries={["/deal-rooms/room-1"]}>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" />
          <LocationDisplay />
        </I18nextProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByText("Memo.pdf")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Question"), {
      target: { value: "valuation" },
    });
    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask"));
    await waitFor(() => {
      expect(api.queryDealRoomKnowledge).toHaveBeenCalledWith("room-1", {
        query: "valuation",
        answer: true,
        top_k: 8,
      });
    });
    expect(await screen.findByText(/The cap is \$10M/)).toBeInTheDocument();
    const hit = screen.getByTestId("deal-room-knowledge-hit");
    expect(hit).toHaveTextContent("valuation cap $10M");
    expect(hit).toHaveTextContent("Memo.pdf · 第3–4页");
    fireEvent.click(screen.getByTestId("knowledge-cite-1"));
    expect(hit.className).toMatch(/border-primary/);
    expect(screen.getByTestId("location-display")).toHaveTextContent(
      "/deal-rooms/room-1",
    );
    fireEvent.click(screen.getByTestId("deal-room-knowledge-jump"));
    expect(screen.getByTestId("location-display")).toHaveTextContent(
      "/viewer/doc-1?page=3",
    );
  });

  it("hides sources when the grounded answer refuses a match", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [],
    });
    vi.mocked(api.queryDealRoomKnowledge).mockResolvedValue({
      query: "是",
      mode: "hybrid",
      answer:
        "The provided context does not contain an answer to the question '是'.",
      results: [
        { chunkId: "c1", text: "NDA header", score: 0.03 },
        { chunkId: "c2", text: "NDA body", score: 0.01 },
      ],
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" />
        </I18nextProvider>
      </MemoryRouter>,
    );

    fireEvent.change(await screen.findByLabelText("Question"), {
      target: { value: "是" },
    });
    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask"));
    expect(
      await screen.findByText(/does not contain an answer/i),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-knowledge-hit")).not.toBeInTheDocument();
    expect(screen.queryByText("NDA header")).not.toBeInTheDocument();
  });
});

describe("isUngroundedKnowledgeAnswer", () => {
  it("detects docling-rag refusal answers", async () => {
    const { isUngroundedKnowledgeAnswer } = await import("./DealRoomKnowledgeTab");
    expect(
      isUngroundedKnowledgeAnswer(
        "The context provided does not contain an answer to the question.",
      ),
    ).toBe(true);
    expect(isUngroundedKnowledgeAnswer("The cap is $10M [1]")).toBe(false);
  });
});

describe("formatHitLocusLabel", () => {
  it("formats pages and sheet without inventing missing pages", async () => {
    const { formatPagesLabel } = await import("./DealRoomKnowledgeTab");
    expect(
      formatHitLocusLabel({
        chunkId: "c",
        text: "t",
        score: 1,
        sourceName: "a.xlsx",
        sheet: "损益表",
      }),
    ).toBe("a.xlsx · Sheet 损益表");
    expect(
      formatHitLocusLabel(
        {
          chunkId: "c",
          text: "t",
          score: 1,
          sourceName: "a.xlsx",
          sheet: "损益表",
        },
        { sheetPrefix: "工作表" },
      ),
    ).toBe("a.xlsx · 工作表 损益表");
    expect(
      formatHitLocusLabel({
        chunkId: "c",
        text: "t",
        score: 1,
        pages: [4, 3],
      }),
    ).toBe("第3–4页");
    expect(formatPagesLabel([1, 3])).toBe("第1、3页");
  });
});

describe("docx citation open without page", () => {
  it("shows room title locus and opens document home when viewerPage is absent", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [],
    });
    vi.mocked(api.queryDealRoomKnowledge).mockResolvedValue({
      query: "保密条款",
      mode: "hybrid",
      answer: "保密义务如下 [1]",
      results: [
        {
          chunkId: "c1",
          documentId: "18b1062d-919b-437a-8d5c-76efc60dec86",
          text: "**单向保密协议 (NDA)**\n1. 目的",
          score: 0.867,
          sourceName: "单向保密协议 (NDA).docx",
        },
      ],
    });

    render(
      <MemoryRouter initialEntries={["/deal-rooms/room-1"]}>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" />
          <LocationDisplay />
        </I18nextProvider>
      </MemoryRouter>,
    );

    fireEvent.change(await screen.findByLabelText("Question"), {
      target: { value: "保密条款" },
    });
    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask"));
    const hit = await screen.findByTestId("deal-room-knowledge-hit");
    expect(screen.getByTestId("deal-room-knowledge-locus")).toHaveTextContent(
      "单向保密协议 (NDA).docx",
    );
    expect(hit).not.toHaveTextContent("第");
    expect(screen.queryByTestId("deal-room-knowledge-jump")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("deal-room-knowledge-jump-doc"));
    expect(screen.getByTestId("location-display")).toHaveTextContent(
      "/viewer/18b1062d-919b-437a-8d5c-76efc60dec86",
    );
  });

  it("restores Q&A draft after remount (viewer back)", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [],
    });
    useKnowledgeQueryStore.getState().setDraft("room-1", {
      query: "保密条款是什么",
      result: {
        query: "保密条款是什么",
        mode: "hybrid",
        answer: "保密义务如下 [1]",
        results: [
          {
            chunkId: "c1",
            documentId: "doc-1",
            text: "4. 保密义务",
            score: 0.95,
            sourceName: "NDA.docx",
            pages: [2],
            viewerPage: 2,
          },
        ],
      },
      activeCite: 1,
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" />
        </I18nextProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByDisplayValue("保密条款是什么")).toBeInTheDocument();
    expect(screen.getByText(/保密义务如下/)).toBeInTheDocument();
    expect(screen.getByTestId("deal-room-knowledge-hit")).toHaveTextContent(
      "4. 保密义务",
    );
    expect(screen.getByTestId("deal-room-knowledge-jump")).toBeInTheDocument();
  });
});
