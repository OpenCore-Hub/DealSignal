// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { exportInsightsDailyVisitsCsv } from "./exportInsightsDailyVisits";

describe("exportInsightsDailyVisitsCsv", () => {
  const originalCreateObjectURL = URL.createObjectURL;
  const originalRevokeObjectURL = URL.revokeObjectURL;

  beforeEach(() => {
    URL.createObjectURL = vi.fn(() => "blob:insights");
    URL.revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    URL.createObjectURL = originalCreateObjectURL;
    URL.revokeObjectURL = originalRevokeObjectURL;
    vi.restoreAllMocks();
  });

  it("downloads a CSV for the selected range", () => {
    const click = vi.fn();
    const append = vi.spyOn(document.body, "appendChild").mockImplementation((node) => node);
    const remove = vi.spyOn(HTMLElement.prototype, "remove").mockImplementation(() => undefined);
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(click);

    exportInsightsDailyVisitsCsv(
      [
        { date: "2026-08-01T00:00:00.000Z", opens: 2, uniqueVisitors: 1 },
        { date: "2026-08-02T00:00:00.000Z", opens: 5, uniqueVisitors: 3 },
      ],
      7,
      ["date", "opens", "unique_visitors"],
    );

    expect(URL.createObjectURL).toHaveBeenCalled();
    const blob = (URL.createObjectURL as ReturnType<typeof vi.fn>).mock.calls[0][0] as Blob;
    expect(blob.type).toContain("text/csv");
    expect(click).toHaveBeenCalled();
    append.mockRestore();
    remove.mockRestore();
  });
});
