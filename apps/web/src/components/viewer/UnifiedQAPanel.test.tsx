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

vi.mock("@/stores/aiStore", () => ({
  useAIStore: () => ({
    messages: [],
    pending: false,
    sendMessage: sendMessageMock,
    setHighlight: setHighlightMock,
  }),
}));

async function renderPanel(props: Partial<React.ComponentProps<typeof UnifiedQAPanel>> = {}) {
  const i18n = await createTestI18n({
    common: {},
    layout: {},
    documents: {
      "viewer.sidebarQA": "Q&A",
      "viewer.qaLoadError": "Could not load questions",
      "viewer.qaLengthError": "Question must be 1–500 characters",
      "viewer.qaDisabled": "Q&A is not available",
      "viewer.qaError": "Failed to submit question",
      "viewer.qaEmptyUnified": "No messages yet.",
      "viewer.qaSourceAI": "AI",
      "viewer.qaSourceOwner": "Owner",
      "viewer.qaModeAI": "Ask AI",
      "viewer.qaModeOwner": "Ask Owner",
      "viewer.qaAIPlaceholder": "Ask AI...",
      "viewer.qaOwnerPlaceholder": "Ask owner...",
      "viewer.qaSubmit": "Ask",
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
    expect(screen.getByText("Owner")).toBeInTheDocument();
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

    const input = screen.getByPlaceholderText("Ask owner...");
    fireEvent.change(input, { target: { value: "Can I get a demo?" } });
    fireEvent.click(screen.getByLabelText("Ask"));

    await waitFor(() => {
      expect(createPublicQuestionMock).toHaveBeenCalledWith("token-1", "Can I get a demo?", { sessionToken: "session-1" });
    });
    await waitFor(() => expect(listPublicQuestionsMock).toHaveBeenCalledTimes(2));
  });
});
