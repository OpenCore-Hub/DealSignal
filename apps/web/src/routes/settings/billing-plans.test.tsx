// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { SettingsBillingPlansPage } from "./billing-plans";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getBillingPlansMock, changeBillingPlanMock, createBillingCheckoutMock, createBillingPortalMock } = vi.hoisted(() => ({
  getBillingPlansMock: vi.fn(),
  changeBillingPlanMock: vi.fn(),
  createBillingCheckoutMock: vi.fn(),
  createBillingPortalMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getBillingPlans: getBillingPlansMock,
    changeBillingPlan: changeBillingPlanMock,
    createBillingCheckout: createBillingCheckoutMock,
    createBillingPortal: createBillingPortalMock,
  },
}));

vi.mock("@/hooks/useWorkspaceAccess", () => ({
  useWorkspaceAccess: () => ({
    role: "owner",
    loading: false,
    canRead: true,
    canWrite: true,
    canManage: true,
    isGuest: false,
  }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), message: vi.fn() },
}));

const gib = 1024 * 1024 * 1024;

const catalog = {
  currentPlan: "trial",
  currentPeriod: "monthly",
  trialExpired: false,
  trialEndsAt: "2026-08-27T00:00:00Z",
  plans: [
    {
      code: "free",
      internalSeats: 1,
      storageBytes: 2 * gib,
      documents: 50,
      links: 20,
      rooms: 1,
      maxUploadBytes: 25 * 1024 * 1024,
      visitorAskAiMonthly: 0,
      customDomain: false,
      watermark: false,
      nda: false,
      visitorAskAi: false,
      branding: false,
      accessControls: false,
      priceMonthlyUsd: 0,
      customPricing: false,
      highlighted: false,
    },
    {
      code: "pro",
      internalSeats: 3,
      storageBytes: 50 * gib,
      documents: 200,
      links: 0,
      rooms: 5,
      maxUploadBytes: 100 * 1024 * 1024,
      visitorAskAiMonthly: 200,
      customDomain: false,
      watermark: true,
      nda: false,
      visitorAskAi: true,
      branding: true,
      accessControls: false,
      priceMonthlyUsd: 49,
      customPricing: false,
      highlighted: false,
    },
    {
      code: "business",
      internalSeats: 10,
      storageBytes: 500 * gib,
      documents: 1000,
      links: 0,
      rooms: 0,
      maxUploadBytes: 250 * 1024 * 1024,
      visitorAskAiMonthly: 1000,
      customDomain: true,
      watermark: true,
      nda: true,
      visitorAskAi: true,
      branding: true,
      accessControls: true,
      priceMonthlyUsd: 99,
      customPricing: false,
      highlighted: true,
    },
    {
      code: "enterprise",
      internalSeats: 0,
      storageBytes: 0,
      documents: 0,
      links: 0,
      rooms: 0,
      maxUploadBytes: 0,
      visitorAskAiMonthly: 0,
      customDomain: true,
      watermark: true,
      nda: true,
      visitorAskAi: true,
      branding: true,
      accessControls: true,
      formalAsk: true,
      priceMonthlyUsd: 0,
      customPricing: true,
      highlighted: false,
    },
  ],
};

async function initI18n() {
  const instance = i18n.createInstance();
  const settingsJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/settings.json"), "utf-8"),
  );
  const commonJson = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"),
  );
  await instance.use(initReactI18next).init({
    lng: "en",
    resources: { en: { settings: settingsJson, common: commonJson } },
    interpolation: { escapeValue: false },
  });
  return instance;
}

