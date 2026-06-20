import { randomUUID } from 'node:crypto';
import { Readable } from 'node:stream';
import type { FastifyInstance } from 'fastify';
import multipart, { type Multipart } from '@fastify/multipart';
import type { DocumentProcessingStatus, DocumentStatus } from '@dealsignal/shared';
import { requireWorkspaceRole, sendForbidden } from '../auth/current-user.js';
import { createDrizzleAuthStore, type AuthStore } from '../auth/store.js';
import { createObjectStorageProvider, getStorageConfig } from '../storage/config.js';
import { isStorageError } from '../storage/errors.js';
import type { ObjectLocation, ObjectStorageProvider } from '../storage/types.js';
import { createDrizzleDocumentStore, DocumentStoreError, type DocumentStore } from './store.js';

const defaultAuthStore = createDrizzleAuthStore();
const defaultDocumentStore = createDrizzleDocumentStore();
const defaultStorageProvider = createObjectStorageProvider();

export type DocumentRoutesOptions = {
  authStore?: AuthStore;
  documentStore?: DocumentStore;
  storageProvider?: ObjectStorageProvider;
};

const MAX_FILE_SIZE_BYTES = 100 * 1024 * 1024;

const SUPPORTED_MIME_TYPES = new Set([
  'application/pdf',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  'image/png',
  'image/jpeg',
  'image/gif',
  'image/webp',
  'video/mp4',
  'video/webm',
  'video/quicktime',
]);

const SUPPORTED_MIME_PREFIXES = ['image/', 'video/'];

function isSupportedMimeType(mimeType: string): boolean {
  if (SUPPORTED_MIME_TYPES.has(mimeType)) return true;
  return SUPPORTED_MIME_PREFIXES.some((prefix) => mimeType.startsWith(prefix));
}

function sanitizeFilename(filename: string): string {
  return filename
    .normalize('NFKD')
    .replace(/[^a-zA-Z0-9._-]/g, '_')
    .replace(/_{2,}/g, '_')
    .slice(0, 200);
}

function buildStorageKey(workspaceId: string, documentId: string, versionNumber: number, filename: string): string {
  const safeFilename = sanitizeFilename(filename) || 'upload';
  return `documents/${workspaceId}/${documentId}/v${versionNumber}/${safeFilename}`;
}

function getFieldValue(field: Multipart | Multipart[] | undefined): string | undefined {
  if (!field) return undefined;
  const single = Array.isArray(field) ? field[0] : field;
  if (single && 'value' in single && typeof single.value === 'string') return single.value;
  return undefined;
}

function parseDocumentId(value: unknown): string | null {
  if (typeof value !== 'string') return null;
  const pattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
  return pattern.test(value) ? value : null;
}

function getErrorStatus(error: unknown): number {
  if (error instanceof DocumentStoreError) return error.statusCode;
  if (isStorageError(error)) return error.statusCode ?? 500;
  return 500;
}

function getErrorResponse(error: unknown) {
  if (error instanceof DocumentStoreError) return { error: error.toResponse() };
  if (isStorageError(error)) return { error: error.toResponse() };
  return {
    error: {
      code: 'DOCUMENT_UPLOAD_FAILED',
      message: 'Document upload failed.',
      action: 'Retry the upload or contact support if the problem continues.',
      retryable: true,
    },
  };
}

async function collectFileToBuffer(stream: Readable, maxBytes: number): Promise<Buffer> {
  const chunks: Buffer[] = [];
  let total = 0;

  for await (const chunk of stream) {
    const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    total += buffer.length;
    if (total > maxBytes) {
      throw new DocumentStoreError('FILE_TOO_LARGE', 'File exceeds the maximum upload size of 100 MB.', {
        action: 'Upload a smaller file.',
        statusCode: 413,
      });
    }
    chunks.push(buffer);
  }

  return Buffer.concat(chunks);
}

async function cleanupStorage(location: ObjectLocation, storageProvider: ObjectStorageProvider): Promise<void> {
  try {
    await storageProvider.deleteObject(location);
  } catch {
    // Best-effort cleanup; log silently.
  }
}

