// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { api } from "@/lib/api";
import { DealRoomAccessControlTab } from "./DealRoomAccessControlTab";
import { createTestI18n } from "@/i18n/test-utils";

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomLinks: vi.fn(),
    getDealRoomDocuments: vi.fn(),
    getLinkAccessRules: vi.fn(),
    getLinkById: vi.fn(),
    updateLinkFull: vi.fn(),
    setLinkAccessRules: vi.fn(),
    listNDATemplates: vi.fn(),
    getDealRoomAccessRequests: vi.fn(),
  },
}));

vi.mock("./DealRoomAccessRequestsPanel", () => ({
  DealRoomAccessRequestsPanel: () => <div data-testid="room-access-requests" />,
}));

vi.mock("@/components/links/share", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/components/links/share")>();
  return {
    ...actual,
    AccessTab: ({ layout }: { layout?: string }) => (
      <div data-testid="access-tab" data-layout={layout ?? "compact"}>
        Access settings form
      </div>
    ),
    LinkAccessRequestsPanel: () => <div data-testid="link-access-requests" />,
  };
});

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

describe("DealRoomAccessControlTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    vi.mocked(api.getDealRoomDocuments).mockResolvedValue({ data: [] });
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({ data: [] });
    vi.mocked(api.listNDATemplates).mockResolvedValue({ data: [] });
    vi.mocked(api.getDealRoomAccessRequests).mockResolvedValue({ data: [] });
  });

  it("renders sectioned access control layout without page header", async () => {
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({
      data: [
        {
          id: "link-1",
          name: "Investor pack",
          shortUrl: "http://localhost/l/abc",
          dealRoomId: "room-1",
          requirePassword: false,
        } as never,
      ],
    });

    const i18n = await createTestI18n({
      dealRooms: {
        "accessControl.pageTitle": "Access control",
        "accessControl.pageDescription": "Configure visitor security",
        "accessControl.defaultsHint": "Saved as room defaults",
      },
      linkShare: {
        "accessRules.title": "Access control",
        "accessRules.saveAccessRules": "Save access rules",
        "accessRules.savedDescription": "Changes take effect immediately",
        "share.savedButtonLabel": "Saved",
      },
      common: { loading: "Loading...", saving: "Saving..." },
    });

    render(
      <I18nextProvider i18n={i18n}>
        <DealRoomAccessControlTab roomId="room-1" />
      </I18nextProvider>
    );

    expect(await screen.findByTestId("deal-room-access-control-tab")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Access control" })).not.toBeInTheDocument();
    expect(screen.queryByText("Configure visitor security")).not.toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByTestId("access-tab")).toHaveAttribute("data-layout", "sections");
    });
    expect(screen.getByRole("button", { name: "Save access rules" })).toBeInTheDocument();
  });

  it("still shows access control when there are no share links", async () => {
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [] });

    const i18n = await createTestI18n({
      dealRooms: {
        "accessControl.pageTitle": "Access control",
        "accessControl.pageDescription": "Configure visitor security",
        "accessControl.defaultsHint": "Saved as room defaults",
      },
      linkShare: {
        "accessRules.saveAccessRules": "Save access rules",
        "accessRules.savedDescription": "Changes take effect immediately",
        "share.savedButtonLabel": "Saved",
      },
      common: { loading: "Loading...", saving: "Saving..." },
    });

    render(
      <I18nextProvider i18n={i18n}>
        <DealRoomAccessControlTab roomId="room-1" />
      </I18nextProvider>
    );

    expect(await screen.findByTestId("access-tab")).toBeInTheDocument();
    expect(screen.queryByText("Saved as room defaults")).not.toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Save access rules" }).length).toBeGreaterThan(0);
  });
});
