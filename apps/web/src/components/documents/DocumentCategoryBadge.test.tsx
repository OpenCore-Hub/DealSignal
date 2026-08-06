// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DocumentCategoryBadge } from "./DocumentCategoryBadge";

async function initI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    resources: {
      en: {
        documents: {
          category: {
            agreement: "Agreement",
            dealRoom: "Data room",
          },
        },
      },
    },
  });
  return instance;
}

describe("DocumentCategoryBadge", () => {
  it("renders nothing for general library docs", async () => {
    const instance = await initI18n();
    const { container } = render(
      <I18nextProvider i18n={instance}>
        <DocumentCategoryBadge category="general" />
      </I18nextProvider>,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("shows agreement and deal_room labels", async () => {
    const instance = await initI18n();
    const { rerender } = render(
      <I18nextProvider i18n={instance}>
        <DocumentCategoryBadge category="agreement" />
      </I18nextProvider>,
    );
    expect(screen.getByText("Agreement")).toBeInTheDocument();

    rerender(
      <I18nextProvider i18n={instance}>
        <DocumentCategoryBadge category="deal_room" />
      </I18nextProvider>,
    );
    expect(screen.getByText("Data room")).toBeInTheDocument();
  });
});
