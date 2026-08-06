import { describe, expect, it } from "vitest";
import { ApiError } from "@/lib/apiClient";
import {
  isViewerAccessErrorKind,
  viewerAccessErrorKind,
  viewerAccessErrorKindFromUnknown,
  viewerPolicyBlockI18nKeys,
} from "./viewerAccessErrors";

describe("viewerAccessErrors", () => {
  it("maps API codes to viewer error kinds", () => {
    expect(viewerAccessErrorKind("document_locked")).toBe("locked");
    expect(viewerAccessErrorKind("document_out_of_scope")).toBe("out_of_scope");
    expect(viewerAccessErrorKind("blocked_email")).toBe("blocked_email");
    expect(viewerAccessErrorKind("not_allowed")).toBe("not_allowed");
    expect(viewerAccessErrorKind("access_denied")).toBe("denied");
    expect(viewerAccessErrorKind(undefined)).toBe("generic");
  });

  it("reads ApiError instances", () => {
    const err = new ApiError({
      status: 403,
      code: "document_locked",
      message: "document is locked in this deal room",
      requestId: "req-1",
    });
    expect(viewerAccessErrorKindFromUnknown(err)).toBe("locked");
    expect(isViewerAccessErrorKind(viewerAccessErrorKindFromUnknown(err))).toBe(true);
    expect(isViewerAccessErrorKind("locked")).toBe(true);
  });

  it("treats blocked email and allowlist denials as policy blocks", () => {
    expect(isViewerAccessErrorKind("blocked_email")).toBe(true);
    expect(isViewerAccessErrorKind("not_allowed")).toBe(true);
    expect(isViewerAccessErrorKind("denied")).toBe(false);

    expect(viewerPolicyBlockI18nKeys("blocked_email")).toEqual({
      titleKey: "documents:viewer.blocked_emailTitle",
      descriptionKey: "documents:viewer.blocked_emailDescription",
    });
    expect(viewerPolicyBlockI18nKeys("not_allowed")).toEqual({
      titleKey: "documents:viewer.not_allowedTitle",
      descriptionKey: "documents:viewer.not_allowedDescription",
    });
  });
});
