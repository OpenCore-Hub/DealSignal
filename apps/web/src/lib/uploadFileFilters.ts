/**
 * Client-side upload guards. Keep in sync with upload.ValidateUploadFilename
 * in apps/api/internal/upload/service.go (defense in depth).
 */

/** Returns true for OS/editor sidecar files that should never be uploaded. */
export function isBlockedUploadSidecar(name: string): boolean {
  const base = name.split(/[/\\]/).pop()?.trim() ?? "";
  if (!base) return true;
  const lower = base.toLowerCase();
  return base.startsWith("~$") || base.startsWith("._") || lower === ".ds_store";
}

/** Returns true when a File should be skipped before upload. */
export function isBlockedUploadFile(file: File): boolean {
  return isBlockedUploadSidecar(file.name) || file.size === 0;
}

/** Drop hidden sidecars and empty files; keep original order. */
export function filterUploadFiles(files: File[]): File[] {
  return files.filter((file) => !isBlockedUploadFile(file));
}

export type UploadSelectionFilterResult = {
  files: File[];
  skippedCount: number;
  allBlocked: boolean;
};

/** Filter a picker/drop selection and report how many entries were skipped. */
export function filterUploadSelection(files: File[]): UploadSelectionFilterResult {
  const filtered = filterUploadFiles(files);
  const skippedCount = files.length - filtered.length;
  return {
    files: filtered,
    skippedCount,
    allBlocked: files.length > 0 && filtered.length === 0,
  };
}

export type UploadFilterNotifier = {
  error(message: string): void;
  message(message: string): void;
};

/**
 * Show toast feedback when sidecars/empty files were skipped.
 * Returns false when every selected file was blocked (caller should abort).
 */
export function notifyUploadSelectionFiltered(
  result: UploadSelectionFilterResult,
  message: string,
  notify: UploadFilterNotifier,
): boolean {
  if (result.skippedCount === 0) return true;
  if (result.allBlocked) {
    notify.error(message);
    return false;
  }
  notify.message(message);
  return true;
}

/** @deprecated Use filterUploadFiles */
export function filterUploadSidecars(files: File[]): File[] {
  return filterUploadFiles(files);
}
