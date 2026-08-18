// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { UnifiedQAPanel } from "./UnifiedQAPanel";
import enDocuments from "@/i18n/locales/en/documents.json";

const { listPublicAskTurnsMock, createPublicAskMock, streamPublicAskTurnMock, listPublicAskFAQsMock } = vi.hoisted(() => ({
  listPublicAskTurnsMock: vi.fn(),
  createPublicAskMock: vi.fn(),
  streamPublicAskTurnMock: vi.fn(),
  listPublicAskFAQsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    listPublicAskTurns: listPublicAskTurnsMock,
    listPublicAskFAQs: listPublicAskFAQsMock,
    createPublicAsk: createPublicAskMock,
    streamPublicAskTurn: streamPublicAskTurnMock,
  },
}));

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: { en: { documents: enDocuments } },
  interpolation: { escapeValue: false },
});

async function renderPanel(props: Partial<React.ComponentProps<typeof UnifiedQAPanel>> = {}) {
  render(
    <I18nextProvider i18n={i18nInstance}>
      <UnifiedQAPanel token="tok123" sessionToken="sess456" qaEnabled {...props} />
    </I18nextProvider>
  );
  await waitFor(() => {
    expect(listPublicAskTurnsMock).toHaveBeenCalled();
  });
}

describe("UnifiedQAPanel (unified Ask host lane)", () => {
  beforeEach(() => {
    listPublicAskTurnsMock.mockReset();
    listPublicAskFAQsMock.mockReset();
    createPublicAskMock.mockReset();
    streamPublicAskTurnMock.mockReset();
    listPublicAskTurnsMock.mockResolvedValue({ data: [] });
    listPublicAskFAQsMock.mockResolvedValue({ data: [] });
    streamPublicAskTurnMock.mockResolvedValue(null);
    createPublicAskMock.mockResolvedValue({
      data: {
        id: "turn1",
        session_id: "sess1",
        question: "test",
        lane: "host",
        status: "host_pending",
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      },
    });
  });

  it("loads and shows empty state", async () => {
    await renderPanel();
    expect(screen.getByText(/No messages yet/i)).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/materials you can access/i)).toBeInTheDocument();
  });

  it("submits unified Ask turn", async () => {
    await renderPanel();
    fireEvent.change(screen.getByPlaceholderText(/materials you can access/i), {
      target: { value: "Can you share the model?" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Ask/i }));
    await waitFor(() => {
      expect(createPublicAskMock).toHaveBeenCalledWith(
        "tok123",
        "Can you share the model?",
        { sessionToken: "sess456" }
      );
    });
  });

  it("shows rate limit error", async () => {
    const { ApiError } = await import("@/lib/apiClient");
    createPublicAskMock.mockRejectedValue(
      new ApiError({ status: 429, code: "rate_limit_exceeded", message: "rate limited", requestId: "r1" })
    );
    await renderPanel();
    fireEvent.change(screen.getByPlaceholderText(/materials you can access/i), {
      target: { value: "spam" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Ask/i }));
    await waitFor(() => {
      expect(screen.getByText(/Too many questions/i)).toBeInTheDocument();
    });
  });

  it("starts AI stream for ai_streaming turns", async () => {
    listPublicAskTurnsMock.mockResolvedValue({
      data: [
        {
          id: "turn_ai",
          session_id: "sess1",
          question: "What was revenue?",
          lane: "ai",
          status: "ai_streaming",
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      ],
    });
    await renderPanel();
    await waitFor(() => {
      expect(streamPublicAskTurnMock).toHaveBeenCalledWith(
        "tok123",
        "turn_ai",
        expect.objectContaining({ creds: { sessionToken: "sess456" } }),
      );
    });
  });

  it("renders answered host turns", async () => {
    listPublicAskTurnsMock.mockResolvedValue({
      data: [
        {
          id: "turn1",
          session_id: "sess1",
          question: "Where is the cap table?",
          host_answer: "In the Legal folder.",
          lane: "host",
          status: "host_answered",
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-02T00:00:00Z",
        },
      ],
    });
    await renderPanel();
    expect(await screen.findByText("Where is the cap table?")).toBeInTheDocument();
    expect(screen.getByText("In the Legal folder.")).toBeInTheDocument();
  });

  it("places host reply after AI refuse and hides escalate once answered", async () => {
    listPublicAskTurnsMock.mockResolvedValue({
      data: [
        {
          id: "turn_hybrid",
          session_id: "sess1",
          question: "What is GMV growth?",
          host_answer: "Not public yet, pending finance.",
          lane: "hybrid",
          status: "host_answered",
          ai_payload: {
            answer: "",
            refused: true,
            resultStatus: "no_hits",
            hits: [],
          },
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-02T00:00:00Z",
        },
      ],
    });
    await renderPanel();
    const aiRefuse = await screen.findByText(/couldn't find supporting material/i);
    const hostReply = screen.getByText("Not public yet, pending finance.");
    expect(aiRefuse.compareDocumentPosition(hostReply) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.queryByText(/send this to the host for confirmation/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Send to host/i })).not.toBeInTheDocument();
  });

  it("refreshes pending host turns without hiding the thread", async () => {
    listPublicAskTurnsMock.mockResolvedValue({
      data: [
        {
          id: "turn_pending",
          session_id: "sess1",
          question: "What was 2025 revenue?",
          lane: "host",
          status: "host_pending",
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      ],
    });
    await renderPanel();
    expect(await screen.findByText("What was 2025 revenue?")).toBeInTheDocument();
    expect(screen.getByText(/Awaiting reply/i)).toBeInTheDocument();
    const callsBefore = listPublicAskTurnsMock.mock.calls.length;
    fireEvent.click(screen.getByTestId("visitor-ask-refresh"));
    await waitFor(() => {
      expect(listPublicAskTurnsMock.mock.calls.length).toBeGreaterThan(callsBefore);
    });
    expect(screen.getByText("What was 2025 revenue?")).toBeInTheDocument();
    expect(screen.getByText(/Awaiting reply/i)).toBeInTheDocument();
  });

  it("sends on Enter and keeps Shift+Enter from submitting", async () => {
    await renderPanel();
    const composer = screen.getByPlaceholderText(/materials you can access/i);
    fireEvent.change(composer, { target: { value: "What was 2025 revenue?" } });
    fireEvent.keyDown(composer, { key: "Enter", shiftKey: true });
    expect(createPublicAskMock).not.toHaveBeenCalled();
    fireEvent.keyDown(composer, { key: "Enter" });
    await waitFor(() => {
      expect(createPublicAskMock).toHaveBeenCalledWith(
        "tok123",
        "What was 2025 revenue?",
        { sessionToken: "sess456" },
      );
    });
  });

  it("does not render pinned FAQ inside the Ask thread", async () => {
    listPublicAskFAQsMock.mockResolvedValue({
      data: [
        {
          id: "faq1",
          question: "What is the data room password policy?",
          answer: "Use your invite email to sign in.",
          source: "host",
          pinned_at: "2026-01-01T00:00:00Z",
        },
      ],
    });
    await renderPanel();
    expect(screen.queryByText(/Common questions/i)).not.toBeInTheDocument();
    expect(screen.queryByText("What is the data room password policy?")).not.toBeInTheDocument();
  });

  it("prefills pending questions without sending", async () => {
    await renderPanel({ pendingQuestion: "What is ARR?" });
    expect(screen.getByPlaceholderText(/materials you can access/i)).toHaveValue("What is ARR?");
    expect(createPublicAskMock).not.toHaveBeenCalled();
  });

  it("auto-submits pending Ask this questions", async () => {
    await renderPanel({ pendingQuestion: "What is the company burn rate?", pendingSubmit: true });
    await waitFor(() => {
      expect(createPublicAskMock).toHaveBeenCalledWith(
        "tok123",
        "What is the company burn rate?",
        { sessionToken: "sess456" },
      );
    });
  });
});
