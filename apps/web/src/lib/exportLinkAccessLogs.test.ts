/** @vitest-environment jsdom */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  exportLinkAccessLogsCsv,
  type AccessLogCsvHeaders,
} from "./exportLinkAccessLogs";

const getAccessLogs = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    getAccessLogs: (...args: unknown[]) => getAccessLogs(...args),
  },
}));

const headers: AccessLogCsvHeaders = [
  "时间",
  "访客邮箱",
  "访客姓名",
  "文档ID",
  "页码",
  "停留秒数",
  "设备",
  "地点",
];

describe("exportLinkAccessLogsCsv", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    getAccessLogs.mockReset();
  });

  it("pages through access logs and returns row count with localized headers", async () => {
    const createObjectURL = vi.fn(() => "blob:mock");
    const revokeObjectURL = vi.fn();
    vi.stubGlobal("URL", {
      ...URL,
      createObjectURL,
      revokeObjectURL,
    });
    const click = vi.fn();
    const remove = vi.fn();
    let blobText = "";
    vi.stubGlobal(
      "Blob",
      class MockBlob {
        constructor(parts: BlobPart[]) {
          blobText = parts.map(String).join("");
        }
      },
    );
    vi.spyOn(document.body, "appendChild").mockImplementation((node) => node);
    vi.spyOn(document, "createElement").mockImplementation(() => {
      return {
        href: "",
        download: "",
        rel: "",
        click,
        remove,
      } as unknown as HTMLAnchorElement;
    });

    getAccessLogs
      .mockResolvedValueOnce({
        data: [
          {
            id: "1",
            linkId: "link-1",
            visitorEmail: "a@example.com",
            documentId: "doc-pdf",
            durationSeconds: 12,
            timestamp: "2026-08-05T00:00:00Z",
          },
        ],
        has_more: true,
      })
      .mockResolvedValueOnce({
        data: [
          {
            id: "2",
            linkId: "link-1",
            visitorEmail: "b@example.com",
            durationSeconds: 3,
            timestamp: "2026-08-05T01:00:00Z",
          },
        ],
        has_more: false,
      });

    const count = await exportLinkAccessLogsCsv("link-1", "Roadmap", headers);
    expect(count).toBe(2);
    expect(getAccessLogs).toHaveBeenCalledTimes(2);
    expect(getAccessLogs).toHaveBeenNthCalledWith(1, "link-1", { limit: 100, offset: 0 });
    expect(getAccessLogs).toHaveBeenNthCalledWith(2, "link-1", { limit: 100, offset: 1 });
    expect(click).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:mock");
    expect(blobText.split("\n")[0]).toBe(headers.join(","));
    expect(blobText.split("\n")[1]).toContain("doc-pdf");
  });
});
