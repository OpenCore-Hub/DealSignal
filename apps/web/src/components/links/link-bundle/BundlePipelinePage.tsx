import { useEffect, useCallback, useState } from "react";
import { useParams } from "react-router";
import { motion } from "motion/react";
import { useReducedMotion } from "@/hooks/useReducedMotion";
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
import type { Contact, Document, PermissionConfig } from "@/types";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { CaretLeftIcon, CaretRightIcon } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "react-i18next";

// ---------------------------------------------------------------------------
// Inner component (inside provider)
// ---------------------------------------------------------------------------

function BundlePipelineInner() {
  const { state, dispatch } = useBundlePipeline();
  const reducedMotion = useReducedMotion();
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation("links");
  const isEdit = !!id;
  const canProceedNav = state.selectedDocuments.length >= 1;

  const [contacts, setContacts] = useState<Contact[]>([]);

  // Load workspace contacts once. They are needed by the security step to
  // auto-fill selected contact emails into the whitelist, and by navigation
  // validation to ensure whitelist/contact consistency before proceeding.
  useEffect(() => {
    let cancelled = false;
    api
      .getContacts()
      .then((res) => {
        if (!cancelled) setContacts(res.data);
      })
      .catch(() => {
        if (!cancelled) setContacts([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

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

        // Fetch all documents for the picker
        const docRes = await api.getDocuments();

        // Build selected documents from link.documents (cross-reference with full Document list).
        // DocumentSummary from the backend now includes fileSize; if a link document is not
        // in the full document list (e.g., deleted), construct a fallback with available fields.
        const selectedDocs: Document[] = link.documents
          .map((ds) => {
            const full = docRes.data.find((d) => d.id === ds.id);
            if (full) return full;
            // Fallback: construct a minimal Document from DocumentSummary fields.
            // Use new Date(0).toISOString() as a sentinel instead of empty strings
            // to avoid Invalid Date errors if any UI component calls toLocaleDateString().
            return {
              id: ds.id,
              title: ds.title,
              sourceType: ds.sourceType,
              fileName: ds.title,
              fileType: ds.sourceType,
              fileSize: ds.fileSize ?? 0,
              pageCount: ds.pageCount,
              status: ds.status,
              createdAt: new Date(0).toISOString(),
              updatedAt: new Date(0).toISOString(),
            } as Document;
          });

        // Merge selected docs into the full list if any are missing from the picker.
        const allDocs = [...docRes.data];
        const existingIds = new Set(allDocs.map((d) => d.id));
        for (const sd of selectedDocs) {
          if (!existingIds.has(sd.id)) {
            allDocs.push(sd);
          }
        }

        // Map maxAccessCount to maxViews.
        const maxViews: number | "unlimited" =
          typeof link.maxAccessCount === "number" && link.maxAccessCount > 0
            ? link.maxAccessCount
            : "unlimited";

        // Derive security flags from both explicit flag and legacy permission_type.
        const hasEmailVerification = link.requireEmailVerification === true
          || link.permissionType === "email"
          || link.permissionType === "nda"
          || link.permissionType === "whitelist";

        // Reconstruct whitelist from backend allowedEmails.
        const whitelist: string[] = [...(link.allowedEmails ?? [])];

        // Reconstruct all contact IDs (multi-contact support).
        const contactIds = link.contactIds ?? [];

        // Compute expiryDays from expiresAt for display/editing in the UI.
        let expiryDays: number | "custom" = 30;
        if (link.expiresAt) {
          const expires = new Date(link.expiresAt);
          const now = new Date();
          const diffMs = expires.getTime() - now.getTime();
          const diffDays = Math.ceil(diffMs / (1000 * 60 * 60 * 24));
          if (diffDays > 0) {
            expiryDays = diffDays;
          } else {
            // Link already expired — show minimum value; _editExpiresAt preserves
            // the original timestamp so the backend still receives the correct date.
            expiryDays = 1;
          }
        }

        const securityConfig: Omit<PermissionConfig, "level" | "isCustomized"> = {
          requireEmailVerification: hasEmailVerification,
          whitelistEnabled: link.permissionType === "whitelist" || whitelist.length > 0,
          whitelist,
          // Use explicit boolean flags when available (v2.6+), fall back to permissionType.
          passwordEnabled: link.requirePassword === true || link.permissionType === "password",
          password: undefined,
          ndaEnabled: link.requireNda === true || link.permissionType === "nda",
          allowDownload: link.downloadEnabled ?? true,
          watermarkEnabled: link.watermarkEnabled ?? true,
          aiCopilotEnabled: link.aiCopilotEnabled ?? false,
          expiryDays,
          maxViews,
          contactIds,
        };
        const { level, isCustomized: customized } = classifyPresetFromConfig(securityConfig);
        // Preserve the original expiresAt to avoid round-trip drift when saving.
        const config: PermissionConfig = {
          ...securityConfig,
          level,
          isCustomized: customized,
          _editExpiresAt: link.expiresAt,
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
              documents: allDocs,
              selectedDocuments: selectedDocs,
              config,
            },
          });
        }
      } catch (e) {
        if (!cancelled) {
          toast.error(e instanceof Error ? e.message : "Failed to load link");
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
    if (step === 2) {
      // Validate whitelist/contact consistency before entering the review step.
      const cfg = state.config;
      if (cfg.requireEmailVerification && cfg.whitelistEnabled && cfg.contactIds.length > 0) {
        const whitelist = new Set(cfg.whitelist.map((e) => e.trim().toLowerCase()));
        const missing = contacts
          .filter((c) => cfg.contactIds.includes(c.id))
          .map((c) => c.email.trim().toLowerCase())
          .filter((email) => !whitelist.has(email));
        if (missing.length > 0) {
          toast.error(
            t("creator.whitelistContactMismatch", {
              emails: missing.join(", "),
            }),
          );
          return;
        }
      }
    }
    if (step < 3 && canProceedNav) {
      dispatch({ type: "GO_STEP", step: (step + 1 as 1 | 2 | 3) });
    }
  }, [step, canProceedNav, dispatch, state.config, contacts]);

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
        {step === 3 && <StepReview contacts={contacts} />}
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
