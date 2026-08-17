// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import type { ComponentProps } from "react";
import { DocumentStats } from "./DocumentStats";
import type { Link, PageAnalytics } from "@/types";

const __dirname = dirname(fileURLToPath(import.meta.url));

async function renderStats(
  props: Partial<ComponentProps<typeof DocumentStats>> & {
    pages?: PageAnalytics[];
  } = {},
) {
  const instance = i18n.createInstance();
  const documents = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/documents.json"), "utf-8"),
  );
  const common = JSON.parse(
    readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"),
  );
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["documents", "common"],
    defaultNS: "documents",
    resources: { en: { documents, common } },
    interpolation: { escapeValue: false },
  });
  return render(
    <I18nextProvider i18n={instance}>
      <DocumentStats links={[]} visitors={[]} pages={[]} {...props} />
    </I18nextProvider>,
  );
}

function libraryLink(partial: Partial<Link> & Pick<Link, "id">): Link {
  return {
    documentIds: ["doc_1"],
    folderPaths: [],
    documentTitle: "Deck",
    shortUrl: "https://example.com/l/x",
    accessCount: 4,
    heatLevel: "cold",
    createdAt: "2026-08-01T00:00:00Z",
    isBundle: false,
    documents: [],
    ...partial,
  };
}

describe("DocumentStats", () => {
  it("shows document-native heat without attaching room shares", async () => {
    const onExplainHeat = vi.fn();
    await renderStats({
      pages: [{ pageNumber: 1, viewCount: 12, avgDurationSeconds: 20, exitRate: 0 }],
      heat: { level: "hot", score: 29 },
      onExplainHeat,
    });
    expect(screen.getByText("Document heat")).toBeInTheDocument();
    expect(screen.getByText("Hot")).toBeInTheDocument();
    expect(screen.getByText("29 pts")).toBeInTheDocument();
    expect(screen.getByText("Views from data-room shares")).toBeInTheDocument();
    expect(screen.queryByText("Link engagement breakdown")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Explain" }));
    expect(onExplainHeat).toHaveBeenCalledTimes(1);
  });

  it("falls back to library-link chips when native heat is unavailable", async () => {
    await renderStats({
      links: [libraryLink({ id: "l1", heatLevel: "warm" })],
    });
    expect(screen.getByText("Link engagement breakdown")).toBeInTheDocument();
    expect(screen.getByText("Warm 1")).toBeInTheDocument();
    expect(screen.queryByText("Explain")).not.toBeInTheDocument();
  });
});
