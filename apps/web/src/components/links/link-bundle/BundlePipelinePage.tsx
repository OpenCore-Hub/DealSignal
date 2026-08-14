import { useEffect, useCallback } from "react";
import { apiErrorMessage } from "@/lib/apiErrors";
import { useParams } from "react-router";
import { motion } from "motion/react";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { useWorkspaceContacts } from "@/hooks/useWorkspaceContacts";
import {
  BundlePipelineProvider,
  createInitialState,
  useBundlePipeline,
} from "./BundlePipelineContext";
import { StepDocuments } from "./StepDocuments";
import { StepSecurity } from "./StepSecurity";
import { StepReview } from "./StepReview";
import {
  classifyPresetFromConfig,
} from "../smart-link/levelConfig";
import {
  SHARE_CONTENT_DOCUMENT_CATEGORY,
  buildEditModeDocumentLists,
  resolveExpiryDaysFromExpiresAt,
  resolveMaxViewsFromAccessCount,
} from "./pipelineUtils";
import type { PermissionConfig } from "@/types";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { CaretLeftIcon, CaretRightIcon } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";

// ---------------------------------------------------------------------------
// Inner component (inside provider)
// ---------------------------------------------------------------------------

function BundlePipelineInner() {
  const { state, dispatch } = useBundlePipeline();
  const reducedMotion = useReducedMotion();
  const { id, workspaceSlug } = useParams<{ id: string; workspaceSlug: string }>();
  const isEdit = !!id;
  const canProceedNav = state.selectedDocuments.length >= 1;

  const { contacts } = useWorkspaceContacts(workspaceSlug);

  // beforeunload protection for edit mode dirty state
  useEffect(() => {
    if (!isEdit) return;
    const handler = (e: BeforeUnloadEvent) => {
      if (state.isDirty) {
        e.preventDefault();
      }
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [isEdit, state.isDirty]);

  // Load link data for edit mode
  useEffect(() => {
    if (!isEdit) return;

    let cancelled = false;
    (async () => {
      try {
        const link = await api.getLinkById(id!);
        if (cancelled) return;

        // Same scope as create mode / Document Library: no agreements or data-room docs
        // in the available picker. Selected tray may still show orphans via fallbacks.
        const docRes = await api.getDocuments("all", SHARE_CONTENT_DOCUMENT_CATEGORY);
        const { pickerDocuments, selectedDocuments: selectedDocs } = buildEditModeDocumentLists(
          docRes.data,
          link.documents,
        );

        const { maxViews, _editMaxViews } = resolveMaxViewsFromAccessCount(
          link.maxAccessCount,
        );

        // Derive security flags from both explicit flag and legacy permission_type.
        const hasEmailVerification = link.requireEmailVerification === true
          || link.permissionType === "email"
          || link.permissionType === "nda"
          || link.permissionType === "whitelist";

        // Reconstruct all contact IDs (multi-contact support).
        const contactIds = link.contactIds ?? [];

        // Snap expiresAt onto 7/15/30 or Custom for the Security select.
        const { expiryDays, _editExpiresAt: resolvedExpiresAt } =
          resolveExpiryDaysFromExpiresAt(link.expiresAt);

        const securityConfig: Omit<PermissionConfig, "level" | "isCustomized"> = {
          requireEmailVerification: hasEmailVerification,
          // Whitelist and password have been removed from the UI. Editing an
          // existing link automatically disables them so users are not surprised
          // by invisible gates.
          whitelistEnabled: false,
          whitelist: [],
          passwordEnabled: false,
          password: undefined,
          // Use explicit boolean flags when available (v2.6+), fall back to permissionType.
          ndaEnabled: link.requireNda === true || link.permissionType === "nda",
          ndaDocumentId: link.ndaDocumentId ?? "",
          ndaTemplateId: link.ndaTemplateId ?? "",
          allowDownload: link.downloadEnabled ?? true,
          watermarkEnabled: link.watermarkEnabled ?? true,
          fileRequestsEnabled: link.fileRequestsEnabled ?? false,
          indexFileEnabled: link.indexFileEnabled ?? false,
          expiryDays,
          maxViews,
          contactIds,
        };
        const { level, isCustomized: customized } = classifyPresetFromConfig(securityConfig);
        // Prefer the exact stored timestamp so save does not drift ±1 day.
        const config: PermissionConfig = {
          ...securityConfig,
          level,
          isCustomized: customized,
          _editExpiresAt: link.expiresAt ?? resolvedExpiresAt,
          _editMaxViews,
        };

        // Parse publicToken from shortUrl. The token is the last path segment.
        // e.g. "https://example.com/l/abc123" → "abc123"
        const token = link.shortUrl.split("/").filter(Boolean).pop() || "";

        if (!cancelled) {
          dispatch({
            type: "INIT_FOR_EDIT",
            payload: {
              linkId: link.id,
              token,
              documents: pickerDocuments,
              selectedDocuments: selectedDocs,
              config,
            },
          });
        }
      } catch (e) {
        if (!cancelled) {
          toast.error(apiErrorMessage(e, { messageKey: "links:creator.loadLinkFailed" }));
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isEdit, id, dispatch]);

  const step = state.step;

  const handleNavBack = useCallback(() => {
    // In edit mode we intentionally allow free navigation between steps:
    // the user is iterating on documents/security settings and should not be
    // interrupted by a confirmation dialog on every step change. Unsaved edits
    // are still protected by the beforeunload handler when leaving the page.
    if (step > 1) {
      dispatch({ type: "GO_STEP", step: (step - 1 as 1 | 2 | 3) });
    }
  }, [step, dispatch]);

  const handleNavForward = useCallback(() => {
    if (step < 3 && canProceedNav) {
      dispatch({ type: "GO_STEP", step: (step + 1 as 1 | 2 | 3) });
    }
  }, [step, canProceedNav, dispatch]);

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <div className="relative">
      {/* Floating step navigation — positioned relative to container */}
      <Button
        type="button"
        variant="ghost"
        size="icon"
        data-testid="pipeline-nav-back"
        onClick={handleNavBack}
        className={`absolute left-[-4.25rem] top-1/2 z-50 h-12 w-12 -translate-y-1/2 rounded-full text-muted-foreground hover:bg-muted hover:text-foreground border animate-pulse-ring bg-muted text-foreground shadow-lg shadow-muted-foreground/15 ${
          step <= 1 ? "cursor-not-allowed" : ""
        }`}
        disabled={step <= 1}
        aria-label={step > 1 ? "Previous step" : "Back"}
      >
        <CaretLeftIcon size={28} weight="bold" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        data-testid="pipeline-nav-forward"
        onClick={handleNavForward}
        disabled={!canProceedNav}
        className={`absolute right-[-4.25rem] top-1/2 z-50 h-12 w-12 -translate-y-1/2 rounded-full text-muted-foreground hover:bg-muted hover:text-foreground border ${
          canProceedNav
            ? "animate-pulse-ring bg-muted text-foreground shadow-lg shadow-muted-foreground/15"
            : ""
        }`}
        aria-label={step < 3 ? "Next step" : ""}
      >
        <CaretRightIcon size={28} weight="bold" />
      </Button>

      <motion.div
        initial={reducedMotion ? false : { opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
      >
        {step === 1 && <StepDocuments />}
        {step === 2 && <StepSecurity contacts={contacts} />}
        {step === 3 && <StepReview />}
      </motion.div>
    </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Public component
// ---------------------------------------------------------------------------

export function BundlePipelinePage() {
  const { id } = useParams<{ id: string }>();
  const isEdit = !!id;

  const initial = createInitialState(
    isEdit
      ? { mode: "edit", editingLinkId: id }
      : { mode: "create" },
  );

  return (
    <BundlePipelineProvider initialState={initial}>
      <BundlePipelineInner />
    </BundlePipelineProvider>
  );
}
