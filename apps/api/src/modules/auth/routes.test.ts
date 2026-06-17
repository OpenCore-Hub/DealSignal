import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import test from 'node:test';
import { createApp } from '../../app.js';
import type {
  AddWorkspaceMemberInput,
  AddWorkspaceMemberResult,
  AuthStore,
  AuthUser,
  AuthWorkspace,
  CreateUserWorkspaceInput,
  CreateWorkspaceForUserInput,
  WorkspaceMembership,
  WorkspaceMembershipWithWorkspace,
} from './store.js';

function createUniqueViolation(): Error & { code: string } {
  const error = new Error('unique violation') as Error & { code: string };
  error.code = '23505';
  return error;
}

function createInMemoryAuthStore(): AuthStore {
  const usersById = new Map<string, AuthUser>();
  const userIdsByEmail = new Map<string, string>();
  const passwordHashesByUserId = new Map<string, string>();
  const workspacesById = new Map<string, AuthWorkspace>();
  const workspaceIdsBySlug = new Map<string, string>();
  const memberships = new Map<string, WorkspaceMembership>();

  const membershipKey = (userId: string, workspaceId: string) => `${userId}:${workspaceId}`;

  function addMembership(userId: string, workspaceId: string, role: WorkspaceMembership['role']) {
    const key = membershipKey(userId, workspaceId);
    if (memberships.has(key)) throw createUniqueViolation();
    const membership = { workspaceId, role };
    memberships.set(key, membership);
    return membership;
  }

  return {
    async createUserWorkspace(input: CreateUserWorkspaceInput) {
      if (userIdsByEmail.has(input.email) || workspaceIdsBySlug.has(input.workspaceSlug)) {
        throw createUniqueViolation();
      }

      const user: AuthUser = {
        id: randomUUID(),
        email: input.email,
        name: input.name,
        avatarUrl: null,
      };
      const workspace: AuthWorkspace = {
        id: randomUUID(),
        name: input.workspaceName,
        slug: input.workspaceSlug,
        mode: input.workspaceMode,
      };

      usersById.set(user.id, user);
      userIdsByEmail.set(user.email, user.id);
      passwordHashesByUserId.set(user.id, input.passwordHash);
      workspacesById.set(workspace.id, workspace);
      workspaceIdsBySlug.set(workspace.slug, workspace.id);
      const membership = addMembership(user.id, workspace.id, 'owner');

      return { user, workspace, role: membership.role };
    },

    async findUserByEmail(email: string) {
      const userId = userIdsByEmail.get(email);
      return userId ? usersById.get(userId) ?? null : null;
    },

    async getUserById(userId: string) {
      return usersById.get(userId) ?? null;
    },

    async getPasswordHash(userId: string) {
      return passwordHashesByUserId.get(userId) ?? null;
    },

    async getWorkspaceById(workspaceId: string) {
      return workspacesById.get(workspaceId) ?? null;
    },

    async getMembership(userId: string, workspaceId: string) {
      return memberships.get(membershipKey(userId, workspaceId)) ?? null;
    },

    async listMembershipsForUser(userId: string) {
      const result: WorkspaceMembershipWithWorkspace[] = [];
      for (const membership of memberships.values()) {
        if (memberships.get(membershipKey(userId, membership.workspaceId)) !== membership) continue;
        const workspace = workspacesById.get(membership.workspaceId);
        if (workspace) result.push({ workspace, role: membership.role });
      }
      return result;
    },

    async createWorkspaceForUser(input: CreateWorkspaceForUserInput) {
      if (workspaceIdsBySlug.has(input.slug)) throw createUniqueViolation();

      const workspace: AuthWorkspace = {
        id: randomUUID(),
        name: input.name,
        slug: input.slug,
        mode: input.mode,
      };
      workspacesById.set(workspace.id, workspace);
      workspaceIdsBySlug.set(workspace.slug, workspace.id);
      const membership = addMembership(input.userId, workspace.id, 'owner');
      return { workspace, role: membership.role };
    },

    async addWorkspaceMember(input: AddWorkspaceMemberInput): Promise<AddWorkspaceMemberResult | null> {
      const user = await this.findUserByEmail(input.email);
      const workspace = await this.getWorkspaceById(input.workspaceId);
      if (!user || !workspace) return null;

      const membership = addMembership(user.id, workspace.id, input.role);
      return { user, workspace, role: membership.role };
    },
  };
}

function parseJson(response: { payload: string }): unknown {
  return JSON.parse(response.payload) as unknown;
}

