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

async function initI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["documents", "common"],
    defaultNS: "documents",
    resources: {
      en: {
        documents: {
          share: {
            title: "Share document",
            description: "Create a share link for “{{name}}” and copy it to the clipboard.",
            defaultsHint: "Uses the same defaults as Create link.",
            createAndCopy: "Create & copy link",
            creating: "Creating…",
            advanced: "Advanced settings",
            copied: "Share link copied",
            createFailed: "Failed to create share link",
          },
        },
        common: {
          cancel: "Cancel",
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

  it("routes to advanced create-link pipeline", async () => {
    const i18nInstance = await initI18n();
    render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={["/acme/documents"]}>
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
    // Navigation is handled by react-router; advanced button closes via onOpenChange.
    await waitFor(() => {
      expect(createLinkMock).not.toHaveBeenCalled();
    });
  });
});
