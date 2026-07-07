import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useBundlePipeline } from "./BundlePipelineContext";
import { PipelineProgress } from "./PipelineProgress";
import { ContactSelector } from "../smart-link/ContactSelector";
import { BundleSecurityOptions } from "./BundleSecurityOptions";
import { ScoreBar } from "../smart-link/ScoreBar";
import {
  calculateFrictionScore,
  calculateSecurityScore,
} from "../smart-link/levelConfig";
import type { Contact, PermissionConfig } from "@/types";

interface StepSecurityProps {
  contacts?: Contact[];
}

// ---------------------------------------------------------------------------
// StepSecurity — main component (fully custom security options, no presets)
// ---------------------------------------------------------------------------

export function StepSecurity({ contacts = [] }: StepSecurityProps) {
  const { state, dispatch } = useBundlePipeline();
  const { t } = useTranslation("links");
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();

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
    <div className="space-y-5">
      {/* Pipeline progress indicator */}
      <div className="flex justify-center">
        <PipelineProgress />
      </div>

      {/* ── Horizontal Score Banner ── */}
      <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
        <div className="space-y-3">
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
        </div>
      </div>

      {/* ── Security Options (always visible) ── */}
      <Card className="shadow-card">
        <CardHeader className="pb-3">
          <CardTitle className="text-lg">
            {t("creator.securityOptions")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <BundleSecurityOptions
            config={state.config}
            onChange={handleConfigChange}
            contactSelector={
              state.config.requireEmailVerification && workspaceSlug ? (
                <div className="px-3 pb-3 pl-[4.5rem]">
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
        </CardContent>
      </Card>
    </div>
  );
}
