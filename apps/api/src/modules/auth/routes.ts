import type { FastifyInstance } from 'fastify';
import {
  authenticateRequest,
  requireWorkspaceRole,
  sendForbidden,
  sendUnauthorized,
} from './current-user.js';
import { hashPassword, verifyPassword } from './password.js';
import { parseMembershipRole, type MembershipRole } from './roles.js';
import { createSessionToken } from './session.js';
import { createDrizzleAuthStore, type AuthStore, type WorkspaceMode } from './store.js';

type RegisterBody = {
  email?: unknown;
  name?: unknown;
  password?: unknown;
  workspaceName?: unknown;
  workspaceMode?: unknown;
};

type LoginBody = {
  email?: unknown;
  password?: unknown;
  workspaceId?: unknown;
};

type CreateWorkspaceBody = {
  name?: unknown;
  mode?: unknown;
};

type AddWorkspaceMemberBody = {
  email?: unknown;
  role?: unknown;
};

type SwitchWorkspaceBody = {
  workspaceId?: unknown;
};

type WorkspaceParams = {
  workspaceId?: string;
};

export type AuthRoutesOptions = {
  authStore?: AuthStore;
};

const defaultAuthStore = createDrizzleAuthStore();

function normalizeEmail(value: unknown): string | null {
  if (typeof value !== 'string') return null;
  const email = value.trim().toLowerCase();
  if (!email || !email.includes('@') || email.length > 320) return null;
  return email;
}

function parseRequiredString(value: unknown, minLength: number, maxLength: number): string | null {
  if (typeof value !== 'string') return null;
  const trimmed = value.trim();
  if (trimmed.length < minLength || trimmed.length > maxLength) return null;
  return trimmed;
}

function parseWorkspaceMode(value: unknown): WorkspaceMode {
  return value === 'founder' ||
    value === 'investment_firm' ||
    value === 'sales' ||
    value === 'mixed'
    ? value
    : 'mixed';
}

const workspaceIdPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

function parseWorkspaceId(value: unknown): string | null {
  const workspaceId = parseRequiredString(value, 36, 36);
  return workspaceId && workspaceIdPattern.test(workspaceId) ? workspaceId : null;
}

function slugifyWorkspaceName(name: string): string {
  const base = name
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48);
  const suffix = Math.random().toString(36).slice(2, 8);
  return `${base || 'workspace'}-${suffix}`;
}

function buildSessionResponse(userId: string, workspaceId: string, role: MembershipRole) {
  const token = createSessionToken({ userId, workspaceId, role });
  return { token, userId, workspaceId, role };
}

function isUniqueViolation(error: unknown): boolean {
  return (error as { code?: string }).code === '23505';
}

