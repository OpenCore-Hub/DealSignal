// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from "vitest";
import {
  DOCUMENTS_UPLOADED_EVENT,
  dispatchDocumentsUploaded,
  isLibraryShareableUpload,
} from "./documentsUploadedEvent";

describe("documentsUploadedEvent", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("treats general (and empty) category as shareable", () => {
    expect(
      isLibraryShareableUpload({
        documentId: "d1",
        documentTitle: "A",
        status: "ready",
        category: "general",
      }),
    ).toBe(true);
    expect(
      isLibraryShareableUpload({
        documentId: "d1",
        documentTitle: "A",
        status: "processing",
      }),
    ).toBe(true);
  });

  it("excludes agreement and deal_room uploads", () => {
    expect(
      isLibraryShareableUpload({
        documentId: "d1",
        documentTitle: "NDA",
        status: "ready",
        category: "agreement",
      }),
    ).toBe(false);
    expect(
      isLibraryShareableUpload({
        documentId: "d1",
        documentTitle: "Deck",
        status: "ready",
        category: "deal_room",
      }),
    ).toBe(false);
  });

  it("dispatches a CustomEvent with detail", () => {
    const handler = vi.fn();
    window.addEventListener(DOCUMENTS_UPLOADED_EVENT, handler);
    dispatchDocumentsUploaded({
      documentId: "d1",
      documentTitle: "Pitch",
      status: "ready",
      category: "general",
    });
    expect(handler).toHaveBeenCalledTimes(1);
    const event = handler.mock.calls[0][0] as CustomEvent;
    expect(event.detail).toEqual({
      documentId: "d1",
      documentTitle: "Pitch",
      status: "ready",
      category: "general",
    });
    window.removeEventListener(DOCUMENTS_UPLOADED_EVENT, handler);
  });
});
