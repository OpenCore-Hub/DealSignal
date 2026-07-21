// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { api } from "@/lib/api";
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
            emptyTitle: "No links yet",
            table: {
              name: "Name",
              link: "Link",
              views: "Views",
              lastViewed: "Last viewed",
              active: "Active",
              actions: "Actions",
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
            },
          },
        },
      },
      common: {
        loading: "Loading...",
        cancel: "Cancel",
        delete: "Delete",
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
    updateLink: vi.fn(),
    deleteLink: vi.fn(),
  },
}));

vi.mock("./DealRoomShareDialog", () => ({
  DealRoomShareDialog: ({
    children,
    onChanged,
  }: {
    children: React.ReactNode;
    onChanged?: () => void | Promise<void>;
  }) => (
    <div>
      {children}
      <button type="button" onClick={() => void onChanged?.()}>
        Simulate create done
      </button>
    </div>
  ),
}));

vi.mock("@/components/links/share", () => ({
  LinkActivityDialog: ({
    link,
    open,
  }: {
    link: { id: string; name?: string };
    open: boolean;
  }) => (open ? <div data-testid="link-activity-dialog">{link.name}</div> : null),
}));

vi.mock("./SendVerificationCodeDialog", () => ({
  SendVerificationCodeDialog: () => null,
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function makeLink(overrides: Partial<Link> = {}): Link {
  return {
    id: "link-1",
    name: "Investor pack",
    shortUrl: "http://localhost/l/abc",
    accessCount: 0,
    isActive: true,
    requireEmailVerification: false,
    ...overrides,
  } as Link;
}

describe("FolderPermissionsSection refresh", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("reloads links when refreshKey bumps after external create", async () => {
    vi.mocked(api.getDealRoomLinks)
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValueOnce({ data: [makeLink()] });

    const { rerender } = render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" refreshKey={0} />
      </I18nextProvider>,
    );

    expect(await screen.findByText("No links yet")).toBeInTheDocument();

    rerender(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" refreshKey={1} />
      </I18nextProvider>,
    );

    expect(await screen.findByText("Investor pack")).toBeInTheDocument();
    expect(api.getDealRoomLinks).toHaveBeenCalledTimes(2);
  });

  it("refetches when in-section create reports onChanged", async () => {
    vi.mocked(api.getDealRoomLinks)
      .mockResolvedValueOnce({ data: [] })
      .mockResolvedValueOnce({ data: [makeLink({ name: "New share" })] });

    render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" />
      </I18nextProvider>,
    );

    expect(await screen.findByText("No links yet")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Simulate create done"));

    await waitFor(() => {
      expect(screen.getByText("New share")).toBeInTheDocument();
    });
  });

  it("opens link activity when clicking a share link row", async () => {
    vi.mocked(api.getDealRoomLinks).mockResolvedValue({ data: [makeLink()] });

    render(
      <I18nextProvider i18n={i18nInstance}>
        <FolderPermissionsSection roomId="room-1" />
      </I18nextProvider>,
    );

    const row = await screen.findByTestId("deal-room-link-row-link-1");
    fireEvent.click(row);

    expect(await screen.findByTestId("link-activity-dialog")).toHaveTextContent("Investor pack");
  });
});