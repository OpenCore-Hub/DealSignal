import { sql } from 'drizzle-orm';
import {
  bigint,
  boolean,
  check,
  customType,
  index,
  inet,
  integer,
  jsonb,
  pgEnum,
  pgTable,
  text,
  timestamp,
  unique,
  uuid,
} from 'drizzle-orm/pg-core';

const citext = customType<{ data: string }>({
  dataType() {
    return 'citext';
  },
});

const bytea = customType<{ data: Buffer }>({
  dataType() {
    return 'bytea';
  },
});

export const workspaceModeEnum = pgEnum('workspace_mode', [
  'founder',
  'investment_firm',
  'sales',
  'mixed',
]);

export const membershipRoleEnum = pgEnum('membership_role', [
  'owner',
  'admin',
  'member',
  'viewer',
]);

export const contactSegmentEnum = pgEnum('contact_segment', [
  'investor',
  'lp',
  'buyer',
  'customer',
  'partner',
  'other',
]);

export const documentStatusEnum = pgEnum('document_status', [
  'draft',
  'processing',
  'ready',
  'archived',
  'failed',
]);

export const documentProcessingStatusEnum = pgEnum('document_processing_status', [
  'uploaded',
  'processing',
  'ready',
  'failed',
]);

export const libraryItemStatusEnum = pgEnum('library_item_status', [
  'draft',
  'in_review',
  'approved',
  'archived',
]);

export const linkAccessModeEnum = pgEnum('link_access_mode', [
  'public',
  'email_verification',
  'allowlist',
  'password',
  'approval_required',
  'nda_required',
]);

export const downloadPolicyEnum = pgEnum('download_policy', [
  'allowed',
  'disabled',
  'watermarked',
]);

export const frictionLevelEnum = pgEnum('friction_level', [
  'low',
  'medium',
  'high',
]);

export const smartLinkStatusEnum = pgEnum('smart_link_status', [
  'active',
  'expired',
  'revoked',
  'archived',
]);

export const recipientStatusEnum = pgEnum('recipient_status', [
  'invited',
  'verified',
  'approved',
  'blocked',
  'revoked',
]);

export const accessScopeTypeEnum = pgEnum('access_scope_type', [
  'smart_link',
  'deal_room',
]);

export const accessGrantStatusEnum = pgEnum('access_grant_status', [
  'pending',
  'approved',
  'denied',
  'revoked',
]);

export const roomTypeEnum = pgEnum('room_type', [
  'seed_fundraising',
  'series_a_fundraising',
  'lp_update',
  'ma_diligence',
  'enterprise_sales',
  'partner_enablement',
  'custom',
]);

export const roomStatusEnum = pgEnum('room_status', [
  'draft',
  'active',
  'archived',
]);

export const roomMemberRoleEnum = pgEnum('room_member_role', [
  'viewer',
  'questioner',
  'collaborator',
]);

export const principalTypeEnum = pgEnum('principal_type', [
  'contact',
  'account',
  'domain',
  'role',
]);

export const questionStatusEnum = pgEnum('question_status', [
  'open',
  'answered',
  'closed',
]);

export const downloadStatusEnum = pgEnum('download_status', [
  'allowed',
  'blocked',
  'failed',
]);

export const scoreTypeEnum = pgEnum('score_type', [
  'investor_intent',
  'lp_engagement',
  'buyer_engagement',
  'deal_intent',
  'room_engagement',
]);

export const scoreLabelEnum = pgEnum('score_label', ['cold', 'warm', 'hot']);

export const recommendationStatusEnum = pgEnum('recommendation_status', [
  'open',
  'dismissed',
  'completed',
]);

export const notificationChannelEnum = pgEnum('notification_channel', [
  'email',
  'slack',
  'crm',
  'in_app',
]);

export const notificationStatusEnum = pgEnum('notification_status', [
  'queued',
  'sent',
  'failed',
  'cancelled',
]);

export const integrationProviderEnum = pgEnum('integration_provider', [
  'slack',
  'hubspot',
  'salesforce',
  'gmail',
  'outlook',
  'google_drive',
  'dropbox',
]);

export const integrationStatusEnum = pgEnum('integration_status', [
  'connected',
  'disconnected',
  'error',
]);

export const crmObjectTypeEnum = pgEnum('crm_object_type', [
  'contact',
  'account',
  'smart_link',
  'deal_room',
  'document',
]);

