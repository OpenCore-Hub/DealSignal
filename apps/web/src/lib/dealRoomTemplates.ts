import type { DealRoomTemplate } from "@/types";

export const CUSTOM_DEAL_ROOM_TEMPLATE_ID = "tmpl_custom";
export const CUSTOM_DEAL_ROOM_SCENARIO = "custom" as const;

/** Local fallback when GET /deal-room-templates still omits the custom scenario. */
export const CUSTOM_DEAL_ROOM_TEMPLATE: DealRoomTemplate = {
  id: CUSTOM_DEAL_ROOM_TEMPLATE_ID,
  name: "Completely Custom",
  description: "Start with an empty room and add your own folders.",
  scenario: CUSTOM_DEAL_ROOM_SCENARIO,
  folderStructure: [],
  recommendedFiles: [],
  defaultPermissionLevel: "standard",
  ndaEnabled: false,
};

export function isCustomDealRoomTemplate(
  template: Pick<DealRoomTemplate, "id" | "scenario">,
): boolean {
  return template.scenario === CUSTOM_DEAL_ROOM_SCENARIO || template.id === CUSTOM_DEAL_ROOM_TEMPLATE_ID;
}

export function templateFolderCount(
  template: Pick<DealRoomTemplate, "folderStructure"> | null | undefined,
): number {
  return Array.isArray(template?.folderStructure) ? template.folderStructure.length : 0;
}

function normalizeDealRoomTemplate(template: DealRoomTemplate): DealRoomTemplate {
  return {
    ...template,
    folderStructure: Array.isArray(template.folderStructure) ? template.folderStructure : [],
    recommendedFiles: Array.isArray(template.recommendedFiles) ? template.recommendedFiles : [],
    ndaEnabled: Boolean(template.ndaEnabled),
  };
}

/** Keep the custom tile visible even if an older API catalog has only the nine presets. */
export function ensureDealRoomTemplates(
  list: DealRoomTemplate[] | null | undefined,
): DealRoomTemplate[] {
  const templates = (list ?? []).map(normalizeDealRoomTemplate);
  if (templates.some(isCustomDealRoomTemplate)) return templates;
  return [...templates, CUSTOM_DEAL_ROOM_TEMPLATE];
}
