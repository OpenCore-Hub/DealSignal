import type { ComponentType } from "react";
import type { PermissionFields } from "@/types";

export interface PresetDef {
  label: string;
  description: string;
  icon: ComponentType<{ size?: number; className?: string }>;
  color: string;
  friction: string;
  usage: string;
  gates: string[];
}

export type PresetConfigTemplate = PermissionFields;
