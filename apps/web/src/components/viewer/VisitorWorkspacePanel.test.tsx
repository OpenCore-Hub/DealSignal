// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { VisitorWorkspacePanel } from "./VisitorWorkspacePanel";
import enDocuments from "@/i18n/locales/en/documents.json";

const { listPublicAskTurnsMock } = vi.hoisted(() => ({
  listPublicAskTurnsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    listPublicAskTurns: listPublicAskTurnsMock,
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
});
