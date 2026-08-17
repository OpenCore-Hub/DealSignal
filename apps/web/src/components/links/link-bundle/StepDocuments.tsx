import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { apiErrorMessage } from "@/lib/apiErrors";
import { useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { clearPipelineDraft, useBundlePipeline } from "./BundlePipelineContext";
import { BundleDocumentPicker } from "./BundleDocumentPicker";
import { PipelineProgress } from "./PipelineProgress";
import { PipelinePaper } from "./PipelinePaper";
import { resolveDraftDocumentRestore, SHARE_CONTENT_DOCUMENT_CATEGORY } from "./pipelineUtils";
import { sortDocumentsByNewestUpload } from "@/lib/sortDocumentsByUploadTime";
import { api } from "@/lib/api";
import type { Document } from "@/types";
import { toast } from "sonner";

function readExplicitDocumentIds(searchParams: URLSearchParams): string[] {
  return [
    ...new Set(
      searchParams
        .getAll("documentId")
        .flatMap((value) => value.split(","))
        .map((id) => id.trim())
        .filter(Boolean),
    ),
  ];
}

export function StepDocuments() {
  const { state, dispatch } = useBundlePipeline();
  const { t } = useTranslation("links");
  const [searchParams] = useSearchParams();
  const explicitDocumentIds = useMemo(
    () => readExplicitDocumentIds(searchParams),
    [searchParams],
  );
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
      const documents = sortDocumentsByNewestUpload(res.data);
      dispatch({ type: "SET_DOCUMENTS", documents });

      // Restore selected documents from pending draft IDs (set in createInitialState).
      // Explicit URL documentIds (row Share / post-upload) always win. If every
      // draft id is gone, start fresh — do not toast "documents expired".
      if (state.pendingDraftDocIds.length > 0) {
        const decision = resolveDraftDocumentRestore({
          draftIds: state.pendingDraftDocIds,
          availableIds: documents.map((doc) => doc.id),
          explicitDocumentIds,
        });
        if (decision.restoreIds.length > 0) {
          const restored = documents.filter((doc: Document) =>
            decision.restoreIds.includes(doc.id),
          );
          dispatch({ type: "SET_SELECTED_DOCUMENTS", documents: restored });
        }
        if (decision.warnMissing) {
          toast.warning(
            t("creator.draftDocsUnavailable", {
              missing: decision.missing,
              total: decision.total,
            }),
          );
        }
        if (decision.clearDraft) {
          clearPipelineDraft();
        }
        dispatch({ type: "CLEAR_PENDING_DRAFT_DOC_IDS" });
      }

      const selectedIds = new Set(state.selectedDocuments.map((doc) => doc.id));
      const toSelect = documents.filter(
        (doc) => explicitDocumentIds.includes(doc.id) && !selectedIds.has(doc.id),
      );
      if (toSelect.length === 1) {
        dispatch({ type: "TOGGLE_DOCUMENT", document: toSelect[0] });
      } else if (toSelect.length > 1) {
        dispatch({
          type: "SET_SELECTED_DOCUMENTS",
          documents: [
            ...state.selectedDocuments,
            ...toSelect,
          ],
        });
      }
    } catch (e) {
      toast.error(apiErrorMessage(e, { messageKey: "links:creator.loadDocsFailed" }));
    } finally {
      setLoading(false);
    }
  }, [dispatch, t, state.mode, state.pendingDraftDocIds, state.selectedDocuments, explicitDocumentIds]);

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
