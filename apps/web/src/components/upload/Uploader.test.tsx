/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { Uploader } from "./Uploader";

const { uploadDocumentMock, getBillingInfoMock } = vi.hoisted(() => ({
  uploadDocumentMock: vi.fn(),
  getBillingInfoMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    uploadDocument: uploadDocumentMock,
    getBillingInfo: getBillingInfoMock,
  },
}));

vi.mock("@/hooks/useDocumentUploadConflict", () => ({
  UploadCancelledError: class UploadCancelledError extends Error {},
  useDocumentUploadConflict: () => ({
    uploadDocument: uploadDocumentMock,
    conflictDialog: null,
    isAwaitingConflict: false,
  }),
}));

// Keep usage-bar rendering independent of i18n resource bundles while still
// exercising apiErrorMessage → common:error.codes.plan_limit_storage.
vi.mock("@/lib/formatters", () => ({
  formatFileSize: (bytes: number) => `${bytes} B`,
}));

vi.mock("@/i18n/config", () => ({
  default: {
    language: "en",
    t: (key: string) => {
      const map: Record<string, string> = {
        "common:error.uploadFailed": "Upload failed",
        "common:error.codes.plan_limit_storage":
          "You've reached the storage limit for your plan. Remove files or upgrade to upload more.",
      };
      return map[key] ?? key;
    },
    exists: (key: string) =>
      key === "common:error.uploadFailed" ||
      key === "common:error.codes.plan_limit_storage",
  },
}));

vi.mock("react-i18next", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-i18next")>();
  return {
    ...actual,
    useTranslation: () => ({
      t: (key: string) => key,
      i18n: { language: "en" },
    }),
  };
});

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}));

describe("Uploader storage quota", () => {
  beforeEach(() => {
    uploadDocumentMock.mockReset();
    getBillingInfoMock.mockReset();
  });

  it("shows storage usage and limit hint when at cap", async () => {
    getBillingInfoMock.mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 1 << 30,
      storageLimit: 1 << 30,
      linksUsed: 0,
      linksLimit: 20,
      roomsUsed: 0,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });

    render(<Uploader />);

    await waitFor(() => {
      expect(screen.getByTestId("upload-storage-usage")).toBeInTheDocument();
    });
    expect(screen.getByTestId("upload-storage-limit-hint")).toHaveTextContent(
      "upload.storageLimitReached",
    );
    expect(screen.getByTestId("file-upload")).toBeInTheDocument();
  });

  it("shows storage usage without limit hint when under cap", async () => {
    getBillingInfoMock.mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 100,
      storageLimit: 1 << 30,
      linksUsed: 0,
      linksLimit: 20,
      roomsUsed: 0,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });

    render(<Uploader />);

    await waitFor(() => {
      expect(screen.getByTestId("upload-storage-usage")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("upload-storage-limit-hint")).toBeNull();
  });

  it("surfaces plan_limit_storage on upload race via inline file error", async () => {
    const { ApiError } = await import("@/lib/apiClient");
    getBillingInfoMock.mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 100,
      storageLimit: 1 << 30,
      linksUsed: 0,
      linksLimit: 20,
      roomsUsed: 0,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });
    uploadDocumentMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "plan_limit_storage",
        message: "storage limit reached for this plan",
        requestId: "r-storage-race",
      }),
    );

    render(<Uploader />);

    await waitFor(() => {
      expect(screen.getByTestId("file-upload")).toBeInTheDocument();
    });

    const file = new File(["%PDF-1.4"], "race.pdf", { type: "application/pdf" });
    fireEvent.change(screen.getByTestId("file-upload"), {
      target: { files: [file] },
    });
    fireEvent.click(screen.getByRole("button", { name: "upload.uploadNow" }));

    await waitFor(() => {
      expect(uploadDocumentMock).toHaveBeenCalled();
      expect(
        screen.getByText(
          /You've reached the storage limit for your plan/i,
        ),
      ).toBeInTheDocument();
    });
  });
});
