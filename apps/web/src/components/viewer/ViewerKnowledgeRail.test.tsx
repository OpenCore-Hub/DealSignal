// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { createTestI18n } from "@/i18n/test-utils";
import { ViewerKnowledgeRail } from "./ViewerKnowledgeRail";

const getActive = vi.fn();
const stream = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    getActiveDealRoomKnowledgeSession: (...args: unknown[]) => getActive(...args),
    streamDealRoomKnowledgeSession: (...args: unknown[]) => stream(...args),
    upsertDealRoomKnowledgeTurnFeedback: vi.fn(),
    recordDealRoomKnowledgeDeskEvent: vi.fn(() => Promise.resolve()),
  },
}));

vi.mock("@/components/deal-rooms/knowledge/GroundedChatShell", () => ({
  GroundedChatShell: () => <div data-testid="grounded-chat-shell-stub" />,
}));

describe("ViewerKnowledgeRail", () => {
  beforeEach(() => {
    getActive.mockReset();
    stream.mockReset();
    getActive.mockResolvedValue({ session: null, turns: [] });
  });

  it("renders trust chip and shell after hydrate", async () => {
    const i18n = await createTestI18n({
      documents: {
        "viewer.knowledgeRailTitle": "Room knowledge",
        "viewer.knowledgeRailLoading": "Loading session…",
        "viewer.sidebarClose": "Close",
      },
      dealRooms: {
        "knowledge.trustScoped": "Research desk",
      },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <MemoryRouter>
          <ViewerKnowledgeRail
            roomId="room-1"
            documentId="doc-1"
            onJumpToPage={vi.fn()}
            onClose={vi.fn()}
          />
        </MemoryRouter>
      </I18nextProvider>,
    );
    await waitFor(() => {
      expect(screen.getByTestId("viewer-knowledge-rail")).toBeInTheDocument();
    });
    expect(screen.getByTestId("viewer-knowledge-trust-chip")).toHaveTextContent(
      "Research desk",
    );
    await waitFor(() => {
      expect(screen.getByTestId("grounded-chat-shell-stub")).toBeInTheDocument();
    });
  });
});