test('auth routes register, resolve current user, and enforce workspace role scope', async () => {
  process.env.SESSION_SECRET = 'routes-test-secret';
  const authStore = createInMemoryAuthStore();
  const app = await createApp({ authStore });
  test.after(async () => {
    await app.close();
  });

  const aliceRegister = await app.inject({
    method: 'POST',
    url: '/auth/register',
    payload: {
      email: 'Alice@Example.com',
      name: 'Alice',
      password: 'correct horse battery staple',
      workspaceName: 'Acme Capital',
      workspaceMode: 'investment_firm',
    },
  });
  assert.equal(aliceRegister.statusCode, 201);
  const alice = parseJson(aliceRegister) as {
    user: AuthUser;
    workspace: AuthWorkspace;
    session: { token: string; role: string; workspaceId: string };
  };
  assert.equal(alice.user.email, 'alice@example.com');
  assert.equal(alice.session.role, 'owner');
  assert.equal(alice.session.workspaceId, alice.workspace.id);

  const me = await app.inject({
    method: 'GET',
    url: '/auth/me',
    headers: { authorization: `Bearer ${alice.session.token}` },
  });
  assert.equal(me.statusCode, 200);
  const currentUser = parseJson(me) as { user: AuthUser; workspace: AuthWorkspace; role: string };
  assert.equal(currentUser.user.id, alice.user.id);
  assert.equal(currentUser.workspace.id, alice.workspace.id);
  assert.equal(currentUser.role, 'owner');

  const bobRegister = await app.inject({
    method: 'POST',
    url: '/auth/register',
    payload: {
      email: 'bob@example.com',
      name: 'Bob',
      password: 'correct horse battery staple',
      workspaceName: 'Bob Workspace',
    },
  });
  assert.equal(bobRegister.statusCode, 201);

  const newWorkspace = await app.inject({
    method: 'POST',
    url: '/workspaces',
    headers: { authorization: `Bearer ${alice.session.token}` },
    payload: { name: 'Acme Follow-on Fund', mode: 'investment_firm' },
  });
  assert.equal(newWorkspace.statusCode, 201);
  const createdWorkspace = parseJson(newWorkspace) as {
    workspace: AuthWorkspace;
    role: string;
    session: { token: string };
  };
  assert.equal(createdWorkspace.role, 'owner');

  const crossWorkspaceAdd = await app.inject({
    method: 'POST',
    url: `/workspaces/${createdWorkspace.workspace.id}/members`,
    headers: { authorization: `Bearer ${alice.session.token}` },
    payload: { email: 'bob@example.com', role: 'viewer' },
  });
  assert.equal(crossWorkspaceAdd.statusCode, 403);

  const addBob = await app.inject({
    method: 'POST',
    url: `/workspaces/${createdWorkspace.workspace.id}/members`,
    headers: { authorization: `Bearer ${createdWorkspace.session.token}` },
    payload: { email: 'bob@example.com', role: 'viewer' },
  });
  assert.equal(addBob.statusCode, 201);
  const bobMembership = parseJson(addBob) as { user: AuthUser; workspace: AuthWorkspace; role: string };
  assert.equal(bobMembership.user.email, 'bob@example.com');
  assert.equal(bobMembership.workspace.id, createdWorkspace.workspace.id);
  assert.equal(bobMembership.role, 'viewer');

  const bobLogin = await app.inject({
    method: 'POST',
    url: '/auth/login',
    payload: {
      email: 'bob@example.com',
      password: 'correct horse battery staple',
      workspaceId: createdWorkspace.workspace.id,
    },
  });
  assert.equal(bobLogin.statusCode, 200);
  const bobSession = parseJson(bobLogin) as { session: { token: string; role: string } };
  assert.equal(bobSession.session.role, 'viewer');

  const viewerAdd = await app.inject({
    method: 'POST',
    url: `/workspaces/${createdWorkspace.workspace.id}/members`,
    headers: { authorization: `Bearer ${bobSession.session.token}` },
    payload: { email: 'alice@example.com', role: 'member' },
  });
  assert.equal(viewerAdd.statusCode, 403);
});

test('workspace switch returns 403 when the user is not a member', async () => {
  process.env.SESSION_SECRET = 'routes-test-secret';
  const authStore = createInMemoryAuthStore();
  const app = await createApp({ authStore });
  test.after(async () => {
    await app.close();
  });

  const aliceRegister = await app.inject({
    method: 'POST',
    url: '/auth/register',
    payload: {
      email: 'alice@example.com',
      name: 'Alice',
      password: 'correct horse battery staple',
      workspaceName: 'Alice Workspace',
    },
  });
  const alice = parseJson(aliceRegister) as { session: { token: string } };

  const bobRegister = await app.inject({
    method: 'POST',
    url: '/auth/register',
    payload: {
      email: 'bob@example.com',
      name: 'Bob',
      password: 'correct horse battery staple',
      workspaceName: 'Bob Workspace',
    },
  });
  const bob = parseJson(bobRegister) as { workspace: AuthWorkspace };

  const forbiddenSwitch = await app.inject({
    method: 'POST',
    url: '/workspaces/switch',
    headers: { authorization: `Bearer ${alice.session.token}` },
    payload: { workspaceId: bob.workspace.id },
  });
  assert.equal(forbiddenSwitch.statusCode, 403);
});
