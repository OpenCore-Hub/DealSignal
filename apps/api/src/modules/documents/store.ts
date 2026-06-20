import { eq, and, max } from 'drizzle-orm';
import type { DocumentProcessingStatus, DocumentStatus } from '@dealsignal/shared';
import { db as defaultDb, type Database } from '../../db/index.js';
import { documents, documentVersions } from '../../db/schema.js';

export type DocumentRecord = {
  id: string;
  workspaceId: string;
  ownerUserId: string | null;
  name: string;
  description: string | null;
  status: DocumentStatus;
  currentVersionId: string | null;
  metadata: unknown;
  createdAt: Date;
  updatedAt: Date;
};

export type DocumentVersionRecord = {
  id: string;
  workspaceId: string;
  documentId: string;
  versionNumber: number;
  originalFilename: string;
  mimeType: string;
  fileSizeBytes: number;
  checksumSha256: string | null;
  storageBucket: string;
  storageKey: string;
  processingStatus: DocumentProcessingStatus;
  pageCount: number | null;
  processingError: string | null;
  createdByUserId: string | null;
  createdAt: Date;
};

export type CreateDocumentVersionInput = {
  workspaceId: string;
  ownerUserId: string;
  createdByUserId: string;
  name: string;
  description?: string | null;
  originalFilename: string;
  mimeType: string;
  fileSizeBytes: number;
  checksumSha256: string;
  storageBucket: string;
  storageKey: string;
  documentId?: string | null;
};

export interface DocumentStore {
  createDocumentVersion(
    input: CreateDocumentVersionInput
  ): Promise<{ document: DocumentRecord; version: DocumentVersionRecord }>;
  listDocuments(workspaceId: string): Promise<DocumentRecord[]>;
  getDocument(
    workspaceId: string,
    documentId: string
  ): Promise<{ document: DocumentRecord; versions: DocumentVersionRecord[] } | null>;
}

const documentSelect = {
  id: documents.id,
  workspaceId: documents.workspaceId,
  ownerUserId: documents.ownerUserId,
  name: documents.name,
  description: documents.description,
  status: documents.status,
  currentVersionId: documents.currentVersionId,
  metadata: documents.metadata,
  createdAt: documents.createdAt,
  updatedAt: documents.updatedAt,
};

const versionSelect = {
  id: documentVersions.id,
  workspaceId: documentVersions.workspaceId,
  documentId: documentVersions.documentId,
  versionNumber: documentVersions.versionNumber,
  originalFilename: documentVersions.originalFilename,
  mimeType: documentVersions.mimeType,
  fileSizeBytes: documentVersions.fileSizeBytes,
  checksumSha256: documentVersions.checksumSha256,
  storageBucket: documentVersions.storageBucket,
  storageKey: documentVersions.storageKey,
  processingStatus: documentVersions.processingStatus,
  pageCount: documentVersions.pageCount,
  processingError: documentVersions.processingError,
  createdByUserId: documentVersions.createdByUserId,
  createdAt: documentVersions.createdAt,
};

export function createDrizzleDocumentStore(database: Database = defaultDb): DocumentStore {
  return {
    async createDocumentVersion(input) {
      return await database.transaction(async (tx) => {
        let document: DocumentRecord;

        if (input.documentId) {
          const [existing] = await tx
            .select(documentSelect)
            .from(documents)
            .where(and(eq(documents.id, input.documentId), eq(documents.workspaceId, input.workspaceId)));

          if (!existing) {
            throw new DocumentStoreError('DOCUMENT_NOT_FOUND', 'Document not found in workspace.', {
              action: 'Verify the document id and workspace.',
              statusCode: 404,
            });
          }

          document = existing;
        } else {
          const [created] = await tx
            .insert(documents)
            .values({
              workspaceId: input.workspaceId,
              ownerUserId: input.ownerUserId,
              name: input.name,
              description: input.description ?? null,
              status: 'draft',
              metadata: {},
            })
            .returning(documentSelect);

          document = created;
        }

        const nextVersionNumber = await getNextVersionNumber(tx, document.id);

        const [version] = await tx
          .insert(documentVersions)
          .values({
            workspaceId: input.workspaceId,
            documentId: document.id,
            versionNumber: nextVersionNumber,
            originalFilename: input.originalFilename,
            mimeType: input.mimeType,
            fileSizeBytes: input.fileSizeBytes,
            checksumSha256: input.checksumSha256,
            storageBucket: input.storageBucket,
            storageKey: input.storageKey,
            processingStatus: 'uploaded',
            createdByUserId: input.createdByUserId,
          })
          .returning(versionSelect);

        const [updatedDocument] = await tx
          .update(documents)
          .set({
            currentVersionId: version.id,
            updatedAt: new Date(),
          })
          .where(eq(documents.id, document.id))
          .returning(documentSelect);

        return { document: updatedDocument, version };
      });
    },

    async listDocuments(workspaceId) {
      return await database
        .select(documentSelect)
        .from(documents)
        .where(eq(documents.workspaceId, workspaceId))
        .orderBy(documents.updatedAt);
    },

    async getDocument(workspaceId, documentId) {
      const [document] = await database
        .select(documentSelect)
        .from(documents)
        .where(and(eq(documents.id, documentId), eq(documents.workspaceId, workspaceId)));

      if (!document) return null;

      const versions = await database
        .select(versionSelect)
        .from(documentVersions)
        .where(eq(documentVersions.documentId, documentId))
        .orderBy(documentVersions.versionNumber);

      return { document, versions };
    },
  };
}

async function getNextVersionNumber(
  tx: Parameters<Parameters<Database['transaction']>[0]>[0],
  documentId: string
): Promise<number> {
  const [result] = await tx
    .select({ maxVersion: max(documentVersions.versionNumber) })
    .from(documentVersions)
    .where(eq(documentVersions.documentId, documentId));

  return (result?.maxVersion ?? 0) + 1;
}

export class DocumentStoreError extends Error {
  readonly code: string;
  readonly action: string;
  readonly statusCode: number;
  readonly retryable: boolean;

  constructor(
    code: string,
    message: string,
    options: { action?: string; statusCode?: number; retryable?: boolean; cause?: unknown } = {}
  ) {
    super(message, { cause: options.cause });
    this.code = code;
    this.action = options.action ?? 'Retry the request or contact support if the problem continues.';
    this.statusCode = options.statusCode ?? 500;
    this.retryable = options.retryable ?? false;
  }

  toResponse() {
    return {
      code: this.code,
      message: this.message,
      action: this.action,
      retryable: this.retryable,
    };
  }
}
