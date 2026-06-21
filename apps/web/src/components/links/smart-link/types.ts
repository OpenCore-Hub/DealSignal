import type { ComponentType } from "react";
import type { Document, PermissionConfig } from "@/types";

export type PermissionLevel = "low" | "medium" | "high";

export interface SmartLinkCreatorState {
  documents: Document[];
  loadingDocs: boolean;
  selectedDocumentId: string;
  level: PermissionLevel;
  config: PermissionConfig;
  generatedLink: string | null;
  creating: boolean;
  copied: boolean;
}

export interface LevelInfo {
  label: string;
  description: string;
  icon: ComponentType<{ size?: number; className?: string }>;
  color: string;
  friction: string;
}
