import { ApiError } from "@/lib/apiClient";

export type ViewerAccessErrorKind =
  | "locked"
  | "out_of_scope"
  | "blocked_email"
  | "not_allowed"
  | "denied"
  | "generic";

export function viewerAccessErrorKind(code: string | undefined): ViewerAccessErrorKind {
  switch (code) {
    case "document_locked":
      return "locked";
    case "document_out_of_scope":
      return "out_of_scope";
    case "blocked_email":
      return "blocked_email";
    case "not_allowed":
      return "not_allowed";
    case "access_denied":
      return "denied";
    default:
      return "generic";
  }
}

export function viewerAccessErrorKindFromUnknown(err: unknown): ViewerAccessErrorKind {
  if (err instanceof ApiError) {
    return viewerAccessErrorKind(err.code);
  }
  return "generic";
}

export function isViewerAccessErrorKind(
  kind: ViewerAccessErrorKind,
): kind is "locked" | "out_of_scope" | "blocked_email" | "not_allowed" {
  return (
    kind === "locked" ||
    kind === "out_of_scope" ||
    kind === "blocked_email" ||
    kind === "not_allowed"
  );
}

export function viewerPolicyBlockI18nKeys(
  kind: ViewerAccessErrorKind,
): { titleKey: string; descriptionKey: string } | null {
  switch (kind) {
    case "locked":
      return {
        titleKey: "documents:viewer.documentLockedTitle",
        descriptionKey: "documents:viewer.documentLockedDescription",
      };
    case "out_of_scope":
      return {
        titleKey: "documents:viewer.documentOutOfScopeTitle",
        descriptionKey: "documents:viewer.documentOutOfScopeDescription",
      };
    case "blocked_email":
      return {
        titleKey: "documents:viewer.blocked_emailTitle",
        descriptionKey: "documents:viewer.blocked_emailDescription",
      };
    case "not_allowed":
      return {
        titleKey: "documents:viewer.not_allowedTitle",
        descriptionKey: "documents:viewer.not_allowedDescription",
      };
    default:
      return null;
  }
}
