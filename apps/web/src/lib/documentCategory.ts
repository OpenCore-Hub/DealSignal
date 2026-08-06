import type { DocumentCategory } from "@/types";

function normalizeCategory(category?: DocumentCategory | string | null): DocumentCategory {
  if (category === "agreement" || category === "deal_room") return category;
  return "general";
}

/** Primary library partition for document list / share-content pickers. */
export const LIBRARY_DOCUMENT_CATEGORY = "general" as const;

/** Exported for display components that need the normalized partition. */
export function normalizeCategoryForDisplay(category?: DocumentCategory | string | null): DocumentCategory {
  return normalizeCategory(category);
}

export function isAgreementCategory(category?: DocumentCategory | string | null): boolean {
  return normalizeCategory(category) === "agreement";
}

export function isDealRoomCategory(category?: DocumentCategory | string | null): boolean {
  return normalizeCategory(category) === "deal_room";
}

/** Library docs only; agreement and deal_room partitions use other surfaces. */
export function canAddDocumentToDealRoom(category?: DocumentCategory | string | null): boolean {
  const normalized = normalizeCategory(category);
  return normalized !== "agreement" && normalized !== "deal_room";
}

export function canToggleAgreementCategory(
  category?: DocumentCategory | string | null,
  opts?: { sourceType?: string; fileType?: string },
): boolean {
  if (isDealRoomCategory(category)) return false;
  if (isAgreementCategory(category)) return true;
  const type = (opts?.fileType ?? opts?.sourceType ?? "").toLowerCase();
  return type === "pdf";
}

export function agreementCategoryErrorCode(code: string): code is keyof typeof AGREEMENT_CATEGORY_ERROR_CODES {
  return code in AGREEMENT_CATEGORY_ERROR_CODES;
}

/** i18n keys under documents:detail.categoryErrors.* */
export const AGREEMENT_CATEGORY_ERROR_CODES = {
  agreement_not_allowed_in_deal_room: true,
  category_immutable: true,
  category_while_in_room: true,
  category_deal_room_via_api: true,
} as const;

export function shouldShowCategoryBadge(category?: DocumentCategory | string | null): boolean {
  return normalizeCategory(category) !== "general";
}
