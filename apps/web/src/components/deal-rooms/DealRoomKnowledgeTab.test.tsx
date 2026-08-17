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
          backToCorpus: "Back to vector library",
          turnQuestion: "Question",
          retrieveQueryLabel: "Searched as",
          sessionTurns: "Session · {{count}} turns",
          newSession: "New session",
          trustScoped: "Research desk",
          trustIsolated: "Data isolated",
          trustGrounded: "Follow-up templates",
          sessionStateTitle: "Desk state",
          sessionStateHint: "Audited gaps and entities from this session.",
          sessionStateOpenQuestions: "{{count}} open gaps",
          sessionStateNoGaps: "No open gaps in this session",
          sessionStateAskGap: "Ask this",
          sessionStateEntities: "Entities",
          sessionStateCoverage: "Recent coverage",
          sessionStateExpand: "Expand",
          sessionStateCollapse: "Collapse",
          askEntryTitle: "AI document Q&A",
          askEntryTagScope: "Scope",
          askEntryTagSecurity: "Security",
          askEntryTagTrust: "Trust",
          askEntryNoteReady: "Ask against synced corpus.",
          answerLabel: "Grounded answer",
          answerHint: "Built from passages in the room corpus",
          sourcesTitle: "Evidence",
          sourcesCount: "{{count}} sources",
          sourcesHidden: "No grounded sources for this answer.",
          corpusTitle: "Semantic vector library",
          corpusHint: "{{synced}} of {{total}} documents synced",
          corpusStageEmpty: "Add room files, then sync.",
          corpusStageBuilding: "Building corpus.",
          corpusStageAttention: "Corpus needs attention.",
          corpusStageReady: "Corpus is ready.",
          corpusReadySummary: "{{synced}} / {{total}} docs synced · ready to ask",
          corpusCollapse: "Hide files",
          corpusExpand: "Show files",
          viewDetails: "View details",
          quotaToggle: "Plan quota",
          quotaUsage: "{{used}}/{{limit}}",
          quotaKnowledgeBases: "Knowledge bases",
          quotaDocuments: "Documents",
          quotaAnswers: "Q&A limit",
          quotaUpgrade: "Upgrade plan",
          quotaPlan: "Plan: {{plan}}",
          quotaPlanUnknown: "Current plan",
          askEntryAction: "Ask",
          askEntryEmpty: "No visitor questions yet",
          askEntryNote: "{{count}} unique visitors asked via share links",
          askEntryBuilding: "Finish syncing the corpus before asking",
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
          stop: "Stop",
          phaseRetrieving: "Searching this room’s corpus…",
          phaseGenerating: "Writing a grounded answer…",
          queryFailed: "Failed to query knowledge base",
          noHits: "No matching passages found.",
          openPage: "Open page {{page}}",
          openDocument: "Open document",
          sheetLabel: "Sheet",
          pageSingle: "p.{{page}}",
          pageRange: "p.{{from}}–{{to}}",
          pageList: "p.{{pages}}",
          pageListSep: ", ",
          sheetMapMissing: "Page map not ready — open document home",
          noPageLocus: "No page locus for this format — open document",
          sessionCloseFailed: "Could not end the current session. Try again.",
          sessionHistory: "Session history",
          sessionHistoryEmpty: "No sessions yet",
          sessionHistoryLoadFailed: "Failed to load session history",
          sessionOpenFailed: "Could not open that session",
          sessionUntitled: "Untitled session",
          sessionStatusActive: "Active",
          sessionStatusClosed: "Archived",
          sessionTurnsShort: "{{count}} turns",
          sessionLoadMore: "Load more",
          followUpLabel: "Suggested follow-ups · this room’s docs",
          followUpUpgrading: "Refining from evidence…",
          followUpSourceEvidence: "Evidence-grounded",
          missionProgressTitle: "Mission progress",
          missionProgressHint: "Checklist coverage from audited session state.",
          missionProgressCount: "{{covered}} / {{total}} covered",
          missionProgressAsk: "Ask this",
          missionProgressComplete: "All checklist items covered.",
          missionProgressLoading: "Loading mission…",
          missionProgressSwitchPack: "Switch mission pack",
          missionProgressChange: "Change",
          missionProgressExpand: "Expand",
          missionProgressCollapse: "Collapse",
          feedback: {
            label: "Turn feedback",
            helpful: "Helpful",
            wrong_citation: "Wrong citation",
            not_answering: "Not answering",
            notePlaceholder: "Which citation is wrong? (optional)",
            saveFailed: "Could not save feedback. Try again.",
          },
          followUp: {
            narrowScope: "Try a more specific file name or clause title?",
            nameClause: "Ask about a named clause in a room document?",
            liabilityInSource: "Ask about liability terms in “{{sourceName}}”?",
            definitionsInSource: "How does “{{sourceName}}” define the key terms?",
            exceptionsInSource: "What exceptions does “{{sourceName}}” list?",
            specificClause: "Drill into a specific clause in this room’s docs?",
            partyObligations: "What obligations does each party have in this room’s docs?",
          },
          errors: {
            knowledge_unavailable: "Knowledge base is unavailable",
            forbidden: "You do not have permission to ask",
            not_found: "Session not found",
            query_failed: "Failed to query knowledge base",
            client_cancelled: "The question was stopped before it finished",
            query_timeout: "The question timed out — try again",
            answer_requires_session: "Answered questions must use the research desk session",
            knowledge_query_busy: "A question is already in progress",
            knowledge_query_rate_limited: "Too many questions",
            knowledge_query_quota_exceeded: "Answer quota used up",
            knowledge_query_quota_unavailable: "Could not verify answer quota",
            knowledge_corpus_not_ready: "The knowledge corpus is not ready",
          },
          status: { ready: "Ready", none: "Not provisioned", syncing: "Syncing" },
          docStatus: { synced: "Synced", pending: "Pending" },
        },
        pageTabs: { knowledge: "Knowledge base" },
        stats: {
          documents: "Documents",
          views: "Views",
          activeLinks: "Active links",
        },
        card: {
          viewDocuments: "View Documents",
          noViewsYet: "No views yet",
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
    getActiveDealRoomKnowledgeSession: vi.fn(),
    listDealRoomKnowledgeSessions: vi.fn(),
    getDealRoomKnowledgeSession: vi.fn(),
    createDealRoomKnowledgeSession: vi.fn(),
    queryDealRoomKnowledgeSession: vi.fn(),
    streamDealRoomKnowledgeSession: vi.fn(),
    closeDealRoomKnowledgeSession: vi.fn(),
    upsertDealRoomKnowledgeTurnFeedback: vi.fn(),
    suggestDealRoomKnowledgeFollowUps: vi.fn(),
    recordDealRoomKnowledgeDeskEvent: vi.fn(),
    getDealRoomKnowledgeMissionProgress: vi.fn(),
    listDealRoomKnowledgeMissions: vi.fn(),
    setDealRoomKnowledgeMission: vi.fn(),
    getDealRoomKnowledgeOps: vi.fn(),
    listDealRoomKnowledgeEvalCandidates: vi.fn(),
    reviewDealRoomKnowledgeEvalCandidate: vi.fn(),
    exportDealRoomKnowledgeEvalCandidates: vi.fn(),
    listDealRoomKnowledgeArchives: vi.fn(),
    getDealRoomKnowledgeArchive: vi.fn(),
    getDealRoomAnalytics: vi.fn(),
    listRoomAsk: vi.fn(),
    getDealRoomLinks: vi.fn(),
  },
}));

