import { describe, it, expect } from "vitest";
import {
  canAddDocumentToDealRoom,
  canToggleAgreementCategory,
  isAgreementCategory,
  isDealRoomCategory,
  shouldShowCategoryBadge,
} from "./documentCategory";

describe("documentCategory", () => {
  it("identifies agreement and deal_room partitions", () => {
    expect(isAgreementCategory("agreement")).toBe(true);
    expect(isDealRoomCategory("deal_room")).toBe(true);
    expect(isAgreementCategory("general")).toBe(false);
    expect(isDealRoomCategory(undefined)).toBe(false);
  });

  it("allows add-to-room only for general library docs", () => {
    expect(canAddDocumentToDealRoom("general")).toBe(true);
    expect(canAddDocumentToDealRoom(undefined)).toBe(true);
    expect(canAddDocumentToDealRoom("agreement")).toBe(false);
    expect(canAddDocumentToDealRoom("deal_room")).toBe(false);
  });

  it("guards agreement toggle by category and PDF type", () => {
    expect(canToggleAgreementCategory("deal_room", { fileType: "pdf" })).toBe(false);
    expect(canToggleAgreementCategory("agreement")).toBe(true);
    expect(canToggleAgreementCategory("general", { fileType: "pdf" })).toBe(true);
    expect(canToggleAgreementCategory("general", { fileType: "docx" })).toBe(false);
  });

  it("shows category badge only for non-general partitions", () => {
    expect(shouldShowCategoryBadge("general")).toBe(false);
    expect(shouldShowCategoryBadge("agreement")).toBe(true);
    expect(shouldShowCategoryBadge("deal_room")).toBe(true);
  });
});
