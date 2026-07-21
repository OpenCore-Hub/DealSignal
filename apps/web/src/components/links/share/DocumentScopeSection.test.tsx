// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { DocumentScopeSection } from "./DocumentScopeSection";
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

const folders: DealRoomFolder[] = [
  { path: "/financials", name: "Financials", sort_order: 0 },
  { path: "/financials/2024", name: "2024", sort_order: 1 },
  { path: "/legal", name: "Legal", sort_order: 2 },
];

const documents: DealRoomFolderDocs[] = [
  {
    folder: "/financials",
    permission: "view",
    documents: [
      { id: "doc-1", document_id: "doc-1", title: "P&L", folder_path: "/financials", sort_order: 0, source_type: "pdf", status: "ready", created_at: new Date().toISOString() },
    ],
  },
  {
    folder: "/financials/2024",
    permission: "view",
    documents: [
      { id: "doc-2", document_id: "doc-2", title: "Q1", folder_path: "/financials/2024", sort_order: 0, source_type: "pdf", status: "ready", created_at: new Date().toISOString() },
      { id: "doc-3", document_id: "doc-3", title: "Q2", folder_path: "/financials/2024", sort_order: 1, source_type: "pdf", status: "ready", created_at: new Date().toISOString() },
    ],
  },
  {
    folder: "/legal",
    permission: "view",
    documents: [
      { id: "doc-4", document_id: "doc-4", title: "NDA", folder_path: "/legal", sort_order: 0, source_type: "pdf", status: "ready", created_at: new Date().toISOString() },
    ],
  },
];

function renderSection(props: {
  selectedPaths?: string[];
  scopeMode?: "full" | "allowlist";
  folders?: DealRoomFolder[];
  documents?: DealRoomFolderDocs[];
}) {
  const onChange = vi.fn();
  render(
    <Wrapper>
      <DocumentScopeSection
        folders={props.folders ?? folders}
        documents={props.documents ?? documents}
        selectedPaths={props.selectedPaths ?? []}
        scopeMode={props.scopeMode ?? "allowlist"}
        onChange={onChange}
      />
    </Wrapper>
  );
  return { onChange };
}

describe("DocumentScopeSection", () => {
  it("renders deny-all hint when allowlist is empty", () => {
    renderSection({});
    expect(
      screen.getByText("No folders authorized — visitors cannot preview any files")
    ).toBeInTheDocument();
    expect(screen.getByText("Financials")).toBeInTheDocument();
    expect(screen.getByText("Legal")).toBeInTheDocument();
  });

  it("renders legacy full-room hint", () => {
    renderSection({ scopeMode: "full" });
    expect(screen.getByText("All documents accessible (legacy)")).toBeInTheDocument();
  });

  it("toggles a top-level folder on and off", () => {
    const { onChange } = renderSection({});
    const row = screen.getByTestId("folder-row-/financials");
    const checkbox = row.querySelector('[role="checkbox"]') as HTMLElement;

    fireEvent.click(checkbox);
    expect(onChange).toHaveBeenCalledWith({
      scopeMode: "allowlist",
      selectedPaths: ["/financials"],
    });
  });

  it("selecting a parent folder removes redundant child selections", () => {
    const { onChange } = renderSection({ selectedPaths: ["/financials/2024"] });
    const row = screen.getByTestId("folder-row-/financials");
    const checkbox = row.querySelector('[role="checkbox"]') as HTMLElement;

    fireEvent.click(checkbox);
    expect(onChange).toHaveBeenCalledWith({
      scopeMode: "allowlist",
      selectedPaths: ["/financials"],
    });
  });

  it("shows indeterminate state when a child is selected", () => {
    renderSection({ selectedPaths: ["/financials/2024"] });
    const row = screen.getByTestId("folder-row-/financials");
    const checkbox = row.querySelector('[role="checkbox"]') as HTMLElement;
    expect(checkbox).toHaveAttribute("aria-checked", "mixed");
  });

  it("shows checked state when a parent is selected", () => {
    renderSection({ selectedPaths: ["/financials"] });
    const row = screen.getByTestId("folder-row-/financials");
    const checkbox = row.querySelector('[role="checkbox"]') as HTMLElement;
    expect(checkbox).toHaveAttribute("aria-checked", "true");
  });

  it("displays selected count and document count", () => {
    renderSection({ selectedPaths: ["/financials"] });
    expect(screen.getByText("1 folders / 3 documents")).toBeInTheDocument();
  });

  it("selects all root folders when select all is clicked", () => {
    const { onChange } = renderSection({});
    fireEvent.click(screen.getByText("Select all"));
    expect(onChange).toHaveBeenCalledWith({
      scopeMode: "allowlist",
      selectedPaths: ["/financials", "/legal"],
    });
  });

  it("clears selection to deny-all when deselect is clicked", () => {
    const { onChange } = renderSection({ selectedPaths: ["/financials"] });
    fireEvent.click(screen.getByText("Deselect"));
    expect(onChange).toHaveBeenCalledWith({
      scopeMode: "allowlist",
      selectedPaths: [],
    });
  });

  it("converts legacy full mode to allowlist on first edit", () => {
    const { onChange } = renderSection({ scopeMode: "full" });
    const row = screen.getByTestId("folder-row-/legal");
    const checkbox = row.querySelector('[role="checkbox"]') as HTMLElement;
    fireEvent.click(checkbox);
    expect(onChange).toHaveBeenCalledWith({
      scopeMode: "allowlist",
      selectedPaths: ["/financials"],
    });
  });
});
