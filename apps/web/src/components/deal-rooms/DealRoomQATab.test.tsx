// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { DealRoomQATab } from "./DealRoomQATab";
import { createTestI18n } from "@/i18n/test-utils";
import enDealRooms from "@/i18n/locales/en/dealRooms.json";
import { api } from "@/lib/api";
import type { Link, VisitorQuestion } from "@/types";

vi.mock("@/lib/api", () => ({
  api: {
    listRoomQuestions: vi.fn(),
    getDealRoomLinks: vi.fn(),
    answerQuestion: vi.fn(),
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
    <I18nextProvider i18n={i18n}>
      <DealRoomQATab roomId="room_1" />
    </I18nextProvider>,
  );
}

describe("DealRoomQATab", () => {
  beforeEach(() => {
    vi.mocked(api.listRoomQuestions).mockReset();
    vi.mocked(api.getDealRoomLinks).mockReset();
    vi.mocked(api.answerQuestion).mockReset();
  });

  it("loads room Ask Host questions and answers without fake seed data", async () => {
    const pending: VisitorQuestion = {
      id: "q1",
      link_id: "link_1",
      visitor_id: "v1",
      visitor_email: "lp@example.com",
      question: "Can you share the updated financial model?",
      status: "pending",
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

    vi.mocked(api.listRoomQuestions).mockResolvedValue({ data: [pending] });
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [link] });
    vi.mocked(api.answerQuestion).mockResolvedValue({
      data: {
        ...pending,
        answer: "Attached in the data room.",
        status: "answered",
        updated_at: "2026-07-20T11:00:00.000Z",
      },
    });

    await renderTab();

    expect(await screen.findByText("Ask Host inbox")).toBeInTheDocument();
    expect(screen.queryByText("When is the next board meeting?")).not.toBeInTheDocument();
    expect(screen.getByText("Can you share the updated financial model?")).toBeInTheDocument();
    expect(screen.getByText("lp@example.com")).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText(/Type your answer/i), {
      target: { value: "Attached in the data room." },
    });
    fireEvent.click(screen.getByRole("button", { name: /Send answer/i }));

    await waitFor(() => {
      expect(api.answerQuestion).toHaveBeenCalledWith(
        "link_1",
        "q1",
        "Attached in the data room.",
      );
    });
    expect(await screen.findByText(/Attached in the data room/i)).toBeInTheDocument();
  });

  it("shows empty state when there are no questions", async () => {
    vi.mocked(api.listRoomQuestions).mockResolvedValue({ data: [] });
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [] });

    await renderTab();

    expect(await screen.findByText(/No Ask Host questions yet/i)).toBeInTheDocument();
    expect(screen.queryByText("When is the next board meeting?")).not.toBeInTheDocument();
  });
});