export const users = pgTable('users', {
  id: uuid('id').primaryKey().defaultRandom(),
  email: citext('email').notNull().unique(),
  name: text('name').notNull(),
  avatarUrl: text('avatar_url'),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
});

export const workspaces = pgTable('workspaces', {
  id: uuid('id').primaryKey().defaultRandom(),
  name: text('name').notNull(),
  slug: citext('slug').notNull().unique(),
  mode: workspaceModeEnum('mode').notNull().default('mixed'),
  defaultSecurityPreset: jsonb('default_security_preset').notNull().default({}),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
});

export const workspaceMemberships = pgTable(
  'workspace_memberships',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    role: membershipRoleEnum('role').notNull().default('member'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [unique('workspace_memberships_workspace_user_idx').on(table.workspaceId, table.userId)]
);

export const accounts = pgTable('accounts', {
  id: uuid('id').primaryKey().defaultRandom(),
  workspaceId: uuid('workspace_id')
    .notNull()
    .references(() => workspaces.id, { onDelete: 'cascade' }),
  name: text('name').notNull(),
  domain: citext('domain'),
  segment: contactSegmentEnum('segment').notNull().default('other'),
  metadata: jsonb('metadata').notNull().default({}),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
});

export const contacts = pgTable(
  'contacts',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    email: citext('email').notNull(),
    name: text('name'),
    title: text('title'),
    segment: contactSegmentEnum('segment').notNull().default('other'),
    metadata: jsonb('metadata').notNull().default({}),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [unique('contacts_workspace_email_idx').on(table.workspaceId, table.email)]
);

export const accountContacts = pgTable(
  'account_contacts',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    accountId: uuid('account_id')
      .notNull()
      .references(() => accounts.id, { onDelete: 'cascade' }),
    contactId: uuid('contact_id')
      .notNull()
      .references(() => contacts.id, { onDelete: 'cascade' }),
    relationshipLabel: text('relationship_label'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [unique('account_contacts_account_contact_idx').on(table.accountId, table.contactId)]
);

export const documents = pgTable('documents', {
  id: uuid('id').primaryKey().defaultRandom(),
  workspaceId: uuid('workspace_id')
    .notNull()
    .references(() => workspaces.id, { onDelete: 'cascade' }),
  ownerUserId: uuid('owner_user_id').references(() => users.id, { onDelete: 'set null' }),
  name: text('name').notNull(),
  description: text('description'),
  status: documentStatusEnum('status').notNull().default('draft'),
  // FK to document_versions.id added in migration SQL to avoid circular type inference.
  currentVersionId: uuid('current_version_id'),
  metadata: jsonb('metadata').notNull().default({}),
  deletedAt: timestamp('deleted_at', { withTimezone: true }),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
});

export const documentVersions = pgTable(
  'document_versions',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    documentId: uuid('document_id')
      .notNull()
      .references(() => documents.id, { onDelete: 'cascade' }),
    versionNumber: integer('version_number').notNull(),
    originalFilename: text('original_filename').notNull(),
    mimeType: text('mime_type').notNull(),
    fileSizeBytes: bigint('file_size_bytes', { mode: 'number' }).notNull(),
    checksumSha256: text('checksum_sha256'),
    storageBucket: text('storage_bucket').notNull(),
    storageKey: text('storage_key').notNull(),
    processingStatus: documentProcessingStatusEnum('processing_status')
      .notNull()
      .default('uploaded'),
    pageCount: integer('page_count'),
    processingError: text('processing_error'),
    createdByUserId: uuid('created_by_user_id').references(() => users.id, {
      onDelete: 'set null',
    }),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    unique('document_versions_document_version_idx').on(table.documentId, table.versionNumber),
    unique('document_versions_storage_idx').on(table.storageBucket, table.storageKey),
    check('document_versions_file_size_check', sql`${table.fileSizeBytes} >= 0`),
    check(
      'document_versions_page_count_check',
      sql`${table.pageCount} IS NULL OR ${table.pageCount} >= 0`
    ),
  ]
);

export const documentPages = pgTable(
  'document_pages',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    documentVersionId: uuid('document_version_id')
      .notNull()
      .references(() => documentVersions.id, { onDelete: 'cascade' }),
    pageNumber: integer('page_number').notNull(),
    thumbnailStorageKey: text('thumbnail_storage_key'),
    textExcerpt: text('text_excerpt'),
    widthPx: integer('width_px'),
    heightPx: integer('height_px'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    unique('document_pages_version_page_idx').on(table.documentVersionId, table.pageNumber),
    check('document_pages_page_number_check', sql`${table.pageNumber} > 0`),
    check('document_pages_width_check', sql`${table.widthPx} IS NULL OR ${table.widthPx} > 0`),
    check('document_pages_height_check', sql`${table.heightPx} IS NULL OR ${table.heightPx} > 0`),
  ]
);

export const documentPageTiles = pgTable(
  'document_page_tiles',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    documentVersionId: uuid('document_version_id')
      .notNull()
      .references(() => documentVersions.id, { onDelete: 'cascade' }),
    pageNumber: integer('page_number').notNull(),
    zoomLevel: integer('zoom_level').notNull().default(1),
    tileSizePx: integer('tile_size_px').notNull().default(512),
    cols: integer('cols').notNull(),
    rows: integer('rows').notNull(),
    tileManifest: jsonb('tile_manifest').notNull(),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    unique('document_page_tiles_version_page_zoom_idx').on(
      table.documentVersionId,
      table.pageNumber,
      table.zoomLevel
    ),
    check('document_page_tiles_page_number_check', sql`${table.pageNumber} > 0`),
  ]
);

