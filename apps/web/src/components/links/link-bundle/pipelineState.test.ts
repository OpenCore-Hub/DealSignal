import { describe, it, expect, beforeEach } from "vitest";
import { createInitialState, pipelineReducer, type BundlePipelineState } from "./BundlePipelineContext";
import { buildConfigFromPreset } from "./pipelineUtils";
import type { Document } from "@/types";

const mockDoc: Document = {
  id: "doc-1",
  title: "Test Document",
  sourceType: "pdf",
  fileName: "test.pdf",
  fileType: "pdf",
  fileSize: 1024,
  pageCount: 10,
  status: "ready",
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
};

const mockDoc2: Document = {
  ...mockDoc,
  id: "doc-2",
  title: "Test Document 2",
};

describe("createInitialState", () => {
  it("returns create mode with default values", () => {
    const state = createInitialState();
    expect(state.mode).toBe("create");
    expect(state.step).toBe(1);
    expect(state.editingLinkId).toBeNull();
    expect(state.linkToken).toBeNull();
    expect(state.documents).toEqual([]);
    expect(state.selectedDocuments).toEqual([]);
    expect(state.searchQuery).toBe("");
    expect(state.config.level).toBe("customized");
    expect(state.isSubmitting).toBe(false);
    expect(state.generatedLink).toBeNull();
    expect(state.copied).toBe(false);
    expect(state.isDirty).toBe(false);
  });

  it("accepts overrides for edit mode", () => {
    const state = createInitialState({ mode: "edit", editingLinkId: "link-123" });
    expect(state.mode).toBe("edit");
    expect(state.editingLinkId).toBe("link-123");
    expect(state.step).toBe(1); // unchanged
  });

  it("overrides config", () => {
    const customConfig = buildConfigFromPreset("confidential");
    const state = createInitialState({ config: customConfig });
    expect(state.config.level).toBe("confidential");
  });
});

