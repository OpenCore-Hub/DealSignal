// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { api } from "@/lib/api";
import type { Link, LinkAnalytics } from "@/types";
import { AnalyticsTab } from "./AnalyticsTab";
import enLinkShare from "@/i18n/locales/en/linkShare.json";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      linkShare: enLinkShare,
      common: { loading: "Loading..." },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

vi.mock("@/lib/api", () => ({
  api: {
    getLinkAnalytics: vi.fn(),
    listLinkQuestions: vi.fn(() => Promise.resolve({ data: [] })),
    listLinkFileRequests: vi.fn(() => Promise.resolve({ data: [] })),
    resendLinkAccessCode: vi.fn(() => Promise.resolve(undefined)),
    resendFailedLinkAccessCodes: vi.fn(() =>
      Promise.resolve({ data: { attempted: 0, sent: 0, failed: 0, skipped: 0 } }),
    ),
  },
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const baseLink = {
  id: "link-1",
  name: "Acme Corp",
  requireEmailVerification: true,
  accessCount: 0,
  avgDurationSeconds: 0,
} as unknown as Link;

const emptyAnalytics: LinkAnalytics = {
  total_views: 0,
  unique_visitors: 0,
  download_attempts: 0,
  views_over_time: [],
  average_duration_seconds: 0,
  recent_visitors: [],
  key_pages: [],
  qa_records: [],
  access_code_contacts: [],
};

describe("AnalyticsTab", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.listLinkQuestions).mockResolvedValue({ data: [] });
    vi.mocked(api.listLinkFileRequests).mockResolvedValue({ data: [] });
  });

  it("shows verification code delivery statuses for contacts", async () => {
    vi.mocked(api.getLinkAnalytics).mockResolvedValue({
      data: {
        ...emptyAnalytics,
        access_code_contacts: [
          {
            email: "ok@example.com",
            name: "Ok User",
            send_status: "sent",
            code_sent_at: "2026-07-20T02:00:00Z",
            can_resend: false,
          },
          {
            email: "bad@example.com",
            send_status: "failed",
            send_error: "SMTP timeout",
            can_resend: true,
          },
          {
            email: "wait@example.com",
            send_status: "pending",
            can_resend: false,
          },
        ],
      },
    });

    render(
      <Wrapper>
        <AnalyticsTab link={baseLink} logs={[]} />
      </Wrapper>,
    );

    expect(await screen.findByText("Verification code delivery")).toBeInTheDocument();
    expect(screen.getByText(/Ok User · ok@example.com/)).toBeInTheDocument();
    expect(screen.getByText("bad@example.com")).toBeInTheDocument();
    expect(screen.getByText("wait@example.com")).toBeInTheDocument();
    expect(screen.getAllByText("Sent").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Failed").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Pending").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("SMTP timeout")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Resend" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Resend undelivered" })).toBeInTheDocument();
  });

  it("resends only remediable contacts and refreshes analytics", async () => {
    vi.mocked(api.getLinkAnalytics)
      .mockResolvedValueOnce({
        data: {
          ...emptyAnalytics,
          access_code_contacts: [
            {
              email: "bad@example.com",
              send_status: "failed",
              can_resend: true,
            },
          ],
        },
      })
      .mockResolvedValueOnce({
        data: {
          ...emptyAnalytics,
          access_code_contacts: [
            {
              email: "bad@example.com",
              send_status: "sent",
              can_resend: false,
            },
          ],
        },
      });
    vi.mocked(api.resendLinkAccessCode).mockResolvedValue(undefined);

    render(
      <Wrapper>
        <AnalyticsTab link={baseLink} logs={[]} />
      </Wrapper>,
    );

    fireEvent.click(await screen.findByRole("button", { name: "Resend" }));
    await waitFor(() => {
      expect(api.resendLinkAccessCode).toHaveBeenCalledWith("link-1", "bad@example.com");
    });
    await waitFor(() => {
      expect(screen.queryByRole("button", { name: "Resend" })).not.toBeInTheDocument();
    });
  });

  it("hides verification code section when verification is off and no contacts", async () => {
    vi.mocked(api.getLinkAnalytics).mockResolvedValue({ data: emptyAnalytics });

    render(
      <Wrapper>
        <AnalyticsTab
          link={{ ...baseLink, requireEmailVerification: false } as Link}
          logs={[]}
        />
      </Wrapper>,
    );

    await waitFor(() => {
      expect(api.getLinkAnalytics).toHaveBeenCalledWith("link-1");
    });
    expect(screen.queryByText("Verification code delivery")).not.toBeInTheDocument();
  });
});
