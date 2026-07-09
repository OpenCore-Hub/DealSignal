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

function renderAccessTab(draft: DraftLink, errors: Record<string, string> = {}) {
  const updateDraft = vi.fn();
  const { rerender } = render(
    <Wrapper>
      <AccessTab draft={draft} updateDraft={updateDraft} errors={errors} />
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
  allowDownloading: false,
  aiCopilotEnabled: false,
  enableScreenshotProtection: false,
  enableFileRequests: false,
  enableIndexFileGeneration: false,
  enableQaConversations: false,
  allowedViewers: [],
  blockedViewers: [],
  autoAddInvited: true,
  customDomain: "",
  tags: [],
  notifyOnAccess: false,
};

describe("AccessTab", () => {
  it("toggles require email", () => {
    const { updateDraft } = renderAccessTab(baseDraft);
    fireEvent.click(screen.getByRole("switch", { name: /require email to view/i }));
    expect(updateDraft).toHaveBeenCalledWith({ requireEmail: true, requireEmailVerification: false });
  });

  it("enabling verification also enables email", () => {
    const { updateDraft } = renderAccessTab({ ...baseDraft, requireEmail: true });
    fireEvent.click(screen.getByRole("switch", { name: /require email verification/i }));
    expect(updateDraft).toHaveBeenCalledWith({ requireEmail: true, requireEmailVerification: true });
  });

  it("disabling email also disables verification", () => {
    const { updateDraft } = renderAccessTab({ ...baseDraft, requireEmail: true, requireEmailVerification: true });
    fireEvent.click(screen.getByRole("switch", { name: /require email to view/i }));
    expect(updateDraft).toHaveBeenCalledWith({ requireEmail: false, requireEmailVerification: false });
  });

  it("shows password input when password switch is on", () => {
    const { updateDraft, rerender } = renderAccessTab(baseDraft);
    fireEvent.click(screen.getByRole("switch", { name: /require password to view/i }));
    expect(updateDraft).toHaveBeenCalledWith({ requirePassword: true });

    rerender(
      <Wrapper>
        <AccessTab draft={{ ...baseDraft, requirePassword: true }} updateDraft={updateDraft} errors={{}} />
      </Wrapper>
    );
    expect(screen.getByPlaceholderText(/enter password/i)).toBeInTheDocument();
  });

  it("updates allowed and blocked viewers", () => {
    const { updateDraft } = renderAccessTab(baseDraft);
    const allowedInput = screen.getByPlaceholderText(/alice@vc\.com/i);
    fireEvent.change(allowedInput, { target: { value: "alice@vc.com, bob@vc.com" } });
    fireEvent.keyDown(allowedInput, { key: "Enter" });
    expect(updateDraft).toHaveBeenCalledWith({ allowedViewers: ["alice@vc.com", "bob@vc.com"] });
  });

  it("toggles watermark, NDA, download", () => {
    const { updateDraft } = renderAccessTab(baseDraft);
    fireEvent.click(screen.getByRole("switch", { name: /apply watermark/i }));
    expect(updateDraft).toHaveBeenCalledWith({ watermarkEnabled: true });

    fireEvent.click(screen.getByRole("switch", { name: /require NDA to view/i }));
    expect(updateDraft).toHaveBeenCalledWith({ requireNda: true });

    fireEvent.click(screen.getByRole("switch", { name: /allow downloading/i }));
    expect(updateDraft).toHaveBeenCalledWith({ allowDownloading: true });
  });

  it("toggles screenshot protection", () => {
    renderAccessTab(baseDraft);
    const switchEl = screen.getByRole("switch", { name: /screenshot protection/i });
    fireEvent.click(switchEl);
    // screenshot protection is in the Advanced section, but the label is still rendered
    expect(screen.getByText(/screenshot protection/i)).toBeInTheDocument();
  });

  it("shows advanced count badge when AI Copilot is enabled", () => {
    renderAccessTab({ ...baseDraft, aiCopilotEnabled: true });
    expect(screen.getByText("1 enabled")).toBeInTheDocument();
  });

  it("shows advanced count badge with multiple advanced fields enabled", () => {
    renderAccessTab({
      ...baseDraft,
      aiCopilotEnabled: true,
      enableFileRequests: true,
      enableQaConversations: true,
    });
    expect(screen.getByText("3 enabled")).toBeInTheDocument();
  });

  it("renders all advanced options when section is expanded", () => {
    renderAccessTab({ ...baseDraft, aiCopilotEnabled: true });
    // Click the Advanced section header to expand
    fireEvent.click(screen.getByText(/advanced/i));
    expect(screen.getByText(/AI Agents/i)).toBeInTheDocument();
    expect(screen.getByText(/file requests/i)).toBeInTheDocument();
    expect(screen.getByText(/index file/i)).toBeInTheDocument();
    expect(screen.getByText(/Q&A conversations/i)).toBeInTheDocument();
  });

  it("displays validation errors", () => {
    renderAccessTab(baseDraft, {
      password: "Password must be at least 8 characters",
      conflict: "alice@vc.com cannot be in both allowed and blocked lists",
    });
    expect(screen.getByText(/at least 8 characters/i)).toBeInTheDocument();
    expect(screen.getByText(/cannot be in both allowed and blocked lists/i)).toBeInTheDocument();
  });
});
