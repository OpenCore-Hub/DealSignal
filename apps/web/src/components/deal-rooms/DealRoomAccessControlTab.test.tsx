// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { api } from "@/lib/api";
import { DealRoomAccessControlTab } from "./DealRoomAccessControlTab";
import { createTestI18n } from "@/i18n/test-utils";

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomAccessPolicy: vi.fn(),
    upsertDealRoomAccessPolicy: vi.fn(),
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
    ContactEmailTagInput: ({
      values,
      onChange,
    }: {
      values: string[];
      onChange: (values: string[]) => void;
    }) => (
      <input
        data-testid="blocked-emails-input"
        value={values.join(",")}
        onChange={(e) =>
          onChange(
            e.target.value
              .split(",")
              .map((v) => v.trim())
              .filter(Boolean),
          )
        }
      />
    ),
  };
});

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const emptyPolicy = {
  dealRoomId: "room-1",
  configured: false,
  requireEmailVerificationFloor: false,
  requireNdaFloor: false,
  blockedEmails: [] as string[],
};

describe("DealRoomAccessControlTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getDealRoomAccessPolicy).mockResolvedValue({ data: emptyPolicy });
    vi.mocked(api.getDealRoomAccessRequests).mockResolvedValue({ data: [] });
    vi.mocked(api.upsertDealRoomAccessPolicy).mockResolvedValue({
      data: { ...emptyPolicy, configured: true },
    });
  });

  it("renders thin room security form without full AccessTab", async () => {
    const i18n = await createTestI18n({
      dealRooms: {
        "accessControl.blocklistTitle": "Access blacklist",
        "accessControl.floorsTitle": "Security domain",
        "accessControl.floorMustVerify": "Must verify email",
        "accessControl.floorMustNda": "Must require NDA",
        "accessControl.saveButton": "Save access policy",
        "accessControl.saveHint": "Access blacklist syncs",
      },
      linkShare: {
        "accessRules.blockedViewers.placeholder": "bad@example.com",
        "accessRules.blockedViewers.roomHint": "Blocked for every link",
        "accessRules.saved": "Saved",
        "share.savedButtonLabel": "Saved",
      },
      common: { loading: "Loading...", saving: "Saving..." },
    });

    render(
      <I18nextProvider i18n={i18n}>
        <DealRoomAccessControlTab roomId="room-1" />
      </I18nextProvider>,
    );

    expect(await screen.findByTestId("deal-room-access-control-tab")).toBeInTheDocument();
    expect(screen.getByTestId("room-access-requests")).toBeInTheDocument();
    expect(await screen.findByTestId("room-security-form")).toBeInTheDocument();
    expect(screen.queryByTestId("access-tab")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save access policy" })).toBeInTheDocument();
    expect(screen.getByLabelText("Must verify email")).toBeInTheDocument();
    expect(screen.getByLabelText("Must require NDA")).toBeInTheDocument();
  });

  it("saves thin floor + blocklist payload", async () => {
    const i18n = await createTestI18n({
      dealRooms: {
        "accessControl.blocklistTitle": "Access blacklist",
        "accessControl.floorsTitle": "Security domain",
        "accessControl.floorMustVerify": "Must verify email",
        "accessControl.floorMustNda": "Must require NDA",
        "accessControl.saveButton": "Save access policy",
        "accessControl.saveHint": "Access blacklist syncs",
      },
      linkShare: {
        "accessRules.blockedViewers.placeholder": "bad@example.com",
        "accessRules.blockedViewers.roomHint": "Blocked for every link",
        "accessRules.saved": "Saved",
        "share.savedButtonLabel": "Saved",
      },
      common: { loading: "Loading...", saving: "Saving...", "error.saveFailed": "Save failed" },
    });

    render(
      <I18nextProvider i18n={i18n}>
        <DealRoomAccessControlTab roomId="room-1" />
      </I18nextProvider>,
    );

    await waitFor(() => expect(screen.getByTestId("room-security-form")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("Must verify email"));
    fireEvent.click(screen.getByRole("button", { name: "Save access policy" }));

    await waitFor(() => {
      expect(api.upsertDealRoomAccessPolicy).toHaveBeenCalledWith("room-1", {
        require_email_verification_floor: true,
        require_nda_floor: false,
        require_email_verification: true,
        require_nda: false,
        blocked_emails: [],
      });
    });
  });

  it("reports dirty then clean after save", async () => {
    const onDirtyChange = vi.fn();
    const i18n = await createTestI18n({
      dealRooms: {
        "accessControl.blocklistTitle": "Access blacklist",
        "accessControl.floorsTitle": "Security domain",
        "accessControl.floorMustVerify": "Must verify email",
        "accessControl.floorMustNda": "Must require NDA",
        "accessControl.saveButton": "Save access policy",
        "accessControl.saveHint": "Access blacklist syncs",
      },
      linkShare: {
        "accessRules.blockedViewers.placeholder": "bad@example.com",
        "accessRules.blockedViewers.roomHint": "Blocked for every link",
        "accessRules.saved": "Saved",
        "share.savedButtonLabel": "Saved",
      },
      common: { loading: "Loading...", saving: "Saving...", "error.saveFailed": "Save failed" },
    });

    render(
      <I18nextProvider i18n={i18n}>
        <DealRoomAccessControlTab roomId="room-1" onDirtyChange={onDirtyChange} />
      </I18nextProvider>,
    );

    await waitFor(() => expect(screen.getByTestId("room-security-form")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("Must verify email"));
    expect(onDirtyChange).toHaveBeenCalledWith(true);

    fireEvent.click(screen.getByRole("button", { name: "Save access policy" }));
    await waitFor(() => expect(onDirtyChange).toHaveBeenCalledWith(false));
  });
});
