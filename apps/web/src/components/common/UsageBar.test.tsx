// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { UsageBar, usagePercent } from "./UsageBar";

const i18nInstance = i18n.createInstance();
void i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      common: {
        unlimited: "Unlimited",
        usageUnlimitedHint: "included",
        usageAtLimit: "At limit",
        usageNearLimit: "Near limit",
        notIncluded: "Not included",
      },
    },
  },
  interpolation: { escapeValue: false },
});

describe("usagePercent", () => {
  it("returns a clamped integer percent", () => {
    expect(usagePercent(50, 100)).toBe(50);
    expect(usagePercent(150, 100)).toBe(100);
    expect(usagePercent(0, 10)).toBe(0);
  });

  it("returns null instead of NaN when the ratio is not computable", () => {
    expect(usagePercent(Number.NaN, 100)).toBeNull();
    expect(usagePercent(10, Number.NaN)).toBeNull();
    expect(usagePercent(10, 0)).toBeNull();
    expect(usagePercent(10, -1)).toBeNull();
  });
});

describe("UsageBar", () => {
  it("renders Unlimited included state without a fake empty percent", () => {
    render(
      <I18nextProvider i18n={i18nInstance}>
        <UsageBar label="Storage" current={1024} max={0} formatCurrent="1 KB" />
      </I18nextProvider>,
    );
    expect(screen.getByText("Storage")).toBeInTheDocument();
    expect(screen.getByText(/1 KB \/ Unlimited/)).toBeInTheDocument();
    expect(screen.getByText("included")).toBeInTheDocument();
    expect(screen.queryByText(/\(\s*—\s*\)/)).toBeNull();
    expect(screen.getByTestId("usage-bar-unlimited")).toBeInTheDocument();
  });

  it("renders At limit when current reaches a finite max", () => {
    render(
      <I18nextProvider i18n={i18nInstance}>
        <UsageBar label="Links" current={20} max={20} />
      </I18nextProvider>,
    );
    expect(screen.getByText(/20 \/ 20 \(100%\)/)).toBeInTheDocument();
    expect(screen.getByText("At limit")).toBeInTheDocument();
    expect(screen.getByTestId("usage-bar-fill")).toBeInTheDocument();
  });

  it("keeps the same usage copy in featured and ledger variants", () => {
    render(
      <I18nextProvider i18n={i18nInstance}>
        <UsageBar variant="featured" label="Documents" current={12} max={50} />
        <UsageBar variant="ledger" label="Ask" current={0} max={200} />
      </I18nextProvider>,
    );
    expect(screen.getByText(/12 \/ 50 \(24%\)/)).toBeInTheDocument();
    expect(screen.getByText(/0 \/ 200 \(0%\)/)).toBeInTheDocument();
    expect(screen.getAllByTestId("usage-bar-fill")).toHaveLength(2);
  });
});
