import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { motion } from "motion/react";
import { useBundlePipeline } from "./BundlePipelineContext";
import { PipelineProgress } from "./PipelineProgress";
import { ContactSelector } from "../smart-link/ContactSelector";
import { BundleSecurityOptions } from "./BundleSecurityOptions";
import { ScoreBar } from "../smart-link/ScoreBar";
import {
  calculateFrictionScore,
  calculateSecurityScore,
} from "../smart-link/levelConfig";
import { useReducedMotion } from "@/hooks/useReducedMotion";
import { cn } from "@/lib/utils";
import type { Contact, PermissionConfig } from "@/types";

interface StepSecurityProps {
  contacts?: Contact[];
}

export function StepSecurity({ contacts = [] }: StepSecurityProps) {
  const { state, dispatch } = useBundlePipeline();
  const { t } = useTranslation("links");
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const reducedMotion = useReducedMotion();

  const frictionScore = calculateFrictionScore(state.config);
  const securityScore = calculateSecurityScore(state.config);

  const handleConfigChange = useCallback(
    (next: PermissionConfig) => {
      dispatch({
        type: "SET_CONFIG",
        config: next,
      });
    },
    [dispatch],
  );

  return (
    <div className="mx-auto w-full max-w-3xl space-y-5">
      <div className="flex justify-center">
        <PipelineProgress />
      </div>

      <motion.div
        initial={reducedMotion ? false : { opacity: 0, y: 6 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.35, ease: [0.16, 1, 0.3, 1] }}
        className={cn("grid gap-3 sm:grid-cols-2 sm:gap-5")}
      >
        <ScoreBar
          label={t("creator.securityScore")}
          score={securityScore}
          variant="security"
        />
        <ScoreBar
          label={t("creator.frictionScore")}
          score={frictionScore}
          variant="friction"
        />
      </motion.div>

      <motion.div
        initial={reducedMotion ? false : { opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, delay: reducedMotion ? 0 : 0.05, ease: [0.16, 1, 0.3, 1] }}
      >
        <BundleSecurityOptions
          config={state.config}
          onChange={handleConfigChange}
          excludeNdaDocumentIds={state.selectedDocuments.map((d) => d.id)}
          contactSelector={
            (state.config.requireEmailVerification || state.config.ndaEnabled) &&
            workspaceSlug ? (
              <div className="animate-in fade-in-0 slide-in-from-top-1 duration-200">
                <ContactSelector
                  workspaceSlug={workspaceSlug}
                  value={state.config.contactIds}
                  onChange={(contactIds) =>
                    handleConfigChange({
                      ...state.config,
                      contactIds,
                    })
                  }
                  contacts={contacts.length > 0 ? contacts : undefined}
                />
              </div>
            ) : null
          }
        />
      </motion.div>
    </div>
  );
}
