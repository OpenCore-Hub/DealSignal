import { describe, expect, it } from "vitest";
import {
  filterUploadFiles,
  filterUploadSelection,
  isBlockedUploadFile,
  isBlockedUploadSidecar,
  notifyUploadSelectionFiltered,
} from "./uploadFileFilters";

describe("uploadFileFilters", () => {
  it("blocks excel lock and appledouble sidecars", () => {
    expect(isBlockedUploadSidecar("~$report.xlsx")).toBe(true);
    expect(isBlockedUploadSidecar("._report.xlsx")).toBe(true);
    expect(isBlockedUploadSidecar(".DS_Store")).toBe(true);
    expect(isBlockedUploadSidecar("report.xlsx")).toBe(false);
  });

  it("blocks empty files", () => {
    expect(isBlockedUploadFile(new File([], "report.xlsx"))).toBe(true);
    expect(isBlockedUploadFile(new File(["x"], "report.xlsx"))).toBe(false);
  });

  it("filters sidecars and empty files from lists", () => {
    const files = [
      new File(["a"], "report.xlsx"),
      new File(["b"], "~$report.xlsx"),
      new File([], "empty.xlsx"),
    ];
    expect(filterUploadFiles(files).map((f) => f.name)).toEqual(["report.xlsx"]);
  });

  it("reports skipped sidecars in selection metadata", () => {
    const files = [
      new File(["a"], "report.xlsx"),
      new File(["b"], "~$report.xlsx"),
    ];
    expect(filterUploadSelection(files)).toEqual({
      files: [files[0]],
      skippedCount: 1,
      allBlocked: false,
    });
  });

  it("notifies when every selected file is blocked", () => {
    const errors: string[] = [];
    const messages: string[] = [];
    const ok = notifyUploadSelectionFiltered(
      filterUploadSelection([new File([], "empty.xlsx")]),
      "skipped",
      {
        error: (message) => errors.push(message),
        message: (message) => messages.push(message),
      },
    );
    expect(ok).toBe(false);
    expect(errors).toEqual(["skipped"]);
    expect(messages).toEqual([]);
  });
});
