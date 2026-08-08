/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { UploadDialog } from "./UploadDialog";

const uploadDocumentMock = vi.hoisted(() => vi.fn());
const setUploadDialogOpen = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", () => ({
  api: {
    uploadDocument: uploadDocumentMock,
  },
}));

vi.mock("@/stores/uiStore", () => ({
  useUIStore: () => ({
    uploadDialogOpen: true,
    setUploadDialogOpen,
  }),
}));

vi.mock("@/components/ui/dialog", () => ({
  Dialog: ({
    children,
    open,
    onOpenChange,
  }: {
    children: React.ReactNode;
    open?: boolean;
    onOpenChange?: (open: boolean) => void;
  }) =>
    open ? (
      <div data-testid="dialog-root">
        <button
          type="button"
          data-testid="attempt-close-dialog"
          onClick={() => onOpenChange?.(false)}
        >
          close
        </button>
        {children}
      </div>
    ) : null,
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogFooter: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>,
  DialogDescription: ({ children }: { children: React.ReactNode }) => <p>{children}</p>,
}));

vi.mock("react-i18next", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-i18next")>();
  return {
    ...actual,
    useTranslation: () => ({
      t: (key: string, opts?: { name?: string }) =>
        opts?.name ? `${key}:${opts.name}` : key,
      i18n: { language: "en" },
    }),
  };
});

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}));

describe("UploadDialog nested conflict prompt", () => {
  beforeEach(() => {
    uploadDocumentMock.mockReset();
    setUploadDialogOpen.mockReset();
  });

  it("blocks host dismiss while overwrite/discard prompt is open", async () => {
    uploadDocumentMock.mockRejectedValueOnce({
      code: "document_exists",
      status: 409,
      message: "exists",
    });

    render(<UploadDialog />);

    const input = screen.getByTestId("file-upload");
    const file = new File(["%PDF-1.4"], "dup.pdf", { type: "application/pdf" });
    fireEvent.change(input, { target: { files: [file] } });

    fireEvent.click(screen.getByRole("button", { name: "upload.uploadNow" }));
    await screen.findByText("upload.replaceTitle");

    // First dialog root is the UploadDialog host (ConfirmDialog portals after it).
    fireEvent.click(screen.getAllByTestId("attempt-close-dialog")[0]!);
    expect(setUploadDialogOpen).not.toHaveBeenCalledWith(false);

    fireEvent.click(screen.getByRole("button", { name: "upload.replaceCancel" }));
    await waitFor(() => {
      expect(screen.queryByText("upload.replaceTitle")).toBeNull();
    });
  });
});
