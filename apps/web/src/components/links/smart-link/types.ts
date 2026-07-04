import type { ComponentType } from "react";
import type { PermissionFields, PermissionPreset } from "@/types";

export type PermissionLevel = PermissionPreset;

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