export const libraryCollections = pgTable('library_collections', {
  id: uuid('id').primaryKey().defaultRandom(),
  workspaceId: uuid('workspace_id')
    .notNull()
    .references(() => workspaces.id, { onDelete: 'cascade' }),
  name: text('name').notNull(),
  description: text('description'),
  createdByUserId: uuid('created_by_user_id').references(() => users.id, {
    onDelete: 'set null',
  }),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
});

export const libraryItems = pgTable(
  'library_items',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    collectionId: uuid('collection_id').references(() => libraryCollections.id, {
      onDelete: 'set null',
    }),
    documentId: uuid('document_id')
      .notNull()
      .references(() => documents.id, { onDelete: 'cascade' }),
    status: libraryItemStatusEnum('status').notNull().default('draft'),
    approvedByUserId: uuid('approved_by_user_id').references(() => users.id, {
      onDelete: 'set null',
    }),
    approvedAt: timestamp('approved_at', { withTimezone: true }),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [unique('library_items_workspace_document_idx').on(table.workspaceId, table.documentId)]
);

export const smartLinks = pgTable(
  'smart_links',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    documentId: uuid('document_id')
      .notNull()
      .references(() => documents.id, { onDelete: 'cascade' }),
    documentVersionId: uuid('document_version_id').references(() => documentVersions.id, {
      onDelete: 'set null',
    }),
    createdByUserId: uuid('created_by_user_id').references(() => users.id, {
      onDelete: 'set null',
    }),
    name: text('name').notNull(),
    slug: text('slug').notNull().unique(),
    accessMode: linkAccessModeEnum('access_mode').notNull().default('email_verification'),
    downloadPolicy: downloadPolicyEnum('download_policy').notNull().default('allowed'),
    watermarkEnabled: boolean('watermark_enabled').notNull().default(false),
    watermarkTemplate: text('watermark_template'),
    passwordHash: text('password_hash'),
    allowedEmailDomains: citext('allowed_email_domains').array().notNull().default([]),
    recipientFrictionLevel: frictionLevelEnum('recipient_friction_level')
      .notNull()
      .default('medium'),
    status: smartLinkStatusEnum('status').notNull().default('active'),
    expiresAt: timestamp('expires_at', { withTimezone: true }),
    revokedAt: timestamp('revoked_at', { withTimezone: true }),
    revokedByUserId: uuid('revoked_by_user_id').references(() => users.id, {
      onDelete: 'set null',
    }),
    settings: jsonb('settings').notNull().default({}),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    check(
      'smart_links_password_check',
      sql`${table.accessMode} <> 'password' OR ${table.passwordHash} IS NOT NULL`
    ),
  ]
);

export const smartLinkRecipients = pgTable(
  'smart_link_recipients',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    smartLinkId: uuid('smart_link_id')
      .notNull()
      .references(() => smartLinks.id, { onDelete: 'cascade' }),
    contactId: uuid('contact_id').references(() => contacts.id, { onDelete: 'set null' }),
    email: citext('email').notNull(),
    status: recipientStatusEnum('status').notNull().default('invited'),
    verifiedAt: timestamp('verified_at', { withTimezone: true }),
    firstOpenedAt: timestamp('first_opened_at', { withTimezone: true }),
    lastOpenedAt: timestamp('last_opened_at', { withTimezone: true }),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [unique('smart_link_recipients_link_email_idx').on(table.smartLinkId, table.email)]
);

