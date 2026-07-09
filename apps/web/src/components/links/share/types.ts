export type LinkPreset = "public" | "standard" | "confidential" | "custom";

export interface AccessConfig {
  requireEmail: boolean;
  requireEmailVerification: boolean;
  requirePassword: boolean;
  password: string;
  watermarkEnabled: boolean;
  requireNda: boolean;
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
  autoAddInvited: boolean;
  customDomain: string;
  tags: string[];
  notifyOnAccess: boolean;
}
