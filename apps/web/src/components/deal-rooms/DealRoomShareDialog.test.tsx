// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import type { Link } from "@/types";
import { DealRoomShareDialog } from "./DealRoomShareDialog";
import enDealRooms from "@/i18n/locales/en/dealRooms.json";
import enLinkShare from "@/i18n/locales/en/linkShare.json";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      dealRooms: enDealRooms,
      linkShare: enLinkShare,
      common: {
        cancel: "Cancel",
        saving: "Saving...",
        loading: "Loading...",
        close: "Close",
        error: { loadFailed: "Failed to load", saveFailed: "Failed to save" },
      },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomLinks: vi.fn(),
    getDealRoomDocuments: vi.fn(),
    getDealRoomFolders: vi.fn(),
    getLinkById: vi.fn(),
    getLinkAccessRules: vi.fn(),
    getContacts: vi.fn(),
    createDealRoomLink: vi.fn(),
    updateLinkFull: vi.fn(),
    setLinkAccessRules: vi.fn(),
    updateLink: vi.fn(),
    getAccessLogs: vi.fn(),
  },
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/clipboard", () => ({ copyToClipboard: vi.fn(() => Promise.resolve(true)) }));

describe("DealRoomShareDialog", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [] });
    vi.mocked(api.getDealRoomDocuments).mockResolvedValue({
      data: [
        {
          folder: "/",
          permission: "view" as const,
          documents: [
            { id: "doc-1", document_id: "doc-1", title: "NDA Agreement", folder_path: "/", sort_order: 0, source_type: "pdf", status: "ready", created_at: new Date().toISOString() },
          ],
        },
      ],
    });
    vi.mocked(api.getDealRoomFolders).mockResolvedValue({ data: [] });
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({ data: [] });
    vi.mocked(api.getContacts).mockResolvedValue({ data: [] });
    vi.mocked(api.getAccessLogs).mockResolvedValue({ data: [] });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("opens in create mode when no links exist", async () => {
    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));

    await waitFor(() => {
      expect(screen.getByText("Create share link")).toBeInTheDocument();
    });
    expect(screen.getByPlaceholderText("Recipient's Organization")).toBeInTheDocument();
  });

  it("renders Share, Access, and Documents tabs in the correct order", async () => {
    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    const tabs = ["Basic configuration", "Access control", "Scope"];
    tabs.forEach((label) => {
      expect(screen.getByText(label)).toBeInTheDocument();
    });
  });

  it("switches to Access tab", async () => {
    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    fireEvent.click(screen.getByText("Access control"));

    await waitFor(() => {
      expect(screen.getByText("Require email to view")).toBeInTheDocument();
    });
  });

  it("switches to Documents tab", async () => {
    vi.mocked(api.getDealRoomFolders).mockResolvedValue({
      data: [{ path: "/financials", name: "Financials", sort_order: 0 }],
    });

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    fireEvent.click(screen.getByText("Scope"));

    await waitFor(() => {
      expect(screen.getByText("Financials")).toBeInTheDocument();
    });
  });

  it("creates a link and persists access rules in a single request", async () => {
    vi.mocked(api.createDealRoomLink).mockResolvedValue({
      id: "link-1",
      name: "Acme DD",
      shortUrl: "http://localhost/l/abc123",
      requireEmail: true,
      requireEmailVerification: false,
      requirePassword: false,
      requireNda: false,
      downloadEnabled: false,
      watermarkEnabled: true,
      aiCopilotEnabled: false,
      folderPaths: [],
    } as unknown as Link);
    vi.mocked(api.setLinkAccessRules).mockResolvedValue(undefined);

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    fireEvent.change(screen.getByPlaceholderText("Recipient's Organization"), {
      target: { value: "Acme DD" },
    });

    // Add an allowed viewer; email gates now require allowed viewers.
    fireEvent.click(screen.getByText("Access control"));
    await waitFor(() => screen.getByText("Require email to view"));
    const allowedInput = screen.getByPlaceholderText(/alice@vc\.com/i);
    fireEvent.change(allowedInput, { target: { value: "alice@vc.com" } });
    fireEvent.keyDown(allowedInput, { key: "Enter" });

    const dialog = screen.getByRole("dialog");
    const createButtons = within(dialog).getAllByRole("button", { name: "Create link" });
    fireEvent.click(createButtons[createButtons.length - 1]);

    await waitFor(() => {
      expect(vi.mocked(api.createDealRoomLink)).toHaveBeenCalledWith(
        "room-1",
        expect.objectContaining({
          name: "Acme DD",
          allowed_emails: ["alice@vc.com"],
          blocked_emails: undefined,
        })
      );
    });
    // New links persist access rules in the create request; no separate rules call is needed.
    expect(vi.mocked(api.setLinkAccessRules)).not.toHaveBeenCalled();
  });

  it("creates a link with password, allowed viewers, and NDA in a single request", async () => {
    vi.mocked(api.createDealRoomLink).mockResolvedValue({
      id: "link-2",
      name: "Secure Link",
      shortUrl: "http://localhost/l/secure123",
      requireEmail: true,
      requireEmailVerification: false,
      requirePassword: true,
      requireNda: true,
      downloadEnabled: false,
      watermarkEnabled: true,
      aiCopilotEnabled: false,
      folderPaths: [],
    } as unknown as Link);

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    fireEvent.change(screen.getByPlaceholderText("Recipient's Organization"), {
      target: { value: "Secure Link" },
    });

    fireEvent.click(screen.getByText("Access control"));
    await waitFor(() => screen.getByText("Require email to view"));

    fireEvent.click(screen.getByRole("switch", { name: /require password to view/i }));
    const passwordInput = await screen.findByPlaceholderText(/enter password/i);
    fireEvent.change(passwordInput, {
      target: { value: "strong-pass-123" },
    });

    const allowedInput = screen.getByPlaceholderText(/alice@vc\.com/i);
    fireEvent.change(allowedInput, { target: { value: "alice@vc.com" } });
    fireEvent.keyDown(allowedInput, { key: "Enter" });

    fireEvent.click(screen.getByRole("switch", { name: /require NDA to view/i }));

    const ndaSelect = screen.getByRole("combobox", { name: /NDA agreement document/i });
    fireEvent.click(ndaSelect);
    const ndaOption = await screen.findByRole("option", { name: /NDA Agreement/i });
    fireEvent.click(ndaOption);

    fireEvent.click(screen.getByRole("button", { name: "Create link" }));

    await waitFor(() => {
      expect(vi.mocked(api.createDealRoomLink)).toHaveBeenCalledWith(
        "room-1",
        expect.objectContaining({
          name: "Secure Link",
          require_password: true,
          password: "strong-pass-123",
          allowed_emails: ["alice@vc.com"],
          require_nda: true,
          nda_document_id: "doc-1",
        })
      );
    });
    expect(vi.mocked(api.setLinkAccessRules)).not.toHaveBeenCalled();
  });

  it("disables create link button until required fields are valid", async () => {
    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    const dialog = screen.getByRole("dialog");
    const createButton = within(dialog).getByRole("button", { name: "Create link" });

    // Initially invalid: empty name and standard preset requires allowed viewers.
    expect(createButton).toBeDisabled();

    fireEvent.change(screen.getByPlaceholderText("Recipient's Organization"), {
      target: { value: "Acme DD" },
    });
    expect(createButton).toBeDisabled();

    fireEvent.click(screen.getByText("Access control"));
    await waitFor(() => screen.getByText("Require email to view"));
    const allowedInput = screen.getByPlaceholderText(/alice@vc\.com/i);
    fireEvent.change(allowedInput, { target: { value: "alice@vc.com" } });
    fireEvent.keyDown(allowedInput, { key: "Enter" });

    expect(createButton).toBeEnabled();
  });

  it("disables create link button when NDA is enabled without a selected document", async () => {
    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    fireEvent.change(screen.getByPlaceholderText("Recipient's Organization"), {
      target: { value: "Acme DD" },
    });

    fireEvent.click(screen.getByText("Access control"));
    await waitFor(() => screen.getByText("Require email to view"));
    const allowedInput = screen.getByPlaceholderText(/alice@vc\.com/i);
    fireEvent.change(allowedInput, { target: { value: "alice@vc.com" } });
    fireEvent.keyDown(allowedInput, { key: "Enter" });

    const dialog = screen.getByRole("dialog");
    const createButton = within(dialog).getByRole("button", { name: "Create link" });
    expect(createButton).toBeEnabled();

    fireEvent.click(screen.getByRole("switch", { name: /require NDA to view/i }));
    expect(createButton).toBeDisabled();

    const ndaSelect = await screen.findByRole("combobox", { name: /NDA agreement document/i });
    fireEvent.click(ndaSelect);
    const ndaOption = await screen.findByRole("option", { name: /NDA Agreement/i });
    fireEvent.click(ndaOption);

    expect(createButton).toBeEnabled();
  });

  it("keeps create link button disabled when password is too short", async () => {
    vi.mocked(api.createDealRoomLink).mockResolvedValue({
      id: "link-3",
    } as unknown as Link);

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    fireEvent.change(screen.getByPlaceholderText("Recipient's Organization"), {
      target: { value: "Short Password Link" },
    });

    fireEvent.click(screen.getByText("Access control"));
    await waitFor(() => screen.getByText("Require email to view"));

    // Add an allowed viewer so the email gate passes validation while we test password length.
    const allowedInput = screen.getByPlaceholderText(/alice@vc\.com/i);
    fireEvent.change(allowedInput, { target: { value: "alice@vc.com" } });
    fireEvent.keyDown(allowedInput, { key: "Enter" });

    fireEvent.click(screen.getByRole("switch", { name: /require password to view/i }));
    const passwordInput = await screen.findByPlaceholderText(/enter password/i);
    fireEvent.change(passwordInput, {
      target: { value: "short" },
    });

    const dialog = screen.getByRole("dialog");
    const createButton = within(dialog).getByRole("button", { name: "Create link" });
    expect(createButton).toBeDisabled();
    expect(screen.getByText(/at least 8 characters/i)).toBeInTheDocument();
    expect(vi.mocked(api.createDealRoomLink)).not.toHaveBeenCalled();
  });

  it("blocks create when link name already exists and does not show success", async () => {
    const { toast } = await import("sonner");
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({
      data: [
        {
          id: "link-existing",
          name: "Acme DD",
          shortUrl: "http://localhost/l/existing",
          dealRoomId: "room-1",
          status: "active",
          isActive: true,
        } as unknown as Link,
      ],
    });

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    fireEvent.change(screen.getByPlaceholderText("Recipient's Organization"), {
      target: { value: "acme dd" },
    });

    expect(screen.getByText("A link with this name already exists")).toBeInTheDocument();

    const dialog = screen.getByRole("dialog");
    const createButton = within(dialog).getByRole("button", { name: "Create link" });
    expect(createButton).toBeDisabled();
    expect(vi.mocked(api.createDealRoomLink)).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("echoes existing deal-room link settings in Share and Access tabs", async () => {
    const editLink: Link = {
      id: "link-edit",
      name: "Acme DD Edit",
      shortUrl: "http://localhost/l/ddr123",
      documentId: "doc-1",
      documentTitle: "Acme DD",
      requireEmail: true,
      requireEmailVerification: true,
      requirePassword: false,
      requireNda: true,
      ndaDocumentId: "doc-1",
      downloadEnabled: true,
      watermarkEnabled: true,
      aiCopilotEnabled: false,
      folderPaths: [],
      accessCount: 3,
      heatLevel: "warm",
      isBundle: false,
      documents: [],
      status: "active",
      isActive: true,
      dealRoomId: "room-1",
      customDomain: "dealroom.example.com",
      notifyOnAccess: true,
      createdAt: new Date().toISOString(),
    } as unknown as Link;

    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [editLink] });
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({
      data: [
        { ruleType: "email", value: "alice@vc.com", action: "allow" },
        { ruleType: "email", value: "leaker@bad.com", action: "block" },
      ],
    });

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1" linkId="link-edit">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme DD Edit")).toBeInTheDocument();
    });

    // Share tab should reflect loaded custom domain in the public URL input.
    expect((screen.getByLabelText(/Public link/i) as HTMLInputElement).value).toMatch(
      /dealroom\.example\.com/
    );
    expect(screen.getByRole("switch", { name: /Notify on access/i })).toBeChecked();

    fireEvent.click(screen.getByText("Access control"));
    await waitFor(() => {
      expect(screen.getByText("Require NDA to view")).toBeInTheDocument();
    });

    expect(screen.getByLabelText(/Require email to view/i)).toBeChecked();
    expect(screen.getByLabelText(/Require email verification/i)).toBeChecked();
    expect(screen.getByLabelText(/Require NDA to view/i)).toBeChecked();
    expect(screen.getByText("alice@vc.com")).toBeInTheDocument();
    expect(screen.getByText("leaker@bad.com")).toBeInTheDocument();
    expect(screen.getByLabelText(/Allow downloading/i)).toBeChecked();
  });

  it("echoes existing deal-room link settings and keeps them after save", async () => {
    const editLink: Link = {
      id: "link-edit",
      name: "Acme DD Edit",
      shortUrl: "http://localhost/l/ddr123",
      documentId: "doc-1",
      documentTitle: "Acme DD",
      requireEmail: true,
      requireEmailVerification: false,
      requirePassword: false,
      requireNda: false,
      downloadEnabled: true,
      watermarkEnabled: true,
      aiCopilotEnabled: false,
      folderPaths: [],
      accessCount: 3,
      heatLevel: "warm",
      isBundle: false,
      documents: [],
      status: "active",
      isActive: true,
      dealRoomId: "room-1",
      customDomain: "dealroom.example.com",
      notifyOnAccess: true,
      createdAt: new Date().toISOString(),
    } as unknown as Link;

    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [editLink] });
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({
      data: [
        { ruleType: "email", value: "alice@vc.com", action: "allow" },
        { ruleType: "email", value: "leaker@bad.com", action: "block" },
      ],
    });
    vi.mocked(api.updateLinkFull).mockResolvedValue(editLink);
    vi.mocked(api.setLinkAccessRules).mockResolvedValue(undefined);

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1" linkId="link-edit">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme DD Edit")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Access control"));
    await waitFor(() => {
      expect(screen.getByText("Require email to view")).toBeInTheDocument();
    });

    expect(screen.getByText("alice@vc.com")).toBeInTheDocument();
    expect(screen.getByText("leaker@bad.com")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Save access rules" }));

    await waitFor(() => {
      expect(vi.mocked(api.setLinkAccessRules)).toHaveBeenCalledWith(
        "link-edit",
        expect.arrayContaining([
          expect.objectContaining({ value: "alice@vc.com", action: "allow" }),
          expect.objectContaining({ value: "leaker@bad.com", action: "block" }),
        ])
      );
    });

    // After save/refetch the rules should still be echoed in the Access tab.
    expect(screen.getByText("alice@vc.com")).toBeInTheDocument();
    expect(screen.getByText("leaker@bad.com")).toBeInTheDocument();
  });

  it("falls back to getLinkById when the link is missing from the deal-room list", async () => {
    const editLink: Link = {
      id: "link-edit",
      name: "Acme DD Edit",
      shortUrl: "http://localhost/l/ddr123",
      documentId: "doc-1",
      documentTitle: "Acme DD",
      requireEmail: true,
      requireEmailVerification: true,
      requirePassword: false,
      requireNda: true,
      ndaDocumentId: "doc-1",
      downloadEnabled: true,
      watermarkEnabled: true,
      aiCopilotEnabled: false,
      folderPaths: [],
      accessCount: 3,
      heatLevel: "warm",
      isBundle: false,
      documents: [],
      status: "active",
      isActive: true,
      dealRoomId: "room-1",
      customDomain: "dealroom.example.com",
      notifyOnAccess: true,
      createdAt: new Date().toISOString(),
    } as unknown as Link;

    // The link is not present in the deal-room list (e.g. stale cache or
    // status filtering), so the dialog should fall back to a direct lookup.
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [] });
    vi.mocked(api.getLinkById).mockResolvedValue(editLink);
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({
      data: [
        { ruleType: "email", value: "alice@vc.com", action: "allow" },
        { ruleType: "email", value: "leaker@bad.com", action: "block" },
      ],
    });

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1" linkId="link-edit">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => {
      expect(screen.getByDisplayValue("Acme DD Edit")).toBeInTheDocument();
    });

    expect(api.getLinkById).toHaveBeenCalledWith("link-edit");

    fireEvent.click(screen.getByText("Access control"));
    await waitFor(() => {
      expect(screen.getByText("Require NDA to view")).toBeInTheDocument();
    });

    expect(screen.getByText("alice@vc.com")).toBeInTheDocument();
    expect(screen.getByText("leaker@bad.com")).toBeInTheDocument();
  });

  it("ignores a direct link lookup that does not belong to the deal room", async () => {
    const wrongLink: Link = {
      id: "link-edit",
      name: "Wrong Room Link",
      shortUrl: "http://localhost/l/ddr123",
      documentId: "doc-1",
      documentTitle: "Acme DD",
      requireEmail: false,
      requireEmailVerification: false,
      requirePassword: false,
      requireNda: false,
      downloadEnabled: false,
      watermarkEnabled: false,
      aiCopilotEnabled: false,
      folderPaths: [],
      accessCount: 0,
      heatLevel: "cold",
      isBundle: false,
      documents: [],
      status: "active",
      isActive: true,
      dealRoomId: "room-other",
      createdAt: new Date().toISOString(),
    } as unknown as Link;

    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [] });
    vi.mocked(api.getLinkById).mockResolvedValue(wrongLink);

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1" linkId="link-edit">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => {
      expect(screen.getByText("Create share link")).toBeInTheDocument();
    });

    expect(api.getLinkById).toHaveBeenCalledWith("link-edit");
  });

  it("loads deal-room folders on open", async () => {
    vi.mocked(api.getDealRoomFolders).mockResolvedValue({
      data: [{ path: "/financials", name: "Financials", sort_order: 0 }],
    });

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => {
      expect(screen.getByText("Create share link")).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(api.getDealRoomFolders).toHaveBeenCalledWith("room-1");
    });
  });

  it("creates a deal-room link with selected folder scope", async () => {
    vi.mocked(api.getDealRoomFolders).mockResolvedValue({
      data: [{ path: "/financials", name: "Financials", sort_order: 0 }],
    });
    vi.mocked(api.getDealRoomDocuments).mockResolvedValue({
      data: [
        {
          folder: "/financials",
          permission: "view" as const,
          documents: [
            { id: "doc-2", document_id: "doc-2", title: "P&L", folder_path: "/financials", sort_order: 0, source_type: "pdf", status: "ready", created_at: new Date().toISOString() },
          ],
        },
      ],
    });
    vi.mocked(api.createDealRoomLink).mockResolvedValue({
      id: "link-scoped",
      name: "Scoped Link",
      shortUrl: "http://localhost/l/scoped123",
      requireEmail: true,
      requireEmailVerification: false,
      requirePassword: false,
      requireNda: false,
      downloadEnabled: false,
      watermarkEnabled: true,
      aiCopilotEnabled: false,
      folderPaths: ["/financials"],
    } as unknown as Link);

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => screen.getByText("Create share link"));

    fireEvent.change(screen.getByPlaceholderText("Recipient's Organization"), {
      target: { value: "Scoped Link" },
    });

    fireEvent.click(screen.getByText("Scope"));
    await waitFor(() => {
      expect(screen.getByText("Financials")).toBeInTheDocument();
    });

    const financialsRow = screen.getByTestId("folder-row-/financials");
    const checkbox = financialsRow.querySelector('[role="checkbox"]') as HTMLElement;
    fireEvent.click(checkbox);

    fireEvent.click(screen.getByText("Access control"));
    await waitFor(() => screen.getByText("Require email to view"));
    const allowedInput = screen.getByPlaceholderText(/alice@vc\.com/i);
    fireEvent.change(allowedInput, { target: { value: "alice@vc.com" } });
    fireEvent.keyDown(allowedInput, { key: "Enter" });

    fireEvent.click(screen.getByRole("button", { name: "Create link" }));

    await waitFor(() => {
      expect(vi.mocked(api.createDealRoomLink)).toHaveBeenCalledWith(
        "room-1",
        expect.objectContaining({
          name: "Scoped Link",
          allowed_emails: ["alice@vc.com"],
          folder_paths: ["/financials"],
        })
      );
    });
  });

  it("updates a deal-room link with selected folder scope", async () => {
    const editLink: Link = {
      id: "link-scoped-edit",
      name: "Scoped Edit",
      shortUrl: "http://localhost/l/scoped-edit",
      documentId: "doc-1",
      documentTitle: "Acme DD",
      requireEmail: true,
      requireEmailVerification: false,
      requirePassword: false,
      requireNda: false,
      downloadEnabled: true,
      watermarkEnabled: true,
      aiCopilotEnabled: false,
      folderPaths: ["/financials"],
      accessCount: 3,
      heatLevel: "warm",
      isBundle: false,
      documents: [],
      status: "active",
      isActive: true,
      dealRoomId: "room-1",
      notifyOnAccess: true,
      createdAt: new Date().toISOString(),
    } as unknown as Link;

    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [editLink] });
    vi.mocked(api.getDealRoomFolders).mockResolvedValue({
      data: [
        { path: "/financials", name: "Financials", sort_order: 0 },
        { path: "/legal", name: "Legal", sort_order: 1 },
      ],
    });
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({
      data: [{ ruleType: "email", value: "alice@vc.com", action: "allow" }],
    });
    vi.mocked(api.updateLinkFull).mockResolvedValue(editLink);
    vi.mocked(api.setLinkAccessRules).mockResolvedValue(undefined);

    render(
      <Wrapper>
        <DealRoomShareDialog roomId="room-1" linkId="link-scoped-edit">
          <Button>Open</Button>
        </DealRoomShareDialog>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));
    await waitFor(() => {
      expect(screen.getByDisplayValue("Scoped Edit")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Scope"));
    await waitFor(() => {
      expect(screen.getByText("Financials")).toBeInTheDocument();
      expect(screen.getByText("Legal")).toBeInTheDocument();
    });

    const legalRow = screen.getByTestId("folder-row-/legal");
    const legalCheckbox = legalRow.querySelector('[role="checkbox"]') as HTMLElement;
    fireEvent.click(legalCheckbox);

    fireEvent.click(screen.getByRole("button", { name: "Save link settings" }));

    await waitFor(() => {
      expect(vi.mocked(api.updateLinkFull)).toHaveBeenCalledWith(
        "link-scoped-edit",
        expect.objectContaining({
          folder_paths: ["/financials", "/legal"],
        })
      );
    });
  });
});
