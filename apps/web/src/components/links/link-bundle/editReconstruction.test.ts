import { describe, it, expect } from "vitest";
import type { Link, PermissionConfig } from "@/types";
import { resolveExpiryDaysFromExpiresAt, resolveMaxViewsFromAccessCount } from "./pipelineUtils";

/**
 * Tests for the edit-mode security config reconstruction logic
 * in BundlePipelinePage.tsx.
 *
 * These tests verify that the INIT_FOR_EDIT payload correctly
 * reconstructs PermissionConfig from link data without losing
 * download/watermark/expiry/maxViews settings.
 */

function reconstructConfig(
  link: Link,
): Pick<PermissionConfig, "allowDownload" | "watermarkEnabled" | "expiryDays" | "maxViews"> & {
  _editExpiresAt?: string;
  _editMaxViews?: number;
} {
  const { expiryDays, _editExpiresAt } = resolveExpiryDaysFromExpiresAt(link.expiresAt);
  const { maxViews, _editMaxViews } = resolveMaxViewsFromAccessCount(link.maxAccessCount);

  return {
    allowDownload: link.downloadEnabled ?? true,
    watermarkEnabled: link.watermarkEnabled ?? true,
    expiryDays,
    maxViews,
    _editExpiresAt: link.expiresAt ?? _editExpiresAt,
    _editMaxViews,
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
    expect(config.expiryDays).toBe(7);
  });

  it("snaps 15-day expiry onto the preset", () => {
    const future = new Date();
    future.setDate(future.getDate() + 15);
    const config = reconstructConfig({
      ...baseLink,
      expiresAt: future.toISOString(),
    });
    expect(config.expiryDays).toBe(15);
  });

  it("maps non-preset expiry (e.g. 90 days) to custom", () => {
    const future = new Date();
    future.setDate(future.getDate() + 90);
    const iso = future.toISOString();
    const config = reconstructConfig({
      ...baseLink,
      expiresAt: iso,
    });
    expect(config.expiryDays).toBe("custom");
    expect(config._editExpiresAt).toBe(iso);
  });

  it("defaults to 30 days when no expiresAt", () => {
    const config = reconstructConfig({ ...baseLink, expiresAt: undefined });
    expect(config.expiryDays).toBe(30);
  });

  it("uses custom when link has already expired", () => {
    const past = new Date();
    past.setDate(past.getDate() - 10);
    const iso = past.toISOString();
    const config = reconstructConfig({
      ...baseLink,
      expiresAt: iso,
    });
    expect(config.expiryDays).toBe("custom");
    expect(config._editExpiresAt).toBe(iso);
  });

  it("maps non-preset maxAccessCount to custom", () => {
    const config = reconstructConfig({ ...baseLink, maxAccessCount: 25 });
    expect(config.maxViews).toBe("custom");
    expect(config._editMaxViews).toBe(25);
  });

  it("maps maxAccessCount=10 to maxViews", () => {
    const config = reconstructConfig({ ...baseLink, maxAccessCount: 10 });
    expect(config.maxViews).toBe(10);
  });

  it("maps missing maxAccessCount to unlimited", () => {
    const config = reconstructConfig({ ...baseLink, maxAccessCount: undefined });
    expect(config.maxViews).toBe("unlimited");
  });

  it("maps zero maxAccessCount to unlimited", () => {
    const config = reconstructConfig({ ...baseLink, maxAccessCount: 0 });
    expect(config.maxViews).toBe("unlimited");
  });

  it("reconstructs a full confidential bundle config", () => {
    const future = new Date();
    future.setDate(future.getDate() + 45);
    const iso = future.toISOString();
    const config = reconstructConfig({
      ...baseLink,
      downloadEnabled: false,
      watermarkEnabled: true,
      maxAccessCount: 50,
      expiresAt: iso,
      isBundle: true,
      documents: [
        { id: "doc-1", title: "A", sourceType: "pdf", pageCount: 10, status: "ready" },
        { id: "doc-2", title: "B", sourceType: "pdf", pageCount: 5, status: "ready" },
      ],
    });
    expect(config.allowDownload).toBe(false);
    expect(config.watermarkEnabled).toBe(true);
    expect(config.maxViews).toBe(50);
    expect(config.expiryDays).toBe("custom");
    expect(config._editExpiresAt).toBe(iso);
  });
});
