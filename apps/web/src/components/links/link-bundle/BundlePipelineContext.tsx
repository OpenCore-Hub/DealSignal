import { createContext, useContext, useEffect, useReducer, useRef, type Dispatch, type ReactNode } from "react";
import type { Document, PermissionConfig } from "@/types";

// ---------------------------------------------------------------------------
// Default security config for new bundles
// ---------------------------------------------------------------------------

const DEFAULT_SECURITY_CONFIG: PermissionConfig = {
  level: "customized",
  isCustomized: true,
  requireEmailVerification: false,
  whitelistEnabled: false,
  whitelist: [],
  passwordEnabled: false,
  ndaEnabled: false,
  allowDownload: true,
  watermarkEnabled: true,
  aiCopilotEnabled: false,
  qaEnabled: false,
  fileRequestsEnabled: false,
  indexFileEnabled: false,
  expiryDays: 30,
  maxViews: "unlimited",
  contactIds: [],
};

// ---------------------------------------------------------------------------
// localStorage draft persistence
// ---------------------------------------------------------------------------

const DRAFT_KEY = "bundle-pipeline-draft";
const EDIT_DRAFT_PREFIX = "bundle-pipeline-edit-draft-";

interface PipelineDraft {
  step: BundlePipelineState["step"];
  selectedDocumentIds: string[];
  searchQuery: string;
  config: PermissionConfig;
}

// eslint-disable-next-line react-refresh/only-export-components
export function saveDraft(state: BundlePipelineState): void {
  try {
    const draft: PipelineDraft = {
      step: state.step,
      selectedDocumentIds: state.selectedDocuments.map((d) => d.id),
      searchQuery: state.searchQuery,
      config: state.config,
    };
    localStorage.setItem(DRAFT_KEY, JSON.stringify(draft));
  } catch { /* ignore quota errors */ }
}

