export type WorkspaceMode =
  | 'founder'
  | 'investment_firm'
  | 'sales'
  | 'mixed';

export type MembershipRole = 'owner' | 'admin' | 'member' | 'viewer';

export type ContactSegment =
  | 'investor'
  | 'lp'
  | 'buyer'
  | 'customer'
  | 'partner'
  | 'other';

export type DocumentStatus =
  | 'draft'
  | 'processing'
  | 'ready'
  | 'archived'
  | 'failed';

export type DocumentProcessingStatus =
  | 'uploaded'
  | 'processing'
  | 'ready'
  | 'failed';

export type LibraryItemStatus =
  | 'draft'
  | 'in_review'
  | 'approved'
  | 'archived';

export type LinkAccessMode =
  | 'public'
  | 'email_verification'
  | 'allowlist'
  | 'password'
  | 'approval_required'
  | 'nda_required';

export type DownloadPolicy = 'allowed' | 'disabled' | 'watermarked';

export type FrictionLevel = 'low' | 'medium' | 'high';

export type SmartLinkStatus = 'active' | 'expired' | 'revoked' | 'archived';

export type RecipientStatus =
  | 'invited'
  | 'verified'
  | 'approved'
  | 'blocked'
  | 'revoked';

export type AccessScopeType = 'smart_link' | 'deal_room';

export type AccessGrantStatus = 'pending' | 'approved' | 'denied' | 'revoked';

export type RoomType =
  | 'seed_fundraising'
  | 'series_a_fundraising'
  | 'lp_update'
  | 'ma_diligence'
  | 'enterprise_sales'
  | 'partner_enablement'
  | 'custom';

export type RoomStatus = 'draft' | 'active' | 'archived';

export type RoomMemberRole = 'viewer' | 'questioner' | 'collaborator';

export type PrincipalType = 'contact' | 'account' | 'domain' | 'role';

export type QuestionStatus = 'open' | 'answered' | 'closed';

export type DownloadStatus = 'allowed' | 'blocked' | 'failed';

export type ScoreType =
  | 'investor_intent'
  | 'lp_engagement'
  | 'buyer_engagement'
  | 'deal_intent'
  | 'room_engagement';

export type ScoreLabel = 'cold' | 'warm' | 'hot';

export type RecommendationStatus = 'open' | 'dismissed' | 'completed';

export type NotificationChannel = 'email' | 'slack' | 'crm' | 'in_app';

export type NotificationStatus = 'queued' | 'sent' | 'failed' | 'cancelled';

export type IntegrationProvider =
  | 'slack'
  | 'hubspot'
  | 'salesforce'
  | 'gmail'
  | 'outlook'
  | 'google_drive'
  | 'dropbox';

export type IntegrationStatus = 'connected' | 'disconnected' | 'error';

export type CrmObjectType =
  | 'contact'
  | 'account'
  | 'smart_link'
  | 'deal_room'
  | 'document';

export interface User {
  id: string;
  email: string;
  name: string;
  avatarUrl: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface Workspace {
  id: string;
  name: string;
  slug: string;
  mode: WorkspaceMode;
  defaultSecurityPreset: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

export interface ApiError {
  code: string;
  message: string;
  statusCode: number;
}