export const accessGrants = pgTable(
  'access_grants',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    scopeType: accessScopeTypeEnum('scope_type').notNull(),
    scopeId: uuid('scope_id').notNull(),
    contactId: uuid('contact_id').references(() => contacts.id, { onDelete: 'set null' }),
    email: citext('email').notNull(),
    status: accessGrantStatusEnum('status').notNull().default('pending'),
    requestedAt: timestamp('requested_at', { withTimezone: true }).notNull().defaultNow(),
    resolvedAt: timestamp('resolved_at', { withTimezone: true }),
    resolvedByUserId: uuid('resolved_by_user_id').references(() => users.id, {
      onDelete: 'set null',
    }),
    reason: text('reason'),
  },
  (table) => [
    unique('access_grants_scope_email_idx').on(table.scopeType, table.scopeId, table.email),
  ]
);

export const dealRooms = pgTable('deal_rooms', {
  id: uuid('id').primaryKey().defaultRandom(),
  workspaceId: uuid('workspace_id')
    .notNull()
    .references(() => workspaces.id, { onDelete: 'cascade' }),
  createdByUserId: uuid('created_by_user_id').references(() => users.id, {
    onDelete: 'set null',
  }),
  name: text('name').notNull(),
  description: text('description'),
  roomType: roomTypeEnum('room_type').notNull().default('custom'),
  status: roomStatusEnum('status').notNull().default('draft'),
  defaultAccessMode: linkAccessModeEnum('default_access_mode')
    .notNull()
    .default('email_verification'),
  downloadPolicy: downloadPolicyEnum('download_policy').notNull().default('watermarked'),
  watermarkEnabled: boolean('watermark_enabled').notNull().default(true),
  expiresAt: timestamp('expires_at', { withTimezone: true }),
  archivedAt: timestamp('archived_at', { withTimezone: true }),
  settings: jsonb('settings').notNull().default({}),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
});

export const dealRoomFolders = pgTable(
  'deal_room_folders',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    dealRoomId: uuid('deal_room_id')
      .notNull()
      .references(() => dealRooms.id, { onDelete: 'cascade' }),
    // Self-referencing FK added in migration SQL to avoid circular type inference.
    parentFolderId: uuid('parent_folder_id'),
    name: text('name').notNull(),
    sortOrder: integer('sort_order').notNull().default(0),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    unique('deal_room_folders_room_parent_name_idx').on(
      table.dealRoomId,
      table.parentFolderId,
      table.name
    ),
  ]
);

export const dealRoomFiles = pgTable(
  'deal_room_files',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    dealRoomId: uuid('deal_room_id')
      .notNull()
      .references(() => dealRooms.id, { onDelete: 'cascade' }),
    folderId: uuid('folder_id')
      .notNull()
      .references(() => dealRoomFolders.id, { onDelete: 'cascade' }),
    documentId: uuid('document_id')
      .notNull()
      .references(() => documents.id, { onDelete: 'cascade' }),
    documentVersionId: uuid('document_version_id').references(() => documentVersions.id, {
      onDelete: 'set null',
    }),
    displayName: text('display_name'),
    sortOrder: integer('sort_order').notNull().default(0),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [unique('deal_room_files_folder_document_idx').on(table.folderId, table.documentId)]
);

export const dealRoomMembers = pgTable(
  'deal_room_members',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    dealRoomId: uuid('deal_room_id')
      .notNull()
      .references(() => dealRooms.id, { onDelete: 'cascade' }),
    contactId: uuid('contact_id').references(() => contacts.id, { onDelete: 'set null' }),
    email: citext('email').notNull(),
    role: roomMemberRoleEnum('role').notNull().default('viewer'),
    status: recipientStatusEnum('status').notNull().default('invited'),
    invitedByUserId: uuid('invited_by_user_id').references(() => users.id, {
      onDelete: 'set null',
    }),
    invitedAt: timestamp('invited_at', { withTimezone: true }).notNull().defaultNow(),
    verifiedAt: timestamp('verified_at', { withTimezone: true }),
    lastOpenedAt: timestamp('last_opened_at', { withTimezone: true }),
  },
  (table) => [unique('deal_room_members_room_email_idx').on(table.dealRoomId, table.email)]
);

