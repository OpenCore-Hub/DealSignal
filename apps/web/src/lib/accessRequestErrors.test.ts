import { describe, expect, it } from "vitest";
import { ApiError } from "@/lib/apiClient";
import {
  accessRequestReviewErrorMessage,
  isAccessCodeSendFailedAfterApprove,
  isAccessCodeSendFailedWarning,
} from "./accessRequestErrors";

const t = (key: string) => `i18n:${key}`;

describe("access code send failed helpers", () => {
  it("detects soft warning and legacy ApiError", () => {
    expect(
      isAccessCodeSendFailedWarning({ code: "access_code_send_failed" }),
    ).toBe(true);
    expect(isAccessCodeSendFailedWarning({ code: "other" })).toBe(false);
    expect(
      isAccessCodeSendFailedAfterApprove(
        new ApiError({
          status: 502,
          code: "access_code_send_failed",
          message: "raw",
          requestId: "r1",
        }),
      ),
    ).toBe(true);
    expect(isAccessCodeSendFailedAfterApprove(new Error("nope"))).toBe(false);
  });
});

describe("accessRequestReviewErrorMessage", () => {
  it("maps known ApiError codes to i18n keys", () => {
    expect(
      accessRequestReviewErrorMessage(
        new ApiError({
          status: 403,
          code: "access_request_forbidden",
          message: "raw",
          requestId: "r1",
        }),
        t,
        "accessRequests.approveError",
      ),
    ).toBe("i18n:accessRequests.notFound");

    expect(
      accessRequestReviewErrorMessage(
        new ApiError({
          status: 404,
          code: "access_request_not_found",
          message: "raw",
          requestId: "r1",
        }),
        t,
        "accessRequests.rejectError",
      ),
    ).toBe("i18n:accessRequests.notFound");

    expect(
      accessRequestReviewErrorMessage(
        new ApiError({
          status: 404,
          code: "link_not_found",
          message: "raw",
          requestId: "r1",
        }),
        t,
        "accessRequests.approveError",
      ),
    ).toBe("i18n:accessRequests.notFound");

    expect(
      accessRequestReviewErrorMessage(
        new ApiError({
          status: 409,
          code: "access_request_not_pending",
          message: "raw",
          requestId: "r1",
        }),
        t,
        "accessRequests.approveError",
      ),
    ).toBe("i18n:accessRequests.notPending");

    expect(
      accessRequestReviewErrorMessage(
        new ApiError({
          status: 403,
          code: "access_request_blocked",
          message: "raw",
          requestId: "r1",
        }),
        t,
        "accessRequests.approveError",
      ),
    ).toBe("i18n:accessRequests.blocked");

    expect(
      accessRequestReviewErrorMessage(
        new ApiError({
          status: 502,
          code: "access_code_send_failed",
          message: "raw",
          requestId: "r1",
        }),
        t,
        "accessRequests.approveError",
      ),
    ).toBe("i18n:accessRequests.codeSendFailed");
  });

  it("falls back without leaking unknown raw messages", () => {
    expect(
      accessRequestReviewErrorMessage(
        new Error("postgres boom"),
        t,
        "accessRequests.approveError",
      ),
    ).toBe("i18n:accessRequests.approveError");

    expect(
      accessRequestReviewErrorMessage(
        new ApiError({
          status: 500,
          code: "internal_error",
          message: "secret detail",
          requestId: "r1",
        }),
        t,
        "accessRequests.rejectError",
      ),
    ).toBe("i18n:accessRequests.rejectError");
  });
});
