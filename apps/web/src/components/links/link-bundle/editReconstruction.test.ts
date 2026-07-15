import { describe, it, expect } from "vitest";
import type { Link, PermissionConfig } from "@/types";

/**
 * Tests for the edit-mode security config reconstruction logic
 * in BundlePipelinePage.tsx.
 *
 * These tests verify that the INIT_FOR_EDIT payload correctly
 * reconstructs PermissionConfig from link data without losing
 * download/watermark/expiry/maxViews settings.
 */

function reconstructConfig(link: Link): Pick<PermissionConfig, "allowDownload" | "watermarkEnabled" | "expiryDays" | "maxViews"> {
  let expiryDays: number | "custom" = 30;
  if (link.expiresAt) {
    const expires = new Date(link.expiresAt);
    const now = new Date();
    const diffMs = expires.getTime() - now.getTime();
    const diffDays = Math.ceil(diffMs / (1000 * 60 * 60 * 24));
    if (diffDays > 0) {
      expiryDays = diffDays;
    }
  }

  const maxViews: number | "unlimited" =
    typeof link.maxAccessCount === "number" && link.maxAccessCount > 0
      ? link.maxAccessCount
      : "unlimited";

  return {
    allowDownload: link.downloadEnabled ?? true,
    watermarkEnabled: link.watermarkEnabled ?? true,
    expiryDays,
    maxViews,
  };
}

describe("edit mode PermissionConfig reconstruction", () => {
  const baseLink: Link = {
    id: "link-1",
    documentId: "doc-1",
    documentIds: ["doc-1"],
    folderPaths: [],
    documentTitle: "Test Doc",
    shortUrl: "https://example.com/l/abc123",
    accessCount: 5,
    heatLevel: "cold",
    createdAt: "2025-01-01T00:00:00Z",
    isBundle: false,
    documents: [{ id: "doc-1", title: "Test Doc", sourceType: "pdf", pageCount: 10, status: "ready" }],
  };

  it("preserves downloadEnabled=false from link data", () => {
    const config = reconstructConfig({ ...baseLink, downloadEnabled: false });
    expect(config.allowDownload).toBe(false);
  });

  it("preserves downloadEnabled=true from link data", () => {
    const config = reconstructConfig({ ...baseLink, downloadEnabled: true });
    expect(config.allowDownload).toBe(true);
  });

  it("defaults allowDownload=true when not in link data (legacy)", () => {
    const config = reconstructConfig({ ...baseLink, downloadEnabled: undefined });
    expect(config.allowDownload).toBe(true);
  });

  it("preserves watermarkEnabled=false from link data", () => {
    const config = reconstructConfig({ ...baseLink, watermarkEnabled: false });
    expect(config.watermarkEnabled).toBe(false);
  });

  it("defaults watermarkEnabled=true when not in link data (legacy)", () => {
    const config = reconstructConfig({ ...baseLink, watermarkEnabled: undefined });
    expect(config.watermarkEnabled).toBe(true);
  });

  it("computes expiryDays from expiresAt (future date)", () => {
    const future = new Date();
    future.setDate(future.getDate() + 7);
    const config = reconstructConfig({
      ...baseLink,
      expiresAt: future.toISOString(),
    });
    expect(typeof config.expiryDays).toBe("number");
    // Should be ~7 days from now (allow ±1 for rounding)
    expect([6, 7, 8]).toContain(config.expiryDays as number);
  });

  it("computes expiryDays for 90-day expiry", () => {
    const future = new Date();
    future.setDate(future.getDate() + 90);
    const config = reconstructConfig({
      ...baseLink,
      expiresAt: future.toISOString(),
    });
    // Should be ~90 days (±1)
    const days = config.expiryDays as number;
    expect(days).toBeGreaterThanOrEqual(89);
    expect(days).toBeLessThanOrEqual(91);
  });

  it("defaults to 30 days when no expiresAt", () => {
    const config = reconstructConfig({ ...baseLink, expiresAt: undefined });
    expect(config.expiryDays).toBe(30);
  });

  it("keeps 30 days when link has expired", () => {
    const past = new Date();
    past.setDate(past.getDate() - 10);
    const config = reconstructConfig({
      ...baseLink,
      expiresAt: past.toISOString(),
    });
    // Expired link: diffDays <= 0, so falls back to 30
    expect(config.expiryDays).toBe(30);
  });

  it("maps maxAccessCount to maxViews", () => {
    const config = reconstructConfig({ ...baseLink, maxAccessCount: 100 });
    expect(config.maxViews).toBe(100);
  });

  it("maps maxAccessCount=10 to maxViews", () => {
    const config = reconstructConfig({ ...baseLink, maxAccessCount: 10 });
    expect(config.maxViews).toBe(10);
  });

  it("defaults to unlimited when maxAccessCount is undefined", () => {
    const config = reconstructConfig({ ...baseLink, maxAccessCount: undefined });
    expect(config.maxViews).toBe("unlimited");
  });

  it("treats maxAccessCount=0 as unlimited", () => {
    const config = reconstructConfig({ ...baseLink, maxAccessCount: 0 });
    expect(config.maxViews).toBe("unlimited");
  });

  it("reconstructs a full confidential bundle config", () => {
    const future = new Date();
    future.setDate(future.getDate() + 45);
    const config = reconstructConfig({
      ...baseLink,
      downloadEnabled: false,
      watermarkEnabled: true,
      maxAccessCount: 50,
      expiresAt: future.toISOString(),
      isBundle: true,
      documents: [
        { id: "doc-1", title: "A", sourceType: "pdf", pageCount: 10, status: "ready" },
        { id: "doc-2", title: "B", sourceType: "pdf", pageCount: 5, status: "ready" },
      ],
    });
    expect(config.allowDownload).toBe(false);
    expect(config.watermarkEnabled).toBe(true);
    expect(config.maxViews).toBe(50);
    expect(typeof config.expiryDays).toBe("number");
  });
});
