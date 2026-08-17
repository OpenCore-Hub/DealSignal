// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DocumentShareDialog } from "./DocumentShareDialog";

const { createLinkMock, copyToClipboardMock, getDocumentsMock } = vi.hoisted(() => ({
  createLinkMock: vi.fn(),
  copyToClipboardMock: vi.fn(),
  getDocumentsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    createLink: createLinkMock,
    getDocuments: getDocumentsMock,
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
            eyebrow: "Share link",
            lead: "A private URL is copied as soon as you create it.",
            description: "Create a share link for “{{name}}” and copy it to the clipboard.",
            defaultsHint: "Uses the same defaults as Create link.",
            createAndCopy: "Create",
            creating: "Creating…",
            advanced: "Advanced",
            copied: "Share link copied",
            createFailed: "Failed to create share link",
            notReady: "Document must be ready before sharing",
            noFilesSelected: "Select at least one document to create a share link.",
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
              custom: "Custom",
            },
            customMaxViews: "Custom max views",
            customMaxViewsRequired: "Enter a custom view limit",
            customMaxViewsInvalid: "Max views must be a whole number from 1 to 1,000,000",
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
  const pitchDoc = {
    id: "doc_1",
    title: "Pitch Deck.pdf",
    status: "ready",
    createdAt: "2026-08-16T00:00:00Z",
  };

  beforeEach(() => {
    vi.clearAllMocks();
    createLinkMock.mockResolvedValue({
      id: "link_1",
      shortUrl: "https://example.test/v/abc",
    });
    copyToClipboardMock.mockResolvedValue(true);
    getDocumentsMock.mockResolvedValue({
      data: [
        {
          id: "doc_old",
          title: "Older Memo.pdf",
          fileName: "Older Memo.pdf",
          status: "ready",
          createdAt: "2026-01-01T00:00:00Z",
        },
        {
          id: "doc_1",
          title: "Pitch Deck.pdf",
          fileName: "Pitch Deck.pdf",
          status: "ready",
          createdAt: "2026-08-16T00:00:00Z",
        },
      ],
    });
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
            documents={[pitchDoc]}
            workspaceSlug="acme"
            onCreated={onCreated}
          />
        </MemoryRouter>
      </I18nextProvider>,
    );

    expect(screen.getByTestId("document-share-dialog")).toBeInTheDocument();
    await waitFor(() => expect(getDocumentsMock).toHaveBeenCalled());
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

  it("shows advanced settings expanded by default instead of navigating away", async () => {
    const onOpenChange = vi.fn();
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={["/acme/documents"]}>
          <DocumentShareDialog
            open
            onOpenChange={onOpenChange}
            documents={[pitchDoc]}
            workspaceSlug="acme"
          />
        </MemoryRouter>
      </I18nextProvider>,
    );

    await waitFor(() => expect(getDocumentsMock).toHaveBeenCalled());
    expect(screen.getByTestId("document-share-advanced-card")).toBeInTheDocument();
    expect(screen.getByText("Access control")).toBeInTheDocument();
    expect(screen.getByText("Content protection")).toBeInTheDocument();
    expect(screen.getByText("Advanced settings")).toBeInTheDocument();
    expect(screen.getByTestId("security-expiry-select")).toBeInTheDocument();
    expect(screen.getByTestId("security-max-views-select")).toBeInTheDocument();
    expect(screen.getByTestId("security-switch-allowDownload")).toBeChecked();
    expect(screen.getByTestId("security-switch-watermarkEnabled")).toBeChecked();
    expect(screen.queryByTestId("document-share-advanced")).not.toBeInTheDocument();
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
            documents={[pitchDoc]}
            workspaceSlug="acme"
          />
        </MemoryRouter>
      </I18nextProvider>,
    );

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
            documents={[pitchDoc]}
            workspaceSlug="acme"
          />
        </MemoryRouter>
      </I18nextProvider>,
    );

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
            documents={[pitchDoc]}
            workspaceSlug="acme"
          />
        </MemoryRouter>
      </I18nextProvider>,
    );

    await waitFor(() => expect(getDocumentsMock).toHaveBeenCalled());
    fireEvent.click(screen.getByTestId("security-switch-ndaEnabled"));

    expect(screen.getByTestId("security-switch-requireEmailVerification")).toBeChecked();
    expect(screen.getByTestId("security-switch-requireEmailVerification")).toBeDisabled();
    expect(screen.getByTestId("document-share-contact-selector")).toBeInTheDocument();
  });

  it("lists library files by newest upload and keeps this batch selected", async () => {
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter>
          <DocumentShareDialog
            open
            onOpenChange={vi.fn()}
            documents={[
              {
                id: "doc_1",
                title: "Pitch Deck.pdf",
                status: "ready",
                createdAt: "2026-08-16T00:00:00Z",
              },
              {
                id: "doc_new",
                title: "CFI-Case-Study.xlsx",
                fileName: "CFI-Case-Study.xlsx",
                status: "ready",
                createdAt: "2026-08-16T12:00:00Z",
              },
            ]}
            workspaceSlug="acme"
          />
        </MemoryRouter>
      </I18nextProvider>,
    );

    const list = await screen.findByTestId("document-share-file-list");
    await waitFor(() => {
      expect(getDocumentsMock).toHaveBeenCalledWith("all", "general");
    });
    const labels = within(list).getAllByTestId(/document-share-file-/);
    expect(labels.map((node) => node.getAttribute("data-testid"))).toEqual([
      "document-share-file-doc_new",
      "document-share-file-doc_1",
      "document-share-file-doc_old",
    ]);
    expect(within(labels[0]!).getByRole("checkbox")).toBeChecked();
    expect(within(labels[1]!).getByRole("checkbox")).toBeChecked();
    expect(within(labels[2]!).getByRole("checkbox")).not.toBeChecked();

    fireEvent.click(screen.getByTestId("document-share-create"));
    await waitFor(() => {
      expect(createLinkMock).toHaveBeenCalledWith(
        ["doc_new", "doc_1"],
        expect.objectContaining({ watermarkEnabled: true }),
      );
    });
  });
});
