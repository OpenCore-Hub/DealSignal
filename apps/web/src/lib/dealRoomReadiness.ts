/**
 * Readiness helpers for deal-room documents home.
 * Prefer required-file gaps over empty-folder occupancy counts.
 */

function normalizeText(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
}

export function matchesRecommendedFile(documentTitle: string, recommendedName: string): boolean {
  const title = normalizeText(documentTitle);
  const rec = normalizeText(recommendedName);
  if (!title || !rec) return false;
  if (title.includes(rec)) return true;
  if (rec.includes(title) && title.length > 3) return true;
  const recWords = rec.split(" ").filter(Boolean);
  if (recWords.length > 1) {
    return recWords.every((word) => title.includes(word));
  }
  return false;
}

export function findMissingRecommendedFiles(
  recommendedFiles: string[],
  documentTitles: string[]
): string[] {
  return recommendedFiles.filter(
    (rec) => !documentTitles.some((title) => matchesRecommendedFile(title, rec))
  );
}

/** Pick the folder whose name best matches a recommended file label. */
export function resolveFolderForRecommended(
  recommendedName: string,
  folders: { path: string; name: string }[]
): string | null {
  const ranked = folders
    .filter((f) => f.path !== "/")
    .map((f) => ({
      path: f.path,
      score: matchesRecommendedFile(f.name, recommendedName)
        ? 2
        : normalizeText(f.name)
              .split(" ")
              .some((w) => w.length > 2 && normalizeText(recommendedName).includes(w))
          ? 1
          : 0,
    }))
    .filter((x) => x.score > 0)
    .sort((a, b) => b.score - a.score);
  return ranked[0]?.path ?? folders.find((f) => f.path !== "/")?.path ?? null;
}
