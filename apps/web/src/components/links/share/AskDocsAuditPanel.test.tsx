// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { AskDocsAuditPanel } from "./AskDocsAuditPanel";
import { createTestI18n } from "@/i18n/test-utils";
import { ApiError } from "@/lib/apiClient";
import type { AskDocsAuditDetail, AskDocsAuditEntry } from "@/types";

const {
  listLinkAskDocsAuditMock,
  getLinkAskDocsAuditMock,
  listRoomAskDocsAuditMock,
} = vi.hoisted(() => ({
  listLinkAskDocsAuditMock: vi.fn(),
  getLinkAskDocsAuditMock: vi.fn(),
  listRoomAskDocsAuditMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    listLinkAskDocsAudit: listLinkAskDocsAuditMock,
    getLinkAskDocsAudit: getLinkAskDocsAuditMock,
    listRoomAskDocsAudit: listRoomAskDocsAuditMock,
  },
}));

const auditI18n = {
  "askDocsAudit.title": "Ask Docs audit",
  "askDocsAudit.description":
    "Read-only ledger of visitor Ask Docs conversations. This is not the Signal inbox.",
  "askDocsAudit.roomTitle": "Ask Docs timeline",
  "askDocsAudit.roomDescription":
    "Room-wide Ask Docs audit. Filter by link. This is not the Signal inbox.",
  "askDocsAudit.loading": "Loading Ask Docs audit...",
  "askDocsAudit.loadFailed": "Failed to load Ask Docs audit",
  "askDocsAudit.forbidden": "You do not have permission to view Ask Docs audit.",
  "askDocsAudit.empty": "No Ask Docs sessions in the last 90 days.",
  "askDocsAudit.emptyArchived": "No Ask Docs sessions found.",
  "askDocsAudit.showArchived": "Include archived (older than 90 days)",
  "askDocsAudit.hotWindowHint": "Showing the last 90 days by default.",
  "askDocsAudit.channel": "Ask Docs",
  "askDocsAudit.evidenceCount": "{{count}} citations",
  "askDocsAudit.archivedBadge": "Archived",
  "askDocsAudit.filterAllLinks": "All links",
  "askDocsAudit.filterByLink": "Filter by link",
  "askDocsAudit.anonymous": "Anonymous visitor",
  "askDocsAudit.detailTitle": "Session detail",
  "askDocsAudit.detailBack": "Back to list",
  "askDocsAudit.detailLoading": "Loading session...",
  "askDocsAudit.detailFailed": "Failed to load session detail",
  "askDocsAudit.messages": "Conversation",
  "askDocsAudit.evidence": "Evidence",
  "askDocsAudit.noEvidence": "No evidence recorded",
  "askDocsAudit.resultStatus": "Result",
  "askDocsAudit.resultStatuses.success": "Answered",
  "askDocsAudit.resultStatuses.no_evidence": "No evidence",
  "askDocsAudit.resultStatuses.kb_unavailable": "Knowledge base unavailable",
  "askDocsAudit.resultStatuses.scope_violation": "Scope violation",
};

function makeEntry(overrides: Partial<AskDocsAuditEntry> = {}): AskDocsAuditEntry {
  return {
    session_id: "sess-1",
    link_id: "link-1",
    visitor_id: "visitor@example.com",
    question_preview: "What is the valuation?",
    result_status: "success",
    evidence_count: 2,
    created_at: "2026-07-18T10:00:00Z",
    archived: false,
    ...overrides,
  };
}

function makeDetail(overrides: Partial<AskDocsAuditDetail> = {}): AskDocsAuditDetail {
  return {
    session_id: "sess-1",
    visitor_id: "visitor@example.com",
    created_at: "2026-07-18T10:00:00Z",
    archived: false,
    messages: [
      {
        role: "user",
        content: "What is the valuation?",
        created_at: "2026-07-18T10:00:00Z",
      },
      {
        role: "assistant",
        content: "Based on the memo, Series A is $40M.",
        created_at: "2026-07-18T10:00:01Z",
      },
    ],
    authorized_document_ids: ["doc-1"],
    retrieval_document_ids: ["doc-1"],
    evidence: [
      {
        chunk_id: "c1",
        document_id: "doc-1",
        quote: "Series A valuation $40M",
        page_number: 3,
        boxes: [],
        score: 0.9,
      },
    ],
    result_status: "success",
    ...overrides,
  };
}

async function renderPanel(
  props:
    | { mode: "link"; linkId: string }
    | {
        mode: "room";
        roomId: string;
        links?: Array<{ id: string; name?: string }>;
      },
) {
  const i18n = await createTestI18n({ linkShare: auditI18n });
  return render(
    <I18nextProvider i18n={i18n}>
      <AskDocsAuditPanel {...props} />
    </I18nextProvider>,
  );
}

