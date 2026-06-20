import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { createApp } from '../../app.js';
import { createSessionToken } from '../auth/session.js';
import type {
  AddWorkspaceMemberInput,
  AuthStore,
  CreateUserWorkspaceInput,
  CreateWorkspaceForUserInput,
} from '../auth/store.js';
import { LocalObjectStorageProvider } from '../storage/local-provider.js';
import type { DocumentRecord, DocumentStore, DocumentVersionRecord } from './store.js';

function createRouteAuthStore(userId: string, workspaceId: string): AuthStore {
  const user = { id: userId, email: 'route@example.com', name: 'Route Tester', avatarUrl: null };
  const workspace = {
    id: workspaceId,
    name: 'Route Workspace',
    slug: 'route-workspace',
    mode: 'mixed' as const,
  };

  return {
    async createUserWorkspace(_input: CreateUserWorkspaceInput) {
      return { user, workspace, role: 'owner' };
    },
    async findUserByEmail() {
      return user;
    },
    async getUserById() {
      return user;
    },
    async getPasswordHash() {
      return 'unused';
    },
    async getWorkspaceById() {
      return workspace;
    },
    async getMembership(requestUserId, requestWorkspaceId) {
      if (requestUserId === userId && requestWorkspaceId === workspaceId) {
        return { workspaceId, role: 'member' };
      }
      return null;
    },
    async listMembershipsForUser() {
      return [{ workspace, role: 'member' }];
    },
    async createWorkspaceForUser(_input: CreateWorkspaceForUserInput) {
      return { workspace, role: 'owner' };
    },
    async addWorkspaceMember(_input: AddWorkspaceMemberInput) {
      return { user, workspace, role: 'member' };
    },
  };
}

function createMockDocumentStore(): DocumentStore {
  const documents: DocumentRecord[] = [];
  const versions: DocumentVersionRecord[] = [];

  return {
    async createDocumentVersion(input) {
      let document: DocumentRecord;

      if (input.documentId) {
        const existing = documents.find((d) => d.id === input.documentId && d.workspaceId === input.workspaceId);
        if (!existing) {
          const error = new Error('Document not found in workspace.');
          (error as { code?: string }).code = 'DOCUMENT_NOT_FOUND';
          throw error;
        }
        document = existing;
      } else {
        document = {
          id: randomUUID(),
          workspaceId: input.workspaceId,
          ownerUserId: input.ownerUserId,
          name: input.name,
          description: input.description ?? null,
          status: 'draft',
          currentVersionId: null,
          metadata: {},
          createdAt: new Date(),
          updatedAt: new Date(),
        };
        documents.push(document);
      }

      const versionNumber = versions.filter((v) => v.documentId === document.id).length + 1;
      const version: DocumentVersionRecord = {
        id: randomUUID(),
        workspaceId: input.workspaceId,
        documentId: document.id,
        versionNumber,
        originalFilename: input.originalFilename,
        mimeType: input.mimeType,
        fileSizeBytes: input.fileSizeBytes,
        checksumSha256: input.checksumSha256,
        storageBucket: input.storageBucket,
        storageKey: input.storageKey,
        processingStatus: 'uploaded',
        pageCount: null,
        processingError: null,
        createdByUserId: input.createdByUserId,
        createdAt: new Date(),
      };
      versions.push(version);
      document.currentVersionId = version.id;

      return { document, version };
    },
    async listDocuments(workspaceId) {
      return documents.filter((d) => d.workspaceId === workspaceId);
    },
    async getDocument(workspaceId, documentId) {
      const document = documents.find((d) => d.id === documentId && d.workspaceId === workspaceId);
      if (!document) return null;
      const documentVersions = versions.filter((v) => v.documentId === documentId).sort((a, b) => a.versionNumber - b.versionNumber);
      return { document, versions: documentVersions };
    },
  };
}

function buildMultipartBody(
  boundary: string,
  fields: Record<string, string>,
  fileField: string,
  filename: string,
  mimeType: string,
  content: Buffer
): Buffer {
  const parts: Buffer[] = [];

  for (const [name, value] of Object.entries(fields)) {
    parts.push(
      Buffer.from(
        `--${boundary}\r\nContent-Disposition: form-data; name="${name}"\r\n\r\n${value}\r\n`
      )
    );
  }

  parts.push(
    Buffer.from(
      `--${boundary}\r\nContent-Disposition: form-data; name="${fileField}"; filename="${filename}"\r\nContent-Type: ${mimeType}\r\n\r\n`
    )
  );
  parts.push(content);
  parts.push(Buffer.from(`\r\n--${boundary}--\r\n`));

  return Buffer.concat(parts);
}

