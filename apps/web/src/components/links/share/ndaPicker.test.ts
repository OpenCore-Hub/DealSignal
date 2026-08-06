import { describe, it, expect, vi, beforeEach } from "vitest";
import { api } from "@/lib/api";
import {
  mapAgreementDocuments,
  mapNdaTemplates,
  resolveNdaDocumentFallback,
  loadNdaPickerSources,
} from "./ndaPicker";
import type { Document } from "@/types";

vi.mock("@/lib/api", () => ({
  api: {
    listNDATemplates: vi.fn(),
    getDocuments: vi.fn(),
  },
}));

describe("ndaPicker", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("maps templates and ready agreement documents", () => {
    expect(
      mapNdaTemplates([
        { id: "t1", name: "NDA", source_document_id: "d1" },
      ]),
    ).toEqual([{ id: "t1", name: "NDA", sourceDocumentId: "d1" }]);

    const docs = [
      { id: "a1", title: "Ready NDA", status: "ready" },
      { id: "a2", title: "Processing", status: "processing" },
      { id: "a3", title: "Archived", status: "archived" },
    ] as Document[];
    expect(mapAgreementDocuments(docs)).toEqual([{ id: "a1", title: "Ready NDA" }]);
  });

  it("keeps agreement library only — never falls back to room/share docs", () => {
    expect(resolveNdaDocumentFallback([{ id: "ag", title: "Agreement" }])).toEqual([
      { id: "ag", title: "Agreement" },
    ]);
    expect(resolveNdaDocumentFallback([])).toEqual([]);
  });

  it("loads templates and agreements independently on partial failure", async () => {
    vi.mocked(api.listNDATemplates).mockRejectedValue(new Error("templates down"));
    vi.mocked(api.getDocuments).mockResolvedValue({
      data: [{ id: "doc_nda_1", title: "Standard Mutual NDA", status: "ready" } as Document],
    });

    const sources = await loadNdaPickerSources();
    expect(sources.templates).toEqual([]);
    expect(sources.agreementDocs).toEqual([
      { id: "doc_nda_1", title: "Standard Mutual NDA" },
    ]);
  });

  it("unwraps bare document arrays from request()", async () => {
    vi.mocked(api.listNDATemplates).mockResolvedValue({ data: [] } as never);
    // Some callers/mocks return the unwrapped list directly.
    vi.mocked(api.getDocuments).mockResolvedValue([
      { id: "doc_nda_1", title: "Standard Mutual NDA", status: "ready" } as Document,
    ] as never);

    const sources = await loadNdaPickerSources();
    expect(sources.agreementDocs).toEqual([
      { id: "doc_nda_1", title: "Standard Mutual NDA" },
    ]);
  });

  it("returns both sources when APIs succeed", async () => {
    vi.mocked(api.listNDATemplates).mockResolvedValue({
      data: [
        {
          id: "nda_tpl_1",
          name: "Standard Mutual NDA",
          source_document_id: "doc_nda_1",
          content_sha256: "x",
          require_signer_name: true,
          status: "active",
          response_count: 0,
          link_count: 0,
          created_at: "",
          updated_at: "",
        },
      ],
    });
    vi.mocked(api.getDocuments).mockResolvedValue({
      data: [{ id: "doc_nda_1", title: "Standard Mutual NDA", status: "ready" } as Document],
    });

    const sources = await loadNdaPickerSources();
    expect(sources.templates).toHaveLength(1);
    expect(sources.agreementDocs).toHaveLength(1);
  });
});