function mockStreamQueryResult(
  result: Awaited<ReturnType<typeof api.streamDealRoomKnowledgeSession>>,
) {
  vi.mocked(api.streamDealRoomKnowledgeSession).mockImplementation(
    async (_roomId, _body, opts) => {
      opts.onEvent({ type: "phase", phase: "retrieving" });
      const grounded = !result.turn.refused && (result.results?.length ?? 0) > 0;
      if (grounded) {
        opts.onEvent({ type: "sources", results: result.results, grounded: true });
      }
      opts.onEvent({
        type: "done",
        answer: result.answer,
        results: result.results,
        refused: result.turn.refused,
        resultStatus: result.turn.resultStatus,
      });
      return result;
    },
  );
}

function mockRoomMetrics() {
  vi.mocked(api.getDealRoomAnalytics).mockResolvedValue({
    totalViews: 0,
    uniqueVisitors: 0,
    activeLinkCount: 0,
    documentCount: 1,
    viewsOverTime: [],
    recentVisitors: [],
  });
  vi.mocked(api.listRoomAsk).mockResolvedValue({ data: [] });
  vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [] });
  vi.mocked(api.getActiveDealRoomKnowledgeSession).mockResolvedValue({
    session: null,
    turns: [],
  });
  vi.mocked(api.closeDealRoomKnowledgeSession).mockResolvedValue({
    id: "sess-closed",
    roomId: "room-1",
    status: "closed",
    createdAt: "2026-08-03T00:00:00Z",
    updatedAt: "2026-08-03T00:00:00Z",
  });
  vi.mocked(api.listDealRoomKnowledgeSessions).mockResolvedValue({ items: [] });
  vi.mocked(api.getDealRoomKnowledgeSession).mockResolvedValue({
    session: null,
    turns: [],
  });
  vi.mocked(api.upsertDealRoomKnowledgeTurnFeedback).mockImplementation(
    async (_roomId, _turnId, body) => ({
      kind: body.kind,
      note: body.note,
    }),
  );
  vi.mocked(api.recordDealRoomKnowledgeDeskEvent).mockResolvedValue(undefined);
  // Keep local templates stable in unit tests (API upgrade is covered by MSW e2e).
  vi.mocked(api.suggestDealRoomKnowledgeFollowUps).mockResolvedValue({
    source: "template",
    items: [],
  });
  vi.mocked(api.getDealRoomKnowledgeMissionProgress).mockResolvedValue({
    packId: "financing_dd_v1",
    title: "Financing due diligence",
    source: "template_default",
    covered: 0,
    total: 1,
    items: [
      {
        id: "valuation_cap",
        prompt: "What valuation cap appears in this room’s financing docs?",
        covered: false,
      },
    ],
  });
  vi.mocked(api.listDealRoomKnowledgeMissions).mockResolvedValue({
    items: [
      {
        packId: "financing_dd_v1",
        title: "Financing due diligence",
        source: "catalog",
      },
    ],
  });
  vi.mocked(api.setDealRoomKnowledgeMission).mockResolvedValue({
    packId: "financing_dd_v1",
    title: "Financing due diligence",
    source: "room",
  });
  vi.mocked(api.getDealRoomKnowledgeOps).mockResolvedValue({
    scope: "workspace",
    windowHours: 24,
    turnsTotal: 0,
    turnsByStatus: {},
    avgDurationMs: 0,
    p95DurationMs: 0,
    costUnitsTotal: 0,
    refusalsByKind: {},
    judgmentsByKind: {},
    evalCandidatesByStatus: {},
    pendingEvalCandidates: 0,
    answersQuota: { used: 0, limit: 100, windowHours: 24 },
    coldArchiveCount: 0,
    retentionDays: 90,
  });
  vi.mocked(api.listDealRoomKnowledgeEvalCandidates).mockResolvedValue({
    items: [],
  });
  vi.mocked(api.reviewDealRoomKnowledgeEvalCandidate).mockResolvedValue({
    id: "cand-1",
    roomId: "room-1",
    turnId: "turn-1",
    feedbackKind: "wrong_citation",
    question: "q",
    reviewStatus: "accepted",
    createdAt: "2026-08-04T00:00:00Z",
  });
  vi.mocked(api.exportDealRoomKnowledgeEvalCandidates).mockResolvedValue({
    description: "Accepted",
    seeds: [],
  });
  vi.mocked(api.listDealRoomKnowledgeArchives).mockResolvedValue({ items: [] });
  vi.mocked(api.getDealRoomKnowledgeArchive).mockRejectedValue(new Error("not found"));
}

