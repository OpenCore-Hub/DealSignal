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
import { LocalObjectStorageProvider } from './local-provider.js';
import type { StorageMetadataStore } from './store.js';

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

function createMetadataStore(
  workspaceId: string,
  bucket: string,
  key: string
): StorageMetadataStore {
  return {
    async getWorkspaceObject(requestWorkspaceId, requestBucket, requestKey) {
      if (requestWorkspaceId !== workspaceId || requestBucket !== bucket || requestKey !== key) {
        return null;
      }

      return {
        id: randomUUID(),
        workspaceId,
        bucket,
        key,
        checksumSha256: null,
      };
    },
  };
}

test('storage routes proxy object bytes only after auth and workspace metadata match', async () => {
  process.env.SESSION_SECRET = 'storage-routes-test-secret';
  process.env.STORAGE_PROXY_SECRET = 'storage-routes-proxy-secret';
  const rootDir = await mkdtemp(join(tmpdir(), 'dealsignal-storage-routes-'));
  const userId = randomUUID();
  const workspaceId = randomUUID();
  const otherWorkspaceId = randomUUID();
  const bucket = 'dealsignal-private';
  const key = `documents/${randomUUID()}/source.pdf`;
  const body = 'route-protected bytes';
  const storageProvider = new LocalObjectStorageProvider({ rootDir });
  await storageProvider.putObject({ bucket, key, body, contentType: 'application/pdf' });

  const app = await createApp({
    authStore: createRouteAuthStore(userId, workspaceId),
    storageMetadataStore: createMetadataStore(workspaceId, bucket, key),
    storageProvider,
  });

  test.after(async () => {
    delete process.env.SESSION_SECRET;
    delete process.env.STORAGE_PROXY_SECRET;
    await app.close();
    await rm(rootDir, { recursive: true, force: true });
  });

  const session = createSessionToken({ userId, workspaceId, role: 'member' });
  const invalidWorkspaceSession = createSessionToken({
    userId: randomUUID(),
    workspaceId: otherWorkspaceId,
    role: 'member',
  });

  const unauthorized = await app.inject({
    method: 'GET',
    url: `/storage/objects/${bucket}/${key}`,
  });
  assert.equal(unauthorized.statusCode, 401);

  const forbidden = await app.inject({
    method: 'GET',
    url: `/storage/objects/${bucket}/${key}`,
    headers: { authorization: `Bearer ${invalidWorkspaceSession}` },
  });
  assert.equal(forbidden.statusCode, 401);

  const unknownObject = await app.inject({
    method: 'GET',
    url: `/storage/objects/${bucket}/documents/${randomUUID()}/source.pdf`,
    headers: { authorization: `Bearer ${session}` },
  });
  assert.equal(unknownObject.statusCode, 403);

  const retrieved = await app.inject({
    method: 'GET',
    url: `/storage/objects/${bucket}/${key}`,
    headers: { authorization: `Bearer ${session}` },
  });
  assert.equal(retrieved.statusCode, 200);
  assert.equal(retrieved.headers['x-dealsignal-storage-bucket'], bucket);
  assert.equal(retrieved.headers['x-dealsignal-storage-key'], key);
  assert.equal(retrieved.payload, body);

  const tokenResponse = await app.inject({
    method: 'POST',
    url: `/storage/object-proxy-tokens/${bucket}/${key}`,
    headers: { authorization: `Bearer ${session}` },
  });
  assert.equal(tokenResponse.statusCode, 200);
  const proxy = JSON.parse(tokenResponse.payload) as { token: string; expiresInSeconds: number };
  assert.equal(proxy.expiresInSeconds, 300);

  const proxied = await app.inject({ method: 'GET', url: `/storage/proxy/${proxy.token}` });
  assert.equal(proxied.statusCode, 200);
  assert.equal(proxied.payload, body);
});
