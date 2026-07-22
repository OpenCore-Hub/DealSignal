// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { createTestI18n } from "@/i18n/test-utils";
import { RightSidebar, shouldGroupDocumentsByFolder } from "./RightSidebar";

describe("shouldGroupDocumentsByFolder", () => {
  it("keeps a flat list when all docs are at root", () => {
    expect(
      shouldGroupDocumentsByFolder([
        { id: "1", title: "A", pageCount: 1, folderPath: "/" },
        { id: "2", title: "B", pageCount: 1 },
      ])
    ).toBe(false);
  });

  it("groups when more than one folder path is present", () => {
    expect(
      shouldGroupDocumentsByFolder([
        { id: "1", title: "A", pageCount: 1, folderPath: "/legal" },
        { id: "2", title: "B", pageCount: 1, folderPath: "/finance" },
      ])
    ).toBe(true);
  });

  it("groups when a single non-root folder is present", () => {
    expect(
      shouldGroupDocumentsByFolder([
        { id: "1", title: "A", pageCount: 1, folderPath: "/legal" },
        { id: "2", title: "B", pageCount: 1, folderPath: "/legal" },
      ])
    ).toBe(true);
  });
});

describe("RightSidebar folder structure", () => {
  it("renders folder headers and documents under them", async () => {
    const i18n = await createTestI18n();
    const onSelectDoc = vi.fn();

    render(
      <I18nextProvider i18n={i18n}>
        <RightSidebar
          open
          onClose={() => {}}
          selectedDocIndex={0}
          onSelectDoc={onSelectDoc}
          documents={[
            { id: "d1", title: "Contract", pageCount: 3, folderPath: "/legal" },
            { id: "d2", title: "Budget", pageCount: 2, folderPath: "/finance" },
          ]}
        />
      </I18nextProvider>
    );

    expect(screen.getByText("legal")).toBeTruthy();
    expect(screen.getByText("finance")).toBeTruthy();
    expect(screen.getByText("Contract")).toBeTruthy();

    fireEvent.click(screen.getByText("finance"));
    fireEvent.click(screen.getByText("Budget"));
    expect(onSelectDoc).toHaveBeenCalledWith(1);
  });

  it("allows collapsing even when only one folder exists", async () => {
    const i18n = await createTestI18n();

    render(
      <I18nextProvider i18n={i18n}>
        <RightSidebar
          open
          onClose={() => {}}
          selectedDocIndex={0}
          documents={[
            {
              id: "d1",
              title: "练习册.pdf",
              pageCount: 48,
              folderPath: "/01-corporate-or-investment-memo",
            },
          ]}
        />
      </I18nextProvider>
    );

    expect(screen.getByText("练习册.pdf")).toBeTruthy();
    const folderToggle = screen.getByRole("button", {
      name: /01-corporate-or-investment-memo/,
    });
    expect(folderToggle).toHaveAttribute("aria-expanded", "true");

    fireEvent.click(folderToggle);
    expect(folderToggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("练习册.pdf")).toBeNull();

    fireEvent.click(folderToggle);
    expect(folderToggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("练习册.pdf")).toBeTruthy();
  });

  it("keeps a flat document list when no folder structure exists", async () => {
    const i18n = await createTestI18n();

    render(
      <I18nextProvider i18n={i18n}>
        <RightSidebar
          open
          onClose={() => {}}
          documents={[
            { id: "d1", title: "Deck", pageCount: 1 },
            { id: "d2", title: "Appendix", pageCount: 2 },
          ]}
        />
      </I18nextProvider>
    );

    expect(screen.getByText("Deck")).toBeTruthy();
    expect(screen.getByText("Appendix")).toBeTruthy();
    expect(screen.queryByText("Root")).toBeNull();
  });

  it("shows Ask tab when either Ask Docs or Ask Host is enabled", async () => {
    const i18n = await createTestI18n({
      documents: { "viewer.sidebarQA": "Ask", "viewer.sidebarDocuments": "Documents", "viewer.pages": "pages" },
      ai: { "viewer.close": "Close" },
    });
    const { rerender } = render(
      <I18nextProvider i18n={i18n}>
        <RightSidebar
          open
          onClose={() => {}}
          aiCopilotEnabled
          qaEnabled={false}
          documents={[{ id: "d1", title: "Deck", pageCount: 1 }]}
        />
      </I18nextProvider>
    );
    expect(screen.getByRole("button", { name: /^Ask$/i })).toBeInTheDocument();

    rerender(
      <I18nextProvider i18n={i18n}>
        <RightSidebar
          open
          onClose={() => {}}
          aiCopilotEnabled={false}
          qaEnabled
          documents={[{ id: "d1", title: "Deck", pageCount: 1 }]}
        />
      </I18nextProvider>
    );
    expect(screen.getByRole("button", { name: /^Ask$/i })).toBeInTheDocument();
  });

  it("hides Ask tab when both channels are off", async () => {
    const i18n = await createTestI18n({
      documents: { "viewer.sidebarQA": "Ask", "viewer.sidebarDocuments": "Documents", "viewer.pages": "pages" },
      ai: { "viewer.close": "Close" },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <RightSidebar
          open
          onClose={() => {}}
          aiCopilotEnabled={false}
          qaEnabled={false}
          documents={[{ id: "d1", title: "Deck", pageCount: 1 }]}
        />
      </I18nextProvider>
    );
    expect(screen.queryByRole("button", { name: /^Ask$/i })).not.toBeInTheDocument();
  });
});
