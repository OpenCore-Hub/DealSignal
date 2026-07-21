export type LinkPreset = "public" | "standard" | "confidential" | "custom";

export type FolderScopeMode = "full" | "allowlist";

export interface AccessConfig {
  requireEmail: boolean;
  requireEmailVerification: boolean;
  requirePassword: boolean;
  password: string;
  watermarkEnabled: boolean;
  requireNda: boolean;
  ndaDocumentId: string;
  ndaTemplateId: string;
  allowDownloading: boolean;
  enableScreenshotProtection: boolean;
  aiCopilotEnabled: boolean;
  enableFileRequests: boolean;
  enableIndexFileGeneration: boolean;
  enableQaConversations: boolean;
}

export interface DraftLink extends AccessConfig {
  name: string;
  expiresAt: string;
  allowedViewers: string[];
  blockedViewers: string[];
  customDomain: string;
  notifyOnAccess: boolean;
  folderPaths: string[];
  folderScopeMode: FolderScopeMode;
  contactIds: string[];
}
