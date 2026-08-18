// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { VisitorWorkspacePanel } from "./VisitorWorkspacePanel";
import enDocuments from "@/i18n/locales/en/documents.json";

const { listPublicAskTurnsMock, listPublicAskFAQsMock } = vi.hoisted(() => ({
  listPublicAskTurnsMock: vi.fn(),
  listPublicAskFAQsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    listPublicAskTurns: listPublicAskTurnsMock,
    listPublicAskFAQs: listPublicAskFAQsMock,
  },
}));

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: { en: { documents: enDocuments } },
  interpolation: { escapeValue: false },
});

function renderPanel(props: Partial<React.ComponentProps<typeof VisitorWorkspacePanel>> = {}) {
  return render(
    <I18nextProvider i18n={i18nInstance}>
      <VisitorWorkspacePanel
        open
        documents={[{ id: "doc-1", title: "Deck", pageCount: 12 }]}
        publicToken="tok-room"
        publicSessionToken="sess-room"
        {...props}
      />
    </I18nextProvider>,
  );
}

describe("VisitorWorkspacePanel", () => {
  beforeEach(() => {
    listPublicAskTurnsMock.mockReset();
    listPublicAskTurnsMock.mockResolvedValue({ data: [] });
    listPublicAskFAQsMock.mockReset();
    listPublicAskFAQsMock.mockResolvedValue({ data: [] });
  });

  it("renders Ask Host for single-doc deal-room links with qa enabled", async () => {
    renderPanel({ qaEnabled: true, fileRequestsEnabled: false });

    await waitFor(() => {
      expect(listPublicAskTurnsMock).toHaveBeenCalledWith(
        "tok-room",
        expect.objectContaining({ sessionToken: "sess-room" }),
      );
    });
    expect(screen.getByPlaceholderText(/Ask about the materials you can access/i)).toBeInTheDocument();
    expect(screen.queryByText("Documents")).not.toBeInTheDocument();
  });

  it("does not render Ask Host when qa is disabled (document-only links)", () => {
    renderPanel({ qaEnabled: false, fileRequestsEnabled: false });
    expect(listPublicAskTurnsMock).not.toHaveBeenCalled();
    expect(screen.queryByPlaceholderText(/Ask about the materials you can access/i)).not.toBeInTheDocument();
  });

  it("shows document and Ask tabs for multi-doc deal-room links", async () => {
    renderPanel({
      qaEnabled: true,
      documents: [
        { id: "doc-1", title: "Deck", pageCount: 12 },
        { id: "doc-2", title: "Financials", pageCount: 8 },
      ],
    });

    expect(screen.getByRole("button", { name: "Documents" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ask" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Ask" }));
    await waitFor(() => {
      expect(listPublicAskTurnsMock).toHaveBeenCalled();
    });
  });

  it("shows FAQ tab to the right of Ask when pins exist", async () => {
    listPublicAskFAQsMock.mockResolvedValue({
      data: [
        {
          id: "faq1",
          question: "What is ARR?",
          answer: "Twelve million.",
          source: "host",
          pinned_at: "2026-01-01T00:00:00Z",
        },
      ],
    });
    renderPanel({
      qaEnabled: true,
      documents: [
        { id: "doc-1", title: "Deck", pageCount: 12 },
        { id: "doc-2", title: "Financials", pageCount: 8 },
      ],
    });

    expect(await screen.findByRole("button", { name: "FAQ" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "FAQ" }));
    expect(await screen.findByPlaceholderText(/Search common questions/i)).toBeInTheDocument();
    expect(screen.getByText("What is ARR?")).toBeInTheDocument();
  });
});
