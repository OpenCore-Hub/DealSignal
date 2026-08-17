import { describe, expect, it, vi } from "vitest";
import {
  isDocumentIngestionFailed,
  waitForDocumentIngestion,
} from "./waitForDocumentIngestion";

describe("isDocumentIngestionFailed", () => {
  it("treats failed and archived as terminal failure", () => {
    expect(isDocumentIngestionFailed("failed")).toBe(true);
    expect(isDocumentIngestionFailed("archived")).toBe(true);
    expect(isDocumentIngestionFailed("processing")).toBe(false);
    expect(isDocumentIngestionFailed("ready")).toBe(false);
  });
});

describe("waitForDocumentIngestion", () => {
  it("returns immediately when the POST payload is already ready", async () => {
    const fetchStatus = vi.fn();
    await expect(
      waitForDocumentIngestion({
        initial: { status: "ready" },
        fetchStatus,
      }),
    ).resolves.toEqual({ outcome: "ready", document: { status: "ready" } });
    expect(fetchStatus).not.toHaveBeenCalled();
  });

  it("returns immediately when the POST payload already failed", async () => {
    await expect(
      waitForDocumentIngestion({
        initial: { status: "failed" },
        fetchStatus: vi.fn(),
      }),
    ).resolves.toEqual({ outcome: "failed", document: { status: "failed" } });
  });

  it("polls until status becomes ready", async () => {
    const fetchStatus = vi
      .fn()
      .mockResolvedValueOnce({ status: "processing" })
      .mockResolvedValueOnce({ status: "ready" });
    const onStatus = vi.fn();
    await expect(
      waitForDocumentIngestion({
        initial: { status: "processing" },
        fetchStatus,
        intervalMs: 1,
        timeoutMs: 1_000,
        onStatus,
      }),
    ).resolves.toEqual({ outcome: "ready", document: { status: "ready" } });
    expect(fetchStatus).toHaveBeenCalledTimes(2);
    expect(onStatus).toHaveBeenCalledTimes(2);
    expect(onStatus).toHaveBeenLastCalledWith({ status: "ready" });
  });

  it("stops when ingestion fails", async () => {
    const fetchStatus = vi.fn().mockResolvedValue({ status: "failed" });
    await expect(
      waitForDocumentIngestion({
        initial: { status: "processing" },
        fetchStatus,
        intervalMs: 1,
        timeoutMs: 1_000,
      }),
    ).resolves.toEqual({ outcome: "failed", document: { status: "failed" } });
  });

  it("times out while still processing", async () => {
    const fetchStatus = vi.fn().mockResolvedValue({ status: "processing" });
    await expect(
      waitForDocumentIngestion({
        initial: { status: "processing" },
        fetchStatus,
        intervalMs: 1,
        timeoutMs: 5,
      }),
    ).resolves.toEqual({ outcome: "timeout", document: { status: "processing" } });
  });

  it("times out when status fetch never returns", async () => {
    const fetchStatus = vi.fn(() => new Promise<{ status: string }>(() => {}));
    await expect(
      waitForDocumentIngestion({
        initial: { status: "processing" },
        fetchStatus,
        intervalMs: 1,
        timeoutMs: 20,
      }),
    ).resolves.toEqual({ outcome: "timeout", document: { status: "processing" } });
  });

  it("aborts without treating the document as ready", async () => {
    const controller = new AbortController();
    const fetchStatus = vi.fn().mockImplementation(async () => {
      controller.abort();
      return { status: "ready" };
    });
    await expect(
      waitForDocumentIngestion({
        initial: { status: "processing" },
        fetchStatus,
        intervalMs: 1,
        timeoutMs: 1_000,
        signal: controller.signal,
      }),
    ).resolves.toMatchObject({ outcome: "aborted" });
  });
});