describe("DealRoomKnowledgeTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useKnowledgeQueryStore.getState().clear();
    mockRoomMetrics();
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
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
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
    const hitPayload = {
      chunkId: "c1",
      documentId: "doc-1",
      text: "valuation cap $10M",
      score: 0.9,
      sourceName: "Memo.pdf",
      pages: [3, 4],
      viewerPage: 3,
    };
    mockStreamQueryResult({
      sessionId: "sess-1",
      query: "valuation",
      mode: "hybrid",
      answer: "The cap is $10M [1]",
      results: [hitPayload],
      turn: {
        id: "turn-1",
        sessionId: "sess-1",
        sequence: 1,
        question: "valuation",
        answer: "The cap is $10M [1]",
        refused: false,
        resultStatus: "answered",
        hits: [hitPayload],
        createdAt: "2026-08-03T00:00:00Z",
      },
    });

    render(
      <MemoryRouter initialEntries={["/deal-rooms/room-1"]}>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
          <LocationDisplay />
        </I18nextProvider>
      </MemoryRouter>,
    );

    expect(
      await screen.findByTestId("deal-room-knowledge-corpus"),
    ).toHaveAttribute("data-corpus-stage", "ready");
    expect(await screen.findByTestId("deal-room-knowledge-ask-entry")).toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-knowledge-desk")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("deal-room-knowledge-corpus-expand"));
    expect(await screen.findByTestId("deal-room-knowledge-corpus-details")).toBeInTheDocument();
    expect(await screen.findByText("Memo.pdf")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("deal-room-knowledge-corpus-expand"));
    expect(screen.queryByTestId("deal-room-knowledge-corpus-details")).not.toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-knowledge-ask-entry-start")).toBeEnabled();
    });
    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask-entry-start"));
    expect(await screen.findByTestId("deal-room-knowledge-desk")).toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-knowledge-ask-entry")).not.toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-knowledge-corpus")).not.toBeInTheDocument();
    expect(screen.getByTestId("deal-room-knowledge-back-to-corpus")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("deal-room-knowledge-back-to-corpus"));
    expect(await screen.findByTestId("deal-room-knowledge-corpus")).toBeInTheDocument();
    expect(screen.getByTestId("deal-room-knowledge-ask-entry")).toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-knowledge-desk")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask-entry-start"));
    expect(await screen.findByTestId("deal-room-knowledge-desk")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Question"), {
      target: { value: "valuation" },
    });
    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask"));
    await waitFor(() => {
      expect(api.streamDealRoomKnowledgeSession).toHaveBeenCalledWith(
        "room-1",
        expect.objectContaining({
          sessionId: undefined,
          query: "valuation",
          answer: true,
          top_k: 8,
          clientRequestId: expect.any(String),
        }),
        expect.objectContaining({ onEvent: expect.any(Function) }),
      );
    });
    expect(await screen.findByText(/The cap is \$10M/)).toBeInTheDocument();
    const hit = screen.getByTestId("deal-room-knowledge-hit");
    expect(hit).toHaveTextContent("valuation cap $10M");
    expect(hit).toHaveTextContent("Memo.pdf · p.3–4");
    expect(await screen.findByTestId("grounded-chat-follow-ups")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("grounded-chat-follow-up-liability-in-source"));
    await waitFor(() => {
      expect(api.streamDealRoomKnowledgeSession).toHaveBeenCalledWith(
        "room-1",
        expect.objectContaining({
          query: "Ask about liability terms in “Memo.pdf”?",
          answer: true,
        }),
        expect.objectContaining({ onEvent: expect.any(Function) }),
      );
    });
    expect(screen.getByTestId("knowledge-turn-feedback-turn-1")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("knowledge-feedback-wrong_citation"));
    await waitFor(() => {
      expect(api.upsertDealRoomKnowledgeTurnFeedback).toHaveBeenCalledWith(
        "room-1",
        "turn-1",
        { kind: "wrong_citation", note: undefined },
      );
    });
    expect(await screen.findByTestId("knowledge-feedback-note")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("knowledge-feedback-helpful"));
    await waitFor(() => {
      expect(api.upsertDealRoomKnowledgeTurnFeedback).toHaveBeenCalledWith(
        "room-1",
        "turn-1",
        { kind: "helpful", note: undefined },
      );
    });
    fireEvent.click(screen.getByTestId("knowledge-cite-1"));
    expect(hit.className).toMatch(/border-foreground/);
    expect(screen.getByTestId("location-display")).toHaveTextContent(
      "/deal-rooms/room-1",
    );
    fireEvent.click(screen.getByTestId("deal-room-knowledge-jump"));
    expect(screen.getByTestId("location-display")).toHaveTextContent(
      "/viewer/doc-1?page=3&roomId=room-1",
    );
  });

  it("opens a historical session from the history menu", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      lastSyncedAt: "2026-08-01T00:00:00Z",
      documents: [
        {
          documentId: "doc-1",
          title: "Memo.pdf",
          status: "synced",
          chunkCount: 2,
        },
      ],
      progress: { synced: 1, total: 1, syncing: 0, pending: 0, failed: 0, jobStatus: "succeeded" },
    });
    vi.mocked(api.listDealRoomKnowledgeSessions).mockResolvedValue({
      items: [
        {
          id: "sess-old",
          roomId: "room-1",
          title: "old question",
          status: "closed",
          turnCount: 1,
          questionPreview: "old question",
          createdAt: "2026-08-01T00:00:00Z",
          updatedAt: "2026-08-01T00:00:00Z",
          lastTurnAt: "2026-08-01T00:00:00Z",
        },
      ],
    });
    vi.mocked(api.getDealRoomKnowledgeSession).mockResolvedValue({
      session: {
        id: "sess-old",
        roomId: "room-1",
        title: "old question",
        status: "closed",
        createdAt: "2026-08-01T00:00:00Z",
        updatedAt: "2026-08-01T00:00:00Z",
      },
      turns: [
        {
          id: "turn-old",
          sessionId: "sess-old",
          sequence: 1,
          question: "old question",
          answer: "Archived answer [1]",
          refused: false,
          resultStatus: "answered",
          hits: [
            {
              chunkId: "c1",
              text: "archived hit",
              score: 0.9,
              documentId: "doc-1",
              sourceName: "Memo.pdf",
              pages: [1],
              viewerPage: 1,
            },
          ],
          createdAt: "2026-08-01T00:00:00Z",
        },
      ],
    });

    render(
      <MemoryRouter initialEntries={["/deal-rooms/room-1"]}>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
        </I18nextProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-knowledge-ask-entry-start")).toBeEnabled();
    });
    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask-entry-start"));
    expect(await screen.findByTestId("deal-room-knowledge-desk")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("deal-room-knowledge-session-history"));
    expect(await screen.findByTestId("deal-room-knowledge-session-sess-old")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("deal-room-knowledge-session-sess-old"));
    await waitFor(() => {
      expect(api.getDealRoomKnowledgeSession).toHaveBeenCalledWith("room-1", "sess-old");
    });
    expect(await screen.findByText("old question")).toBeInTheDocument();
    expect(await screen.findByText(/Archived answer/)).toBeInTheDocument();
  });

  it("hydrates active session after stop so a persisted turn is not lost", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [
        { documentId: "doc-1", title: "Memo.pdf", status: "synced", chunkCount: 3 },
      ],
    });
    const persisted = {
      id: "turn-aborted",
      sessionId: "sess-aborted",
      sequence: 1,
      question: "valuation",
      answer: "Persisted after abort [1]",
      refused: false,
      resultStatus: "answered" as const,
      hits: [
        {
          chunkId: "c1",
          documentId: "doc-1",
          text: "cap",
          score: 0.9,
          sourceName: "Memo.pdf",
          viewerPage: 3,
        },
      ],
      createdAt: "2026-08-03T00:00:00Z",
    };
    vi.mocked(api.streamDealRoomKnowledgeSession).mockImplementation(
      (_roomId, _body, opts) =>
        new Promise((_resolve, reject) => {
          opts.onEvent({ type: "phase", phase: "retrieving" });
          opts.signal?.addEventListener("abort", () => {
            reject(new DOMException("Aborted", "AbortError"));
          });
        }),
    );
    vi.mocked(api.getActiveDealRoomKnowledgeSession)
      .mockResolvedValueOnce({ session: null, turns: [] }) // mount hydrate
      .mockResolvedValueOnce({ session: null, turns: [] }) // abort: server still writing
      .mockResolvedValueOnce({
        session: {
          id: "sess-aborted",
          roomId: "room-1",
          status: "active",
          createdAt: "2026-08-03T00:00:00Z",
          updatedAt: "2026-08-03T00:00:00Z",
        },
        turns: [persisted],
      });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
        </I18nextProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-knowledge-ask-entry-start")).toBeEnabled();
    });
    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask-entry-start"));
    fireEvent.change(await screen.findByLabelText("Question"), {
      target: { value: "valuation" },
    });
    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask"));
    expect(await screen.findByTestId("grounded-chat-stop")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("grounded-chat-stop"));
    expect(await screen.findByTestId("knowledge-answer-markdown")).toHaveTextContent(
      /Persisted after abort/,
    );
    // mount + empty abort poll + successful poll
    expect(vi.mocked(api.getActiveDealRoomKnowledgeSession).mock.calls.length).toBeGreaterThanOrEqual(3);
  });

  it("ignores a second Ask click while a stream is already in flight", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [
        { documentId: "doc-1", title: "Memo.pdf", status: "synced", chunkCount: 3 },
      ],
    });
    let resolveStream: ((value: Awaited<ReturnType<typeof api.streamDealRoomKnowledgeSession>>) => void) | undefined;
    vi.mocked(api.streamDealRoomKnowledgeSession).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveStream = resolve;
        }),
    );

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
        </I18nextProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-knowledge-ask-entry-start")).toBeEnabled();
    });
    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask-entry-start"));
    fireEvent.change(await screen.findByLabelText("Question"), {
      target: { value: "first" },
    });
    const ask = screen.getByTestId("deal-room-knowledge-ask");
    fireEvent.click(ask);
    fireEvent.click(ask);
    fireEvent.click(ask);
    expect(api.streamDealRoomKnowledgeSession).toHaveBeenCalledTimes(1);
    resolveStream?.({
      sessionId: "sess-1",
      turn: {
        id: "t1",
        sessionId: "sess-1",
        sequence: 1,
        question: "first",
        answer: "ok",
        refused: false,
        resultStatus: "answered",
        hits: [],
        createdAt: "2026-08-03T00:00:00Z",
      },
      query: "first",
      mode: "hybrid",
      answer: "ok",
      results: [],
    });
    expect(await screen.findByTestId("knowledge-answer-markdown")).toHaveTextContent(
      "ok",
    );
  });

  it("disables start-ask until the vector library is truly ready", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "syncing",
      documents: [
        { documentId: "doc-1", title: "Memo.pdf", status: "syncing", chunkCount: 0 },
      ],
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
        </I18nextProvider>
      </MemoryRouter>,
    );

    const start = await screen.findByTestId("deal-room-knowledge-ask-entry-start");
    expect(start).toBeDisabled();
    expect(screen.queryByTestId("deal-room-knowledge-desk")).not.toBeInTheDocument();
  });

  it("hides sources when the grounded answer refuses a match", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [
        { documentId: "doc-1", title: "NDA.pdf", status: "synced", chunkCount: 2 },
      ],
    });
    mockStreamQueryResult({
      sessionId: "sess-1",
      query: "是",
      mode: "hybrid",
      answer:
        "The provided context does not contain an answer to the question '是'.",
      results: [],
      turn: {
        id: "turn-1",
        sessionId: "sess-1",
        sequence: 1,
        question: "是",
        answer:
          "The provided context does not contain an answer to the question '是'.",
        refused: true,
        resultStatus: "refused",
        hits: [],
        createdAt: "2026-08-03T00:00:00Z",
      },
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
        </I18nextProvider>
      </MemoryRouter>,
    );

    const start = await screen.findByTestId("deal-room-knowledge-ask-entry-start");
    await waitFor(() => expect(start).toBeEnabled());
    fireEvent.click(start);
    fireEvent.change(await screen.findByLabelText("Question"), {
      target: { value: "是" },
    });
    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask"));
    await waitFor(() => {
      expect(api.streamDealRoomKnowledgeSession).toHaveBeenCalled();
    });
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
    expect(
      isUngroundedKnowledgeAnswer(
        "根据您提供的上下文，文档属于单向保密协议。因此，无法根据现有上下文回答该问题。",
      ),
    ).toBe(true);
    expect(isUngroundedKnowledgeAnswer("The cap is $10M [1]")).toBe(false);
  });
});