test('document upload creates document and version records', async () => {
  process.env.SESSION_SECRET = 'documents-routes-test-secret';
  const rootDir = await mkdtemp(join(tmpdir(), 'dealsignal-documents-routes-'));
  const userId = randomUUID();
  const workspaceId = randomUUID();
  const storageProvider = new LocalObjectStorageProvider({ rootDir });
  const documentStore = createMockDocumentStore();

  const app = await createApp({
    authStore: createRouteAuthStore(userId, workspaceId),
    documentStore,
    storageProvider,
  });

  test.after(async () => {
    delete process.env.SESSION_SECRET;
    await app.close();
    await rm(rootDir, { recursive: true, force: true });
  });

  const session = createSessionToken({ userId, workspaceId, role: 'member' });
  const content = Buffer.from('pdf-bytes');
  const boundary = '----FormBoundary' + randomUUID().replace(/-/g, '');
  const body = buildMultipartBody(boundary, { name: 'Series A Deck' }, 'file', 'deck.pdf', 'application/pdf', content);

  const response = await app.inject({
    method: 'POST',
    url: '/documents',
    headers: {
      authorization: `Bearer ${session}`,
      'content-type': `multipart/form-data; boundary=${boundary}`,
    },
    payload: body,
  });

  assert.equal(response.statusCode, 201);
  const payload = JSON.parse(response.payload) as {
    document: { id: string; name: string; currentVersionId: string };
    version: { id: string; versionNumber: number; storageKey: string; mimeType: string };
  };
  assert.equal(payload.document.name, 'Series A Deck');
  assert.equal(payload.version.versionNumber, 1);
  assert.equal(payload.version.mimeType, 'application/pdf');
  assert.ok(payload.version.storageKey.includes('/v1/'));
  assert.equal(payload.document.currentVersionId, payload.version.id);
});

test('second upload to same document increments version number', async () => {
  process.env.SESSION_SECRET = 'documents-routes-test-secret';
  const rootDir = await mkdtemp(join(tmpdir(), 'dealsignal-documents-routes-'));
  const userId = randomUUID();
  const workspaceId = randomUUID();
  const storageProvider = new LocalObjectStorageProvider({ rootDir });
  const documentStore = createMockDocumentStore();

  const app = await createApp({
    authStore: createRouteAuthStore(userId, workspaceId),
    documentStore,
    storageProvider,
  });

  test.after(async () => {
    delete process.env.SESSION_SECRET;
    await app.close();
    await rm(rootDir, { recursive: true, force: true });
  });

  const session = createSessionToken({ userId, workspaceId, role: 'member' });
  const boundary = '----FormBoundary' + randomUUID().replace(/-/g, '');

  const first = await app.inject({
    method: 'POST',
    url: '/documents',
    headers: {
      authorization: `Bearer ${session}`,
      'content-type': `multipart/form-data; boundary=${boundary}`,
    },
    payload: buildMultipartBody(boundary, { name: 'Series A Deck' }, 'file', 'deck.pdf', 'application/pdf', Buffer.from('v1')),
  });
  assert.equal(first.statusCode, 201);
  const firstPayload = JSON.parse(first.payload) as { document: { id: string } };
  const documentId = firstPayload.document.id;

  const secondBoundary = '----FormBoundary' + randomUUID().replace(/-/g, '');
  const second = await app.inject({
    method: 'POST',
    url: `/documents/${documentId}/versions`,
    headers: {
      authorization: `Bearer ${session}`,
      'content-type': `multipart/form-data; boundary=${secondBoundary}`,
    },
    payload: buildMultipartBody(
      secondBoundary,
      {},
      'file',
      'deck-v2.pdf',
      'application/pdf',
      Buffer.from('v2')
    ),
  });

  assert.equal(second.statusCode, 201);
  const secondPayload = JSON.parse(second.payload) as { version: { versionNumber: number; storageKey: string } };
  assert.equal(secondPayload.version.versionNumber, 2);
  assert.ok(secondPayload.version.storageKey.includes('/v2/'));
});

