// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { ShareTab } from "./ShareTab";
import type { DraftLink } from "./types";
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
  return (
    <MemoryRouter>
      <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>
    </MemoryRouter>
  );
}

function renderShareTab(
  draft: DraftLink,
  options: {
    link?: Parameters<typeof ShareTab>[0]["link"];
    slug?: string;
    availableDomains?: string[];
    pendingHostname?: string;
    brandSettingsHref?: string;
  } = {}
) {
  const updateDraft = vi.fn();
  const onEditAccess = vi.fn();

  const { rerender } = render(
    <Wrapper>
      <ShareTab
        draft={draft}
        updateDraft={updateDraft}
        link={options.link ?? null}
        onEditAccess={onEditAccess}
        errors={{}}
        slug={options.slug}
        availableDomains={options.availableDomains}
        pendingHostname={options.pendingHostname}
        brandSettingsHref={options.brandSettingsHref}
      />
    </Wrapper>
  );
  return { updateDraft, onEditAccess, rerender };
}

const baseDraft: DraftLink = {
  name: "",
  expiresAt: "",
  requireEmail: false,
  requireEmailVerification: true,
  requirePassword: false,
  password: "",
  watermarkEnabled: true,
  requireNda: false,
  ndaDocumentId: "",
  ndaTemplateId: "",
  allowDownloading: false,
  enableScreenshotProtection: false,
  enableFileRequests: false,
  enableIndexFileGeneration: false,
  visitorAskExperience: "host_only" as const,
  allowedViewers: [],
  blockedViewers: [],
  customDomain: "",
  notifyOnAccess: false,
  folderPaths: [],
  folderScopeMode: "allowlist",
  contactIds: [],
};

describe("ShareTab", () => {
  it("updates link name", () => {
    const { updateDraft } = renderShareTab(baseDraft);
    fireEvent.change(screen.getByPlaceholderText("Recipient's Organization"), {
      target: { value: "Acme DD" },
    });
    expect(updateDraft).toHaveBeenCalledWith({ name: "Acme DD" });
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
          link={null}
          onEditAccess={vi.fn()}
          errors={{}}
        />
      </Wrapper>
    );
    fireEvent.click(screen.getByRole("switch", { name: /expires on/i }));
    expect(updateDraft).toHaveBeenCalledWith({ expiresAt: "" });
  });

  it("lists verified Brand domains in the custom domain dropdown", async () => {
    renderShareTab(baseDraft, { availableDomains: ["invest.example.com"] });
    const trigger = screen.getByRole("combobox", { name: /custom domain/i });
    fireEvent.pointerDown(trigger);
    fireEvent.click(trigger);
    expect(
      await waitFor(() => screen.getByRole("option", { name: "invest.example.com" })),
    ).toBeInTheDocument();
  });

  it("selects a verified Brand domain from the dropdown", async () => {
    const { updateDraft } = renderShareTab(baseDraft, {
      availableDomains: ["invest.example.com"],
    });
    const trigger = screen.getByRole("combobox", { name: /custom domain/i });
    fireEvent.pointerDown(trigger);
    fireEvent.click(trigger);
    const option = await waitFor(() =>
      screen.getByRole("option", { name: "invest.example.com" }),
    );
    fireEvent.pointerDown(option);
    fireEvent.click(option);
    expect(updateDraft).toHaveBeenCalledWith({ customDomain: "invest.example.com" });
  });

  it("routes Custom domain… to Brand settings instead of free-form input", async () => {
    renderShareTab(baseDraft, {
      brandSettingsHref: "/acme/settings/brand",
    });
    const trigger = screen.getByRole("combobox", { name: /custom domain/i });
    fireEvent.pointerDown(trigger);
    fireEvent.click(trigger);
    const customOption = await waitFor(() =>
      screen.getByRole("option", { name: /custom domain\.\.\./i }),
    );
    fireEvent.pointerDown(customOption);
    fireEvent.click(customOption);

    expect(screen.getByTestId("share-custom-domain-brand-cta")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /open brand settings/i })).toHaveAttribute(
      "href",
      "/acme/settings/brand",
    );
    expect(screen.queryByPlaceholderText(/yourdomain\.com/i)).not.toBeInTheDocument();
  });

  it("shows pending Brand hostname hint when DNS is not verified yet", () => {
    renderShareTab(baseDraft, { pendingHostname: "invest.example.com" });
    expect(screen.getByTestId("share-custom-domain-pending")).toHaveTextContent(
      "invest.example.com",
    );
  });

  it("keeps free-form input for legacy per-link custom domains", () => {
    const { updateDraft } = renderShareTab({
      ...baseDraft,
      customDomain: "legacy.example.com",
    });
    fireEvent.change(screen.getByPlaceholderText(/yourdomain\.com/i), {
      target: { value: "legacy-updated.example.com" },
    });
    expect(updateDraft).toHaveBeenCalledWith({ customDomain: "legacy-updated.example.com" });
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
    fireEvent.click(screen.getByRole("button", { name: /preview/i }));
    expect(openSpy).toHaveBeenCalledWith(`${window.location.origin}/l/abc123`, "_blank", "noopener,noreferrer");
    openSpy.mockRestore();
  });

  it("shows custom domain invalid message for malformed legacy domains", () => {
    renderShareTab({
      ...baseDraft,
      customDomain: "not a valid domain",
    });
    expect(screen.getByText(/please enter a valid domain/i)).toBeInTheDocument();
  });

  it("sets min attribute on expiration datetime input", () => {
    const { updateDraft, rerender } = renderShareTab(baseDraft);
    fireEvent.click(screen.getByRole("switch", { name: /expires on/i }));
    const expiresAt = updateDraft.mock.calls[0][0].expiresAt as string;
    rerender(
      <Wrapper>
        <ShareTab
          draft={{ ...baseDraft, expiresAt }}
          updateDraft={updateDraft}
          link={null}
          onEditAccess={vi.fn()}
          errors={{}}
        />
      </Wrapper>
    );
    const input = document.querySelector('input[type="datetime-local"]') as HTMLInputElement;
    expect(input.min).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/);
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
