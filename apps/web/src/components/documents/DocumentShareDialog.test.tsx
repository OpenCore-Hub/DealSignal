// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DocumentShareDialog } from "./DocumentShareDialog";

const { createLinkMock, copyToClipboardMock } = vi.hoisted(() => ({
  createLinkMock: vi.fn(),
  copyToClipboardMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    createLink: createLinkMock,
  },
}));

vi.mock("@/lib/clipboard", () => ({
  copyToClipboard: copyToClipboardMock,
}));

vi.mock("@/components/links/share/hooks", () => ({
  useNdaPickerSources: () => ({ ndaTemplates: [], agreementDocs: [] }),
}));

vi.mock("@/components/links/smart-link/ContactSelector", () => ({
  ContactSelector: ({
    value,
    onChange,
  }: {
    value: string[];
    onChange: (ids: string[]) => void;
  }) => (
    <div data-testid="document-share-contact-selector">
      <button
        type="button"
        data-testid="document-share-add-contact"
        onClick={() => onChange(["contact_1"])}
      >
        Add contact
      </button>
      <span data-testid="document-share-contact-ids">{value.join(",")}</span>
    </div>
  ),
}));

async function initI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["documents", "common", "links", "linkShare"],
    defaultNS: "documents",
    resources: {
      en: {
        documents: {
          share: {
            title: "Share document",
            description: "Create a share link for “{{name}}” and copy it to the clipboard.",
            defaultsHint: "Uses the same defaults as Create link.",
            createAndCopy: "Create",
            creating: "Creating…",
            advanced: "Advanced",
            copied: "Share link copied",
            createFailed: "Failed to create share link",
            notReady: "Document must be ready before sharing",
          },
        },
        common: {
          cancel: "Cancel",
          close: "Close",
        },
        links: {
          creator: {
            sectionAccessControl: "Access control",
            sectionContentProtection: "Content protection",
            sectionAdvanced: "Advanced settings",
            requireEmailVerification: "Require email verification code",
            allowDownload: "Allow download",
            watermark: "Dynamic watermark",
            expiry: "Expiration",
            expiryPlaceholder: "Expiration",
            expiryDays: {
              "7": "7 days",
              "15": "15 days",
              "30": "30 days",
              custom: "Custom",
            },
            customExpiresAt: "Custom expiration",
            customExpiresAtRequired: "Choose a custom expiration date",
            customExpiresAtFuture: "Expiration must be in the future",
            maxViews: "Max views",
            maxViewsPlaceholder: "Max views",
            maxViewsOptions: {
              unlimited: "unlimited",
              "10": "10",
              "50": "50",
              "100": "100",
            },
            contactRequired: "Please select a contact to send the verification code to.",
            ndaDocumentRequired: "Please select an NDA agreement document",
          },
        },
        linkShare: {
          accessRules: {
            additionalProtections: {
              requireNda: "Require NDA to view",
              ndaDocument: "NDA document",
              ndaDocumentPlaceholder: "Select an NDA",
            },
            errors: {
              ndaDocumentRequired: "NDA document is required",
            },
          },
        },
      },
    },
    interpolation: { escapeValue: false },
  });
  return instance;
}

