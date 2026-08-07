// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { I18nextProvider } from "react-i18next";
import { DealRoomQATab } from "./DealRoomQATab";
import { createTestI18n } from "@/i18n/test-utils";
import enDealRooms from "@/i18n/locales/en/dealRooms.json";
import { api } from "@/lib/api";
import type { Link, OwnerAskTurn } from "@/types";

vi.mock("@/lib/api", () => ({
  api: {
    listRoomAsk: vi.fn(),
    getDealRoomLinks: vi.fn(),
    answerQuestion: vi.fn(),
    answerAskTurn: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

async function renderTab() {
  const i18n = await createTestI18n({
    dealRooms: enDealRooms as unknown as Record<string, string>,
  });
  return render(
    <MemoryRouter initialEntries={["/acme-capital/deal-rooms/room_1?tab=qa"]}>
      <I18nextProvider i18n={i18n}>
        <DealRoomQATab roomId="room_1" />
      </I18nextProvider>
    </MemoryRouter>,
  );
}

describe("DealRoomQATab", () => {
  beforeEach(() => {
    vi.mocked(api.listRoomAsk).mockReset();
    vi.mocked(api.getDealRoomLinks).mockReset();
    vi.mocked(api.answerQuestion).mockReset();
    vi.mocked(api.answerAskTurn).mockReset();
  });

  it("loads needs-host turns and answers without fake seed data", async () => {
    const pending: OwnerAskTurn = {
      id: "turn-1",
      session_id: "sess-1",
      link_id: "link_1",
      visitor_id: "v1",
      visitor_email: "lp@example.com",
      question: "Can you share the updated financial model?",
      lane: "host",
      status: "host_pending",
      created_at: "2026-07-20T10:00:00.000Z",
      updated_at: "2026-07-20T10:00:00.000Z",
    };
    const link: Link = {
      id: "link_1",
      documentId: "doc_1",
      documentIds: ["doc_1"],
      folderPaths: [],
      documentTitle: "Pitch Deck",
      name: "Series A link",
      shortUrl: "https://example.com/d/x",
      accessCount: 1,
      heatLevel: "warm",
      createdAt: "2026-07-01T00:00:00.000Z",
      isBundle: false,
      documents: [],
      dealRoomId: "room_1",
    };

    vi.mocked(api.listRoomAsk).mockResolvedValue({ data: [pending] });
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [link] });
    vi.mocked(api.answerAskTurn).mockResolvedValue({
      data: {
        ...pending,
        status: "host_answered",
        host_answer: "Attached in the data room.",
        updated_at: "2026-07-20T11:00:00.000Z",
      },
    });

    await renderTab();

    expect(await screen.findByText("Ask inbox")).toBeInTheDocument();
    expect(api.listRoomAsk).toHaveBeenCalledWith("room_1", {
      lane: "host",
      status: "host_pending",
    });
    expect(await screen.findByText("Can you share the updated financial model?")).toBeInTheDocument();
    expect(screen.getByText("lp@example.com")).toBeInTheDocument();

    const textarea = await screen.findByPlaceholderText(/Type your answer/i);
    fireEvent.change(textarea, {
      target: { value: "Attached in the data room." },
    });
    fireEvent.click(await screen.findByRole("button", { name: /Send answer/i }));

    await waitFor(() => {
      expect(api.answerAskTurn).toHaveBeenCalledWith(
        "link_1",
        "turn-1",
        "Attached in the data room.",
      );
    });
    expect(await screen.findByText(/Attached in the data room/i)).toBeInTheDocument();
  });

  it("shows AI handled turns when switching inbox tab", async () => {
    const aiTurn: OwnerAskTurn = {
      id: "turn-ai-1",
      session_id: "sess-ai",
      link_id: "link_1",
      visitor_id: "v2",
      visitor_email: "analyst@example.com",
      question: "What was revenue growth?",
      lane: "ai",
      status: "ai_answered",
      ai_payload: {
        answer: "Revenue grew 12% year over year [1].",
        refused: false,
        resultStatus: "answered",
        hits: [
          {
            chunkId: "chunk-1",
            documentId: "doc_1",
            text: "Revenue increased 12% YoY.",
            score: 0.9,
            sourceName: "Financial Summary.pdf",
            pages: [3],
            viewerPage: 3,
          },
        ],
      },
      created_at: "2026-07-20T10:00:00.000Z",
      updated_at: "2026-07-20T10:00:00.000Z",
    };

    vi.mocked(api.listRoomAsk).mockImplementation((_roomId, params) => {
      if (params?.lane === "ai" && params?.status === "ai_answered") {
        return Promise.resolve({ data: [aiTurn] });
      }
      return Promise.resolve({ data: [] });
    });
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [] });

    await renderTab();

    fireEvent.click(await screen.findByRole("tab", { name: /AI handled/i }));

    expect(await screen.findByText("What was revenue growth?")).toBeInTheDocument();
    expect(screen.getByText(/Revenue grew 12%/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Open page 3/i })).toBeInTheDocument();
    expect(api.listRoomAsk).toHaveBeenCalledWith("room_1", {
      lane: "ai",
      status: "ai_answered",
    });
  });

  it("shows empty state when there are no questions", async () => {
    vi.mocked(api.listRoomAsk).mockResolvedValue({ data: [] });
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [] });

    await renderTab();

    expect(await screen.findByText(/No Ask questions yet/i)).toBeInTheDocument();
  });
});