export const dealRoomAccessRules = pgTable(
  'deal_room_access_rules',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    dealRoomId: uuid('deal_room_id')
      .notNull()
      .references(() => dealRooms.id, { onDelete: 'cascade' }),
    folderId: uuid('folder_id').references(() => dealRoomFolders.id, { onDelete: 'cascade' }),
    documentId: uuid('document_id').references(() => documents.id, { onDelete: 'cascade' }),
    principalType: principalTypeEnum('principal_type').notNull(),
    principalValue: text('principal_value').notNull(),
    canView: boolean('can_view').notNull().default(true),
    canDownload: boolean('can_download').notNull().default(false),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    check(
      'deal_room_access_rules_scope_check',
      sql`${table.folderId} IS NOT NULL OR ${table.documentId} IS NOT NULL`
    ),
  ]
);

export const dealRoomQuestions = pgTable('deal_room_questions', {
  id: uuid('id').primaryKey().defaultRandom(),
  workspaceId: uuid('workspace_id')
    .notNull()
    .references(() => workspaces.id, { onDelete: 'cascade' }),
  dealRoomId: uuid('deal_room_id')
    .notNull()
    .references(() => dealRooms.id, { onDelete: 'cascade' }),
  askedByContactId: uuid('asked_by_contact_id').references(() => contacts.id, {
    onDelete: 'set null',
  }),
  assignedToUserId: uuid('assigned_to_user_id').references(() => users.id, {
    onDelete: 'set null',
  }),
  relatedDocumentId: uuid('related_document_id').references(() => documents.id, {
    onDelete: 'set null',
  }),
  questionText: text('question_text').notNull(),
  answerText: text('answer_text'),
  status: questionStatusEnum('status').notNull().default('open'),
  answeredByUserId: uuid('answered_by_user_id').references(() => users.id, {
    onDelete: 'set null',
  }),
  answeredAt: timestamp('answered_at', { withTimezone: true }),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
});

export const viewSessions = pgTable(
  'view_sessions',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    smartLinkId: uuid('smart_link_id').references(() => smartLinks.id, {
      onDelete: 'set null',
    }),
    dealRoomId: uuid('deal_room_id').references(() => dealRooms.id, {
      onDelete: 'set null',
    }),
    dealRoomFileId: uuid('deal_room_file_id').references(() => dealRoomFiles.id, {
      onDelete: 'set null',
    }),
    contactId: uuid('contact_id').references(() => contacts.id, { onDelete: 'set null' }),
    recipientEmail: citext('recipient_email'),
    ipAddress: inet('ip_address'),
    userAgent: text('user_agent'),
    referrer: text('referrer'),
    countryCode: text('country_code'),
    region: text('region'),
    city: text('city'),
    deviceType: text('device_type'),
    browserName: text('browser_name'),
    startedAt: timestamp('started_at', { withTimezone: true }).notNull().defaultNow(),
    endedAt: timestamp('ended_at', { withTimezone: true }),
  },
  (table) => [
    check(
      'view_sessions_scope_check',
      sql`${table.smartLinkId} IS NOT NULL OR ${table.dealRoomId} IS NOT NULL`
    ),
  ]
);

export const activityEvents = pgTable(
  'activity_events',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    viewSessionId: uuid('view_session_id').references(() => viewSessions.id, {
      onDelete: 'set null',
    }),
    smartLinkId: uuid('smart_link_id').references(() => smartLinks.id, {
      onDelete: 'set null',
    }),
    dealRoomId: uuid('deal_room_id').references(() => dealRooms.id, {
      onDelete: 'set null',
    }),
    documentId: uuid('document_id').references(() => documents.id, {
      onDelete: 'set null',
    }),
    contactId: uuid('contact_id').references(() => contacts.id, {
      onDelete: 'set null',
    }),
    actorEmail: citext('actor_email'),
    eventType: text('event_type').notNull(),
    metadata: jsonb('metadata').notNull().default({}),
    occurredAt: timestamp('occurred_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    index('idx_activity_events_workspace_occurred_at').on(
      table.workspaceId,
      table.occurredAt.desc()
    ),
    index('idx_activity_events_type_occurred_at').on(
      table.workspaceId,
      table.eventType,
      table.occurredAt.desc()
    ),
    index('idx_activity_events_smart_link_id').on(table.smartLinkId),
    index('idx_activity_events_deal_room_id').on(table.dealRoomId),
    index('idx_activity_events_contact_id').on(table.contactId),
  ]
);