// eslint-disable-next-line react-refresh/only-export-components
export function loadDraft(): PipelineDraft | null {
  try {
    const raw = localStorage.getItem(DRAFT_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as PipelineDraft;
  } catch {
    return null;
  }
}

function saveEditDraft(linkId: string, state: BundlePipelineState): void {
  try {
    const draft: PipelineDraft = {
      step: state.step,
      selectedDocumentIds: state.selectedDocuments.map((d) => d.id),
      searchQuery: state.searchQuery,
      config: state.config,
    };
    localStorage.setItem(EDIT_DRAFT_PREFIX + linkId, JSON.stringify(draft));
  } catch { /* ignore */ }
}

// eslint-disable-next-line react-refresh/only-export-components
export function clearPipelineDraft(): void {
  try {
    localStorage.removeItem(DRAFT_KEY);
  } catch { /* ignore */ }
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

export interface BundlePipelineState {
  step: 1 | 2 | 3;
  mode: "create" | "edit";
  editingLinkId: string | null;
  linkToken: string | null;

  // Step 1 — documents
  documents: Document[];
  selectedDocuments: Document[];
  /** Draft document IDs restored from localStorage (cleared after restore) */
  pendingDraftDocIds: string[];
  searchQuery: string;

  // Step 2 — security
  config: PermissionConfig;

  // Step 3 — submission
  isSubmitting: boolean;
  generatedLink: string | null;
  copied: boolean;
  isDirty: boolean;
}

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

export type BundlePipelineAction =
  | { type: "GO_STEP"; step: 1 | 2 | 3 }
  | { type: "INIT_FOR_EDIT"; payload: { linkId: string; token: string; documents: Document[]; selectedDocuments: Document[]; config: PermissionConfig } }
  | { type: "SET_DOCUMENTS"; documents: Document[] }
  | { type: "TOGGLE_DOCUMENT"; document: Document }
  | { type: "REMOVE_DOCUMENT"; documentId: string }
  | { type: "MOVE_DOCUMENT_UP"; documentId: string }
  | { type: "MOVE_DOCUMENT_DOWN"; documentId: string }
  | { type: "SET_SEARCH_QUERY"; query: string }
  | { type: "SET_CONFIG"; config: PermissionConfig }
  | { type: "SET_SUBMITTING"; isSubmitting: boolean }
  | { type: "SET_GENERATED_LINK"; link: string | null }
  | { type: "SET_COPIED"; copied: boolean }
  | { type: "SET_DIRTY"; isDirty: boolean }
  | { type: "SET_SELECTED_DOCUMENTS"; documents: Document[] }
  | { type: "CLEAR_PENDING_DRAFT_DOC_IDS" }
  | { type: "RESET" };

// ---------------------------------------------------------------------------
// Reducer
// ---------------------------------------------------------------------------

/** Helper: mark dirty in edit mode without duplicating the check in every case. */
function markDirty(state: BundlePipelineState): Pick<BundlePipelineState, "isDirty"> {
  return { isDirty: state.mode === "edit" ? true : state.isDirty };
}

// eslint-disable-next-line react-refresh/only-export-components
export function pipelineReducer(state: BundlePipelineState, action: BundlePipelineAction): BundlePipelineState {
  switch (action.type) {
    case "GO_STEP":
      // Prevent navigating to step 2+ without selected documents.
      if (action.step > 1 && state.selectedDocuments.length === 0) {
        return state;
      }
      return { ...state, step: action.step };

    case "INIT_FOR_EDIT": {
      const { linkId, token, documents, selectedDocuments, config } = action.payload;
      return {
        ...state,
        mode: "edit",
        editingLinkId: linkId,
        linkToken: token,
        documents,
        selectedDocuments,
        config,
        isDirty: false,
        step: 1,
      };
    }

    case "SET_DOCUMENTS":
      return { ...state, documents: action.documents };

    case "TOGGLE_DOCUMENT": {
      const doc = action.document;
      const idx = state.selectedDocuments.findIndex((d) => d.id === doc.id);
      if (idx >= 0) {
        return {
          ...state,
          selectedDocuments: state.selectedDocuments.filter((d) => d.id !== doc.id),
          ...markDirty(state),
        };
      }
      return {
        ...state,
        selectedDocuments: [...state.selectedDocuments, doc],
        ...markDirty(state),
      };
    }

    case "REMOVE_DOCUMENT":
      return {
        ...state,
        selectedDocuments: state.selectedDocuments.filter((d) => d.id !== action.documentId),
        ...markDirty(state),
      };

    case "MOVE_DOCUMENT_UP": {
      const idx = state.selectedDocuments.findIndex((d) => d.id === action.documentId);
      if (idx <= 0) return state;
      const next = [...state.selectedDocuments];
      [next[idx - 1], next[idx]] = [next[idx], next[idx - 1]];
      return { ...state, selectedDocuments: next, ...markDirty(state) };
    }

    case "MOVE_DOCUMENT_DOWN": {
      const idx = state.selectedDocuments.findIndex((d) => d.id === action.documentId);
      if (idx < 0 || idx >= state.selectedDocuments.length - 1) return state;
      const next = [...state.selectedDocuments];
      [next[idx], next[idx + 1]] = [next[idx + 1], next[idx]];
      return { ...state, selectedDocuments: next, ...markDirty(state) };
    }

    case "SET_SEARCH_QUERY":
      return { ...state, searchQuery: action.query };

    case "SET_CONFIG": {
      // When expiryDays changes in edit mode, clear _editExpiresAt so the
      // new value takes effect instead of being silently discarded.
      const expiryChanged =
        typeof action.config.expiryDays !== typeof state.config.expiryDays ||
        action.config.expiryDays !== state.config.expiryDays;
      const nextConfig = expiryChanged
        ? { ...action.config, _editExpiresAt: undefined }
        : action.config;
      return { ...state, config: nextConfig, ...markDirty(state) };
    }

    case "SET_SUBMITTING":
      return { ...state, isSubmitting: action.isSubmitting };

    case "SET_GENERATED_LINK":
      return { ...state, generatedLink: action.link };

    case "SET_COPIED":
      return { ...state, copied: action.copied };

    case "SET_DIRTY":
      return { ...state, isDirty: action.isDirty };

    case "SET_SELECTED_DOCUMENTS":
      return { ...state, selectedDocuments: action.documents };

    case "CLEAR_PENDING_DRAFT_DOC_IDS":
      return { ...state, pendingDraftDocIds: [] };

    case "RESET":
      return {
        ...state,
        step: 1,
        isDirty: false,
        generatedLink: null,
        copied: false,
        isSubmitting: false,
        editingLinkId: null,
        linkToken: null,
        selectedDocuments: [],
        config: { ...DEFAULT_SECURITY_CONFIG },
      };

    default:
      return state;
  }
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

interface BundlePipelineCtx {
  state: BundlePipelineState;
  dispatch: Dispatch<BundlePipelineAction>;
  selectedIds: Set<string>;
}

const Ctx = createContext<BundlePipelineCtx | null>(null);

// eslint-disable-next-line react-refresh/only-export-components
export function useBundlePipeline(): BundlePipelineCtx {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useBundlePipeline must be used within BundlePipelineProvider");
  return ctx;
}

export function BundlePipelineProvider({ children, initialState }: { children: ReactNode; initialState: BundlePipelineState }) {
  const [state, dispatch] = useReducer(pipelineReducer, initialState);
  const mounted = useRef(false);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Persist draft with debounce to avoid excessive localStorage writes on rapid
  // keystrokes (e.g., typing whitelist entries). The debounce uses a 500ms
  // trailing edge so only the final state after rapid changes is persisted.
  // Watch only the state slices that affect the serialized draft content to
  // avoid unnecessary timer churn from unrelated state changes (e.g., isDirty,
  // searchQuery mutation via sequential keystrokes).
  //
  // Create mode uses a shared key (DRAFT_KEY), edit mode uses a per-link key
  // (EDIT_DRAFT_PREFIX + editingLinkId).
  useEffect(() => {
    if (!mounted.current) {
      mounted.current = true;
      return;
    }
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => {
      if (state.mode === "create") {
        saveDraft(state);
      } else if (state.mode === "edit" && state.editingLinkId) {
        saveEditDraft(state.editingLinkId, state);
      }
    }, 500);
    return () => {
      if (saveTimer.current) clearTimeout(saveTimer.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- see comment above
  }, [state.mode, state.step, state.selectedDocuments, state.searchQuery, state.config, state.editingLinkId]);

  // Clear any stale draft when entering edit mode to prevent cross-contamination
  // between create-mode drafts and edit-mode data.
  useEffect(() => {
    if (state.mode === "edit") {
      clearPipelineDraft();
    }
  }, [state.mode]);

  const selectedIds = new Set(state.selectedDocuments.map((d) => d.id));

  return <Ctx.Provider value={{ state, dispatch, selectedIds }}>{children}</Ctx.Provider>;
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

// eslint-disable-next-line react-refresh/only-export-components
export function createInitialState(overrides?: Partial<BundlePipelineState>): BundlePipelineState {
  // Restore draft for create mode
  const draft = overrides?.mode !== "edit" ? loadDraft() : null;

  // When edit-mode overrides are provided, apply them last so they take
  // precedence over draft defaults. The spread order matters:
  //   base → draft → overrides
  // This prevents overrides from accidentally assigning empty arrays when
  // only { mode, editingLinkId } is passed.
  return {
    step: 1,
    mode: "create",
    editingLinkId: null,
    linkToken: null,
    documents: [],
    selectedDocuments: [],
    pendingDraftDocIds: draft?.selectedDocumentIds ?? [],
    searchQuery: draft?.searchQuery ?? "",
    config: draft?.config ?? { ...DEFAULT_SECURITY_CONFIG },
    isSubmitting: false,
    generatedLink: null,
    copied: false,
    isDirty: false,
    ...overrides,
  };
}
