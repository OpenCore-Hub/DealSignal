// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import i18next from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { BundleSecurityOptions } from "./BundleSecurityOptions";
import { buildConfigFromPreset } from "./pipelineUtils";
import type { PermissionConfig } from "@/types";

async function setupI18n() {
  const i18n = i18next.createInstance();
  await i18n.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    resources: {
      en: {
        links: {
          "creator.sectionAccessControl": "Access Control",
          "creator.requireEmailVerification": "Require email verification",
          "creator.requireEmailDesc": "Visitor must verify email",
          "creator.whitelist": "Whitelist",
          "creator.whitelistDesc": "Only allow specified emails",
          "creator.password": "Password",
          "creator.passwordDesc": "Require password",
          "creator.sectionContentProtection": "Content Protection",
          "creator.nda": "NDA",
          "creator.ndaDesc": "Require NDA",
          "creator.allowDownload": "Allow download",
          "creator.allowDownloadDesc": "Allow download",
          "creator.watermark": "Dynamic watermark",
          "creator.watermarkDesc": "Dynamic watermark",
          "creator.sectionAdvanced": "Advanced",
          "creator.expiry": "Expiry",
          "creator.expiryPlaceholder": "Expiry",
          "creator.expiryDays.7": "7 days",
          "creator.expiryDays.30": "30 days",
          "creator.expiryDays.90": "90 days",
          "creator.expiryDays.custom": "Custom",
          "creator.maxViews": "Max views",
          "creator.maxViewsPlaceholder": "Max views",
          "creator.maxViewsOptions.unlimited": "Unlimited",
          "creator.maxViewsOptions.10": "10",
          "creator.maxViewsOptions.50": "50",
          "creator.maxViewsOptions.100": "100",
          "creator.whitelistPlaceholder": "Emails/domains",
        },
      },
    },
    interpolation: { escapeValue: false },
  });
  return i18n;
}

async function renderSecurityOptions(
  config: PermissionConfig,
  onChange = vi.fn(),
) {
  const i18n = await setupI18n();
  return render(
    <I18nextProvider i18n={i18n}>
      <BundleSecurityOptions config={config} onChange={onChange} />
    </I18nextProvider>,
  );
}

describe("BundleSecurityOptions switch interaction", () => {
  it("toggles email verification switch", async () => {
    const onChange = vi.fn();
    const config = buildConfigFromPreset("customized");
    await renderSecurityOptions(config, onChange);

    const switches = screen.getAllByRole("switch");
    // Order: email, whitelist, password, nda, download, watermark
    expect(switches.length).toBe(6);

    fireEvent.click(switches[0]);
    expect(onChange).toHaveBeenCalledOnce();
    const next = onChange.mock.calls[0][0] as PermissionConfig;
    expect(next.requireEmailVerification).toBe(true);
  });

  it("toggles allow download switch", async () => {
    const onChange = vi.fn();
    const config = buildConfigFromPreset("customized");
    await renderSecurityOptions(config, onChange);

    const switches = screen.getAllByRole("switch");
    fireEvent.click(switches[4]); // allow download
    expect(onChange).toHaveBeenCalledOnce();
    const next = onChange.mock.calls[0][0] as PermissionConfig;
    expect(next.allowDownload).toBe(true);
  });

  it("toggles dynamic watermark switch off", async () => {
    const onChange = vi.fn();
    const config = buildConfigFromPreset("customized");
    expect(config.watermarkEnabled).toBe(true);
    await renderSecurityOptions(config, onChange);

    const switches = screen.getAllByRole("switch");
    fireEvent.click(switches[5]); // watermark
    expect(onChange).toHaveBeenCalledOnce();
    const next = onChange.mock.calls[0][0] as PermissionConfig;
    expect(next.watermarkEnabled).toBe(false);
  });
});
