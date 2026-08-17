// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { api } from "@/lib/api";
import { copyToClipboard } from "@/lib/clipboard";
import { FolderPermissionsSection } from "./FolderPermissionsSection";
import type { Link } from "@/types";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      dealRooms: {
        permissions: {
          links: {
            title: "Share links",
            createLink: "Create link",
            createNewLink: "Create new link",
            bulkDelete: "Batch delete",
            searchPlaceholder: "Search links…",
            searchAria: "Search share links",
            noSearchResults: "No links match your search.",
            selectAll: "Select all links",
            selectRow: "Select {{name}}",
            emptyTitle: "No links yet",
            table: {
              name: "Name",
              link: "Link",
              views: "Visits",
              visits: "Visits",
              visitsHint:
                "Visits are the share access count (quota). Last viewed excludes workspace members.",
              lastViewed: "Last viewed",
              createdAt: "Created",
              sortCreatedDesc: "Sort by created time, descending",
              sortCreatedAsc: "Sort by created time, ascending",
              active: "Active",
              actions: "Actions",
            },
            pagination: {
              first: "First",
              last: "Last",
              prev: "Previous page",
              next: "Next page",
              pageInput: "Page number",
              pageOf: "Page {{page}} of {{totalPages}}",
              go: "Go",
            },
            actions: {
              view: "View",
              edit: "Edit",
              sendCode: "Send code",
              delete: "Delete",
            },
            delete: {
              title: "Delete",
              description: "Delete {{name}}?",
              success: "Deleted",
              error: "Failed",
              loading: "Deleting…",
              bulkTitle: "Delete selected links",
              bulkDescription: "Delete {{count}} selected share link(s)?",
              bulkSuccess: "Deleted {{count}} link(s)",
              bulkPartialError: "Deleted {{succeeded}}, failed {{failed}}",
            },
          },
        },
        share: {
          copyLink: "Copy link",
          copied: "Link copied",
        },
      },
      common: {
        loading: "Loading...",
        cancel: "Cancel",
        delete: "Delete",
        retry: "Retry",
        emDash: "—",
        "error.saveFailed": "Save failed",
        "error.codes.plan_limit_links":
          "You've reached the share link limit for your plan. Upgrade to create more.",
      },
      linkShare: {
        activity: { title: "Link activity" },
      },
    },
  },
  interpolation: { escapeValue: false },
});

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomLinks: vi.fn(),
    getPendingLinkAccessRequests: vi.fn(),
    updateLink: vi.fn(),
    deleteLink: vi.fn(),
  },
}));

vi.mock("@/lib/clipboard", () => ({
  copyToClipboard: vi.fn(() => Promise.resolve(true)),
}));

vi.mock("./DealRoomShareDialog", () => ({
  DealRoomShareDialog: ({
    children,
    open,
  }: {
    children?: React.ReactNode;
    open?: boolean;
  }) => (open || children ? <div data-testid="deal-room-share-dialog">{children}</div> : null),
}));

vi.mock("@/components/links/share", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/components/links/share")>();
  return {
    ...actual,
    LinkActivityDialog: ({
      link,
      open,
    }: {
      link: { id: string; name?: string };
      open: boolean;
    }) => (open ? <div data-testid="link-activity-dialog">{link.name}</div> : null),
  };
});

vi.mock("./SendVerificationCodeDialog", () => ({
  SendVerificationCodeDialog: () => null,
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), custom: vi.fn(), dismiss: vi.fn() },
}));

function makeLink(overrides: Partial<Link> = {}): Link {
  return {
    id: "link-1",
    name: "Investor pack",
    shortUrl: "http://localhost/l/abc",
    accessCount: 0,
    isActive: true,
    requireEmailVerification: false,
    createdAt: "2026-06-20T10:00:00Z",
    ...overrides,
  } as Link;
}

function pageResponse(data: Link[], total = data.length, page = 1, pageSize = 10) {
  return {
    data,
    pagination: {
      page,
      page_size: pageSize,
      total,
      has_more: page * pageSize < total,
    },
  };
}

