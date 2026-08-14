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
import { PipelinePaper } from "./PipelinePaper";
import { cn } from "@/lib/utils";
import type { Contact, PermissionConfig } from "@/types";

interface StepSecurityProps {
  contacts?: Contact[];
}

const enterEase = [0.32, 0.72, 0, 1] as const;

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
    <div className="mx-auto w-full max-w-3xl space-y-6">
      <div className="flex justify-center">
        <PipelineProgress />
      </div>

      <motion.div
        initial={reducedMotion ? false : { opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.55, ease: enterEase }}
      >
        <PipelinePaper>
          <div
            className={cn(
              "grid grid-cols-1 gap-6 border-b border-foreground/[0.06]",
              "px-6 py-5 sm:grid-cols-2 sm:px-7",
            )}
          >
            <ScoreBar
              label={t("creator.securityScore")}
              score={securityScore}
              variant="security"
              layout="card"
            />
            <ScoreBar
              label={t("creator.frictionScore")}
              score={frictionScore}
              variant="friction"
              layout="card"
            />
          </div>

          <div className="px-6 py-5 sm:px-7 sm:py-6">
            <BundleSecurityOptions
              variant="atelier"
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
          </div>
        </PipelinePaper>
      </motion.div>
    </div>
  );
}
