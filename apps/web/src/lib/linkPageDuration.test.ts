import { describe, expect, it } from "vitest";
import { buildPageDurationData, buildPageDurationDataFromMetrics } from "./linkPageDuration";
import type { AccessLog } from "@/types";

function log(partial: Partial<AccessLog> & Pick<AccessLog, "id" | "pageNumber">): AccessLog {
  return {
    linkId: "link_1",
    visitorEmail: "a@example.com",
    durationSeconds: 10,
    timestamp: "2026-08-16T00:00:00Z",
    ...partial,
  };
}

describe("buildPageDurationData", () => {
  it("fills 1..pageCount for a single document without labels", () => {
    const data = buildPageDurationData(
      [
        log({ id: "1", pageNumber: 1, durationSeconds: 20 }),
        log({ id: "2", pageNumber: 1, durationSeconds: 10 }),
        log({ id: "3", pageNumber: 3, durationSeconds: 40 }),
      ],
      {
        documents: [{ id: "doc_xlsx", title: "Model.xlsx", pageCount: 3 }],
        primaryDocumentId: "doc_xlsx",
      },
    );
    expect(data).toEqual([
      { page: 1, duration: 15 },
      { page: 2, duration: 0 },
      { page: 3, duration: 40 },
    ]);
  });

  it("does not merge the same page number across bundle documents", () => {
    const data = buildPageDurationData(
      [
        log({ id: "1", documentId: "doc_xlsx", pageNumber: 3, durationSeconds: 12 }),
        log({ id: "2", documentId: "doc_pdf", pageNumber: 8, durationSeconds: 30 }),
        log({ id: "3", documentId: "doc_pdf", pageNumber: 16, durationSeconds: 45 }),
        log({ id: "4", documentId: "doc_pdf", pageNumber: 3, durationSeconds: 9 }),
      ],
      {
        documents: [
          { id: "doc_xlsx", title: "Model.xlsx", pageCount: 3 },
          { id: "doc_pdf", title: "Memo.pdf", pageCount: 16 },
        ],
        primaryDocumentId: "doc_xlsx",
        formatBundleLabel: (title, page) => `${title} · p.${page}`,
      },
    );

    expect(data).toHaveLength(19);
    expect(data.slice(0, 3)).toEqual([
      { page: 1, duration: 0, label: "Model.xlsx · p.1" },
      { page: 2, duration: 0, label: "Model.xlsx · p.2" },
      { page: 3, duration: 12, label: "Model.xlsx · p.3" },
    ]);
    expect(data[2 + 8]).toEqual({ page: 8, duration: 30, label: "Memo.pdf · p.8" });
    expect(data[2 + 16]).toEqual({ page: 16, duration: 45, label: "Memo.pdf · p.16" });
    expect(data[2 + 3]).toEqual({ page: 3, duration: 9, label: "Memo.pdf · p.3" });
  });

  it("attributes legacy logs without documentId to the primary document only", () => {
    const data = buildPageDurationData(
      [
        log({ id: "1", pageNumber: 8, durationSeconds: 99 }),
        log({ id: "2", documentId: "doc_pdf", pageNumber: 8, durationSeconds: 20 }),
      ],
      {
        documents: [
          { id: "doc_xlsx", title: "Model.xlsx", pageCount: 3 },
          { id: "doc_pdf", title: "Memo.pdf", pageCount: 16 },
        ],
        primaryDocumentId: "doc_xlsx",
        formatBundleLabel: (title, page) => `${title} · p.${page}`,
      },
    );

    const xlsxPage3 = data.find((d) => d.label === "Model.xlsx · p.3");
    const xlsxHasPage8 = data.some((d) => d.label === "Model.xlsx · p.8");
    const pdfPage8 = data.find((d) => d.label === "Memo.pdf · p.8");
    expect(xlsxHasPage8).toBe(false);
    expect(xlsxPage3?.duration).toBe(0);
    expect(pdfPage8?.duration).toBe(20);
  });

  it("builds the same series from member-excluded page metrics", () => {
    const data = buildPageDurationDataFromMetrics(
      [
        { documentId: "doc_xlsx", pageNumber: 1, avgDurationSeconds: 15 },
        { documentId: "doc_xlsx", pageNumber: 3, avgDurationSeconds: 40 },
      ],
      {
        documents: [{ id: "doc_xlsx", title: "Model.xlsx", pageCount: 3 }],
        primaryDocumentId: "doc_xlsx",
      },
    );
    expect(data).toEqual([
      { page: 1, duration: 15 },
      { page: 2, duration: 0 },
      { page: 3, duration: 40 },
    ]);
  });
});