export async function registerDocumentRoutes(
  app: FastifyInstance,
  options: DocumentRoutesOptions = {}
): Promise<void> {
  const authStore = options.authStore ?? defaultAuthStore;
  const documentStore = options.documentStore ?? defaultDocumentStore;
  const storageProvider = options.storageProvider ?? defaultStorageProvider;

  await app.register(multipart, {
    limits: {
      fileSize: MAX_FILE_SIZE_BYTES,
      files: 1,
    },
  });

  app.post('/documents', async (request, reply) => {
    const claims = await requireWorkspaceRole(request, reply, 'member', authStore);
    if (!claims) return;

    const data = await request.file();
    if (!data) {
      return reply.status(400).send({
        error: { code: 'FILE_REQUIRED', message: 'A file is required.', action: 'Attach a file to the request.' },
      });
    }

    const filename = data.filename || 'upload';
    const mimeType = data.mimetype || 'application/octet-stream';
    if (!isSupportedMimeType(mimeType)) {
      return reply.status(415).send({
        error: {
          code: 'UNSUPPORTED_FILE_TYPE',
          message: 'File type is not supported.',
          action: 'Upload PDF, Office Open XML, image, or video files.',
        },
      });
    }

    let buffer: Buffer;
    try {
      buffer = await collectFileToBuffer(data.file, MAX_FILE_SIZE_BYTES);
    } catch (error) {
      return reply.status(getErrorStatus(error)).send(getErrorResponse(error));
    }

    const nameValue = getFieldValue(data.fields?.name);
    const descriptionValue = getFieldValue(data.fields?.description);
    const name = nameValue?.trim() || filename;
    const description = descriptionValue?.trim() || null;

    if (!name) {
      return reply.status(400).send({
        error: { code: 'NAME_REQUIRED', message: 'Document name is required.', action: 'Provide a name or filename.' },
      });
    }

    const documentId = randomUUID();
    const bucket = getStorageConfig().bucket;
    const key = buildStorageKey(claims.workspaceId, documentId, 1, filename);
    let location: ObjectLocation = { bucket, key };

    try {
      const putResult = await storageProvider.putObject({
        bucket,
        key,
        body: buffer,
        contentType: mimeType,
        contentLength: buffer.length,
      });

      location = { bucket: putResult.bucket, key: putResult.key };

      const result = await documentStore.createDocumentVersion({
        workspaceId: claims.workspaceId,
        ownerUserId: claims.userId,
        createdByUserId: claims.userId,
        name,
        description,
        originalFilename: filename,
        mimeType,
        fileSizeBytes: putResult.sizeBytes,
        checksumSha256: putResult.checksumSha256,
        storageBucket: putResult.bucket,
        storageKey: putResult.key,
      });

      return reply.status(201).send({
        document: serializeDocument(result.document),
        version: serializeVersion(result.version),
      });
    } catch (error) {
      await cleanupStorage(location, storageProvider);
      request.log.error({ error }, 'documents.upload_failed');
      return reply.status(getErrorStatus(error)).send(getErrorResponse(error));
    }
  });

  app.get('/documents', async (request, reply) => {
    const claims = await requireWorkspaceRole(request, reply, 'viewer', authStore);
    if (!claims) return;

    const records = await documentStore.listDocuments(claims.workspaceId);
    return reply.send({ documents: records.map(serializeDocument) });
  });

  app.get('/documents/:documentId', async (request, reply) => {
    const claims = await requireWorkspaceRole(request, reply, 'viewer', authStore);
    if (!claims) return;

    const params = request.params as { documentId?: string };
    const documentId = parseDocumentId(params.documentId);
    if (!documentId) {
      return reply.status(400).send({
        error: { code: 'INVALID_DOCUMENT_ID', message: 'Document id is invalid.', action: 'Provide a valid document id.' },
      });
    }

    const result = await documentStore.getDocument(claims.workspaceId, documentId);
    if (!result) return sendForbidden(reply);

    return reply.send({
      document: serializeDocument(result.document),
      versions: result.versions.map(serializeVersion),
    });
  });

  app.post('/documents/:documentId/versions', async (request, reply) => {
    const claims = await requireWorkspaceRole(request, reply, 'member', authStore);
    if (!claims) return;

    const params = request.params as { documentId?: string };
    const documentId = parseDocumentId(params.documentId);
    if (!documentId) {
      return reply.status(400).send({
        error: { code: 'INVALID_DOCUMENT_ID', message: 'Document id is invalid.', action: 'Provide a valid document id.' },
      });
    }

    const data = await request.file();
    if (!data) {
      return reply.status(400).send({
        error: { code: 'FILE_REQUIRED', message: 'A file is required.', action: 'Attach a file to the request.' },
      });
    }

    const filename = data.filename || 'upload';
    const mimeType = data.mimetype || 'application/octet-stream';
    if (!isSupportedMimeType(mimeType)) {
      return reply.status(415).send({
        error: {
          code: 'UNSUPPORTED_FILE_TYPE',
          message: 'File type is not supported.',
          action: 'Upload PDF, Office Open XML, image, or video files.',
        },
      });
    }

    let buffer: Buffer;
    try {
      buffer = await collectFileToBuffer(data.file, MAX_FILE_SIZE_BYTES);
    } catch (error) {
      return reply.status(getErrorStatus(error)).send(getErrorResponse(error));
    }

    const bucket = getStorageConfig().bucket;
    let location: ObjectLocation = { bucket, key: '' };

    try {
      // Determine next version number from existing versions so the storage key is correct.
      const existing = await documentStore.getDocument(claims.workspaceId, documentId);
      if (!existing) return sendForbidden(reply);

      const nextVersionNumber = (existing.versions.at(-1)?.versionNumber ?? 0) + 1;
      const key = buildStorageKey(claims.workspaceId, documentId, nextVersionNumber, filename);
      location = { bucket, key };

      const putResult = await storageProvider.putObject({
        bucket,
        key,
        body: buffer,
        contentType: mimeType,
        contentLength: buffer.length,
      });

      location = { bucket: putResult.bucket, key: putResult.key };

      const result = await documentStore.createDocumentVersion({
        workspaceId: claims.workspaceId,
        ownerUserId: claims.userId,
        createdByUserId: claims.userId,
        name: existing.document.name,
        documentId,
        originalFilename: filename,
        mimeType,
        fileSizeBytes: putResult.sizeBytes,
        checksumSha256: putResult.checksumSha256,
        storageBucket: putResult.bucket,
        storageKey: putResult.key,
      });

      return reply.status(201).send({
        document: serializeDocument(result.document),
        version: serializeVersion(result.version),
      });
    } catch (error) {
      if (location.key) await cleanupStorage(location, storageProvider);
      request.log.error({ error }, 'documents.version_upload_failed');
      return reply.status(getErrorStatus(error)).send(getErrorResponse(error));
    }
  });
}

function serializeDocument(document: {
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
}) {
  return {
    id: document.id,
    workspaceId: document.workspaceId,
    ownerUserId: document.ownerUserId,
    name: document.name,
    description: document.description,
    status: document.status,
    currentVersionId: document.currentVersionId,
    metadata: document.metadata,
    createdAt: document.createdAt.toISOString(),
    updatedAt: document.updatedAt.toISOString(),
  };
}

function serializeVersion(version: {
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
}) {
  return {
    id: version.id,
    workspaceId: version.workspaceId,
    documentId: version.documentId,
    versionNumber: version.versionNumber,
    originalFilename: version.originalFilename,
    mimeType: version.mimeType,
    fileSizeBytes: version.fileSizeBytes,
    checksumSha256: version.checksumSha256,
    storageBucket: version.storageBucket,
    storageKey: version.storageKey,
    processingStatus: version.processingStatus,
    pageCount: version.pageCount,
    processingError: version.processingError,
    createdByUserId: version.createdByUserId,
    createdAt: version.createdAt.toISOString(),
  };
}
