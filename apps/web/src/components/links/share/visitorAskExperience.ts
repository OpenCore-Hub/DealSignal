/** Unified visitor Ask UX: one choice maps to ask_ai_enabled + ask_mode. */
export type VisitorAskExperience =
  | "host_only"
  | "ai_supervised"
  | "ai_self_serve"
  | "formal";

export type AskRoutingMode = "supervised" | "self_serve" | "formal";

export const VISITOR_ASK_EXPERIENCE_OPTIONS: ReadonlyArray<{
  value: VisitorAskExperience;
  recommended?: boolean;
}> = [
  { value: "host_only" },
  { value: "ai_supervised", recommended: true },
  { value: "ai_self_serve" },
  { value: "formal" },
];

export const DEFAULT_VISITOR_ASK_EXPERIENCE: VisitorAskExperience = "ai_supervised";

export function resolveAskPolicyFromExperience(experience: VisitorAskExperience): {
  askAiEnabled: boolean;
  askMode: AskRoutingMode;
} {
  switch (experience) {
    case "host_only":
      return { askAiEnabled: false, askMode: "supervised" };
    case "ai_supervised":
      return { askAiEnabled: true, askMode: "supervised" };
    case "ai_self_serve":
      return { askAiEnabled: true, askMode: "self_serve" };
    case "formal":
      return { askAiEnabled: false, askMode: "formal" };
  }
}

export function resolveExperienceFromAskPolicy(
  askAiEnabled: boolean | undefined,
  askMode: string | undefined,
): VisitorAskExperience {
  const mode = askMode === "self_serve" || askMode === "formal" ? askMode : "supervised";
  if (mode === "formal") return "formal";
  if (mode === "self_serve" && askAiEnabled) return "ai_self_serve";
  if (askAiEnabled) return "ai_supervised";
  return "host_only";
}

export function experienceUsesAiLane(experience: VisitorAskExperience): boolean {
  return experience === "ai_supervised" || experience === "ai_self_serve";
}

/** @deprecated Use visitorAskExperience — maps legacy drafts. */
export function experienceFromLegacyAiToggle(enabled: boolean): VisitorAskExperience {
  return enabled ? "ai_supervised" : "host_only";
}
