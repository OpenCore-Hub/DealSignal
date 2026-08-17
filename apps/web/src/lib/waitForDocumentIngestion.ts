import { isDocumentReadyForLibraryShare } from "@/lib/documentsUploadedEvent";

/** Ingestion is usually a few seconds; large files can take longer. */
export const DOCUMENT_INGESTION_POLL_MS = 2_000;
export const DOCUMENT_INGESTION_TIMEOUT_MS = 120_000;

export function isDocumentIngestionFailed(
  status: string | undefined | null,
): boolean {
  const normalized = (status ?? "").trim().toLowerCase();
  return normalized === "failed" || normalized === "archived";
}

export type DocumentIngestionOutcome<T> =
  | { outcome: "ready"; document: T }
  | { outcome: "failed"; document: T }
  | { outcome: "timeout"; document: T }
  | { outcome: "aborted"; document: T };

function raceWithTimeout<T>(
  promise: Promise<T>,
  ms: number,
  signal?: AbortSignal,
): Promise<T> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException("Aborted", "AbortError"));
      return;
    }
    const timer = setTimeout(() => {
      cleanup();
      reject(new DOMException("Ingestion poll timed out", "TimeoutError"));
    }, ms);
    const onAbort = () => {
      cleanup();
      reject(new DOMException("Aborted", "AbortError"));
    };
    const cleanup = () => {
      clearTimeout(timer);
      signal?.removeEventListener("abort", onAbort);
    };
    signal?.addEventListener("abort", onAbort, { once: true });
    promise.then(
      (value) => {
        cleanup();
        resolve(value);
      },
      (error) => {
        cleanup();
        reject(error);
      },
    );
  });
}

function delay(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException("Aborted", "AbortError"));
      return;
    }
    const onAbort = () => {
      clearTimeout(timer);
      reject(new DOMException("Aborted", "AbortError"));
    };
    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

/**
 * POST /documents returns as soon as the object is stored — status is usually
 * still "processing". Create-link requires ready. Poll the status endpoint
 * until ingestion finishes, fails, times out, or the caller aborts.
 */
export async function waitForDocumentIngestion<T extends { status: string }>(opts: {
  initial: T;
  fetchStatus: () => Promise<T>;
  intervalMs?: number;
  timeoutMs?: number;
  signal?: AbortSignal;
  /** Fired after each successful status poll (not the initial POST payload). */
  onStatus?: (document: T) => void;
}): Promise<DocumentIngestionOutcome<T>> {
  const intervalMs = opts.intervalMs ?? DOCUMENT_INGESTION_POLL_MS;
  const timeoutMs = opts.timeoutMs ?? DOCUMENT_INGESTION_TIMEOUT_MS;
  let current = opts.initial;

  const classify = (document: T): DocumentIngestionOutcome<T> | null => {
    if (isDocumentReadyForLibraryShare(document.status)) {
      return { outcome: "ready", document };
    }
    if (isDocumentIngestionFailed(document.status)) {
      return { outcome: "failed", document };
    }
    return null;
  };

  const immediate = classify(current);
  if (immediate) return immediate;
  if (opts.signal?.aborted) return { outcome: "aborted", document: current };

  const startedAt = Date.now();
  while (!opts.signal?.aborted) {
    const remaining = timeoutMs - (Date.now() - startedAt);
    if (remaining <= 0) return { outcome: "timeout", document: current };
    try {
      current = await raceWithTimeout(opts.fetchStatus(), remaining, opts.signal);
      opts.onStatus?.(current);
    } catch (error) {
      if (opts.signal?.aborted) return { outcome: "aborted", document: current };
      if (error instanceof DOMException && error.name === "AbortError") {
        return { outcome: "aborted", document: current };
      }
      if (error instanceof DOMException && error.name === "TimeoutError") {
        return { outcome: "timeout", document: current };
      }
      // Transient GET failures: keep polling until timeout.
    }
    // Leaving the page / removing the row always wins over a late ready payload.
    if (opts.signal?.aborted) return { outcome: "aborted", document: current };
    const next = classify(current);
    if (next) return next;
    if (Date.now() - startedAt >= timeoutMs) {
      return { outcome: "timeout", document: current };
    }
    try {
      await delay(intervalMs, opts.signal);
    } catch {
      return { outcome: "aborted", document: current };
    }
  }
  return { outcome: "aborted", document: current };
}
