// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { UnifiedQAPanel } from "./UnifiedQAPanel";
import { createTestI18n } from "@/i18n/test-utils";
import type { VisitorQuestion } from "@/types";

const { listPublicQuestionsMock, createPublicQuestionMock } = vi.hoisted(() => ({
  listPublicQuestionsMock: vi.fn(),
  createPublicQuestionMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    listPublicQuestions: listPublicQuestionsMock,
    createPublicQuestion: createPublicQuestionMock,
  },
}));

const sendMessageMock = vi.fn();
const setHighlightMock = vi.fn();
const useAIStoreMock = vi.fn(() => ({
  messages: [] as Array<{
    id: string;
    role: "user" | "assistant";
    content: string;
    createdAt: string;
    evidences?: unknown[];
    resultStatus?: string;
    suggestAskHost?: boolean;
  }>,
  pending: false,
  sendMessage: sendMessageMock,
  setHighlight: setHighlightMock,
}));

vi.mock("@/stores/aiStore", () => ({
  useAIStore: () => useAIStoreMock(),
}));

async function renderPanel(props: Partial<React.ComponentProps<typeof UnifiedQAPanel>> = {}) {
  const i18n = await createTestI18n({
    common: {},
    layout: {},
    documents: {
      "viewer.sidebarQA": "Ask",
      "viewer.qaLoadError": "Could not load questions",
      "viewer.qaLengthError": "Question must be 1–500 characters",
      "viewer.qaDisabled": "Q&A is not available",
      "viewer.qaError": "Failed to submit question",
      "viewer.qaEmptyUnified": "No messages yet.",
      "viewer.qaSourceAI": "AI",
      "viewer.qaSourceOwner": "Host",
      "viewer.qaModeAI": "Ask Docs",
      "viewer.qaModeOwner": "Ask Host",
      "viewer.qaAIPlaceholder": "Ask about authorized materials...",
      "viewer.qaOwnerPlaceholder": "Ask the host...",
      "viewer.qaSubmit": "Ask",
      "viewer.qaNoEvidence": "I couldn't find supporting material in the documents you can access for this link.",
      "viewer.qaSuggestAskHost": "You can ask the host instead.",
      "viewer.qaSwitchToAskHost": "Ask the host instead",
    },
    ai: {
      "viewer.thinking": "Thinking...",
      "evidence.page": "Page {{pageNumber}}",
    },
  });

  const view = render(
    <I18nextProvider i18n={i18n}>
      <UnifiedQAPanel
        token="token-1"
        sessionToken="session-1"
        documentId="doc-1"
        qaEnabled
        aiCopilotEnabled={false}
        {...props}
      />
    </I18nextProvider>
  );

  // Flush pending async state updates so tests don't warn about unwrapped act.
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });

  return view;
}

