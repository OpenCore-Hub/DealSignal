// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { UnifiedQAPanel } from "./UnifiedQAPanel";
import enDocuments from "@/i18n/locales/en/documents.json";

const { listPublicAskTurnsMock, createPublicAskMock } = vi.hoisted(() => ({
  listPublicAskTurnsMock: vi.fn(),
  createPublicAskMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    listPublicAskTurns: listPublicAskTurnsMock,
    createPublicAsk: createPublicAskMock,
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
    createPublicAskMock.mockReset();
    listPublicAskTurnsMock.mockResolvedValue({ data: [] });
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
    expect(screen.getByPlaceholderText(/Ask the host a question/i)).toBeInTheDocument();
  });

  it("submits unified Ask turn", async () => {
    await renderPanel();
    fireEvent.change(screen.getByPlaceholderText(/Ask the host a question/i), {
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
    fireEvent.change(screen.getByPlaceholderText(/Ask the host a question/i), {
      target: { value: "spam" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Ask/i }));
    await waitFor(() => {
      expect(screen.getByText(/Too many Ask Host questions/i)).toBeInTheDocument();
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
});
