// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { SettingsIntegrationsPage } from "./integrations";
import { toast } from "sonner";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getIntegrationsMock, connectSlackMock, disconnectHubSpotMock } = vi.hoisted(() => ({
  getIntegrationsMock: vi.fn(),
  connectSlackMock: vi.fn(),
  disconnectHubSpotMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getIntegrations: getIntegrationsMock,
    connectSlack: connectSlackMock,
    connectHubSpot: vi.fn(),
    disconnectSlack: vi.fn(),
    disconnectHubSpot: disconnectHubSpotMock,
  },
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
  },
}));

const mockStatus = {
  slack: false,
  hubspot: true,
  zapier: false,
};

async function initI18n() {
  const instance = i18n.createInstance();
  const settingsJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/settings.json"), "utf-8"));
  const commonJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"));
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["settings", "common"],
    defaultNS: "settings",
    resources: { en: { settings: settingsJson, common: commonJson } },
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderPage(initialEntry = "/acme/settings/integrations") {
  const i18nInstance = await initI18n();
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={[initialEntry]}>
          <Routes>
            <Route path=":workspaceSlug/settings/integrations" element={<SettingsIntegrationsPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("SettingsIntegrationsPage", () => {
  beforeEach(() => {
    getIntegrationsMock.mockReset();
    connectSlackMock.mockReset();
    disconnectHubSpotMock.mockReset();
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.error).mockClear();
    vi.mocked(toast.info).mockClear();

    getIntegrationsMock.mockResolvedValue({ data: mockStatus });
  });

  it("renders integration statuses", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Slack")).toBeInTheDocument();
    });

    expect(screen.getByText("HubSpot")).toBeInTheDocument();
    expect(screen.getByText("Zapier")).toBeInTheDocument();
  });

  it("connects slack and opens oauth url", async () => {
    connectSlackMock.mockResolvedValue({ url: "https://slack.com/oauth" });
    const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);

    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Slack")).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole("button", { name: /^Connect$/i });
    fireEvent.click(buttons[0]);

    await waitFor(() => {
      expect(connectSlackMock).toHaveBeenCalled();
    });
    expect(openSpy).toHaveBeenCalledWith("https://slack.com/oauth", "_blank", "noopener,noreferrer");

    openSpy.mockRestore();
  });

  it("disconnects hubspot and refetches status", async () => {
    disconnectHubSpotMock.mockResolvedValue(undefined);

    await renderPage();

    await waitFor(() => {
      expect(screen.getAllByRole("button", { name: /disconnect/i }).length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getAllByRole("button", { name: /disconnect/i })[0]);

    await waitFor(() => {
      expect(disconnectHubSpotMock).toHaveBeenCalled();
    });
    expect(toast.success).toHaveBeenCalled();
  });

  it("handles oauth callback query params", async () => {
    await renderPage("/acme/settings/integrations?provider=slack&status=connected");

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalled();
    });
    expect(getIntegrationsMock).toHaveBeenCalledTimes(2);
  });
});
