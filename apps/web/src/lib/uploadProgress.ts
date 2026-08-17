/**
 * Byte-transfer is only the first slice of the bar. Ingestion uses the same
 * 25 / 50 / 100 steps as the documents list (`documentProgress`), so the long
 * wait sits at 50% — not 90% — until the file is actually ready.
 */
export const UPLOAD_TRANSFER_MAX = 40;

/**
 * Real multipart bytes → bar percent. Returns null when the browser cannot
 * report a total so callers keep the last known value instead of inventing one.
 */
export function transferBarPercent(loaded: number, total: number): number | null {
  if (!Number.isFinite(total) || total <= 0) return null;
  if (!Number.isFinite(loaded) || loaded < 0) return 0;
  return Math.min(UPLOAD_TRANSFER_MAX, Math.round((loaded / total) * UPLOAD_TRANSFER_MAX));
}

function serverProgressFromStatus(status: string): number {
  const normalized = status.trim().toLowerCase();
  if (normalized === "ready" || normalized === "completed") return 100;
  if (normalized === "failed") return 0;
  if (normalized === "processing") return 50;
  return 25;
}

/**
 * Documents-page progress: pending 25, processing 50, ready 100.
 * A leftover transfer floor cannot pull the bar back up into the 90s.
 */
export function ingestionBarPercent(
  serverProgress: number | undefined,
  floor: number,
  status?: string,
): number {
  let raw = serverProgress;
  if (raw == null && status) {
    raw = serverProgressFromStatus(status);
  }
  if (raw == null || !Number.isFinite(raw)) {
    raw = status ? serverProgressFromStatus(status) : 50;
  }
  if (raw >= 100) return 100;
  if (raw <= 0) return 0;
  const transferFloor = Math.min(Math.max(floor, 0), UPLOAD_TRANSFER_MAX);
  return Math.min(99, Math.max(raw, transferFloor));
}
