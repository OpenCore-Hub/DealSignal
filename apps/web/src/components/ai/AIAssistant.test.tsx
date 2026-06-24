// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import i18next from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { MemoryRouter, useLocation } from "react-router";
import { AIAssistant } from "./AIAssistant";
import enAi from "@/i18n/locales/en/ai.json";
import { useAIStore } from "@/stores/aiStore";
import type { Evidence } from "@/types";

const demoEvidence: Evidence[] = [
  {
    chunk_id: "chk_demo_001",
    quote: "Revenue grew 3x year over year.",
    page_number: 2,
    boxes: [{ x: 0.12, y: 0.34, w: 0.45, h: 0.06 }],
    score: 0.92,
  },
];

const {
  apiMock,
  resolveAssistantChat,
  resolveSearch,
} = vi.hoisted(() => {
  let chatResolve: (value: {
    session_id: string;
    answer: string;
    evidence?: Evidence[];
    follow_up_questions?: string[];
  }) => void = () => {};
  let searchResolve: (value: {
    document_id?: string;
    query: string;
    results: Evidence[];
  }) => void = () => {};
  return {
    apiMock: {
      assistantChat: vi.fn(
        () =>
          new Promise<{
            session_id: string;
            answer: string;
            evidence?: Evidence[];
            follow_up_questions?: string[];
          }>((r) => { chatResolve = r; })
      ),
      searchDocument: vi.fn(
        () =>
          new Promise<{
            document_id?: string;
            query: string;
            results: Evidence[];
          }>((r) => { searchResolve = r; })
      ),
    },
    resolveAssistantChat: (
      value: {
        session_id: string;
        answer: string;
        evidence?: Evidence[];
        follow_up_questions?: string[];
      }
    ) => chatResolve(value),
    resolveSearch: (
      value: {
        document_id?: string;
        query: string;
        results: Evidence[];
      }
    ) => searchResolve(value),
  };
});

vi.mock("@/lib/api", () => ({ api: apiMock }));

function LocationDisplay() {
  const location = useLocation();
  return (
    <div data-testid="location">
      {location.pathname}
      {location.search}
    </div>
  );
}

async function createAiI18n() {
  const instance = i18next.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["ai"],
    defaultNS: "ai",
    resources: { en: { ai: enAi } },
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderWithProviders(initialRoute = "/documents/doc-001") {
  const i18n = await createAiI18n();
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[initialRoute]}>
        <AIAssistant />
        <LocationDisplay />
      </MemoryRouter>
    </I18nextProvider>
  );
}

async function sendSearchResponse() {
  await act(async () => {
    resolveSearch({
      document_id: "doc-001",
      query: "What is the revenue trend?",
      results: demoEvidence,
    });
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

async function sendAssistantResponse() {
  await act(async () => {
    resolveAssistantChat({
      session_id: "sess_001",
      answer: "Revenue grew 3x based on the latest report.",
      evidence: demoEvidence,
      follow_up_questions: ["What drove the growth?"],
    });
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

Element.prototype.scrollIntoView = vi.fn();

describe("AIAssistant", () => {
  beforeEach(() => {
    apiMock.assistantChat.mockClear();
    apiMock.searchDocument.mockClear();
    useAIStore.getState().reset();
  });

  it("opens the assistant and searches current document", async () => {
    await renderWithProviders();

    fireEvent.click(screen.getByRole("button", { name: /Open AI assistant/i }));
    const input = await screen.findByPlaceholderText(/Ask about signals/i);
    fireEvent.change(input, { target: { value: "What is the revenue trend?" } });
    fireEvent.submit(input.closest("form")!);

    expect(apiMock.searchDocument).toHaveBeenCalledWith(
      expect.objectContaining({ query: "What is the revenue trend?", document_id: "doc-001" })
    );
    expect(apiMock.assistantChat).not.toHaveBeenCalled();

    await sendSearchResponse();

    expect(await screen.findByText((content) => content.includes("Here are the most relevant passages"))).toBeInTheDocument();
    expect(screen.getByText(/Revenue grew 3x year over year/i)).toBeInTheDocument();
  });

  it("falls back to assistant chat when no document context", async () => {
    await renderWithProviders("/dashboard");

    fireEvent.click(screen.getByRole("button", { name: /Open AI assistant/i }));
    const input = await screen.findByPlaceholderText(/Ask about signals/i);
    fireEvent.change(input, { target: { value: "What are today’s signals?" } });
    fireEvent.submit(input.closest("form")!);

    expect(apiMock.assistantChat).toHaveBeenCalledWith(
      expect.objectContaining({ message: "What are today’s signals?" })
    );
    expect(apiMock.searchDocument).not.toHaveBeenCalled();

    await sendAssistantResponse();

    expect(await screen.findByText("Revenue grew 3x based on the latest report.")).toBeInTheDocument();
  });

  it("navigates to viewer page when evidence is clicked", async () => {
    await renderWithProviders();

    fireEvent.click(screen.getByRole("button", { name: /Open AI assistant/i }));
    const input = await screen.findByPlaceholderText(/Ask about signals/i);
    fireEvent.change(input, { target: { value: "Show evidence" } });
    fireEvent.submit(input.closest("form")!);

    await sendSearchResponse();

    const evidenceCard = await screen.findByText(/Revenue grew 3x year over year/i);
    fireEvent.click(evidenceCard.closest("button")!);

    await waitFor(() => {
      expect(screen.getByTestId("location").textContent).toBe("/viewer/doc-001?page=2");
    });
  });

  it("resets conversation", async () => {
    await renderWithProviders();

    fireEvent.click(screen.getByRole("button", { name: /Open AI assistant/i }));
    const input = await screen.findByPlaceholderText(/Ask about signals/i);
    fireEvent.change(input, { target: { value: "Hello" } });
    fireEvent.submit(input.closest("form")!);

    await sendSearchResponse();
    await screen.findByText((content) => content.includes("Here are the most relevant passages"));

    fireEvent.click(screen.getByRole("button", { name: /Reset conversation/i }));
    fireEvent.click(screen.getByRole("button", { name: /Reset/i }));

    await waitFor(() => {
      expect(screen.queryByText(/Here are the most relevant passages/i)).not.toBeInTheDocument();
    });
  });
});
