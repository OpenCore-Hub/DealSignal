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
  ndaTemplates: { id: string; name: string; sourceDocumentId: string }[] = [],
  roomSecurityFloors?: { requireEmailVerification?: boolean; requireNda?: boolean },
  roomBlockedEmails?: string[],
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
        ndaTemplates={ndaTemplates}
        passwordAlreadySet={passwordAlreadySet}
        roomSecurityFloors={roomSecurityFloors}
        roomBlockedEmails={roomBlockedEmails}
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
  enableScreenshotProtection: false,
  enableFileRequests: false,
  enableIndexFileGeneration: false,
  enableAiAssistant: true,
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

  it("shows advanced count badge when file requests is enabled", () => {
    renderAccessTab({ ...baseDraft, enableFileRequests: true, enableAiAssistant: false }, {}, true);
    expect(screen.getByText("1 enabled")).toBeInTheDocument();
  });

  it("counts file requests and index file separately in advanced badge", () => {
    renderAccessTab({
      ...baseDraft,
      enableFileRequests: true,
      enableIndexFileGeneration: true,
      enableAiAssistant: false,
    }, {}, true);
    expect(screen.getByText("2 enabled")).toBeInTheDocument();
  });

  it("counts AI assistant in advanced badge for deal-room links", () => {
    renderAccessTab({ ...baseDraft, enableAiAssistant: true }, {}, true);
    expect(screen.getByText("1 enabled")).toBeInTheDocument();
  });

  it("renders advanced options without Ask Host toggle", () => {
    renderAccessTab(baseDraft);
    fireEvent.click(screen.getByText(/advanced/i));
    expect(screen.queryByText(/Enable AI assistant/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("switch", { name: /Ask Docs/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/AI Agents/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Q&A conversations/i)).not.toBeInTheDocument();
  });

  it("shows AI assistant toggle for deal-room links", () => {
    renderAccessTab(baseDraft, {}, true);
    fireEvent.click(screen.getByText(/advanced/i));
    expect(screen.getByTestId("deal-room-ai-assistant-toggle")).toBeInTheDocument();
    expect(screen.getByText(/AI assistant/i)).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: /AI assistant/i })).toBeInTheDocument();
  });

  it("toggles AI assistant for deal-room links", () => {
    const { updateDraft } = renderAccessTab(baseDraft, {}, true);
    fireEvent.click(screen.getByText(/advanced/i));
    fireEvent.click(screen.getByRole("switch", { name: /AI assistant/i }));
    expect(updateDraft).toHaveBeenCalledWith({ enableAiAssistant: false });
  });

  it("does not show AI assistant toggle for document-only links", () => {
    renderAccessTab(baseDraft, {}, false);
    fireEvent.click(screen.getByText(/advanced/i));
    expect(screen.queryByTestId("deal-room-ai-assistant-toggle")).not.toBeInTheDocument();
  });

  it("renders all advanced options when section is expanded", () => {
    renderAccessTab(baseDraft);
    fireEvent.click(screen.getByText(/advanced/i));
    expect(screen.getByText(/file requests/i)).toBeInTheDocument();
    expect(screen.getByText(/index file/i)).toBeInTheDocument();
  });

  it("enables functional advanced switches except screenshot protection", () => {
    renderAccessTab(baseDraft);
    fireEvent.click(screen.getByText(/advanced/i));
    expect(screen.getByRole("switch", { name: /file requests/i })).not.toBeDisabled();
    expect(screen.getByRole("switch", { name: /index file/i })).not.toBeDisabled();
    expect(screen.getByRole("switch", { name: /AI assistant/i })).not.toBeDisabled();
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

  it("lists NDA templates and extra agreement docs, deduping template sources", () => {
    const { updateDraft } = renderAccessTab(
      { ...baseDraft, requireNda: true },
      {},
      true,
      [
        { id: "doc-1", title: "NDA v1" },
        { id: "doc_nda_1", title: "Raw agreement PDF" },
      ],
      false,
      [
        {
          id: "nda_tpl_1",
          name: "Standard Mutual NDA",
          sourceDocumentId: "doc_nda_1",
        },
      ],
    );
    fireEvent.click(screen.getByRole("combobox", { name: /NDA agreement document/i }));
    expect(screen.getByRole("option", { name: "Standard Mutual NDA" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "NDA v1" })).toBeInTheDocument();
    // Source doc already covered by the template — do not duplicate.
    expect(screen.queryByRole("option", { name: "Raw agreement PDF" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("option", { name: "Standard Mutual NDA" }));
    expect(updateDraft).toHaveBeenCalledWith({
      ndaTemplateId: "nda_tpl_1",
      ndaDocumentId: "doc_nda_1",
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

  it("locks email verification and NDA when room floors are on; password stays editable", () => {
    const { updateDraft } = renderAccessTab(
      { ...baseDraft, requireEmailVerification: true, requireNda: true },
      {},
      true,
      [{ id: "doc-1", title: "NDA v1" }],
      false,
      [],
      { requireEmailVerification: true, requireNda: true },
    );

    expect(screen.queryByTestId("room-security-floor-banner")).not.toBeInTheDocument();
    expect(screen.getByTestId("access-switch-require-verification")).toHaveAttribute(
      "data-locked",
      "true",
    );
    expect(screen.getByTestId("access-switch-require-nda")).toHaveAttribute("data-locked", "true");

    const verifySwitch = screen.getByRole("switch", { name: /require email verification/i });
    const ndaSwitch = screen.getByRole("switch", { name: /require NDA to view/i });
    const passwordSwitch = screen.getByRole("switch", { name: /require password to view/i });

    expect(verifySwitch).toBeDisabled();
    expect(verifySwitch).toBeChecked();
    expect(ndaSwitch).toBeDisabled();
    expect(ndaSwitch).toBeChecked();
    expect(passwordSwitch).not.toBeDisabled();

    fireEvent.click(verifySwitch);
    fireEvent.click(ndaSwitch);
    expect(updateDraft).not.toHaveBeenCalled();

    fireEvent.click(passwordSwitch);
    expect(updateDraft).toHaveBeenCalledWith({ requirePassword: true });
  });

  it("keeps room blocklist entries locked and re-applies them on edit", () => {
    const { updateDraft } = renderAccessTab(
      {
        ...baseDraft,
        requireEmail: true,
        allowedViewers: ["good@example.com"],
        blockedViewers: ["room-block@example.com", "link-only@example.com"],
      },
      {},
      true,
      [],
      false,
      [],
      undefined,
      ["room-block@example.com"],
    );

    expect(
      screen.getByText(/Locked entries come from deal room access policy/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /remove room-block@example.com/i }),
    ).not.toBeInTheDocument();

    const textboxes = screen.getAllByRole("textbox");
    const blockedInput = textboxes[textboxes.length - 1];
    fireEvent.change(blockedInput, { target: { value: "extra@example.com" } });
    fireEvent.keyDown(blockedInput, { key: "Enter" });
    expect(updateDraft).toHaveBeenCalledWith({
      blockedViewers: [
        "room-block@example.com",
        "link-only@example.com",
        "extra@example.com",
      ],
    });
  });
});
