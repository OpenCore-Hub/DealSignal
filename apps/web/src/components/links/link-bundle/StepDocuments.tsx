import { useCallback, useEffect, useRef, useState } from "react";
import { apiErrorMessage } from "@/lib/apiErrors";
import { useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { clearPipelineDraft, useBundlePipeline } from "./BundlePipelineContext";
import { BundleDocumentPicker } from "./BundleDocumentPicker";
import { PipelineProgress } from "./PipelineProgress";
import { PipelinePaper } from "./PipelinePaper";
import { SHARE_CONTENT_DOCUMENT_CATEGORY } from "./pipelineUtils";
import { api } from "@/lib/api";
import type { Document } from "@/types";
import { toast } from "sonner";

export function StepDocuments() {
  const { state, dispatch } = useBundlePipeline();
  const { t } = useTranslation("links");
  const [searchParams] = useSearchParams();
  const initialDocumentId = searchParams.get("documentId") ?? undefined;
  const [loading, setLoading] = useState(false);
  const loadedRef = useRef(false);

  const loadDocuments = useCallback(async () => {
    // In edit mode, documents are loaded by BundlePipelinePage. Use a ref guard
    // to prevent re-fetching on re-renders. Previously this used state.documents.length
    // as a dependency, which could create a race with INIT_FOR_EDIT dispatching.
    if (state.mode === "edit") return;
    if (loadedRef.current) return;
    loadedRef.current = true;
    setLoading(true);
    try {
      // Share content picker: same scope as Document Library (not agreements / data-room docs).
      // NDA templates are chosen separately in the security step via category=agreement.
      const res = await api.getDocuments("all", SHARE_CONTENT_DOCUMENT_CATEGORY);
      dispatch({ type: "SET_DOCUMENTS", documents: res.data });

      // Restore selected documents from pending draft IDs (set in createInitialState).
      // When the user enters from a single-document action (e.g. clicking "Create link"
      // on a document row), discard any stale draft so it doesn't interfere with the
      // explicit document choice or show confusing "draft unavailable" warnings.
      if (state.pendingDraftDocIds.length > 0) {
        if (initialDocumentId) {
          clearPipelineDraft();
        } else {
          const draftIds = state.pendingDraftDocIds;
          const restored = res.data.filter((d: Document) => draftIds.includes(d.id));
          if (restored.length > 0) {
            dispatch({ type: "SET_SELECTED_DOCUMENTS", documents: restored });
          }
          const missing = draftIds.length - restored.length;
          if (missing > 0) {
            console.warn("Draft restore: some documents are no longer available", {
              total: draftIds.length,
              restored: restored.length,
              missing,
            });
            toast.warning(
              t("creator.draftDocsUnavailable", {
                missing,
                total: draftIds.length,
              }),
            );
            // When none of the draft documents are available, clear the stale draft
            // so the user doesn't keep seeing this warning on subsequent visits.
            if (restored.length === 0) {
              clearPipelineDraft();
            }
          }
        }
        dispatch({ type: "CLEAR_PENDING_DRAFT_DOC_IDS" });
      }

      // Single-document entry point: auto-select the document from the URL query
      // param (e.g. /links/new?documentId=xxx). This unifies the single-doc and
      // multi-doc creation flows under the same bundle pipeline.
      if (
        initialDocumentId &&
        !state.selectedDocuments.some((d) => d.id === initialDocumentId)
      ) {
        const target = res.data.find((d) => d.id === initialDocumentId);
        if (target) {
          dispatch({ type: "TOGGLE_DOCUMENT", document: target });
        }
      }
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "links:creator.loadDocsFailed" }));
    } finally {
      setLoading(false);
    }
  }, [dispatch, t, state.mode, state.pendingDraftDocIds, state.selectedDocuments, initialDocumentId]);

  useEffect(() => {
    loadDocuments();
  }, [loadDocuments]);

  return (
    <div className="mx-auto w-full max-w-3xl space-y-6">
      <div className="flex justify-center">
        <PipelineProgress />
      </div>

      <PipelinePaper>
        <BundleDocumentPicker
          allDocuments={state.documents}
          loading={loading}
          selectedDocuments={state.selectedDocuments}
          selectedIds={new Set(state.selectedDocuments.map((d) => d.id))}
          searchQuery={state.searchQuery}
          onSearchChange={(query) => dispatch({ type: "SET_SEARCH_QUERY", query })}
          onToggle={(doc) => dispatch({ type: "TOGGLE_DOCUMENT", document: doc })}
          onRemove={(id) => dispatch({ type: "REMOVE_DOCUMENT", documentId: id })}
          onMoveUp={(id) => dispatch({ type: "MOVE_DOCUMENT_UP", documentId: id })}
          onMoveDown={(id) => dispatch({ type: "MOVE_DOCUMENT_DOWN", documentId: id })}
        />
      </PipelinePaper>
    </div>
  );
}
