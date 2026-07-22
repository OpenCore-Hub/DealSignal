// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { AccessTab } from "./AccessTab";
import type { DraftLink } from "./types";
import enLinkShare from "@/i18n/locales/en/linkShare.json";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      linkShare: enLinkShare,
      common: { loading: "Loading..." },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

function renderAccessTab(
  draft: DraftLink,
  errors: Record<string, string> = {},
  isDealRoomLink = true,
  documents: { id: string; title: string }[] = [],
  passwordAlreadySet = false,
  extras: {
    knowledgeBaseStatus?: import("@/types").DealRoomKnowledgeBaseStatus | null;
    knowledgeBaseHref?: string;
  } = {}
) {
  const updateDraft = vi.fn();
  const { rerender } = render(
    <Wrapper>
      <AccessTab
        draft={draft}
        updateDraft={updateDraft}
        errors={errors}
        isDealRoomLink={isDealRoomLink}
        documents={documents}
        passwordAlreadySet={passwordAlreadySet}
        knowledgeBaseStatus={extras.knowledgeBaseStatus}
        knowledgeBaseHref={extras.knowledgeBaseHref}
      />
    </Wrapper>
  );
  return { updateDraft, rerender };
}

const baseDraft: DraftLink = {
  name: "",
  expiresAt: "",
  requireEmail: false,
  requireEmailVerification: false,
  requirePassword: false,
  password: "",
  watermarkEnabled: false,
  requireNda: false,
  ndaDocumentId: "",
  ndaTemplateId: "",
  allowDownloading: false,
  aiCopilotEnabled: false,
  enableScreenshotProtection: false,
  enableFileRequests: false,
  enableIndexFileGeneration: false,
  enableQaConversations: false,
  allowedViewers: [],
  blockedViewers: [],
  customDomain: "",
  notifyOnAccess: false,
  folderPaths: [],
  folderScopeMode: "allowlist",
  contactIds: [],
};