describe("FolderPermissionsSection refresh", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  it("reloads links when refreshKey bumps after external create", async () => {
    vi.mocked(api.getDealRoomLinks)
      .mockResolvedValueOnce(pageResponse([]))
      .mockResolvedValueOnce(pageResponse([makeLink()]));

    const { rerender } = render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" canManage refreshKey={0} />
      </I18nextProvider>,
    );

    expect(await screen.findByText("No links yet")).toBeInTheDocument();

    rerender(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" canManage refreshKey={1} />
      </I18nextProvider>,
    );

    expect(await screen.findByText("Investor pack")).toBeInTheDocument();
    expect(api.getDealRoomLinks).toHaveBeenCalledTimes(2);
  });

  it("opens create share dialog without requiring room security setup first", async () => {
    vi.mocked(api.getDealRoomLinks).mockResolvedValue(pageResponse([]));

    render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" canManage />
      </I18nextProvider>,
    );

    expect(await screen.findByText("No links yet")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Create link" }));
    expect(screen.getByTestId("deal-room-share-dialog")).toBeInTheDocument();
  });

  it("labels share access count as visits, not analytics views", async () => {
    vi.mocked(api.getDealRoomLinks).mockResolvedValue(pageResponse([makeLink()]));
    vi.mocked(api.getPendingLinkAccessRequests).mockResolvedValue({ data: [] });

    render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" canManage />
      </I18nextProvider>,
    );

    expect(await screen.findByText("Visits")).toBeInTheDocument();
    expect(
      screen.getByText(/Visits are the share access count/),
    ).toBeInTheDocument();
  });

  it("opens link activity when clicking a share link row", async () => {
    vi.mocked(api.getDealRoomLinks).mockResolvedValue(pageResponse([makeLink()]));
    vi.mocked(api.getPendingLinkAccessRequests).mockResolvedValue({ data: [] });

    render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" canManage />
      </I18nextProvider>,
    );

    const row = await screen.findByTestId("deal-room-link-row-link-1");
    fireEvent.click(row);

    expect(await screen.findByTestId("link-activity-dialog")).toHaveTextContent("Investor pack");
  });

  it("copies the full short URL from the link column copy button", async () => {
    const link = makeLink({
      shortUrl: "http://localhost/l/13bb3665bfd8254deae860c0cd20ffa6",
    });
    vi.mocked(api.getDealRoomLinks).mockResolvedValue(pageResponse([link]));
    vi.mocked(api.getPendingLinkAccessRequests).mockResolvedValue({ data: [] });

    render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" canManage />
      </I18nextProvider>,
    );

    const copyBtn = await screen.findByTestId("deal-room-link-copy-link-1");
    fireEvent.click(copyBtn);
    expect(copyToClipboard).toHaveBeenCalledWith(
      "http://localhost/l/13bb3665bfd8254deae860c0cd20ffa6",
      "Link copied",
    );
    expect(screen.queryByTestId("link-activity-dialog")).not.toBeInTheDocument();
  });

  it("shows create-new-link above the table when links already exist", async () => {
    vi.mocked(api.getDealRoomLinks).mockResolvedValue(pageResponse([makeLink()]));
    vi.mocked(api.getPendingLinkAccessRequests).mockResolvedValue({ data: [] });

    render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" canManage />
      </I18nextProvider>,
    );

    const createBtn = await screen.findByTestId("deal-room-create-new-link");
    expect(createBtn).toHaveTextContent("Create new link");
    fireEvent.click(createBtn);
    expect(screen.getByTestId("deal-room-share-dialog")).toBeInTheDocument();
  });

  it("requests paginated links with created_at_desc by default", async () => {
    vi.mocked(api.getDealRoomLinks).mockResolvedValue(pageResponse([makeLink()]));
    vi.mocked(api.getPendingLinkAccessRequests).mockResolvedValue({ data: [] });

    render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" canManage />
      </I18nextProvider>,
    );

    await screen.findByTestId("deal-room-link-row-link-1");
    expect(api.getDealRoomLinks).toHaveBeenCalledWith("room-1", {
      page: 1,
      page_size: 10,
      sort: "created_at_desc",
      q: undefined,
    });
    expect(screen.getByTestId("deal-room-links-pagination")).toBeInTheDocument();
    expect(screen.getByTestId("deal-room-links-sort-created")).toBeInTheDocument();
  });

  it("cycles created-at sort: first click keeps desc, second click asc", async () => {
    vi.mocked(api.getDealRoomLinks).mockResolvedValue(pageResponse([makeLink()]));
    vi.mocked(api.getPendingLinkAccessRequests).mockResolvedValue({ data: [] });

    render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" canManage />
      </I18nextProvider>,
    );

    await screen.findByTestId("deal-room-link-row-link-1");
    const sortBtn = screen.getByTestId("deal-room-links-sort-created");
    expect(sortBtn).toHaveAttribute("data-sort", "desc");

    fireEvent.click(sortBtn);
    expect(sortBtn).toHaveAttribute("data-sort", "desc");
    expect(api.getDealRoomLinks).toHaveBeenCalledWith(
      "room-1",
      expect.objectContaining({ sort: "created_at_desc" }),
    );

    fireEvent.click(sortBtn);
    await waitFor(() => {
      expect(api.getDealRoomLinks).toHaveBeenCalledWith(
        "room-1",
        expect.objectContaining({ sort: "created_at_asc", page: 1 }),
      );
    });
    expect(sortBtn).toHaveAttribute("data-sort", "asc");
  });

  it("searches via backend q and batch-deletes selected rows", async () => {
    vi.mocked(api.getDealRoomLinks).mockImplementation(async (_roomId, opts) => {
      const all = [
        makeLink({ id: "link-1", name: "Investor pack", shortUrl: "http://localhost/l/abc" }),
        makeLink({ id: "link-2", name: "Bank pack", shortUrl: "http://localhost/l/xyz" }),
      ];
      const q = (opts?.q || "").toLowerCase();
      const data = q ? all.filter((l) => (l.name || "").toLowerCase().includes(q)) : all;
      return pageResponse(data);
    });
    vi.mocked(api.getPendingLinkAccessRequests).mockResolvedValue({ data: [] });
    vi.mocked(api.deleteLink).mockResolvedValue(undefined as never);

    render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" canManage />
      </I18nextProvider>,
    );

    await screen.findByTestId("deal-room-link-row-link-1");
    const toolbar = screen.getByTestId("deal-room-links-toolbar");
    expect(toolbar).toHaveTextContent("Create new link");
    expect(toolbar).toHaveTextContent("Batch delete");

    fireEvent.change(screen.getByLabelText("Search share links"), {
      target: { value: "Bank" },
    });

    await waitFor(() => {
      expect(api.getDealRoomLinks).toHaveBeenCalledWith(
        "room-1",
        expect.objectContaining({ q: "Bank" }),
      );
    });
    expect(await screen.findByTestId("deal-room-link-row-link-2")).toBeInTheDocument();
    expect(screen.queryByTestId("deal-room-link-row-link-1")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("checkbox", { name: "Select Bank pack" }));
    const bulkBtn = screen.getByTestId("deal-room-bulk-delete-links");
    expect(bulkBtn).toBeEnabled();
    fireEvent.click(bulkBtn);
    fireEvent.click(screen.getByTestId("deal-room-bulk-delete-confirm"));

    await waitFor(() => {
      expect(api.deleteLink).toHaveBeenCalledWith("link-2");
    });
  });

  it("toggles Active via updateLink and surfaces plan_limit_links on reactivate denial", async () => {
    const { toast } = await import("sonner");
    const { ApiError } = await import("@/lib/apiClient");
    vi.mocked(api.getDealRoomLinks).mockResolvedValue(
      pageResponse([makeLink({ id: "link-1", name: "Investor pack", isActive: false })]),
    );
    vi.mocked(api.getPendingLinkAccessRequests).mockResolvedValue({ data: [] });
    vi.mocked(api.updateLink).mockRejectedValue(
      new ApiError({
        status: 403,
        code: "plan_limit_links",
        message: "share link limit reached for this plan",
        requestId: "r1",
      }),
    );

    render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" canManage />
      </I18nextProvider>,
    );

    await screen.findByTestId("deal-room-link-row-link-1");
    const toggle = screen.getByRole("switch", { name: "Active" });
    expect(toggle).toHaveAttribute("aria-checked", "false");
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(api.updateLink).toHaveBeenCalledWith("link-1", { status: "active" });
      expect(toast.error).toHaveBeenCalled();
    });
    const [[message]] = vi.mocked(toast.error).mock.calls;
    expect(String(message)).toMatch(/share link limit/i);
  });
});