export async function registerAuthRoutes(
  app: FastifyInstance,
  options: AuthRoutesOptions = {}
): Promise<void> {
  const authStore = options.authStore ?? defaultAuthStore;

  app.post('/auth/register', async (request, reply) => {
    const body = (request.body ?? {}) as RegisterBody;
    const email = normalizeEmail(body.email);
    const name = parseRequiredString(body.name, 1, 160);
    const password = parseRequiredString(body.password, 8, 256);
    const workspaceName = parseRequiredString(body.workspaceName ?? body.name, 1, 160);
    const workspaceMode = parseWorkspaceMode(body.workspaceMode);

    if (!email || !name || !password || !workspaceName) {
      return reply.status(400).send({
        error: {
          code: 'INVALID_REGISTER_INPUT',
          message: 'Email, name, password, and workspaceName are required.',
        },
      });
    }

    try {
      const result = await authStore.createUserWorkspace({
        email,
        name,
        passwordHash: hashPassword(password),
        workspaceName,
        workspaceSlug: slugifyWorkspaceName(workspaceName),
        workspaceMode,
      });

      const session = buildSessionResponse(result.user.id, result.workspace.id, result.role);
      return reply.status(201).send({
        user: result.user,
        workspace: result.workspace,
        session,
      });
    } catch (error) {
      if (isUniqueViolation(error)) {
        return reply.status(409).send({
          error: {
            code: 'EMAIL_OR_WORKSPACE_EXISTS',
            message: 'Email or workspace already exists.',
          },
        });
      }
      request.log.error({ error }, 'auth.register_failed');
      return reply.status(500).send({
        error: { code: 'REGISTER_FAILED', message: 'Failed to register user.' },
      });
    }
  });

  app.post('/auth/login', async (request, reply) => {
    const body = (request.body ?? {}) as LoginBody;
    const email = normalizeEmail(body.email);
    const password = parseRequiredString(body.password, 1, 256);
    const requestedWorkspaceId = parseWorkspaceId(body.workspaceId);

    if (!email || !password) {
      return reply.status(400).send({
        error: { code: 'INVALID_LOGIN_INPUT', message: 'Email and password are required.' },
      });
    }

    if (body.workspaceId !== undefined && !requestedWorkspaceId) {
      return reply.status(400).send({
        error: { code: 'INVALID_WORKSPACE_INPUT', message: 'Workspace id is invalid.' },
      });
    }

    const user = await authStore.findUserByEmail(email);
    if (!user) return sendUnauthorized(reply);

    const passwordHash = await authStore.getPasswordHash(user.id);
    if (!passwordHash || !verifyPassword(password, passwordHash)) {
      return sendUnauthorized(reply);
    }

    if (requestedWorkspaceId) {
      const membership = await authStore.getMembership(user.id, requestedWorkspaceId);
      if (!membership) return sendUnauthorized(reply);

      const session = buildSessionResponse(user.id, membership.workspaceId, membership.role);
      return reply.send({ user, session });
    }

    const [membership] = await authStore.listMembershipsForUser(user.id);
    if (!membership) return sendUnauthorized(reply);

    const session = buildSessionResponse(user.id, membership.workspace.id, membership.role);
    return reply.send({ user, session });
  });

  app.get('/auth/me', async (request, reply) => {
    const claims = await authenticateRequest(request, authStore);
    if (!claims) return sendUnauthorized(reply);

    const user = await authStore.getUserById(claims.userId);
    const workspace = await authStore.getWorkspaceById(claims.workspaceId);
    if (!user || !workspace) return sendUnauthorized(reply);

    return reply.send({ user, workspace, role: claims.role });
  });

  app.get('/workspaces', async (request, reply) => {
    const claims = await authenticateRequest(request, authStore);
    if (!claims) return sendUnauthorized(reply);

    const memberships = await authStore.listMembershipsForUser(claims.userId);
    return reply.send({ memberships });
  });

  app.post('/workspaces', async (request, reply) => {
    const claims = await authenticateRequest(request, authStore);
    if (!claims) return sendUnauthorized(reply);

    const body = (request.body ?? {}) as CreateWorkspaceBody;
    const name = parseRequiredString(body.name, 1, 160);
    const mode = parseWorkspaceMode(body.mode);

    if (!name) {
      return reply.status(400).send({
        error: { code: 'INVALID_WORKSPACE_INPUT', message: 'Workspace name is required.' },
      });
    }

    try {
      const membership = await authStore.createWorkspaceForUser({
        userId: claims.userId,
        name,
        slug: slugifyWorkspaceName(name),
        mode,
      });
      const session = buildSessionResponse(claims.userId, membership.workspace.id, membership.role);

      return reply.status(201).send({ ...membership, session });
    } catch (error) {
      if (isUniqueViolation(error)) {
        return reply.status(409).send({
          error: { code: 'WORKSPACE_EXISTS', message: 'Workspace already exists.' },
        });
      }
      request.log.error({ error }, 'auth.workspace_create_failed');
      return reply.status(500).send({
        error: { code: 'WORKSPACE_CREATE_FAILED', message: 'Failed to create workspace.' },
      });
    }
  });

  app.post('/workspaces/switch', async (request, reply) => {
    const claims = await authenticateRequest(request, authStore);
    if (!claims) return sendUnauthorized(reply);

    const body = (request.body ?? {}) as SwitchWorkspaceBody;
    const workspaceId = parseWorkspaceId(body.workspaceId);
    if (!workspaceId) {
      return reply.status(400).send({
        error: { code: 'INVALID_WORKSPACE_INPUT', message: 'Workspace id is required.' },
      });
    }

    const membership = await authStore.getMembership(claims.userId, workspaceId);
    const workspace = await authStore.getWorkspaceById(workspaceId);
    if (!membership || !workspace) return sendForbidden(reply);

    const session = buildSessionResponse(claims.userId, workspace.id, membership.role);
    return reply.send({ workspace, role: membership.role, session });
  });

  app.post('/workspaces/:workspaceId/members', async (request, reply) => {
    const claims = await requireWorkspaceRole(request, reply, 'admin', authStore);
    if (!claims) return;

    const params = request.params as WorkspaceParams;
    const workspaceId = parseWorkspaceId(params.workspaceId);
    if (!workspaceId) {
      return reply.status(400).send({
        error: { code: 'INVALID_WORKSPACE_INPUT', message: 'Workspace id is required.' },
      });
    }
    if (workspaceId !== claims.workspaceId) return sendForbidden(reply);

    const body = (request.body ?? {}) as AddWorkspaceMemberBody;
    const email = normalizeEmail(body.email);
    const role = parseMembershipRole(body.role) ?? 'member';

    if (!email) {
      return reply.status(400).send({
        error: { code: 'INVALID_MEMBER_INPUT', message: 'Member email is required.' },
      });
    }

    try {
      const result = await authStore.addWorkspaceMember({ workspaceId, email, role });
      if (!result) {
        return reply.status(404).send({
          error: { code: 'USER_OR_WORKSPACE_NOT_FOUND', message: 'User or workspace not found.' },
        });
      }

      return reply.status(201).send(result);
    } catch (error) {
      if (isUniqueViolation(error)) {
        return reply.status(409).send({
          error: { code: 'MEMBERSHIP_EXISTS', message: 'Workspace membership already exists.' },
        });
      }
      request.log.error({ error }, 'auth.member_add_failed');
      return reply.status(500).send({
        error: { code: 'MEMBER_ADD_FAILED', message: 'Failed to add workspace member.' },
      });
    }
  });
}
