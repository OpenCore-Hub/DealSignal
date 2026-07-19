// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { AccessSummaryCard } from "./AccessSummaryCard";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      linkShare: {
        share: {
          accessSummary: "Access summary",
          accessSummaryEmpty: "No restrictions",
          editAccessRules: "Edit",
          accessSummaryScope: "{{folders}} folders / {{documents}} docs",
        },
        accessRules: {
          authentication: {
            requireEmail: "Require email",
            requireVerification: "Require verification",
            requirePassword: "Require password",
          },
          additionalProtections: {
            watermark: "Watermark",
            requireNda: "NDA",
            allowDownloading: "Download",
            screenshotProtection: "Screenshot",
          },
        },
      },
    },
  },
  interpolation: { escapeValue: false },
});

const baseProps = {
  requireEmail: false,
  requireEmailVerification: false,
  requirePassword: false,
  watermarkEnabled: false,
  requireNda: false,
  allowDownloading: false,
  enableScreenshotProtection: false,
  allowedViewers: [],
  blockedViewers: [],
  onEditAccess: vi.fn(),
};

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

describe("AccessSummaryCard", () => {
  it("renders empty state when no restrictions", () => {
    render(
      <Wrapper>
        <AccessSummaryCard {...baseProps} />
      </Wrapper>
    );
    expect(screen.getByText("No restrictions")).toBeInTheDocument();
  });

  it("renders active protections", () => {
    render(
      <Wrapper>
        <AccessSummaryCard
          {...baseProps}
          requirePassword
          watermarkEnabled
          allowedViewers={["alice@vc.com", "bob@vc.com"]}
        />
      </Wrapper>
    );
    expect(screen.getByText("Require password")).toBeInTheDocument();
    expect(screen.getByText("Watermark")).toBeInTheDocument();
    expect(screen.getByText("alice@vc.com")).toBeInTheDocument();
    expect(screen.getByText("bob@vc.com")).toBeInTheDocument();
  });

  it("calls onEditAccess when edit clicked", () => {
    const onEditAccess = vi.fn();
    render(
      <Wrapper>
        <AccessSummaryCard {...baseProps} requirePassword onEditAccess={onEditAccess} />
      </Wrapper>
    );
    fireEvent.click(screen.getByText("Edit"));
    expect(onEditAccess).toHaveBeenCalled();
  });

  it("renders document scope chip when folder paths are selected", () => {
    render(
      <Wrapper>
        <AccessSummaryCard
          {...baseProps}
          folderPaths={["/financials"]}
          documents={[
            {
              folder: "/financials",
              permission: "view",
              documents: [
                { id: "doc-1", document_id: "doc-1", title: "P&L", folder_path: "/financials", sort_order: 0, source_type: "pdf", status: "ready", created_at: new Date().toISOString() },
              ],
            },
          ]}
        />
      </Wrapper>
    );
    expect(screen.getByText("1 folders / 1 docs")).toBeInTheDocument();
  });
});