describe("UnifiedQAPanel", () => {
  beforeEach(() => {
    listPublicQuestionsMock.mockReset();
    createPublicQuestionMock.mockReset();
    sendMessageMock.mockReset();
    setHighlightMock.mockReset();
    useAIStoreMock.mockReset();
    useAIStoreMock.mockReturnValue({
      messages: [],
      pending: false,
      sendMessage: sendMessageMock,
      setHighlight: setHighlightMock,
    });
  });

  it("renders owner questions and answers with source tags", async () => {
    const questions: VisitorQuestion[] = [
      {
        id: "q1",
        link_id: "link-1",
        visitor_id: "v1",
        question: "What is the pricing?",
        answer: "Pricing starts at $99.",
        status: "answered",
        created_at: "2026-07-10T10:00:00Z",
        updated_at: "2026-07-10T11:00:00Z",
      },
    ];
    listPublicQuestionsMock.mockResolvedValue({ data: questions });

    await renderPanel();

    await waitFor(() => {
      expect(screen.getByText("What is the pricing?")).toBeInTheDocument();
    });
    expect(screen.getByText("Pricing starts at $99.")).toBeInTheDocument();
    expect(screen.getByText("Host")).toBeInTheDocument();
  });

  it("submits a question to the owner and refreshes the list", async () => {
    listPublicQuestionsMock.mockResolvedValue({ data: [] });
    createPublicQuestionMock.mockResolvedValue({
      data: {
        id: "q2",
        link_id: "link-1",
        visitor_id: "v1",
        question: "Can I get a demo?",
        status: "pending",
        created_at: "2026-07-11T10:00:00Z",
        updated_at: "2026-07-11T10:00:00Z",
      },
    });

    await renderPanel();
    await waitFor(() => expect(listPublicQuestionsMock).toHaveBeenCalledTimes(1));

    const input = screen.getByPlaceholderText("Ask the host...");
    fireEvent.change(input, { target: { value: "Can I get a demo?" } });
    fireEvent.click(screen.getByLabelText("Ask"));

    await waitFor(() => {
      expect(createPublicQuestionMock).toHaveBeenCalledWith("token-1", "Can I get a demo?", { sessionToken: "session-1" });
    });
    await waitFor(() => expect(listPublicQuestionsMock).toHaveBeenCalledTimes(2));
  });

  it("does not re-list questions when sessionToken rotates", async () => {
    listPublicQuestionsMock.mockResolvedValue({ data: [] });
    const { rerender } = await renderPanel({ sessionToken: "session-1" });
    await waitFor(() => expect(listPublicQuestionsMock).toHaveBeenCalledTimes(1));

    const i18n = await createTestI18n({
      common: {},
      layout: {},
      documents: {
        "viewer.sidebarQA": "Ask",
        "viewer.qaLoadError": "Could not load questions",
        "viewer.qaLengthError": "Question must be 1–500 characters",
        "viewer.qaDisabled": "Q&A is not available",
        "viewer.qaError": "Failed to submit question",
        "viewer.qaEmptyUnified": "No messages yet.",
        "viewer.qaSourceAI": "AI",
        "viewer.qaSourceOwner": "Host",
        "viewer.qaModeAI": "Ask Docs",
        "viewer.qaModeOwner": "Ask Host",
        "viewer.qaAIPlaceholder": "Ask about authorized materials...",
        "viewer.qaOwnerPlaceholder": "Ask the host...",
        "viewer.qaSubmit": "Ask",
      },
      ai: {
        "viewer.thinking": "Thinking...",
        "evidence.page": "Page {{pageNumber}}",
      },
    });

    rerender(
      <I18nextProvider i18n={i18n}>
        <UnifiedQAPanel
          token="token-1"
          sessionToken="session-2"
          documentId="doc-1"
          qaEnabled
          aiCopilotEnabled={false}
        />
      </I18nextProvider>
    );

    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50));
    });

    expect(listPublicQuestionsMock).toHaveBeenCalledTimes(1);
  });

  it("defaults to Ask Docs when both channels are enabled", async () => {
    listPublicQuestionsMock.mockResolvedValue({ data: [] });
    await renderPanel({ aiCopilotEnabled: true, qaEnabled: true });
    expect(screen.getByPlaceholderText("Ask about authorized materials...")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Ask Docs/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Ask Host/i })).toBeInTheDocument();
  });

  it("hides mode toggle when only Ask Host is enabled", async () => {
    listPublicQuestionsMock.mockResolvedValue({ data: [] });
    await renderPanel({ aiCopilotEnabled: false, qaEnabled: true });
    expect(screen.queryByRole("button", { name: /Ask Docs/i })).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText("Ask the host...")).toBeInTheDocument();
  });

  it("empty Ask Docs state does not deep-link to file requests", async () => {
    listPublicQuestionsMock.mockResolvedValue({ data: [] });
    await renderPanel({ aiCopilotEnabled: true, qaEnabled: false });
    expect(screen.getByText("No messages yet.")).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.queryByText(/file request/i)).not.toBeInTheDocument();
  });

  it("offers Ask Host switch after no-evidence refusal when Ask Host is enabled", async () => {
    listPublicQuestionsMock.mockResolvedValue({ data: [] });
    useAIStoreMock.mockReturnValue({
      messages: [
        {
          id: "a1",
          role: "assistant",
          content: "I couldn't find supporting material in the documents you can access for this link.",
          createdAt: "2026-07-22T00:00:00Z",
          resultStatus: "no_evidence",
          suggestAskHost: true,
        },
      ],
      pending: false,
      sendMessage: sendMessageMock,
      setHighlight: setHighlightMock,
    });

    await renderPanel({ aiCopilotEnabled: true, qaEnabled: true });
    const switchBtn = screen.getByRole("button", { name: /Ask the host instead/i });
    expect(switchBtn).toBeInTheDocument();
    fireEvent.click(switchBtn);
    expect(screen.getByPlaceholderText("Ask the host...")).toBeInTheDocument();
  });
});