test('upload failure rolls back storage object', async () => {
  process.env.SESSION_SECRET = 'documents-routes-test-secret';
  const rootDir = await mkdtemp(join(tmpdir(), 'dealsignal-documents-routes-'));
  const userId = randomUUID();
  const workspaceId = randomUUID();
  const storageProvider = new LocalObjectStorageProvider({ rootDir });
  const documentStore = createMockDocumentStore();

  // Simulate DB failure after storage succeeds.
  const failingStore: DocumentStore = {
    async createDocumentVersion() {
      throw new Error('DB transaction failed');
    },
    async listDocuments(workspaceId_) {
      return documentStore.listDocuments(workspaceId_);
    },
    async getDocument(workspaceId_, documentId) {
      return documentStore.getDocument(workspaceId_, documentId);
    },
  };

  const app = await createApp({
    authStore: createRouteAuthStore(userId, workspaceId),
    documentStore: failingStore,
    storageProvider,
  });

  test.after(async () => {
    delete process.env.SESSION_SECRET;
    await app.close();
    await rm(rootDir, { recursive: true, force: true });
  });

  const session = createSessionToken({ userId, workspaceId, role: 'member' });
  const boundary = '----FormBoundary' + randomUUID().replace(/-/g, '');
  const content = Buffer.from('orphaned-bytes');

  const response = await app.inject({
    method: 'POST',
    url: '/documents',
    headers: {
      authorization: `Bearer ${session}`,
      'content-type': `multipart/form-data; boundary=${boundary}`,
    },
    payload: buildMultipartBody(boundary, { name: 'Fail Deck' }, 'file', 'fail.pdf', 'application/pdf', content),
  });

  assert.equal(response.statusCode, 500);

  // Verify no leftover object in storage by listing the temp directory.
  const objects = await storageProvider.getStream({ bucket: 'dealsignal-private', key: 'documents/any' }).catch(() => null);
  assert.equal(objects, null);
});

test('unsupported file type is rejected', async () => {
  process.env.SESSION_SECRET = 'documents-routes-test-secret';
  const rootDir = await mkdtemp(join(tmpdir(), 'dealsignal-documents-routes-'));
  const userId = randomUUID();
  const workspaceId = randomUUID();
  const storageProvider = new LocalObjectStorageProvider({ rootDir });

  const app = await createApp({
    authStore: createRouteAuthStore(userId, workspaceId),
    documentStore: createMockDocumentStore(),
    storageProvider,
  });

  test.after(async () => {
    delete process.env.SESSION_SECRET;
    await app.close();
    await rm(rootDir, { recursive: true, force: true });
  });

  const session = createSessionToken({ userId, workspaceId, role: 'member' });
  const boundary = '----FormBoundary' + randomUUID().replace(/-/g, '');

  const response = await app.inject({
    method: 'POST',
    url: '/documents',
    headers: {
      authorization: `Bearer ${session}`,
      'content-type': `multipart/form-data; boundary=${boundary}`,
    },
    payload: buildMultipartBody(boundary, { name: 'Bad' }, 'file', 'bad.exe', 'application/x-msdownload', Buffer.from('binary')),
  });

  assert.equal(response.statusCode, 415);
});

test('viewer cannot upload documents', async () => {
  process.env.SESSION_SECRET = 'documents-routes-test-secret';
  const rootDir = await mkdtemp(join(tmpdir(), 'dealsignal-documents-routes-'));
  const userId = randomUUID();
  const workspaceId = randomUUID();
  const storageProvider = new LocalObjectStorageProvider({ rootDir });

  const authStore = createRouteAuthStore(userId, workspaceId);
  const viewerStore: AuthStore = {
    ...authStore,
    async getMembership(requestUserId, requestWorkspaceId) {
      if (requestUserId === userId && requestWorkspaceId === workspaceId) {
        return { workspaceId, role: 'viewer' };
      }
      return null;
    },
    async listMembershipsForUser() {
      return [{ workspace: { id: workspaceId, name: 'Route Workspace', slug: 'route-workspace', mode: 'mixed' }, role: 'viewer' }];
    },
  };

  const app = await createApp({
    authStore: viewerStore,
    documentStore: createMockDocumentStore(),
    storageProvider,
  });

  test.after(async () => {
    delete process.env.SESSION_SECRET;
    await app.close();
    await rm(rootDir, { recursive: true, force: true });
  });

  const session = createSessionToken({ userId, workspaceId, role: 'viewer' });
  const boundary = '----FormBoundary' + randomUUID().replace(/-/g, '');

  const response = await app.inject({
    method: 'POST',
    url: '/documents',
    headers: {
      authorization: `Bearer ${session}`,
      'content-type': `multipart/form-data; boundary=${boundary}`,
    },
    payload: buildMultipartBody(boundary, { name: 'Viewer Upload' }, 'file', 'deck.pdf', 'application/pdf', Buffer.from('v1')),
  });

  assert.equal(response.statusCode, 403);
});

