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
  "documents:viewer.link_archivedDescription":
    "This share link is no longer active. Please contact the sender if you need access.",
  "linkShare:share.linkNameDuplicate": "Name taken",
  "auth:login.errorInvalidCredentials": "Bad credentials",
  "auth:verifyEmail.error": "Verify failed",
  "auth:acceptInvitation.emailMismatch":
    "You're signed in with a different email than this invitation. Sign out and continue with the invited email.",
  "auth:acceptInvitation.planLimitSeats":
    "This workspace has no available team seats. Ask an admin to free a seat or upgrade the plan, then try again.",
  "common:error.codes.plan_limit_seats":
    "This workspace has no available team seats.",
  "common:error.codes.plan_payment_required":
    "This plan requires checkout before it can be activated.",
      "common:error.codes.plan_sales_assisted":
        "Enterprise is provisioned with our sales team, not self-serve checkout.",
      "common:error.codes.plan_manage_via_portal":
        "Manage this subscription in the billing portal.",
      "common:error.codes.stripe_not_configured": "Checkout is not configured.",
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

  it("maps archived share links to visitor-facing unavailable copy", () => {
    expect(
      apiErrorMessage(
        new ApiError({ status: 410, code: "link_archived", message: "link archived", requestId: "r" }),
      ),
    ).toBe("This share link is no longer active. Please contact the sender if you need access.");
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

  it("maps plan_limit_seats for invitee accept context", () => {
    const err = new ApiError({
      status: 403,
      code: "plan_limit_seats",
      message: "internal seat limit reached for this plan",
      requestId: "r",
    });
    expect(apiErrorMessage(err, { context: "acceptInvitation" })).toBe(
      "This workspace has no available team seats. Ask an admin to free a seat or upgrade the plan, then try again.",
    );
    expect(apiErrorMessage(err, { context: "acceptInvitation" })).not.toMatch(/invite more/i);
  });

  it("maps unpaid plan change codes", () => {
    expect(
      apiErrorMessage(
        new ApiError({
          status: 402,
          code: "plan_payment_required",
          message: "pay",
          requestId: "r",
        }),
      ),
    ).toBe("This plan requires checkout before it can be activated.");
    expect(
      apiErrorMessage(
        new ApiError({
          status: 403,
          code: "plan_sales_assisted",
          message: "sales",
          requestId: "r",
        }),
      ),
    ).toBe("Enterprise is provisioned with our sales team, not self-serve checkout.");
    expect(
      apiErrorMessage(
        new ApiError({
          status: 409,
          code: "plan_manage_via_portal",
          message: "portal",
          requestId: "r",
        }),
      ),
    ).toBe("Manage this subscription in the billing portal.");
  });
});
