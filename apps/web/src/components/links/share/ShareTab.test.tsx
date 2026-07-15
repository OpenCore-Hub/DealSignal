// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { ShareTab } from "./ShareTab";
import type { DraftLink, LinkPreset } from "./types";
import enLinkShare from "@/i18n/locales/en/linkShare.json";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      linkShare: enLinkShare,
      common: { cancel: "Cancel", saving: "Saving...", loading: "Loading..." },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

function renderShareTab(
  draft: DraftLink,
  options: {
    preset?: LinkPreset;
    link?: Parameters<typeof ShareTab>[0]["link"];
    slug?: string;
  } = {}
) {
  const updateDraft = vi.fn();
  const setPreset = vi.fn();
  const onEditAccess = vi.fn();

  const { rerender } = render(
    <Wrapper>
      <ShareTab
        draft={draft}
        updateDraft={updateDraft}
        preset={options.preset ?? "standard"}
        setPreset={setPreset}
        link={options.link ?? null}
        onEditAccess={onEditAccess}
        errors={{}}
        slug={options.slug}
      />
    </Wrapper>
  );
  return { updateDraft, setPreset, onEditAccess, rerender };
}

const baseDraft: DraftLink = {
  name: "",
  expiresAt: "",
  requireEmail: true,
  requireEmailVerification: false,
  requirePassword: false,
  password: "",
  watermarkEnabled: true,
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
  folderPaths: [],
};

describe("ShareTab", () => {
  it("updates link name", () => {
    const { updateDraft } = renderShareTab(baseDraft);
    fireEvent.change(screen.getByPlaceholderText("Recipient's Organization"), {
      target: { value: "Acme DD" },
    });
    expect(updateDraft).toHaveBeenCalledWith({ name: "Acme DD" });
  });

  it("fires preset change", async () => {
    const { setPreset } = renderShareTab(baseDraft);
    const trigger = screen.getByRole("combobox", { name: /link preset/i });
    fireEvent.pointerDown(trigger);
    fireEvent.click(trigger);
    const option = await waitFor(() => screen.getByRole("option", { name: /Confidential/i }));
    fireEvent.pointerDown(option);
    fireEvent.click(option);
    expect(setPreset).toHaveBeenCalledWith("confidential");
  });

  it("toggles expiration and updates expiresAt", () => {
    const { updateDraft, rerender } = renderShareTab(baseDraft);
    fireEvent.click(screen.getByRole("switch", { name: /expires on/i }));
    expect(updateDraft).toHaveBeenCalledWith(
      expect.objectContaining({ expiresAt: expect.stringMatching(/^\d{4}-/) })
    );

    const expiresAt = updateDraft.mock.calls[0][0].expiresAt as string;
    updateDraft.mockClear();
    rerender(
      <Wrapper>
        <ShareTab
          draft={{ ...baseDraft, expiresAt }}
          updateDraft={updateDraft}
          preset="standard"
          setPreset={vi.fn()}
          link={null}
          onEditAccess={vi.fn()}
          errors={{}}
        />
      </Wrapper>
    );
    fireEvent.click(screen.getByRole("switch", { name: /expires on/i }));
    expect(updateDraft).toHaveBeenCalledWith({ expiresAt: "" });
  });

  it("updates custom domain", async () => {
    const { updateDraft } = renderShareTab(baseDraft);
    const trigger = screen.getByRole("combobox", { name: /custom domain/i });
    fireEvent.pointerDown(trigger);
    fireEvent.click(trigger);
    const customOption = await waitFor(() => screen.getByRole("option", { name: /custom domain\.\.\./i }));
    fireEvent.pointerDown(customOption);
    fireEvent.click(customOption);

    fireEvent.change(screen.getByPlaceholderText(/yourdomain\.com/i), {
      target: { value: "invest.acme.capital" },
    });
    expect(updateDraft).toHaveBeenCalledWith({ customDomain: "invest.acme.capital" });
  });

  it("parses tags from comma-separated input", () => {
    const { updateDraft } = renderShareTab(baseDraft);
    fireEvent.change(screen.getByPlaceholderText(/investor, q3-2026/i), {
      target: { value: "investors, lp-report;q2-2026" },
    });
    expect(updateDraft).toHaveBeenCalledWith({
      tags: ["investors", "lp-report", "q2-2026"],
    });
  });

  it("toggles notify on access", () => {
    const { updateDraft } = renderShareTab(baseDraft);
    fireEvent.click(screen.getByRole("switch", { name: /notify on access/i }));
    expect(updateDraft).toHaveBeenCalledWith({ notifyOnAccess: true });
  });

  it("opens preview in a new tab when link exists", () => {
    const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);
    renderShareTab(baseDraft, {
      link: {
        id: "link-1",
        name: "Acme",
        shortUrl: "http://localhost/l/abc123",
      } as unknown as Parameters<typeof ShareTab>[0]["link"],
    });
    fireEvent.click(screen.getByRole("button", { name: /preview as visitor/i }));
    expect(openSpy).toHaveBeenCalledWith(`${window.location.origin}/l/abc123`, "_blank", "noopener,noreferrer");
    openSpy.mockRestore();
  });

  it("shows old slug hint when slug and link are provided", () => {
    renderShareTab(baseDraft, {
      slug: "acme-room",
      link: {
        id: "link-1",
        name: "Acme",
        shortUrl: "http://localhost/l/abc123",
      } as unknown as Parameters<typeof ShareTab>[0]["link"],
    });
    expect(screen.getByText(/\/r\/acme-room/i)).toBeInTheDocument();
    expect(screen.getByText(/\/l\/abc123/i)).toBeInTheDocument();
  });
});