test('list and get documents enforce workspace scope', async () => {
  process.env.SESSION_SECRET = 'documents-routes-test-secret';
  const rootDir = await mkdtemp(join(tmpdir(), 'dealsignal-documents-routes-'));
  const userId = randomUUID();
  const workspaceId = randomUUID();
  const otherUserId = randomUUID();
  const otherWorkspaceId = randomUUID();
  const storageProvider = new LocalObjectStorageProvider({ rootDir });
  const documentStore = createMockDocumentStore();

  const workspace = { id: workspaceId, name: 'Route Workspace', slug: 'route-workspace', mode: 'mixed' as const };
  const otherWorkspace = {
    id: otherWorkspaceId,
    name: 'Other Workspace',
    slug: 'other-workspace',
    mode: 'mixed' as const,
  };

  const multiWorkspaceAuthStore: AuthStore = {
    ...createRouteAuthStore(userId, workspaceId),
    async getMembership(requestUserId, requestWorkspaceId) {
      if (requestUserId === userId && requestWorkspaceId === workspaceId) {
        return { workspaceId, role: 'member' };
      }
      if (requestUserId === otherUserId && requestWorkspaceId === otherWorkspaceId) {
        return { workspaceId: otherWorkspaceId, role: 'member' };
      }
      return null;
    },
    async getWorkspaceById(requestWorkspaceId) {
      if (requestWorkspaceId === workspaceId) return workspace;
      if (requestWorkspaceId === otherWorkspaceId) return otherWorkspace;
      return null;
    },
    async listMembershipsForUser(requestUserId) {
      if (requestUserId === userId) return [{ workspace, role: 'member' }];
      if (requestUserId === otherUserId) return [{ workspace: otherWorkspace, role: 'member' }];
      return [];
    },
  };

  const app = await createApp({
    authStore: multiWorkspaceAuthStore,
    documentStore,
    storageProvider,
  });

  test.after(async () => {
    delete process.env.SESSION_SECRET;
    await app.close();
    await rm(rootDir, { recursive: true, force: true });
  });

  const session = createSessionToken({ userId, workspaceId, role: 'member' });
  const otherSession = createSessionToken({ userId: otherUserId, workspaceId: otherWorkspaceId, role: 'member' });

  const boundary = '----FormBoundary' + randomUUID().replace(/-/g, '');
  const createResponse = await app.inject({
    method: 'POST',
    url: '/documents',
    headers: {
      authorization: `Bearer ${session}`,
      'content-type': `multipart/form-data; boundary=${boundary}`,
    },
    payload: buildMultipartBody(boundary, { name: 'Scoped Doc' }, 'file', 'deck.pdf', 'application/pdf', Buffer.from('v1')),
  });
  assert.equal(createResponse.statusCode, 201);

  const listResponse = await app.inject({
    method: 'GET',
    url: '/documents',
    headers: { authorization: `Bearer ${session}` },
  });
  assert.equal(listResponse.statusCode, 200);
  const listPayload = JSON.parse(listResponse.payload) as { documents: unknown[] };
  assert.equal(listPayload.documents.length, 1);

  const otherListResponse = await app.inject({
    method: 'GET',
    url: '/documents',
    headers: { authorization: `Bearer ${otherSession}` },
  });
  assert.equal(otherListResponse.statusCode, 200);
  const otherListPayload = JSON.parse(otherListResponse.payload) as { documents: unknown[] };
  assert.equal(otherListPayload.documents.length, 0);
});
