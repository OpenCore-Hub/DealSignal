// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { MemoryRouter } from "react-router";
import { api } from "@/lib/api";
import { DealRoomKnowledgeTab } from "./DealRoomKnowledgeTab";

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
        { chunkId: "c1", documentId: "doc-1", text: "valuation cap $10M", score: 0.9 },
      ],
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" />
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
    expect(screen.getByTestId("deal-room-knowledge-hit")).toHaveTextContent(
      "valuation cap $10M",
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
