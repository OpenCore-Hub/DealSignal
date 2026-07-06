// @vitest-environment jsdom
/**
 * Unit tests for draft persistence (saveDraft / loadDraft / clearPipelineDraft).
 *
 * These functions were previously module-private and untested.
 */
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import {
  createInitialState,
  saveDraft,
  loadDraft,
  clearPipelineDraft,
  type BundlePipelineState,
} from "./BundlePipelineContext";
import { buildConfigFromPreset } from "./pipelineUtils";
import type { Document } from "@/types";

// ---------------------------------------------------------------------------
// localStorage mock — mirrors the pattern in detectors.test.ts
// ---------------------------------------------------------------------------

function createStorage() {
  const store: Record<string, string> = {};
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = value;
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      for (const key of Object.keys(store)) {
        delete store[key];
      }
    },
  };
}

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

function makeState(overrides?: Partial<BundlePipelineState>): BundlePipelineState {
  return createInitialState({
    documents: [mockDoc],
    selectedDocuments: [mockDoc],
    searchQuery: "investor",
    config: buildConfigFromPreset("confidential"),
    ...overrides,
  });
}

describe("draft persistence", () => {
  beforeEach(() => {
    vi.stubGlobal("localStorage", createStorage());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  describe("saveDraft", () => {
    it("persists step, selectedDocumentIds, searchQuery, and config", () => {
      const state = makeState({ step: 2 });
      saveDraft(state);

      const raw = localStorage.getItem("bundle-pipeline-draft");
      expect(raw).not.toBeNull();

      const parsed = JSON.parse(raw!);
      expect(parsed.step).toBe(2);
      expect(parsed.selectedDocumentIds).toEqual(["doc-1"]);
      expect(parsed.searchQuery).toBe("investor");
      expect(parsed.config.level).toBe("confidential");
    });

    it("persists empty selectedDocumentIds when none selected", () => {
      const state = makeState({ selectedDocuments: [] });
      saveDraft(state);

      const raw = localStorage.getItem("bundle-pipeline-draft");
      const parsed = JSON.parse(raw!);
      expect(parsed.selectedDocumentIds).toEqual([]);
    });
  });

  describe("loadDraft", () => {
    it("returns null when no draft exists", () => {
      expect(loadDraft()).toBeNull();
    });

    it("returns parsed draft when one exists", () => {
      const state = makeState({ step: 3 });
      saveDraft(state);

      const draft = loadDraft();
      expect(draft).not.toBeNull();
      expect(draft!.step).toBe(3);
      expect(draft!.selectedDocumentIds).toEqual(["doc-1"]);
      expect(draft!.searchQuery).toBe("investor");
      expect(draft!.config.level).toBe("confidential");
    });

    it("returns null on malformed JSON", () => {
      localStorage.setItem("bundle-pipeline-draft", "{invalid json");
      expect(loadDraft()).toBeNull();
    });
  });

  describe("clearPipelineDraft", () => {
    it("removes draft from localStorage", () => {
      const state = makeState();
      saveDraft(state);
      expect(localStorage.getItem("bundle-pipeline-draft")).not.toBeNull();

      clearPipelineDraft();
      expect(localStorage.getItem("bundle-pipeline-draft")).toBeNull();
    });

    it("no-ops when no draft exists", () => {
      expect(() => clearPipelineDraft()).not.toThrow();
      expect(localStorage.getItem("bundle-pipeline-draft")).toBeNull();
    });
  });

  describe("createInitialState — draft restoration", () => {
    it("restores searchQuery and config from saved draft", () => {
      const state = makeState({
        searchQuery: "term sheet",
        config: buildConfigFromPreset("confidential"),
      });
      saveDraft(state);

      const restored = createInitialState();
      expect(restored.searchQuery).toBe("term sheet");
      expect(restored.config.level).toBe("confidential");
    });

    it("sets pendingDraftDocIds from draft selectedDocumentIds", () => {
      const state = makeState({ selectedDocuments: [mockDoc] });
      saveDraft(state);

      const restored = createInitialState();
      expect(restored.pendingDraftDocIds).toEqual(["doc-1"]);
    });

    it("does not restore draft in edit mode", () => {
      // Save a draft first
      const state = makeState({
        searchQuery: "ignored",
        config: buildConfigFromPreset("public"),
      });
      saveDraft(state);

      // createInitialState in edit mode should ignore the draft
      const restored = createInitialState({ mode: "edit" });
      expect(restored.searchQuery).toBe(""); // not from draft
      expect(restored.config.level).toBe("customized"); // default, not public from draft
    });

    it("handles localStorage.setItem errors gracefully", () => {
      // Simulate quota error
      vi.stubGlobal(
        "localStorage",
        Object.create(createStorage(), {
          setItem: { value: vi.fn(() => { throw new Error("QuotaExceededError"); }) },
        }),
      );

      const state = makeState();
      // Should not throw
      expect(() => saveDraft(state)).not.toThrow();
    });
  });
});
