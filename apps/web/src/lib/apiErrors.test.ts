import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/apiClient";

const translations: Record<string, string> = {
  "common:error.loadFailed": "Failed to load",
  "common:error.saveFailed": "Failed to save",
  "common:error.codes.internal_error": "Something went wrong.",
  "common:error.codes.access_code_send_failed":
    "Could not send the verification code. Please try again.",
  "common:error.codes.blocked_email": "Email blocked",
  "common:error.codes.email_mismatch": "Email does not match the invitation.",
  "common:error.codes.invitation_email_mismatch":
    "You're signed in with a different email than this invitation.",
  "common:error.codes.folder_exists": "Folder exists",
  "common:error.duplicateSlug": "Slug taken",
  "documents:viewer.emailBlocked": "This email is blocked.",
  "documents:viewer.emailMismatch":
    "The reserved delivery email does not match this link’s verification code.",
  "documents:viewer.invalidPassword": "Incorrect password.",
  "documents:viewer.blocked_emailDescription": "Blocked from this link.",
  "linkShare:share.linkNameDuplicate": "Name taken",
  "auth:login.errorInvalidCredentials": "Bad credentials",
  "auth:verifyEmail.error": "Verify failed",
  "auth:acceptInvitation.emailMismatch":
    "You're signed in with a different email than this invitation. Sign out and continue with the invited email.",
};

vi.mock("@/i18n/config", () => ({
  default: {
    t: (key: string) => translations[key] ?? "",
    exists: (key: string) => key in translations,
  },
}));

import { apiErrorMessage } from "./apiErrors";

describe("apiErrorMessage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("never returns raw ApiError.message", () => {
    const err = new ApiError({
      status: 403,
      code: "blocked_email",
      message: "email is blocked",
      requestId: "req-1",
    });
    expect(apiErrorMessage(err, { context: "viewerGate" })).toBe("This email is blocked.");
    expect(apiErrorMessage(err, { context: "viewerGate" })).not.toBe("email is blocked");
  });

  it("maps login context", () => {
    const err = new ApiError({
      status: 401,
      code: "unauthorized",
      message: "unauthorized",
      requestId: "req-2",
    });
    expect(apiErrorMessage(err, { context: "login" })).toBe("Bad credentials");
  });

  it("maps duplicate slug and name", () => {
    expect(
      apiErrorMessage(
        new ApiError({ status: 409, code: "duplicate_slug", message: "dup", requestId: "r" }),
      ),
    ).toBe("Slug taken");
    expect(
      apiErrorMessage(
        new ApiError({ status: 409, code: "duplicate_name", message: "dup", requestId: "r" }),
      ),
    ).toBe("Name taken");
  });

  it("falls back for unknown codes", () => {
    expect(
      apiErrorMessage(
        new ApiError({ status: 500, code: "unknown_code", message: "db exploded", requestId: "r" }),
        { fallback: "saveFailed" },
      ),
    ).toBe("Failed to save");
  });

  it("maps common error codes registry", () => {
    expect(
      apiErrorMessage(
        new ApiError({ status: 500, code: "internal_error", message: "SQLSTATE", requestId: "r" }),
      ),
    ).toBe("Something went wrong.");
    expect(
      apiErrorMessage(
        new ApiError({
          status: 502,
          code: "access_code_send_failed",
          message: 'smtp close data: 550 "Queueing failed"',
          requestId: "r",
        }),
      ),
    ).toBe("Could not send the verification code. Please try again.");
  });

  it("maps viewer gate inline messages", () => {
    expect(
      apiErrorMessage(
        new ApiError({ status: 403, code: "invalid_password", message: "bad", requestId: "r" }),
        { context: "viewerGate" },
      ),
    ).toBe("Incorrect password.");
  });

  it("keeps workspace invite email mismatch off viewer copy", () => {
    const err = new ApiError({
      status: 403,
      code: "invitation_email_mismatch",
      message: "email does not match invitation",
      requestId: "r",
    });
    expect(apiErrorMessage(err, { context: "acceptInvitation" })).toBe(
      "You're signed in with a different email than this invitation. Sign out and continue with the invited email.",
    );
    expect(apiErrorMessage(err)).toBe("You're signed in with a different email than this invitation.");
    expect(
      apiErrorMessage(
        new ApiError({ status: 403, code: "email_mismatch", message: "mismatch", requestId: "r" }),
        { context: "viewerGate" },
      ),
    ).toBe("The reserved delivery email does not match this link’s verification code.");
  });
});