export const pageViewEvents = pgTable(
  'page_view_events',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    viewSessionId: uuid('view_session_id')
      .notNull()
      .references(() => viewSessions.id, { onDelete: 'cascade' }),
    documentId: uuid('document_id')
      .notNull()
      .references(() => documents.id, { onDelete: 'cascade' }),
    documentVersionId: uuid('document_version_id')
      .notNull()
      .references(() => documentVersions.id, { onDelete: 'cascade' }),
    pageNumber: integer('page_number').notNull(),
    visibleStartedAt: timestamp('visible_started_at', { withTimezone: true }).notNull(),
    visibleEndedAt: timestamp('visible_ended_at', { withTimezone: true }),
    durationMs: integer('duration_ms'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    check('page_view_events_page_number_check', sql`${table.pageNumber} > 0`),
    check(
      'page_view_events_duration_check',
      sql`${table.durationMs} IS NULL OR ${table.durationMs} >= 0`
    ),
    index('idx_page_view_events_session_id').on(table.viewSessionId),
    index('idx_page_view_events_document_page').on(table.documentVersionId, table.pageNumber),
  ]
);

export const downloadEvents = pgTable(
  'download_events',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    viewSessionId: uuid('view_session_id').references(() => viewSessions.id, {
      onDelete: 'set null',
    }),
    smartLinkId: uuid('smart_link_id').references(() => smartLinks.id, {
      onDelete: 'set null',
    }),
    dealRoomId: uuid('deal_room_id').references(() => dealRooms.id, {
      onDelete: 'set null',
    }),
    documentId: uuid('document_id')
      .notNull()
      .references(() => documents.id, { onDelete: 'cascade' }),
    contactId: uuid('contact_id').references(() => contacts.id, {
      onDelete: 'set null',
    }),
    actorEmail: citext('actor_email'),
    downloadStatus: downloadStatusEnum('download_status').notNull(),
    watermarked: boolean('watermarked').notNull().default(false),
    blockedReason: text('blocked_reason'),
    occurredAt: timestamp('occurred_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    index('idx_download_events_workspace_occurred_at').on(
      table.workspaceId,
      table.occurredAt.desc()
    ),
    index('idx_download_events_smart_link_id').on(table.smartLinkId),
    index('idx_download_events_deal_room_id').on(table.dealRoomId),
  ]
);

export const intentScores = pgTable(
  'intent_scores',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    scoreType: scoreTypeEnum('score_type').notNull(),
    score: integer('score').notNull(),
    label: scoreLabelEnum('label').notNull(),
    explanation: text('explanation').notNull(),
    factors: jsonb('factors').notNull().default({}),
    contactId: uuid('contact_id').references(() => contacts.id, { onDelete: 'cascade' }),
    accountId: uuid('account_id').references(() => accounts.id, { onDelete: 'cascade' }),
    smartLinkId: uuid('smart_link_id').references(() => smartLinks.id, {
      onDelete: 'cascade',
    }),
    dealRoomId: uuid('deal_room_id').references(() => dealRooms.id, {
      onDelete: 'cascade',
    }),
    documentId: uuid('document_id').references(() => documents.id, {
      onDelete: 'cascade',
    }),
    calculatedAt: timestamp('calculated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    check(
      'intent_scores_target_check',
      sql`${table.contactId} IS NOT NULL
        OR ${table.accountId} IS NOT NULL
        OR ${table.smartLinkId} IS NOT NULL
        OR ${table.dealRoomId} IS NOT NULL
        OR ${table.documentId} IS NOT NULL`
    ),
    check('intent_scores_score_check', sql`${table.score} >= 0 AND ${table.score} <= 100`),
    index('idx_intent_scores_workspace_type_calculated_at').on(
      table.workspaceId,
      table.scoreType,
      table.calculatedAt.desc()
    ),
    index('idx_intent_scores_contact_type_calculated_at').on(
      table.contactId,
      table.scoreType,
      table.calculatedAt.desc()
    ),
    index('idx_intent_scores_smart_link_type_calculated_at').on(
      table.smartLinkId,
      table.scoreType,
      table.calculatedAt.desc()
    ),
    index('idx_intent_scores_deal_room_type_calculated_at').on(
      table.dealRoomId,
      table.scoreType,
      table.calculatedAt.desc()
    ),
  ]
);

