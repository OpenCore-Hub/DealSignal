// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { toast } from "sonner";
import { SettingsSecurityPage } from "./security";

const { getSecuritySettingsMock, getBillingInfoMock, updateSecuritySettingsMock } = vi.hoisted(
  () => ({
    getSecuritySettingsMock: vi.fn(),
    getBillingInfoMock: vi.fn(),
    updateSecuritySettingsMock: vi.fn(),
  }),
);

vi.mock("@/lib/api", () => ({
  api: {
    getSecuritySettings: getSecuritySettingsMock,
    getBillingInfo: getBillingInfoMock,
    updateSecuritySettings: updateSecuritySettingsMock,
  },
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

async function initI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    resources: {
      en: {
        settings: {
          security: {
            title: "Security",
            forceEmailVerification: "Force email verification",
            forceEmailVerificationDescription: "Require email verification",
            watermarkDownloads: "Watermark downloads",
            watermarkDownloadsDescription: "Add visitor email watermark",
            watermarkPlanRequired: "Watermark downloads require Pro or higher",
            twoFactor: "Two-factor authentication",
            twoFactorDescription: "Enable 2FA",
            configure: "Configure",
            twoFactorDisabled: "2FA requires backend support",
            auditLog: "Access audit",
            auditLogDescription: "View access events",
            viewAuditLog: "View audit log",
            loadFailed: "Failed to load",
          },
        },
        common: {
          retry: "Retry",
          error: {
            saveFailed: "Failed to save",
            codes: {
              plan_feature_watermark:
                "Watermark and viewer protection features are not available on your plan. Upgrade to Pro or higher.",
            },
          },
        },
      },
    },
  });
  return instance;
}

function renderPage(i18nInstance: Awaited<ReturnType<typeof initI18n>>) {
  return render(
    <I18nextProvider i18n={i18nInstance}>
      <MemoryRouter initialEntries={["/acme/settings/security"]}>
        <Routes>
          <Route path="/:workspaceSlug/settings/security" element={<SettingsSecurityPage />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("SettingsSecurityPage", () => {
  beforeEach(() => {
    getSecuritySettingsMock.mockReset();
    getBillingInfoMock.mockReset();
    updateSecuritySettingsMock.mockReset();
    getSecuritySettingsMock.mockResolvedValue({
      forceEmailVerification: false,
      watermarkDownloads: false,
      twoFactorEnabled: false,
    });
    getBillingInfoMock.mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 1 << 30,
      linksUsed: 0,
      linksLimit: 20,
      roomsUsed: 0,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });
  });

  it("disables enabling watermark when plan lacks the feature", async () => {
    const i18nInstance = await initI18n();
    renderPage(i18nInstance);

    await waitFor(() => {
      expect(screen.getByText(/Watermark downloads require Pro/)).toBeInTheDocument();
    });
    const switches = screen.getAllByRole("switch");
    const watermarkSwitch = switches[1];
    expect(watermarkSwitch).toBeDisabled();
    expect(updateSecuritySettingsMock).not.toHaveBeenCalled();
  });

  it("allows turning watermark off when grandfathered on free", async () => {
    getSecuritySettingsMock.mockResolvedValue({
      forceEmailVerification: false,
      watermarkDownloads: true,
      twoFactorEnabled: false,
    });
    updateSecuritySettingsMock.mockResolvedValue({
      forceEmailVerification: false,
      watermarkDownloads: false,
      twoFactorEnabled: false,
    });
    const i18nInstance = await initI18n();
    renderPage(i18nInstance);

    await waitFor(() => {
      expect(screen.getByText(/Add visitor email watermark/)).toBeInTheDocument();
    });
    const watermarkSwitch = screen.getAllByRole("switch")[1];
    expect(watermarkSwitch).not.toBeDisabled();
  });

  it("surfaces plan_feature_watermark when enable races past the client gate", async () => {
    const { ApiError } = await import("@/lib/apiClient");
    // Stale client billing still shows entitled; server denies.
    getBillingInfoMock.mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 1 << 30,
      linksUsed: 0,
      linksLimit: 20,
      roomsUsed: 0,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: true,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });
    updateSecuritySettingsMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "plan_feature_watermark",
        message: "watermark not available",
        requestId: "r1",
      }),
    );

    const i18nInstance = await initI18n();
    renderPage(i18nInstance);

    await waitFor(() => {
      expect(screen.getByText(/Add visitor email watermark/)).toBeInTheDocument();
    });
    const watermarkSwitch = screen.getAllByRole("switch")[1];
    expect(watermarkSwitch).not.toBeDisabled();
    fireEvent.click(watermarkSwitch);

    await waitFor(() => {
      expect(updateSecuritySettingsMock).toHaveBeenCalledWith(
        expect.objectContaining({ watermarkDownloads: true }),
      );
      expect(toast.error).toHaveBeenCalled();
    });
    const [[message]] = vi.mocked(toast.error).mock.calls;
    expect(String(message)).toMatch(/Watermark|not available|Failed to save/i);
  });
});
