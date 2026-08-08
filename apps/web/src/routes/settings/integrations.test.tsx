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

const {
  getIntegrationsMock,
  updateIntegrationsMock,
  connectSlackMock,
  disconnectHubSpotMock,
  getOutboundWebhookMock,
  saveOutboundWebhookMock,
  deleteOutboundWebhookMock,
} = vi.hoisted(() => ({
  getIntegrationsMock: vi.fn(),
  updateIntegrationsMock: vi.fn(),
  connectSlackMock: vi.fn(),
  disconnectHubSpotMock: vi.fn(),
  getOutboundWebhookMock: vi.fn(),
  saveOutboundWebhookMock: vi.fn(),
  deleteOutboundWebhookMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getIntegrations: getIntegrationsMock,
    updateIntegrations: updateIntegrationsMock,
    connectSlack: connectSlackMock,
    connectHubSpot: vi.fn(),
    disconnectSlack: vi.fn(),
    disconnectHubSpot: disconnectHubSpotMock,
    getOutboundWebhook: getOutboundWebhookMock,
    saveOutboundWebhook: saveOutboundWebhookMock,
    deleteOutboundWebhook: deleteOutboundWebhookMock,
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
  emailEnabled: true,
  dailyDigestEnabled: false,
  keyPageSlackEnabled: false,
  slack: false,
  hubspot: true,
};

const mockWebhook = {
  configured: false,
  enabled: false,
};

async function initI18n() {
  const instance = i18n.createInstance();
  const settingsJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/settings.json"), "utf-8"));
  const commonJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"));
  await instance.use(initReactI18next).init({
    lng: "en",
    resources: {
      en: { settings: settingsJson, common: commonJson },
    },
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderPage(initialEntry = "/acme/settings/integrations") {
  const instance = await initI18n();
  await act(async () => {
    render(
      <I18nextProvider i18n={instance}>
        <MemoryRouter initialEntries={[initialEntry]}>
          <Routes>
            <Route path=":workspaceSlug/settings/integrations" element={<SettingsIntegrationsPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );
  });
}

describe("SettingsIntegrationsPage", () => {
  beforeEach(() => {
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.error).mockClear();
    vi.mocked(toast.info).mockClear();

    getIntegrationsMock.mockReset();
    updateIntegrationsMock.mockReset();
    connectSlackMock.mockReset();
    disconnectHubSpotMock.mockReset();
    getOutboundWebhookMock.mockReset();
    saveOutboundWebhookMock.mockReset();
    deleteOutboundWebhookMock.mockReset();

    getIntegrationsMock.mockResolvedValue(mockStatus);
    getOutboundWebhookMock.mockResolvedValue(mockWebhook);
  });

  it("renders integration statuses", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Slack")).toBeInTheDocument();
    });

    expect(screen.getByText("HubSpot")).toBeInTheDocument();
    expect(screen.getByText("Outbound webhook")).toBeInTheDocument();
    expect(screen.queryByText("Zapier")).not.toBeInTheDocument();
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

  it("renders email notification toggle", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Email notifications")).toBeInTheDocument();
    });
  });

  it("toggles email notifications and saves preference", async () => {
    updateIntegrationsMock.mockResolvedValue({ ...mockStatus, emailEnabled: false });

    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Email notifications")).toBeInTheDocument();
    });

    const toggle = screen.getByRole("switch", { name: /email notifications/i });
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(updateIntegrationsMock).toHaveBeenCalledWith(
        expect.objectContaining({
          emailEnabled: false,
        }),
      );
    });
  });

  it("saves outbound webhook", async () => {
    saveOutboundWebhookMock.mockResolvedValue({
      configured: true,
      enabled: true,
      url: "https://hooks.zapier.com/hooks/catch/1/abc",
      secret: "abcdef0123456789abcdef0123456789",
      secretHint: "••••6789",
    });

    await renderPage();

    await waitFor(() => {
      expect(screen.getByLabelText(/webhook url/i)).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/webhook url/i), {
      target: { value: "https://hooks.zapier.com/hooks/catch/1/abc" },
    });
    fireEvent.click(screen.getByRole("switch", { name: /deliver events/i }));
    fireEvent.click(screen.getByRole("button", { name: /save webhook/i }));

    await waitFor(() => {
      expect(saveOutboundWebhookMock).toHaveBeenCalledWith(
        expect.objectContaining({
          url: "https://hooks.zapier.com/hooks/catch/1/abc",
          enabled: true,
          rotateSecret: false,
        }),
      );
    });
    expect(toast.success).toHaveBeenCalled();
    expect(screen.getByText(/copy this secret now/i)).toBeInTheDocument();
  });

  it("disables key-page slack toggle until slack is connected", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Sensitive-page Slack alerts")).toBeInTheDocument();
    });

    expect(screen.getByRole("switch", { name: /sensitive-page slack alerts/i })).toBeDisabled();
  });
});