describe("formatHitLocusLabel", () => {
  const zhFmt = {
    sheetPrefix: "工作表",
    pageSingle: (page: number) => `第${page}页`,
    pageRange: (from: number, to: number) => `第${from}–${to}页`,
    pageListSep: "、",
    pageList: (pages: string) => `第${pages}页`,
  };
  const enFmt = {
    sheetPrefix: "Sheet",
    pageSingle: (page: number) => `p.${page}`,
    pageRange: (from: number, to: number) => `p.${from}–${to}`,
    pageListSep: ", ",
    pageList: (pages: string) => `p.${pages}`,
  };

  it("formats pages and sheet without inventing missing pages", async () => {
    const { formatPagesLabel } = await import("./DealRoomKnowledgeTab");
    expect(
      formatHitLocusLabel(
        {
          chunkId: "c",
          text: "t",
          score: 1,
          sourceName: "a.xlsx",
          sheet: "损益表",
        },
        { ...enFmt, sheetPrefix: "" },
      ),
    ).toBe("a.xlsx · 损益表");
    expect(
      formatHitLocusLabel(
        {
          chunkId: "c",
          text: "t",
          score: 1,
          sourceName: "a.xlsx",
          sheet: "损益表",
        },
        enFmt,
      ),
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
        zhFmt,
      ),
    ).toBe("a.xlsx · 工作表 损益表");
    expect(
      formatHitLocusLabel(
        {
          chunkId: "c",
          text: "t",
          score: 1,
          pages: [4, 3],
        },
        zhFmt,
      ),
    ).toBe("第3–4页");
    expect(formatPagesLabel([1, 3], zhFmt)).toBe("第1、3页");
  });
});