export const recommendations = pgTable(
  'recommendations',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    createdForUserId: uuid('created_for_user_id').references(() => users.id, {
      onDelete: 'set null',
    }),
    contactId: uuid('contact_id').references(() => contacts.id, { onDelete: 'cascade' }),
    accountId: uuid('account_id').references(() => accounts.id, { onDelete: 'cascade' }),
    smartLinkId: uuid('smart_link_id').references(() => smartLinks.id, {
      onDelete: 'cascade',
    }),
    dealRoomId: uuid('deal_room_id').references(() => dealRooms.id, {
      onDelete: 'cascade',
    }),
    title: text('title').notNull(),
    body: text('body').notNull(),
    recommendedAction: text('recommended_action').notNull(),
    status: recommendationStatusEnum('status').notNull().default('open'),
    metadata: jsonb('metadata').notNull().default({}),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    index('idx_recommendations_workspace_status').on(
      table.workspaceId,
      table.status,
      table.createdAt.desc()
    ),
  ]
);

export const notificationPreferences = pgTable(
  'notification_preferences',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    userId: uuid('user_id')
      .notNull()
      .references(() => users.id, { onDelete: 'cascade' }),
    channel: notificationChannelEnum('channel').notNull(),
    eventType: text('event_type').notNull(),
    enabled: boolean('enabled').notNull().default(true),
    settings: jsonb('settings').notNull().default({}),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    unique('notification_preferences_workspace_user_channel_event_idx').on(
      table.workspaceId,
      table.userId,
      table.channel,
      table.eventType
    ),
  ]
);

export const notifications = pgTable(
  'notifications',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    userId: uuid('user_id').references(() => users.id, { onDelete: 'set null' }),
    channel: notificationChannelEnum('channel').notNull(),
    status: notificationStatusEnum('status').notNull().default('queued'),
    subject: text('subject'),
    body: text('body').notNull(),
    destination: text('destination'),
    relatedEventId: uuid('related_event_id').references(() => activityEvents.id, {
      onDelete: 'set null',
    }),
    metadata: jsonb('metadata').notNull().default({}),
    queuedAt: timestamp('queued_at', { withTimezone: true }).notNull().defaultNow(),
    sentAt: timestamp('sent_at', { withTimezone: true }),
    failedAt: timestamp('failed_at', { withTimezone: true }),
    failureReason: text('failure_reason'),
  },
  (table) => [
    index('idx_notifications_workspace_status').on(
      table.workspaceId,
      table.status,
      table.queuedAt.desc()
    ),
  ]
);

export const integrations = pgTable(
  'integrations',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    provider: integrationProviderEnum('provider').notNull(),
    status: integrationStatusEnum('status').notNull().default('connected'),
    externalAccountId: text('external_account_id'),
    encryptedCredentials: bytea('encrypted_credentials'),
    settings: jsonb('settings').notNull().default({}),
    connectedByUserId: uuid('connected_by_user_id').references(() => users.id, {
      onDelete: 'set null',
    }),
    connectedAt: timestamp('connected_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    unique('integrations_workspace_provider_external_idx').on(
      table.workspaceId,
      table.provider,
      table.externalAccountId
    ),
    index('idx_integrations_workspace_provider').on(table.workspaceId, table.provider),
  ]
);

export const crmMappings = pgTable(
  'crm_mappings',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    workspaceId: uuid('workspace_id')
      .notNull()
      .references(() => workspaces.id, { onDelete: 'cascade' }),
    integrationId: uuid('integration_id')
      .notNull()
      .references(() => integrations.id, { onDelete: 'cascade' }),
    objectType: crmObjectTypeEnum('object_type').notNull(),
    localObjectId: uuid('local_object_id').notNull(),
    externalObjectId: text('external_object_id').notNull(),
    externalObjectUrl: text('external_object_url'),
    metadata: jsonb('metadata').notNull().default({}),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => [
    unique('crm_mappings_integration_local_object_idx').on(
      table.integrationId,
      table.objectType,
      table.localObjectId
    ),
    unique('crm_mappings_integration_external_object_idx').on(
      table.integrationId,
      table.objectType,
      table.externalObjectId
    ),
    index('idx_crm_mappings_local_object').on(table.objectType, table.localObjectId),
  ]
);