describe("BundlePipelineState transitions", () => {
  let state: BundlePipelineState;

  beforeEach(() => {
    state = createInitialState();
  });

  describe("navigation actions", () => {
    it("GO_STEP changes step when documents are selected", () => {
      // Cannot advance past step 1 without selected documents.
      state = pipelineReducer(state, { type: "GO_STEP", step: 2 });
      expect(state.step).toBe(1); // blocked — no documents selected

      // Select a document, then step changes are allowed.
      state = pipelineReducer(state, { type: "TOGGLE_DOCUMENT", document: mockDoc });
      state = pipelineReducer(state, { type: "GO_STEP", step: 2 });
      expect(state.step).toBe(2);

      state = pipelineReducer(state, { type: "GO_STEP", step: 3 });
      expect(state.step).toBe(3);

      state = pipelineReducer(state, { type: "GO_STEP", step: 1 });
      expect(state.step).toBe(1);
    });

    it("GO_STEP rejects forward navigation without selected documents", () => {
      state = pipelineReducer(state, { type: "GO_STEP", step: 2 });
      expect(state.step).toBe(1);
      state = pipelineReducer(state, { type: "GO_STEP", step: 3 });
      expect(state.step).toBe(1);
    });
  });

  describe("document actions", () => {
    it("SET_DOCUMENTS replaces document list", () => {
      state = pipelineReducer(state, { type: "SET_DOCUMENTS", documents: [mockDoc] });
      expect(state.documents).toEqual([mockDoc]);
    });

    it("TOGGLE_DOCUMENT adds a document", () => {
      state = pipelineReducer(state, { type: "TOGGLE_DOCUMENT", document: mockDoc });
      expect(state.selectedDocuments).toEqual([mockDoc]);
    });

    it("TOGGLE_DOCUMENT removes a document", () => {
      state = pipelineReducer(state, { type: "TOGGLE_DOCUMENT", document: mockDoc });
      state = pipelineReducer(state, { type: "TOGGLE_DOCUMENT", document: mockDoc });
      expect(state.selectedDocuments).toEqual([]);
    });

    it("TOGGLE_DOCUMENT marks dirty in edit mode", () => {
      state.mode = "edit";
      state = pipelineReducer(state, { type: "TOGGLE_DOCUMENT", document: mockDoc });
      expect(state.isDirty).toBe(true);
    });

    it("TOGGLE_DOCUMENT does not mark dirty in create mode", () => {
      state.mode = "create";
      state = pipelineReducer(state, { type: "TOGGLE_DOCUMENT", document: mockDoc });
      expect(state.isDirty).toBe(false);
    });

    it("REMOVE_DOCUMENT removes by id", () => {
      state.selectedDocuments = [mockDoc, mockDoc2];
      state = pipelineReducer(state, { type: "REMOVE_DOCUMENT", documentId: "doc-1" });
      expect(state.selectedDocuments).toEqual([mockDoc2]);
    });

    it("MOVE_DOCUMENT_UP swaps with previous", () => {
      state.selectedDocuments = [mockDoc, mockDoc2];
      state = pipelineReducer(state, { type: "MOVE_DOCUMENT_UP", documentId: "doc-2" });
      expect(state.selectedDocuments).toEqual([mockDoc2, mockDoc]);
    });

    it("MOVE_DOCUMENT_UP no-ops for first element", () => {
      state.selectedDocuments = [mockDoc, mockDoc2];
      state = pipelineReducer(state, { type: "MOVE_DOCUMENT_UP", documentId: "doc-1" });
      expect(state.selectedDocuments).toEqual([mockDoc, mockDoc2]);
    });

    it("MOVE_DOCUMENT_DOWN swaps with next", () => {
      state.selectedDocuments = [mockDoc, mockDoc2];
      state = pipelineReducer(state, { type: "MOVE_DOCUMENT_DOWN", documentId: "doc-1" });
      expect(state.selectedDocuments).toEqual([mockDoc2, mockDoc]);
    });

    it("MOVE_DOCUMENT_DOWN no-ops for last element", () => {
      state.selectedDocuments = [mockDoc, mockDoc2];
      state = pipelineReducer(state, { type: "MOVE_DOCUMENT_DOWN", documentId: "doc-2" });
      expect(state.selectedDocuments).toEqual([mockDoc, mockDoc2]);
    });

    it("SEARCH_QUERY updates search query", () => {
      state = pipelineReducer(state, { type: "SET_SEARCH_QUERY", query: "investor" });
      expect(state.searchQuery).toBe("investor");
    });
  });

  describe("config actions", () => {
    it("SET_CONFIG updates config and marks dirty in edit mode", () => {
      state.mode = "edit";
      const newConfig = buildConfigFromPreset("confidential");
      state = pipelineReducer(state, { type: "SET_CONFIG", config: newConfig });
      expect(state.config.level).toBe("confidential");
      expect(state.isDirty).toBe(true);
    });

    it("SET_CONFIG does not mark dirty in create mode", () => {
      state.mode = "create";
      const newConfig = buildConfigFromPreset("confidential");
      state = pipelineReducer(state, { type: "SET_CONFIG", config: newConfig });
      expect(state.isDirty).toBe(false);
    });
  });

  describe("submission state", () => {
    it("SET_SUBMITTING toggles submitting state", () => {
      state = pipelineReducer(state, { type: "SET_SUBMITTING", isSubmitting: true });
      expect(state.isSubmitting).toBe(true);
      state = pipelineReducer(state, { type: "SET_SUBMITTING", isSubmitting: false });
      expect(state.isSubmitting).toBe(false);
    });

    it("SET_GENERATED_LINK stores link", () => {
      state = pipelineReducer(state, { type: "SET_GENERATED_LINK", link: "https://example.com/l/token" });
      expect(state.generatedLink).toBe("https://example.com/l/token");
    });

    it("SET_COPIED toggles copied state", () => {
      state = pipelineReducer(state, { type: "SET_COPIED", copied: true });
      expect(state.copied).toBe(true);
    });

    it("SET_DIRTY sets dirty state", () => {
      state = pipelineReducer(state, { type: "SET_DIRTY", isDirty: true });
      expect(state.isDirty).toBe(true);
      state = pipelineReducer(state, { type: "SET_DIRTY", isDirty: false });
      expect(state.isDirty).toBe(false);
    });
  });

  describe("RESET action", () => {
    it("resets step, dirty, link, copied, and submitting", () => {
      state.step = 3;
      state.isDirty = true;
      state.generatedLink = "https://example.com/l/abc";
      state.copied = true;
      state.isSubmitting = true;
      state = pipelineReducer(state, { type: "RESET" });
      expect(state.step).toBe(1);
      expect(state.isDirty).toBe(false);
      expect(state.generatedLink).toBeNull();
      expect(state.copied).toBe(false);
      expect(state.isSubmitting).toBe(false);
    });

    it("resets selectedDocuments and config for fresh start", () => {
      state.selectedDocuments = [mockDoc];
      state.mode = "create";
      state = pipelineReducer(state, { type: "RESET" });
      expect(state.selectedDocuments).toEqual([]);
      // config is reset to fresh default config (not empty)
      expect(state.config.level).toBe("customized");
    });
  });

  describe("INIT_FOR_EDIT action", () => {
    it("initializes edit mode with full payload", () => {
      const config = buildConfigFromPreset("standard");
      state = pipelineReducer(state, {
        type: "INIT_FOR_EDIT",
        payload: {
          linkId: "link-456",
          token: "abc123",
          documents: [mockDoc, mockDoc2],
          selectedDocuments: [mockDoc],
          config,
        },
      });
      expect(state.mode).toBe("edit");
      expect(state.editingLinkId).toBe("link-456");
      expect(state.linkToken).toBe("abc123");
      expect(state.documents).toEqual([mockDoc, mockDoc2]);
      expect(state.selectedDocuments).toEqual([mockDoc]);
      expect(state.config).toEqual(config);
      expect(state.isDirty).toBe(false);
      expect(state.step).toBe(1);
    });
  });
});
