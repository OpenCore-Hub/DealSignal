// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DocumentsTab } from "./DocumentsTab";
import type { DealRoomFolder, DealRoomFolderDocs } from "@/types";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      linkShare: {
        share: {
          documentScope: {
            allDocuments: "All documents accessible",
            legacyAllDocuments: "All documents accessible (legacy)",
            noneAuthorized: "No folders authorized — visitors cannot preview any files",
            selectedDocuments: "{{folders}} folders / {{documents}} documents",
            selectAll: "Select all",
            deselectAll: "Deselect",
            empty: "No folders available",
          },
        },
      },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

const folders: DealRoomFolder[] = [{ path: "/financials", name: "Financials", sort_order: 0 }];
const documents: DealRoomFolderDocs[] = [
  {
    folder: "/financials",
    permission: "view",
    documents: [
      { id: "doc-1", document_id: "doc-1", title: "P&L", folder_path: "/financials", sort_order: 0, source_type: "pdf", status: "ready", created_at: new Date().toISOString() },
    ],
  },
];

describe("DocumentsTab", () => {
  it("renders folder scope section and propagates selection changes", () => {
    const onChange = vi.fn();
    render(
      <Wrapper>
        <DocumentsTab
          folders={folders}
          documents={documents}
          selectedPaths={[]}
          scopeMode="allowlist"
          onChange={onChange}
        />
      </Wrapper>
    );

    expect(screen.getByText("Financials")).toBeInTheDocument();

    const row = screen.getByTestId("folder-row-/financials");
    const checkbox = row.querySelector('[role="checkbox"]') as HTMLElement;
    fireEvent.click(checkbox);
    expect(onChange).toHaveBeenCalledWith({
      scopeMode: "allowlist",
      selectedPaths: ["/financials"],
    });
  });
});
