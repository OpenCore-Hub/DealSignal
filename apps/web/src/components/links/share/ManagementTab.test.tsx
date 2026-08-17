// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { I18nextProvider } from "react-i18next";
import { ManagementTab } from "./ManagementTab";
import { createTestI18n } from "@/i18n/test-utils";
import enLinkShare from "@/i18n/locales/en/linkShare.json";
import { api } from "@/lib/api";
import type { FileRequest, OwnerAskTurn } from "@/types";

vi.mock("@/lib/api", () => ({
  api: {
    listLinkAsk: vi.fn(),
    answerAskTurn: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

async function renderTab(props: {
  fileRequests?: FileRequest[];
  onUpdateFileRequest?: (id: string, status: string) => Promise<void>;
}) {
  const i18n = await createTestI18n({
    linkShare: enLinkShare as unknown as Record<string, string>,
  });
  return render(
    <MemoryRouter initialEntries={["/acme-capital/links/link_1"]}>
      <I18nextProvider i18n={i18n}>
        <ManagementTab
          linkId="link_1"
          dealRoomId="room_1"
          canManageAsk
          fileRequests={props.fileRequests ?? []}
          onUpdateFileRequest={props.onUpdateFileRequest ?? vi.fn()}
        />
      </I18nextProvider>
    </MemoryRouter>,
  );
}

function makeFileRequest(overrides: Partial<FileRequest> = {}): FileRequest {
  return {
    id: "fr1",
    link_id: "l1",
    visitor_id: "v1",
    visitor_email: "visitor@example.com",
    message: "Please send the full report.",
    status: "pending",
    created_at: "2026-07-11T10:00:00Z",
    updated_at: "2026-07-11T10:00:00Z",
    ...overrides,
  };
}

describe("ManagementTab", () => {
  beforeEach(() => {
    vi.mocked(api.listLinkAsk).mockReset();
    vi.mocked(api.answerAskTurn).mockReset();
  });

  it("labels Ask inbox separately from audit and Signal", async () => {
    vi.mocked(api.listLinkAsk).mockResolvedValue({ data: [] });
    await renderTab({});
    expect(screen.queryByText("Grounded AI answers")).not.toBeInTheDocument();
    expect(screen.getByText("Ask inbox")).toBeInTheDocument();
    expect(screen.getByText(/not the Signal inbox/i)).toBeInTheDocument();
    expect(await screen.findByText(/No Ask questions yet/i)).toBeInTheDocument();
  });

  it("renders ask turns and file requests", async () => {
    const pending: OwnerAskTurn = {
      id: "turn-1",
      session_id: "sess-1",
      link_id: "link_1",
      visitor_id: "v1",
      visitor_email: "visitor@example.com",
      question: "What is the pricing?",
      lane: "host",
      status: "host_pending",
      created_at: "2026-07-11T10:00:00Z",
      updated_at: "2026-07-11T10:00:00Z",
    };
    vi.mocked(api.listLinkAsk).mockResolvedValue({ data: [pending] });

    await renderTab({ fileRequests: [makeFileRequest()] });

    expect(await screen.findByText("What is the pricing?")).toBeInTheDocument();
    expect(screen.getByText("Please send the full report.")).toBeInTheDocument();
  });

  it("submits a host answer via unified ask API", async () => {
    const pending: OwnerAskTurn = {
      id: "turn-1",
      session_id: "sess-1",
      link_id: "link_1",
      visitor_id: "v1",
      visitor_email: "visitor@example.com",
      question: "What is the pricing?",
      lane: "host",
      status: "host_pending",
      created_at: "2026-07-11T10:00:00Z",
      updated_at: "2026-07-11T10:00:00Z",
    };
    vi.mocked(api.listLinkAsk).mockResolvedValue({ data: [pending] });
    vi.mocked(api.answerAskTurn).mockResolvedValue({
      data: {
        ...pending,
        status: "host_answered",
        host_answer: "Pricing starts at $99.",
        updated_at: "2026-07-11T11:00:00Z",
      },
    });

    await renderTab({});

    const textarea = await screen.findByPlaceholderText("Type your answer...");
    fireEvent.change(textarea, { target: { value: "Pricing starts at $99." } });
    fireEvent.click(screen.getByText("Send answer"));

    await waitFor(() => {
      expect(api.answerAskTurn).toHaveBeenCalledWith(
        "link_1",
        "turn-1",
        "Pricing starts at $99.",
      );
    });
  });
});