describe("AskDocsAuditPanel — link mode", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listLinkAskDocsAuditMock.mockResolvedValue({ data: [makeEntry()] });
    getLinkAskDocsAuditMock.mockResolvedValue(makeDetail());
  });

  it("lists Ask Docs audit rows from the link API (default excludes archived query)", async () => {
    await renderPanel({ mode: "link", linkId: "link-1" });

    await waitFor(() => {
      expect(listLinkAskDocsAuditMock).toHaveBeenCalledWith("link-1", { archived: false });
    });
    expect(await screen.findByText(/What is the valuation\?/)).toBeInTheDocument();
    expect(screen.getByText("Ask Docs")).toBeInTheDocument();
    expect(screen.getByText(/2 citations/)).toBeInTheDocument();
    expect(screen.getByText(/not the Signal inbox/i)).toBeInTheDocument();
  });

  it("opens session detail from the link audit API", async () => {
    await renderPanel({ mode: "link", linkId: "link-1" });
    await screen.findByText(/What is the valuation\?/);

    fireEvent.click(screen.getByRole("button", { name: /What is the valuation/i }));

    await waitFor(() => {
      expect(getLinkAskDocsAuditMock).toHaveBeenCalledWith("link-1", "sess-1");
    });
    expect(
      await screen.findByText("Based on the memo, Series A is $40M."),
    ).toBeInTheDocument();
    expect(screen.getByText("Series A valuation $40M")).toBeInTheDocument();
  });

  it("shows empty state when there are no sessions", async () => {
    listLinkAskDocsAuditMock.mockResolvedValue({ data: [] });
    await renderPanel({ mode: "link", linkId: "link-1" });

    expect(
      await screen.findByText("No Ask Docs sessions in the last 90 days."),
    ).toBeInTheDocument();
  });

  it("shows forbidden state on 403", async () => {
    listLinkAskDocsAuditMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "forbidden",
        message: "ask docs audit forbidden",
        requestId: "r1",
      }),
    );
    await renderPanel({ mode: "link", linkId: "link-1" });

    expect(
      await screen.findByText("You do not have permission to view Ask Docs audit."),
    ).toBeInTheDocument();
  });

  it("refetches with archived=true when include-archived is toggled", async () => {
    await renderPanel({ mode: "link", linkId: "link-1" });
    await screen.findByText(/What is the valuation\?/);

    listLinkAskDocsAuditMock.mockResolvedValue({
      data: [makeEntry({ session_id: "sess-old", archived: true, question_preview: "Old Q" })],
    });

    fireEvent.click(screen.getByLabelText(/Include archived/i));

    await waitFor(() => {
      expect(listLinkAskDocsAuditMock).toHaveBeenLastCalledWith("link-1", { archived: true });
    });
    expect(await screen.findByText(/Old Q/)).toBeInTheDocument();
  });
});

describe("AskDocsAuditPanel — room mode", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listRoomAskDocsAuditMock.mockResolvedValue({
      data: [
        makeEntry({ session_id: "s-a", link_id: "link-a", question_preview: "Room Q A" }),
        makeEntry({ session_id: "s-b", link_id: "link-b", question_preview: "Room Q B" }),
      ],
    });
    getLinkAskDocsAuditMock.mockResolvedValue(makeDetail());
  });

  it("lists room timeline and filters by link_id", async () => {
    await renderPanel({
      mode: "room",
      roomId: "room-1",
      links: [
        { id: "link-a", name: "Investor link" },
        { id: "link-b", name: "LP link" },
      ],
    });

    await waitFor(() => {
      expect(listRoomAskDocsAuditMock).toHaveBeenCalledWith("room-1", {
        archived: false,
        linkId: undefined,
      });
    });
    expect(await screen.findByText(/Room Q A/)).toBeInTheDocument();
    expect(screen.getByText(/Room Q B/)).toBeInTheDocument();
    expect(screen.getByText("Ask Docs timeline")).toBeInTheDocument();

    listRoomAskDocsAuditMock.mockResolvedValue({
      data: [makeEntry({ session_id: "s-a", link_id: "link-a", question_preview: "Room Q A" })],
    });

    fireEvent.change(screen.getByLabelText(/Filter by link/i), {
      target: { value: "link-a" },
    });

    await waitFor(() => {
      expect(listRoomAskDocsAuditMock).toHaveBeenLastCalledWith("room-1", {
        archived: false,
        linkId: "link-a",
      });
    });
  });

  it("loads detail via the entry link_id", async () => {
    await renderPanel({
      mode: "room",
      roomId: "room-1",
      links: [{ id: "link-a", name: "Investor link" }],
    });
    await screen.findByText(/Room Q A/);

    fireEvent.click(screen.getByRole("button", { name: /Room Q A/i }));

    await waitFor(() => {
      expect(getLinkAskDocsAuditMock).toHaveBeenCalledWith("link-a", "s-a");
    });
  });
});