describe("docx citation open without page", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useKnowledgeQueryStore.getState().clear();
    mockRoomMetrics();
  });

  it("shows room title locus and opens document home when viewerPage is absent", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [
        {
          documentId: "18b1062d-919b-437a-8d5c-76efc60dec86",
          title: "单向保密协议 (NDA).docx",
          status: "synced",
          chunkCount: 3,
        },
      ],
    });
    const hitPayload = {
      chunkId: "c1",
      documentId: "18b1062d-919b-437a-8d5c-76efc60dec86",
      text: "**单向保密协议 (NDA)**\n1. 目的",
      score: 0.867,
      sourceName: "单向保密协议 (NDA).docx",
    };
    mockStreamQueryResult({
      sessionId: "sess-1",
      query: "保密条款",
      mode: "hybrid",
      answer: "保密义务如下 [1]",
      results: [hitPayload],
      turn: {
        id: "turn-1",
        sessionId: "sess-1",
        sequence: 1,
        question: "保密条款",
        answer: "保密义务如下 [1]",
        refused: false,
        resultStatus: "answered",
        hits: [hitPayload],
        createdAt: "2026-08-03T00:00:00Z",
      },
    });

    render(
      <MemoryRouter initialEntries={["/deal-rooms/room-1"]}>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
          <LocationDisplay />
        </I18nextProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("deal-room-knowledge-ask-entry-start")).toBeEnabled();
    });
    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask-entry-start"));
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
      "/viewer/18b1062d-919b-437a-8d5c-76efc60dec86?roomId=room-1",
    );
  });

  it("restores Q&A draft after remount (viewer back)", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [
        { documentId: "doc-1", title: "NDA.docx", status: "synced", chunkCount: 1 },
      ],
    });
    // Avoid hydrate overwriting the in-memory draft.
    vi.mocked(api.getActiveDealRoomKnowledgeSession).mockResolvedValue({
      session: null,
      turns: [],
    });
    useKnowledgeQueryStore.getState().setDraft("room-1", {
      query: "",
      activeSessionId: "sess-1",
      turns: [
        {
          id: "turn-1",
          sessionId: "sess-1",
          sequence: 1,
          question: "保密条款是什么",
          answer: "保密义务如下 [1]",
          refused: false,
          resultStatus: "answered",
          hits: [
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
          createdAt: "2026-08-03T00:00:00Z",
        },
      ],
      activeCite: 1,
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
        </I18nextProvider>
      </MemoryRouter>,
    );

    // Prior turns restore directly into chat (cards stay hidden).
    expect(await screen.findByTestId("deal-room-knowledge-desk")).toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-knowledge-ask-entry")).not.toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-knowledge-corpus")).not.toBeInTheDocument();
    expect(screen.getByText("保密条款是什么")).toBeInTheDocument();
    expect(screen.getByText(/保密义务如下/)).toBeInTheDocument();
    expect(screen.getByTestId("deal-room-knowledge-hit")).toHaveTextContent(
      "4. 保密义务",
    );
    expect(screen.getByTestId("deal-room-knowledge-jump")).toBeInTheDocument();
  });

  it("hydrates an active session from the server after refresh", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [
        { documentId: "doc-1", title: "Memo.pdf", status: "synced", chunkCount: 2 },
      ],
    });
    vi.mocked(api.getActiveDealRoomKnowledgeSession).mockResolvedValue({
      session: {
        id: "sess-9",
        roomId: "room-1",
        status: "active",
        title: "valuation",
        createdAt: "2026-08-03T00:00:00Z",
        updatedAt: "2026-08-03T00:00:00Z",
        turnCount: 1,
      },
      turns: [
        {
          id: "turn-9",
          sessionId: "sess-9",
          sequence: 1,
          question: "valuation",
          answer: "The cap is $10M [1]",
          refused: false,
          resultStatus: "answered",
          hits: [
            {
              chunkId: "c1",
              documentId: "doc-1",
              text: "cap $10M",
              score: 0.9,
              sourceName: "Memo.pdf",
              viewerPage: 3,
            },
          ],
          createdAt: "2026-08-03T00:00:00Z",
          // C1: persisted feedback survives refresh hydrate
          feedback: {
            kind: "wrong_citation",
            note: "page 3 locus wrong",
          },
        },
      ],
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
        </I18nextProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByTestId("deal-room-knowledge-desk")).toBeInTheDocument();
    expect(screen.getByText("valuation")).toBeInTheDocument();
    expect(screen.getByText(/The cap is \$10M/)).toBeInTheDocument();
    const wrongCite = screen.getByTestId("knowledge-feedback-wrong_citation");
    expect(wrongCite).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("knowledge-feedback-note")).toHaveValue(
      "page 3 locus wrong",
    );
  });

  it("toggles the session state rail from the Research desk chip", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [
        { documentId: "doc-1", title: "Memo.pdf", status: "synced", chunkCount: 2 },
      ],
    });
    vi.mocked(api.getActiveDealRoomKnowledgeSession).mockResolvedValue({
      session: {
        id: "sess-rail",
        roomId: "room-1",
        status: "active",
        title: "nda",
        createdAt: "2026-08-03T00:00:00Z",
        updatedAt: "2026-08-03T00:00:00Z",
        turnCount: 1,
        state: {
          openQuestions: [{ text: "Gap about term", sourceTurnId: "turn-r1" }],
        },
      },
      turns: [
        {
          id: "turn-r1",
          sessionId: "sess-rail",
          sequence: 1,
          question: "nda",
          answer: "Term is two years [1]",
          refused: false,
          resultStatus: "answered",
          hits: [
            {
              chunkId: "c1",
              documentId: "doc-1",
              text: "two years",
              score: 0.9,
              sourceName: "NDA.pdf",
              viewerPage: 1,
            },
          ],
          createdAt: "2026-08-03T00:00:00Z",
        },
      ],
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
        </I18nextProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByTestId("deal-room-knowledge-desk")).toBeInTheDocument();
    const toggle = screen.getByTestId("deal-room-knowledge-session-state-toggle");
    expect(toggle).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryByTestId("knowledge-session-state-rail")).not.toBeInTheDocument();

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-pressed", "true");
    expect(await screen.findByTestId("knowledge-session-state-rail")).toBeInTheDocument();
    expect(screen.getByText("Gap about term")).toBeInTheDocument();

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryByTestId("knowledge-session-state-rail")).not.toBeInTheDocument();
  });

  it("shows follow-up templates on an empty desk and hides them once Q&A has turns", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [
        { documentId: "doc-1", title: "Memo.pdf", status: "synced", chunkCount: 2 },
      ],
    });
    vi.mocked(api.getActiveDealRoomKnowledgeSession).mockResolvedValue({
      session: null,
      turns: [],
    });
    vi.mocked(api.getDealRoomKnowledgeMissionProgress).mockResolvedValue({
      packId: "financing_dd_v1",
      title: "Financing DD",
      source: "template_default",
      covered: 0,
      total: 2,
      items: [
        { id: "cap", prompt: "What is the valuation cap?", covered: false },
        { id: "pool", prompt: "How is the option pool sized?", covered: false },
      ],
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
        </I18nextProvider>
      </MemoryRouter>,
    );

    fireEvent.click(
      await screen.findByTestId("deal-room-knowledge-ask-entry-start"),
    );
    expect(await screen.findByTestId("deal-room-knowledge-desk")).toBeInTheDocument();
    const toggle = screen.getByTestId("deal-room-knowledge-mission-progress-toggle");
    expect(toggle).toHaveAttribute("aria-pressed", "true");
    expect(
      await screen.findByTestId("knowledge-mission-progress-rail"),
    ).toBeInTheDocument();

    // Hydrate a turn into the store → templates auto-hide during Q&A.
    useKnowledgeQueryStore.getState().setDraft("room-1", {
      activeSessionId: "sess-mission",
      turns: [
        {
          id: "turn-m1",
          sessionId: "sess-mission",
          sequence: 1,
          question: "nda",
          answer: "Term is two years [1]",
          refused: false,
          resultStatus: "answered",
          hits: [
            {
              chunkId: "c1",
              documentId: "doc-1",
              text: "two years",
              score: 0.9,
              sourceName: "NDA.pdf",
              viewerPage: 1,
            },
          ],
          createdAt: "2026-08-03T00:00:00Z",
        },
      ],
      query: "",
      activeCite: null,
      sessionState: null,
    });

    await waitFor(() => {
      expect(
        screen.getByTestId("deal-room-knowledge-mission-progress-toggle"),
      ).toHaveAttribute("aria-pressed", "false");
    });
    expect(screen.queryByTestId("knowledge-mission-progress-rail")).not.toBeInTheDocument();

    // New session restores the empty-desk default (templates shown).
    fireEvent.click(screen.getByTestId("deal-room-knowledge-new-session"));
    await waitFor(() => {
      expect(
        screen.getByTestId("deal-room-knowledge-mission-progress-toggle"),
      ).toHaveAttribute("aria-pressed", "true");
    });
    expect(
      await screen.findByTestId("knowledge-mission-progress-rail"),
    ).toBeInTheDocument();
  });

  it("toggles the mission progress rail from the Follow-up templates chip during Q&A", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [
        { documentId: "doc-1", title: "Memo.pdf", status: "synced", chunkCount: 2 },
      ],
    });
    vi.mocked(api.getActiveDealRoomKnowledgeSession).mockResolvedValue({
      session: {
        id: "sess-mission",
        roomId: "room-1",
        status: "active",
        title: "nda",
        createdAt: "2026-08-03T00:00:00Z",
        updatedAt: "2026-08-03T00:00:00Z",
        turnCount: 1,
      },
      turns: [
        {
          id: "turn-m1",
          sessionId: "sess-mission",
          sequence: 1,
          question: "nda",
          answer: "Term is two years [1]",
          refused: false,
          resultStatus: "answered",
          hits: [
            {
              chunkId: "c1",
              documentId: "doc-1",
              text: "two years",
              score: 0.9,
              sourceName: "NDA.pdf",
              viewerPage: 1,
            },
          ],
          createdAt: "2026-08-03T00:00:00Z",
        },
      ],
    });

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
        </I18nextProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByTestId("deal-room-knowledge-desk")).toBeInTheDocument();
    const toggle = screen.getByTestId("deal-room-knowledge-mission-progress-toggle");
    expect(toggle).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryByTestId("knowledge-mission-progress-rail")).not.toBeInTheDocument();

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-pressed", "true");
    expect(await screen.findByTestId("knowledge-mission-progress-rail")).toBeInTheDocument();

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryByTestId("knowledge-mission-progress-rail")).not.toBeInTheDocument();
  });

  it("hides previous turn feedback once the next ask starts", async () => {
    vi.mocked(api.getDealRoomKnowledge).mockResolvedValue({
      enabled: true,
      status: "ready",
      documents: [
        { documentId: "doc-1", title: "Memo.pdf", status: "synced", chunkCount: 3 },
      ],
    });
    vi.mocked(api.getActiveDealRoomKnowledgeSession).mockResolvedValue({
      session: {
        id: "sess-9",
        roomId: "room-1",
        status: "active",
        title: "valuation",
        createdAt: "2026-08-03T00:00:00Z",
        updatedAt: "2026-08-03T00:00:00Z",
        turnCount: 1,
      },
      turns: [
        {
          id: "turn-9",
          sessionId: "sess-9",
          sequence: 1,
          question: "valuation",
          answer: "The cap is $10M [1]",
          refused: false,
          resultStatus: "answered",
          hits: [
            {
              chunkId: "c1",
              documentId: "doc-1",
              text: "cap $10M",
              score: 0.9,
              sourceName: "Memo.pdf",
              viewerPage: 3,
            },
          ],
          createdAt: "2026-08-03T00:00:00Z",
          feedback: { kind: "helpful" },
        },
      ],
    });

    let resolveStream:
      | ((value: Awaited<ReturnType<typeof api.streamDealRoomKnowledgeSession>>) => void)
      | undefined;
    vi.mocked(api.streamDealRoomKnowledgeSession).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveStream = resolve;
        }),
    );

    render(
      <MemoryRouter>
        <I18nextProvider i18n={i18nInstance}>
          <DealRoomKnowledgeTab roomId="room-1" canContribute />
        </I18nextProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByTestId("knowledge-turn-feedback-turn-9")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Question"), {
      target: { value: "next question" },
    });
    fireEvent.click(screen.getByTestId("deal-room-knowledge-ask"));
    await waitFor(() => {
      expect(screen.queryByTestId("knowledge-turn-feedback-turn-9")).not.toBeInTheDocument();
    });
    resolveStream?.({
      sessionId: "sess-9",
      turn: {
        id: "turn-10",
        sessionId: "sess-9",
        sequence: 2,
        question: "next question",
        answer: "second answer",
        refused: false,
        resultStatus: "answered",
        hits: [],
        createdAt: "2026-08-03T00:01:00Z",
      },
      query: "next question",
      mode: "hybrid",
      answer: "second answer",
      results: [],
    });
    expect(await screen.findByTestId("knowledge-turn-feedback-turn-10")).toBeInTheDocument();
    expect(screen.queryByTestId("knowledge-turn-feedback-turn-9")).not.toBeInTheDocument();
  });
});
