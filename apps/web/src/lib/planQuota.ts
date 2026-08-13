/** Finite plan caps: limit <= 0 means unlimited. */
export function usageAtCap(used: number, limit: number): boolean {
  return Number.isFinite(limit) && limit > 0 && Number.isFinite(used) && used >= limit;
}