describe("AccessTab", () => {
  it("toggles require email and clears verification", () => {
    const { updateDraft } = renderAccessTab(baseDraft);
    fireEvent.click(screen.getByRole("switch", { name: /require email to view/i }));
    expect(updateDraft).toHaveBeenCalledWith({ requireEmail: true, requireEmailVerification: false });
  });

  it("toggles verification mutually exclusive with email", () => {
    const { updateDraft } = renderAccessTab(baseDraft);
    fireEvent.click(screen.getByRole("switch", { name: /require email verification/i }));
    expect(updateDraft).toHaveBeenCalledWith({ requireEmailVerification: true, requireEmail: false });
  });

  it("enabling email turns off verification", () => {
    const { updateDraft } = renderAccessTab({
      ...baseDraft,
      requireEmail: false,
      requireEmailVerification: true,
    });
    fireEvent.click(screen.getByRole("switch", { name: /require email to view/i }));
    expect(updateDraft).toHaveBeenCalledWith({ requireEmail: true, requireEmailVerification: false });
  });

  it("disabling email clears email without keeping verification", () => {
    const { updateDraft } = renderAccessTab({ ...baseDraft, requireEmail: true, requireEmailVerification: false });
    fireEvent.click(screen.getByRole("switch", { name: /require email to view/i }));
    expect(updateDraft).toHaveBeenCalledWith({ requireEmail: false, requireEmailVerification: false });
  });

  it("shows email identity mutual-exclusion hint", () => {
    renderAccessTab(baseDraft);
    expect(
      screen.getByText(/Choose one: ask visitors to enter an email, or verify with a one-time code/i)
    ).toBeInTheDocument();
  });

  it("disables verification toggle for non-deal-room links", () => {
    renderAccessTab(baseDraft, {}, false);
    expect(screen.getByRole("switch", { name: /require email verification/i })).toBeDisabled();
  });

  it("shows password input when password switch is on", () => {
    const { updateDraft, rerender } = renderAccessTab(baseDraft);
    fireEvent.click(screen.getByRole("switch", { name: /require password to view/i }));
    expect(updateDraft).toHaveBeenCalledWith({ requirePassword: true });

    rerender(
      <Wrapper>
        <AccessTab
          draft={{ ...baseDraft, requirePassword: true }}
          updateDraft={updateDraft}
          errors={{}}
          isDealRoomLink={true}
        />
      </Wrapper>
    );
    expect(screen.getByPlaceholderText(/enter password/i)).toBeInTheDocument();
  });

  it("masks a stored password and keeps the field password-only", () => {
    renderAccessTab(
      { ...baseDraft, requirePassword: true, password: "" },
      {},
      true,
      [],
      true
    );

    const input = screen.getByDisplayValue("••••••••") as HTMLInputElement;
    expect(input.type).toBe("password");
    expect(screen.getByText(/Password is set\. Leave blank to keep it/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /show password/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /hide password/i })).not.toBeInTheDocument();
  });

  it("updates allowed viewers and auto-enables email when missing", () => {
    const { updateDraft } = renderAccessTab(baseDraft);
    const allowedInput = screen.getByPlaceholderText(/alice@vc\.com/i);
    fireEvent.change(allowedInput, { target: { value: "alice@vc.com, bob@vc.com" } });
    fireEvent.keyDown(allowedInput, { key: "Enter" });
    expect(updateDraft).toHaveBeenCalledWith({ allowedViewers: ["alice@vc.com", "bob@vc.com"], requireEmail: true });
  });

  it("does not re-enable email when adding allowed viewers with verification already on", () => {
    const { updateDraft } = renderAccessTab({
      ...baseDraft,
      requireEmailVerification: true,
    });
    const allowedInput = screen.getByPlaceholderText(/alice@vc\.com/i);
    fireEvent.change(allowedInput, { target: { value: "alice@vc.com" } });
    fireEvent.keyDown(allowedInput, { key: "Enter" });
    expect(updateDraft).toHaveBeenCalledWith({ allowedViewers: ["alice@vc.com"] });
  });

  it("shows password strength hint and min-length warning", () => {
    const { rerender } = renderAccessTab({ ...baseDraft, requirePassword: true, password: "" });
    expect(screen.queryByText(/Strength:/i)).not.toBeInTheDocument();

    rerender(
      <Wrapper>
        <AccessTab
          draft={{ ...baseDraft, requirePassword: true, password: "short" }}
          updateDraft={vi.fn()}
          errors={{}}
          isDealRoomLink={true}
        />
      </Wrapper>
    );
    expect(screen.getByText(/Strength: Weak/i)).toBeInTheDocument();
    expect(screen.getByText(/Password must be at least 8 characters/i)).toBeInTheDocument();

    rerender(
      <Wrapper>
        <AccessTab
          draft={{ ...baseDraft, requirePassword: true, password: "StrongP@ssw0rd!" }}
          updateDraft={vi.fn()}
          errors={{}}
          isDealRoomLink={true}
        />
      </Wrapper>
    );
    expect(screen.getByText(/Strength: Strong/i)).toBeInTheDocument();
  });

  it("shows real-time conflict error when value is in both lists", () => {
    renderAccessTab({
      ...baseDraft,
      requireEmail: true,
      allowedViewers: ["alice@vc.com"],
      blockedViewers: ["alice@vc.com"],
    });
    expect(screen.getByText(/alice@vc\.com cannot be in both allowed and blocked lists/i)).toBeInTheDocument();
  });

  it("toggles watermark, NDA, download", () => {
    const { updateDraft } = renderAccessTab(baseDraft);
    fireEvent.click(screen.getByRole("switch", { name: /apply watermark/i }));
    expect(updateDraft).toHaveBeenCalledWith({ watermarkEnabled: true });

    fireEvent.click(screen.getByRole("switch", { name: /require NDA to view/i }));
    expect(updateDraft).toHaveBeenCalledWith({ requireNda: true, ndaDocumentId: "", ndaTemplateId: "" });

    fireEvent.click(screen.getByRole("switch", { name: /allow downloading/i }));
    expect(updateDraft).toHaveBeenCalledWith({ allowDownloading: true });
  });

  it("toggles screenshot protection switch", () => {
    const { updateDraft } = renderAccessTab(baseDraft);
    const switchEl = screen.getByRole("switch", { name: /screenshot protection/i });
    expect(switchEl).not.toBeDisabled();
    fireEvent.click(switchEl);
    expect(updateDraft).toHaveBeenCalledWith({ enableScreenshotProtection: true });
  });

  it("exposes screenshot protection help on the question trigger", () => {
    renderAccessTab(baseDraft);
    expect(screen.getByRole("button", { name: /reduce leak risk/i })).toBeInTheDocument();
    expect(screen.queryByTitle(/reduce leak risk/i)).not.toBeInTheDocument();
  });

  it("shows advanced count badge when AI Copilot is enabled", () => {
    renderAccessTab({ ...baseDraft, aiCopilotEnabled: true });
    expect(screen.getByText("1 enabled")).toBeInTheDocument();
  });

  it("counts Visitor Ask as one when both Ask Docs and Ask Host are enabled", () => {
    renderAccessTab({
      ...baseDraft,
      aiCopilotEnabled: true,
      enableFileRequests: true,
      enableQaConversations: true,
    });
    // Visitor Ask (Docs+Host) = 1, file requests = 1 → 2
    expect(screen.getByText("2 enabled")).toBeInTheDocument();
  });

  it("renders Visitor Ask master with Ask Docs and Ask Host sub-channels", () => {
    renderAccessTab({ ...baseDraft, aiCopilotEnabled: true });
    fireEvent.click(screen.getByText(/advanced/i));
    expect(screen.getByText(/Visitor Ask/i)).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: /Ask Docs/i })).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: /Ask Host/i })).toBeInTheDocument();
    expect(screen.queryByText(/AI Agents/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Q&A conversations/i)).not.toBeInTheDocument();
  });

  it("disables Ask Docs and shows KB guide when room knowledge base is missing", () => {
    const { updateDraft } = renderAccessTab(
      { ...baseDraft, enableQaConversations: true },
      {},
      true,
      [],
      false,
      {
        knowledgeBaseStatus: "none",
        knowledgeBaseHref: "/acme/deal-rooms/room-1?tab=documents",
      }
    );
    fireEvent.click(screen.getByText(/advanced/i));

    const askDocs = screen.getByRole("switch", { name: /Ask Docs/i });
    expect(askDocs).toBeDisabled();
    expect(
      screen.getByText(/Create or rebuild the room knowledge base before enabling Ask Docs/i)
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Open knowledge base/i })).toHaveAttribute(
      "href",
      "/acme/deal-rooms/room-1?tab=documents"
    );

    fireEvent.click(askDocs);
    expect(updateDraft).not.toHaveBeenCalledWith(
      expect.objectContaining({ aiCopilotEnabled: true })
    );
  });

  it("allows Ask Docs when room knowledge base is ready", () => {
    const { updateDraft } = renderAccessTab(
      { ...baseDraft, enableQaConversations: true },
      {},
      true,
      [],
      false,
      { knowledgeBaseStatus: "ready" }
    );
    fireEvent.click(screen.getByText(/advanced/i));

    const askDocs = screen.getByRole("switch", { name: /Ask Docs/i });
    expect(askDocs).not.toBeDisabled();
    fireEvent.click(askDocs);
    expect(updateDraft).toHaveBeenCalledWith({ aiCopilotEnabled: true });
  });

  it("turning off Visitor Ask master clears both Ask Docs and Ask Host", () => {
    const { updateDraft } = renderAccessTab({
      ...baseDraft,
      aiCopilotEnabled: true,
      enableQaConversations: true,
    });
    fireEvent.click(screen.getByText(/advanced/i));
    fireEvent.click(screen.getByRole("switch", { name: /Visitor Ask/i }));
    expect(updateDraft).toHaveBeenCalledWith({
      aiCopilotEnabled: false,
      enableQaConversations: false,
    });
  });

  it("renders all advanced options when section is expanded", () => {
    renderAccessTab({ ...baseDraft, aiCopilotEnabled: true });
    // Click the Advanced section header to expand
    fireEvent.click(screen.getByText(/advanced/i));
    expect(screen.getByText(/Visitor Ask/i)).toBeInTheDocument();
    expect(screen.getByText(/file requests/i)).toBeInTheDocument();
    expect(screen.getByText(/index file/i)).toBeInTheDocument();
  });

  it("enables functional advanced switches except screenshot protection", () => {
    renderAccessTab(baseDraft);
    fireEvent.click(screen.getByText(/advanced/i));
    expect(screen.getByRole("switch", { name: /file requests/i })).not.toBeDisabled();
    expect(screen.getByRole("switch", { name: /index file/i })).not.toBeDisabled();
    expect(screen.getByRole("switch", { name: /Visitor Ask/i })).not.toBeDisabled();
  });

  it("displays validation errors", () => {
    renderAccessTab({
      ...baseDraft,
      requirePassword: true,
      password: "short",
    }, {
      password: "Password must be at least 8 characters",
    });
    expect(screen.getByText(/at least 8 characters/i)).toBeInTheDocument();
  });

  it("shows NDA document selector when NDA is enabled", () => {
    renderAccessTab(
      { ...baseDraft, requireNda: true },
      {},
      true,
      [
        { id: "doc-1", title: "NDA v1" },
        { id: "doc-2", title: "NDA v2" },
      ]
    );
    const select = screen.getByRole("combobox", { name: /NDA agreement document/i });
    expect(select).toBeInTheDocument();
    expect(screen.getByText(/Select a document/i)).toBeInTheDocument();
  });

  it("selects an NDA document without toggling controlled state", () => {
    const { updateDraft } = renderAccessTab(
      { ...baseDraft, requireNda: true },
      {},
      true,
      [
        { id: "doc-1", title: "NDA v1" },
        { id: "doc-2", title: "NDA v2" },
      ]
    );
    fireEvent.click(screen.getByRole("combobox", { name: /NDA agreement document/i }));
    fireEvent.click(screen.getByRole("option", { name: "NDA v1" }));
    expect(updateDraft).toHaveBeenCalledWith({
      ndaTemplateId: "",
      ndaDocumentId: "doc-1",
    });
  });

  it("shows NDA document required error", () => {
    renderAccessTab(
      { ...baseDraft, requireNda: true },
      { ndaDocumentId: "Please select an NDA agreement document" },
      true,
      [{ id: "doc-1", title: "NDA v1" }]
    );
    expect(screen.getByText(/Please select an NDA agreement document/i)).toBeInTheDocument();
  });

  it("clears NDA document when NDA is disabled", () => {
    const { updateDraft } = renderAccessTab(
      { ...baseDraft, requireNda: true, ndaDocumentId: "doc-1" },
      {},
      true,
      [{ id: "doc-1", title: "NDA v1" }]
    );
    fireEvent.click(screen.getByRole("switch", { name: /require NDA to view/i }));
    expect(updateDraft).toHaveBeenCalledWith({ requireNda: false, ndaDocumentId: "", ndaTemplateId: "" });
  });
});
