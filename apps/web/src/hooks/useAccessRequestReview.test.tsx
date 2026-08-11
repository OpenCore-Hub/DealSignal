// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import type { ReactNode } from "react";
import { createTestI18n } from "@/i18n/test-utils";
import { ApiError } from "@/lib/apiClient";
import { useAccessRequestReview } from "./useAccessRequestReview";

const {
  approveLinkAccessRequestMock,
  rejectLinkAccessRequestMock,
  resendLinkAccessCodeMock,
  toastSuccess,
  toastError,
  toastWarning,
} = vi.hoisted(() => ({
  approveLinkAccessRequestMock: vi.fn(),
  rejectLinkAccessRequestMock: vi.fn(),
  resendLinkAccessCodeMock: vi.fn(),
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  toastWarning: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    approveLinkAccessRequest: approveLinkAccessRequestMock,
    rejectLinkAccessRequest: rejectLinkAccessRequestMock,
    resendLinkAccessCode: resendLinkAccessCodeMock,
  },
}));

vi.mock("sonner", () => ({
  toast: {
    success: toastSuccess,
    error: toastError,
    warning: toastWarning,
    message: vi.fn(),
  },
}));

async function renderReviewHook(
  afterReview?: Parameters<typeof useAccessRequestReview>[0],
) {
  const i18nInstance = await createTestI18n({
    linkShare: {
      "accessRequests.approveSuccess": "approved",
      "accessRequests.approveError": "approve failed",
      "accessRequests.rejectSuccess": "rejected",
      "accessRequests.rejectError": "reject failed",
      "accessRequests.codeSendFailed": "approved but code failed",
      "accessRequests.resendCode": "Resend",
      "accessRequests.resendCodeSuccess": "resent",
      "accessRequests.resendCodeFailed": "resend failed",
      "analytics.resendRateLimited": "rate limited",
      "analytics.resendNotNeeded": "not needed",
    },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>
  );
  return renderHook(() => useAccessRequestReview(afterReview), { wrapper });
}

describe("useAccessRequestReview", () => {
  beforeEach(() => {
    approveLinkAccessRequestMock.mockReset();
    rejectLinkAccessRequestMock.mockReset();
    resendLinkAccessCodeMock.mockReset();
    toastSuccess.mockReset();
    toastError.mockReset();
    toastWarning.mockReset();
  });

  it("toasts success and runs afterReview on approve", async () => {
    const afterReview = vi.fn();
    approveLinkAccessRequestMock.mockResolvedValue({
      data: { id: "req-1", status: "approved" },
    });
    const { result } = await renderReviewHook(afterReview);

    await act(async () => {
      await result.current.approve("link-1", {
        id: "req-1",
        email: "visitor@example.com",
      });
    });

    expect(toastSuccess).toHaveBeenCalledWith("approved");
    expect(toastWarning).not.toHaveBeenCalled();
    expect(afterReview).toHaveBeenCalledWith({
      email: "visitor@example.com",
      action: "approve",
      linkId: "link-1",
      requestId: "req-1",
    });
  });

  it("treats code-send warning as approved and offers resend", async () => {
    const afterReview = vi.fn();
    approveLinkAccessRequestMock.mockResolvedValue({
      data: { id: "req-1", status: "approved" },
      warning: {
        code: "access_code_send_failed",
        message: "could not send verification code",
      },
    });
    const { result } = await renderReviewHook(afterReview);

    await act(async () => {
      await result.current.approve("link-1", {
        id: "req-1",
        email: "visitor@example.com",
      });
    });

    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastWarning).toHaveBeenCalledWith(
      "approved but code failed",
      expect.objectContaining({
        action: expect.objectContaining({ label: "Resend" }),
      }),
    );
    expect(afterReview).toHaveBeenCalledWith(
      expect.objectContaining({ action: "approve", requestId: "req-1" }),
    );

    const warningOpts = toastWarning.mock.calls[0]?.[1] as {
      action: { onClick: () => void };
    };
    resendLinkAccessCodeMock.mockResolvedValue(undefined);
    await act(async () => {
      warningOpts.action.onClick();
    });
    await waitFor(() => {
      expect(resendLinkAccessCodeMock).toHaveBeenCalledWith(
        "link-1",
        "visitor@example.com",
        true,
      );
    });
    expect(toastSuccess).toHaveBeenCalledWith("resent");
  });

  it("treats legacy 502 code-send failure as approved", async () => {
    const afterReview = vi.fn();
    approveLinkAccessRequestMock.mockRejectedValue(
      new ApiError({
        status: 502,
        code: "access_code_send_failed",
        message: "could not send verification code",
        requestId: "r1",
      }),
    );
    const { result } = await renderReviewHook(afterReview);

    await act(async () => {
      await result.current.approve("link-1", {
        id: "req-1",
        email: "visitor@example.com",
      });
    });

    expect(toastError).not.toHaveBeenCalled();
    expect(toastWarning).toHaveBeenCalled();
    expect(afterReview).toHaveBeenCalledWith(
      expect.objectContaining({ action: "approve" }),
    );
  });
});
