// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { VisitorWorkspacePanel } from "./VisitorWorkspacePanel";
import enDocuments from "@/i18n/locales/en/documents.json";

const { listPublicQuestionsMock } = vi.hoisted(() => ({
  listPublicQuestionsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    listPublicQuestions: listPublicQuestionsMock,
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
        onClose={() => undefined}
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
    listPublicQuestionsMock.mockReset();
    listPublicQuestionsMock.mockResolvedValue({ data: [] });
  });

  it("renders Ask Host for single-doc deal-room links with qa enabled", async () => {
    renderPanel({ qaEnabled: true, fileRequestsEnabled: false });

    await waitFor(() => {
      expect(listPublicQuestionsMock).toHaveBeenCalledWith(
        "tok-room",
        expect.objectContaining({ sessionToken: "sess-room" }),
      );
    });
    expect(screen.getByPlaceholderText(/Ask the host a question/i)).toBeInTheDocument();
    expect(screen.queryByText("Documents")).not.toBeInTheDocument();
  });

  it("does not render Ask Host when qa is disabled (document-only links)", () => {
    renderPanel({ qaEnabled: false, fileRequestsEnabled: false });
    expect(listPublicQuestionsMock).not.toHaveBeenCalled();
    expect(screen.queryByPlaceholderText(/Ask the host a question/i)).not.toBeInTheDocument();
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
      expect(listPublicQuestionsMock).toHaveBeenCalled();
    });
  });
});
