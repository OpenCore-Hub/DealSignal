// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useSearchParams } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { SettingsBillingPage } from "./billing";
import { toast } from "sonner";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getBillingInfoMock } = vi.hoisted(() => ({
  getBillingInfoMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getBillingInfo: getBillingInfoMock,
    createBillingPortal: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}));

async function initI18n() {
  const instance = i18n.createInstance();
  const settingsJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/settings.json"), "utf-8"),
  );
  const commonJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"),
  );
  const formattersJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/formatters.json"), "utf-8"),
  );
  await instance.use(initReactI18next).init({
    lng: "en",
    resources: {
      en: { settings: settingsJson, common: commonJson, formatters: formattersJson },
    },
    interpolation: { escapeValue: false },
  });
  return instance;
}

function SearchProbe() {
  const [params] = useSearchParams();
  return <div data-testid="billing-search">{params.toString()}</div>;
}

function renderPage(
  i18nInstance: Awaited<ReturnType<typeof initI18n>>,
  path = "/acme/settings/billing",
) {
  return render(
    <I18nextProvider i18n={i18nInstance}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route
            path="/:workspaceSlug/settings/billing"
            element={
              <>
                <SettingsBillingPage />
                <SearchProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

function trialBilling(overrides: Record<string, unknown> = {}) {
  return {
    plan: "trial",
    period: "monthly",
    trialExpired: false,
    trialEndsAt: "2026-08-27T00:00:00Z",
    storageUsed: 0,
    storageLimit: 500 * 1024 * 1024 * 1024,
    linksUsed: 0,
    linksLimit: 0,
    roomsUsed: 0,
    roomsLimit: 0,
    seatsUsed: 1,
    seatsLimit: 10,
    documentsUsed: 0,
    documentsLimit: 1000,
    askAiUsed: 0,
    askAiLimit: 1000,
    maxUploadBytes: 250 * 1024 * 1024,
    customDomainEnabled: true,
    watermarkEnabled: true,
    ndaEnabled: true,
    visitorAskAiEnabled: true,
    brandingEnabled: true,
    accessControlsEnabled: true,
    knowledgeDeskEnabled: true,
    webhooksEnabled: true,
    hubspotEnabled: true,
    dailyDigestEnabled: true,
    slackAlertsEnabled: true,
    roomAnalyticsEnabled: true,
    roomInsightsEnabled: true,
    formalAskEnabled: true,
    knowledgeAnswersUsed: 0,
    knowledgeAnswersLimit: 200,
    hasStripeSubscription: false,
    ...overrides,
  };
}

describe("SettingsBillingPage", () => {
  beforeEach(() => {
    vi.mocked(toast.success).mockReset();
    vi.mocked(toast.info).mockReset();
    vi.mocked(toast.error).mockReset();
    getBillingInfoMock.mockReset();
    getBillingInfoMock.mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 5 * 1024 * 1024,
      storageLimit: 2 * 1024 * 1024 * 1024,
      linksUsed: 3,
      linksLimit: 20,
      roomsUsed: 1,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      documentsUsed: 12,
      documentsLimit: 50,
      askAiUsed: 0,
      askAiLimit: 0,
      maxUploadBytes: 25 * 1024 * 1024,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
      brandingEnabled: false,
      accessControlsEnabled: false,
      knowledgeDeskEnabled: false,
      webhooksEnabled: false,
      hubspotEnabled: false,
      dailyDigestEnabled: false,
      slackAlertsEnabled: false,
      roomAnalyticsEnabled: false,
      roomInsightsEnabled: false,
      formalAskEnabled: false,
    });
  });

  it("renders localized plan and byte-accurate usage without NaN", async () => {
    const i18nInstance = await initI18n();
    renderPage(i18nInstance);

    await waitFor(() => {
      expect(screen.getByText(/Free/)).toBeInTheDocument();
    });
    expect(screen.getByText(/Monthly/)).toBeInTheDocument();
    expect(screen.queryByText(/NaN/)).not.toBeInTheDocument();
    expect(screen.getByText(/5 MB/)).toBeInTheDocument();
    expect(screen.getByText(/2 GB/)).toBeInTheDocument();
    expect(screen.getByText(/12 \/ 50/)).toBeInTheDocument();
    expect(screen.getByText(/3 \/ 20/)).toBeInTheDocument();
    expect(screen.getAllByText(/not included/i).length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText(/1 \/ 1/).length).toBeGreaterThanOrEqual(2);
    expect(screen.getByTestId("billing-features")).toBeInTheDocument();
    const upgrade = screen.getByTestId("billing-upgrade");
    expect(upgrade).toHaveAttribute("href", "/acme/settings/billing/plans");
    expect(upgrade.getAttribute("href")).not.toContain("mailto:");
  });

  it("shows expired trial copy when trialExpired is true", async () => {
    getBillingInfoMock.mockResolvedValue({
      plan: "trial",
      period: "monthly",
      trialExpired: true,
      trialEndsAt: "2026-08-01T00:00:00Z",
      storageUsed: 0,
      storageLimit: 2 << 30,
      linksUsed: 0,
      linksLimit: 20,
      roomsUsed: 0,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      documentsUsed: 0,
      documentsLimit: 50,
      askAiUsed: 0,
      askAiLimit: 0,
      maxUploadBytes: 25 * 1024 * 1024,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
      brandingEnabled: false,
      accessControlsEnabled: false,
      knowledgeDeskEnabled: false,
      webhooksEnabled: false,
      hubspotEnabled: false,
      dailyDigestEnabled: false,
      slackAlertsEnabled: false,
      roomAnalyticsEnabled: false,
      roomInsightsEnabled: false,
      formalAskEnabled: false,
    });
    const i18nInstance = await initI18n();
    renderPage(i18nInstance);
    await waitFor(() => {
      expect(screen.getByText(/Trial/)).toBeInTheDocument();
    });
    expect(screen.getByText(/Expired/)).toBeInTheDocument();
    expect(screen.getByText(/0 \/ 20/)).toBeInTheDocument();
    expect(screen.getAllByText(/1 \/ 1/).length).toBeGreaterThanOrEqual(1);
  });

  it("shows active trial finite storage/rooms, unlimited links, ends date, and features", async () => {
    getBillingInfoMock.mockResolvedValue(
      trialBilling({
        trialEndsAt: "2026-08-20T00:00:00Z",
        storageUsed: 1024,
        storageLimit: 500 * 1024 * 1024 * 1024,
        linksUsed: 1,
        linksLimit: 0,
        roomsUsed: 2,
        roomsLimit: 0,
        documentsUsed: 40,
        documentsLimit: 1000,
        askAiUsed: 12,
        askAiLimit: 1000,
        knowledgeAnswersUsed: 0,
        knowledgeAnswersLimit: 200,
      }),
    );
    const i18nInstance = await initI18n();
    renderPage(i18nInstance);
    await waitFor(() => {
      expect(screen.getByTestId("billing-plan-summary")).toHaveTextContent(/Trial/);
    });
    expect(screen.getByTestId("billing-plan-summary")).toHaveTextContent(/Trial ends/);
    expect(screen.queryByText(/Expired/)).not.toBeInTheDocument();
    // Storage / seats / documents / ask are finite; links and rooms unlimited (Business-aligned trial).
    expect(screen.getByText(/40 \/ 1000 \(4%\)/)).toBeInTheDocument();
    expect(screen.getByText(/12 \/ 1000 \(1%\)/)).toBeInTheDocument();
    expect(screen.getByText(/0 \/ 200 \(0%\)/)).toBeInTheDocument();
    expect(screen.getByText(/1 \/ 10 \(10%\)/)).toBeInTheDocument();
    expect(screen.getAllByText(/1 \/ Unlimited/)).toHaveLength(1);
    expect(screen.getAllByTestId("usage-bar-unlimited").length).toBeGreaterThanOrEqual(2);
    const features = screen.getByTestId("billing-features");
    expect(features).toHaveTextContent("Custom domain");
    expect(features).toHaveTextContent("Visitor Ask AI");
    expect(features).toHaveTextContent("Webhooks");
    expect(features).toHaveTextContent("HubSpot CRM");
    expect(features).toHaveTextContent("Knowledge Desk");
    expect(features).toHaveTextContent("Data Room insights");
    expect(features).toHaveTextContent("Formal Ask");
    expect(features).not.toHaveTextContent("24/7 Email support");
    expect(screen.queryByText(/NaN/)).not.toBeInTheDocument();
    expect(screen.queryByText(/not available yet/i)).not.toBeInTheDocument();
  });

  it("toasts and strips checkout=cancel without polling", async () => {
    getBillingInfoMock.mockResolvedValue(trialBilling());
    const i18nInstance = await initI18n();
    renderPage(i18nInstance, "/acme/settings/billing?checkout=cancel");

    await waitFor(() => {
      expect(toast.info).toHaveBeenCalledWith("Checkout canceled — your plan was not changed");
    });
    await waitFor(() => {
      expect(screen.getByTestId("billing-search")).not.toHaveTextContent("checkout=");
    });
    expect(toast.success).not.toHaveBeenCalled();
    expect(getBillingInfoMock).toHaveBeenCalledTimes(1);
  });

  it("polls after checkout=success until Stripe plan is visible", async () => {
    getBillingInfoMock.mockImplementation(async () => {
      if (getBillingInfoMock.mock.calls.length < 2) {
        return trialBilling();
      }
      return trialBilling({ plan: "pro", hasStripeSubscription: true });
    });
    const i18nInstance = await initI18n();
    renderPage(i18nInstance, "/acme/settings/billing?checkout=success");

    await waitFor(() => {
      expect(toast.info).toHaveBeenCalledWith("Payment received — updating your plan");
    });
    await waitFor(
      () => {
        expect(toast.success).toHaveBeenCalledWith("Your plan is now active");
      },
      { timeout: 4000 },
    );
    await waitFor(() => {
      expect(screen.getByTestId("billing-plan-summary")).toHaveTextContent(/Pro/);
    });
    await waitFor(() => {
      expect(screen.getByTestId("billing-search")).not.toHaveTextContent("checkout=");
    });
    expect(getBillingInfoMock.mock.calls.length).toBeGreaterThanOrEqual(2);
  });
});