function renderPage(i18nInstance: Awaited<ReturnType<typeof initI18n>>) {
  return render(
    <I18nextProvider i18n={i18nInstance}>
      <MemoryRouter initialEntries={["/acme/settings/billing/plans"]}>
        <Routes>
          <Route path="/:workspaceSlug/settings/billing/plans" element={<SettingsBillingPlansPage />} />
          <Route path="/:workspaceSlug/settings/billing" element={<div>usage</div>} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("SettingsBillingPlansPage", () => {
  beforeEach(() => {
    getBillingPlansMock.mockReset();
    changeBillingPlanMock.mockReset();
    createBillingCheckoutMock.mockReset();
    createBillingPortalMock.mockReset();
    vi.stubGlobal("location", { assign: vi.fn(), href: "http://localhost/" });
    getBillingPlansMock.mockResolvedValue(catalog);
    createBillingCheckoutMock.mockResolvedValue({ url: "https://checkout.stripe.test/pro" });
    changeBillingPlanMock.mockResolvedValue({
      plan: "pro",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 50 * gib,
      linksUsed: 0,
      linksLimit: 0,
      roomsUsed: 0,
      roomsLimit: 5,
      seatsUsed: 1,
      seatsLimit: 3,
      documentsUsed: 0,
      documentsLimit: 200,
      askAiUsed: 0,
      askAiLimit: 200,
      maxUploadBytes: 100 * 1024 * 1024,
      customDomainEnabled: false,
      watermarkEnabled: true,
      ndaEnabled: false,
      visitorAskAiEnabled: true,
      brandingEnabled: true,
      accessControlsEnabled: false,
    });
  });

  it("renders DealSignal catalog cards and chooses Pro via API", async () => {
    const i18nInstance = await initI18n();
    renderPage(i18nInstance);

    await waitFor(() => {
      expect(screen.getByTestId("billing-plans-page")).toBeInTheDocument();
    });
    expect(screen.getByTestId("billing-plan-card-free")).toBeInTheDocument();
    expect(screen.getByTestId("billing-plan-card-pro")).toBeInTheDocument();
    expect(screen.getByTestId("billing-plan-card-business")).toBeInTheDocument();
    expect(screen.getByTestId("billing-plan-card-enterprise")).toBeInTheDocument();
    expect(screen.getByText("2 GB document storage")).toBeInTheDocument();
    expect(screen.getByText("50 GB document storage")).toBeInTheDocument();
    expect(screen.getAllByText("Unlimited data rooms").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText("Most popular")).toBeInTheDocument();
    expect(screen.getByText("$49/mo")).toBeInTheDocument();
    expect(screen.getByText("$99/mo")).toBeInTheDocument();
    expect(screen.getByText("Save up to 15%")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Annually/ }));
    expect(screen.getByText("$500/yr")).toBeInTheDocument();
    expect(screen.getByText("$1010/yr")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^Monthly$/ }));
    expect(screen.getByText("50 documents")).toBeInTheDocument();
    expect(screen.getByText("200 documents")).toBeInTheDocument();
    expect(screen.getByText("1,000 documents")).toBeInTheDocument();
    expect(screen.getByText("5 data rooms")).toBeInTheDocument();
    expect(screen.getByText("200 Visitor Ask AI turns / month")).toBeInTheDocument();
    expect(screen.getByText("1,000 Visitor Ask AI turns / month")).toBeInTheDocument();
    const proCard = screen.getByTestId("billing-plan-card-pro");
    expect(proCard).toHaveTextContent("Page-by-page analytics");
    expect(proCard).toHaveTextContent("Data Room analytics");
    expect(proCard).toHaveTextContent("Dynamic watermark");
    expect(proCard).toHaveTextContent("Screenshot protection");
    expect(proCard).toHaveTextContent("Large file uploads");
    expect(proCard).not.toHaveTextContent("NDA agreements");
    expect(proCard).not.toHaveTextContent("Custom domain");
    expect(proCard).not.toHaveTextContent("HubSpot CRM");
    const freeCard = screen.getByTestId("billing-plan-card-free");
    expect(freeCard).toHaveTextContent("Unlimited visitors");
    expect(freeCard).toHaveTextContent("Page-by-page analytics");
    expect(freeCard).toHaveTextContent("Creating folders");
    expect(freeCard).not.toHaveTextContent("Data Room analytics");
    const businessCard = screen.getByTestId("billing-plan-card-business");
    expect(businessCard).toHaveTextContent("Page-by-page analytics");
    expect(businessCard).toHaveTextContent("Data Room analytics");
    expect(businessCard).toHaveTextContent("Data Room insights");
    expect(businessCard).toHaveTextContent("NDA agreements");
    expect(businessCard).toHaveTextContent("Custom domain");
    expect(businessCard).toHaveTextContent("Require email verification");
    expect(businessCard).toHaveTextContent("Allow/Block list");
    expect(businessCard).toHaveTextContent("HubSpot CRM");
    expect(businessCard).toHaveTextContent("Daily Insights digest");
    expect(businessCard).not.toHaveTextContent("Formal Ask");
    const enterpriseCard = screen.getByTestId("billing-plan-card-enterprise");
    expect(enterpriseCard).toHaveTextContent("Data Room analytics");
    expect(enterpriseCard).toHaveTextContent("Formal Ask");
    expect(enterpriseCard).toHaveTextContent("24/7 Email support");
    expect(screen.queryByText(/mailto:/i)).toBeNull();

    fireEvent.click(screen.getByTestId("billing-choose-pro"));
    await waitFor(() => {
      expect(createBillingCheckoutMock).toHaveBeenCalledWith("pro", "monthly");
    });
    expect(changeBillingPlanMock).not.toHaveBeenCalled();
    createBillingCheckoutMock.mockClear();
    fireEvent.click(screen.getByTestId("billing-choose-enterprise"));
    expect(createBillingCheckoutMock).not.toHaveBeenCalled();
    expect(changeBillingPlanMock).not.toHaveBeenCalled();
    expect(screen.getByTestId("billing-choose-enterprise")).toHaveTextContent("Contact sales");
  });
});
