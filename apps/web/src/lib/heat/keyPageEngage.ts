/** Same gate as backend documentHeatKeyPageMinSeconds / engaged key-page SQL. */
export const KEY_PAGE_ENGAGED_MIN_SECONDS = 3;

export function isKeyPageEngaged(
  durationSeconds: number,
  minSeconds: number = KEY_PAGE_ENGAGED_MIN_SECONDS,
): boolean {
  return Number.isFinite(durationSeconds) && durationSeconds >= minSeconds;
}