describe("DocumentShareDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createLinkMock.mockResolvedValue({
      id: "link_1",
      shortUrl: "https://example.test/v/abc",
    });
    copyToClipboardMock.mockResolvedValue(true);
  });

  it("creates a link with library defaults and copies the URL", async () => {
    const onOpenChange = vi.fn();
    const onCreated = vi.fn();
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter>
          <DocumentShareDialog
            open
            onOpenChange={onOpenChange}
            documentId="doc_1"
            documentTitle="Pitch Deck.pdf"
            workspaceSlug="acme"
            onCreated={onCreated}
          />
        </MemoryRouter>
      </I18nextProvider>,
    );

    expect(screen.getByTestId("document-share-dialog")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("document-share-create"));

    await waitFor(() => {
      expect(createLinkMock).toHaveBeenCalledWith(
        ["doc_1"],
        expect.objectContaining({
          allowDownload: true,
          watermarkEnabled: true,
          expiryDays: 30,
          ndaEnabled: false,
        }),
      );
    });
    expect(copyToClipboardMock).toHaveBeenCalledWith(
      "https://example.test/v/abc",
      "Share link copied",
    );
    expect(onCreated).toHaveBeenCalled();
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("expands an inline advanced settings card instead of navigating away", async () => {
    const onOpenChange = vi.fn();
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={["/acme/documents"]}>
          <DocumentShareDialog
            open
            onOpenChange={onOpenChange}
            documentId="doc_1"
            documentTitle="Pitch Deck.pdf"
            workspaceSlug="acme"
          />
        </MemoryRouter>
      </I18nextProvider>,
    );

    expect(screen.queryByTestId("document-share-advanced-card")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("document-share-advanced"));
    expect(await screen.findByTestId("document-share-advanced-card")).toBeInTheDocument();
    expect(screen.getByText("Access control")).toBeInTheDocument();
    expect(screen.getByText("Content protection")).toBeInTheDocument();
    expect(screen.getByTestId("security-switch-allowDownload")).toBeChecked();
    expect(screen.getByTestId("security-switch-watermarkEnabled")).toBeChecked();
    expect(onOpenChange).not.toHaveBeenCalled();
    expect(createLinkMock).not.toHaveBeenCalled();
  });

  it("applies toggled advanced settings when creating the link", async () => {
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter>
          <DocumentShareDialog
            open
            onOpenChange={vi.fn()}
            documentId="doc_1"
            documentTitle="Pitch Deck.pdf"
            workspaceSlug="acme"
          />
        </MemoryRouter>
      </I18nextProvider>,
    );

    fireEvent.click(screen.getByTestId("document-share-advanced"));
    fireEvent.click(screen.getByTestId("security-switch-allowDownload"));
    fireEvent.click(screen.getByTestId("document-share-create"));

    await waitFor(() => {
      expect(createLinkMock).toHaveBeenCalledWith(
        ["doc_1"],
        expect.objectContaining({
          allowDownload: false,
          watermarkEnabled: true,
        }),
      );
    });
  });

  it("requires contacts when email verification is enabled, matching Create link", async () => {
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter>
          <DocumentShareDialog
            open
            onOpenChange={vi.fn()}
            documentId="doc_1"
            documentTitle="Pitch Deck.pdf"
            workspaceSlug="acme"
          />
        </MemoryRouter>
      </I18nextProvider>,
    );

    fireEvent.click(screen.getByTestId("document-share-advanced"));
    fireEvent.click(screen.getByTestId("security-switch-requireEmailVerification"));

    expect(screen.getByTestId("document-share-contact-selector")).toBeInTheDocument();
    expect(screen.getByTestId("document-share-create")).toBeDisabled();
    expect(createLinkMock).not.toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("document-share-add-contact"));
    expect(screen.getByTestId("document-share-create")).toBeEnabled();
    fireEvent.click(screen.getByTestId("document-share-create"));

    await waitFor(() => {
      expect(createLinkMock).toHaveBeenCalledWith(
        ["doc_1"],
        expect.objectContaining({
          requireEmailVerification: true,
          contactIds: ["contact_1"],
        }),
      );
    });
  });

  it("keeps email verification on when NDA is enabled", async () => {
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter>
          <DocumentShareDialog
            open
            onOpenChange={vi.fn()}
            documentId="doc_1"
            documentTitle="Pitch Deck.pdf"
            workspaceSlug="acme"
          />
        </MemoryRouter>
      </I18nextProvider>,
    );

    fireEvent.click(screen.getByTestId("document-share-advanced"));
    fireEvent.click(screen.getByTestId("security-switch-ndaEnabled"));

    expect(screen.getByTestId("security-switch-requireEmailVerification")).toBeChecked();
    expect(screen.getByTestId("security-switch-requireEmailVerification")).toBeDisabled();
    expect(screen.getByTestId("document-share-contact-selector")).toBeInTheDocument();
  });
});
