import type { FastifyReply, FastifyRequest } from 'fastify';
import { getBearerToken, verifySessionToken, type SessionClaims } from './session.js';
import { hasMinimumWorkspaceRole, type MembershipRole } from './roles.js';
import { createDrizzleAuthStore, type AuthStore } from './store.js';

export type AuthenticatedRequestContext = SessionClaims;

const defaultAuthStore = createDrizzleAuthStore();

export async function authenticateRequest(
  request: FastifyRequest,
  authStore: AuthStore = defaultAuthStore
): Promise<AuthenticatedRequestContext | null> {
  const token = getBearerToken(request.headers.authorization);
  if (!token) return null;

  const claims = verifySessionToken(token);
  if (!claims) return null;

  const membership = await authStore.getMembership(claims.userId, claims.workspaceId);
  if (!membership || membership.role !== claims.role) return null;

  return claims;
}

export async function requireAuthenticatedRequest(
  request: FastifyRequest,
  reply: FastifyReply,
  authStore: AuthStore = defaultAuthStore
): Promise<AuthenticatedRequestContext | null> {
  const claims = await authenticateRequest(request, authStore);
  if (!claims) {
    sendUnauthorized(reply);
    return null;
  }

  return claims;
}

export async function requireWorkspaceRole(
  request: FastifyRequest,
  reply: FastifyReply,
  minimumRole: MembershipRole,
  authStore: AuthStore = defaultAuthStore
): Promise<AuthenticatedRequestContext | null> {
  const claims = await requireAuthenticatedRequest(request, reply, authStore);
  if (!claims) return null;

  if (!hasMinimumWorkspaceRole(claims.role, minimumRole)) {
    sendForbidden(reply);
    return null;
  }

  return claims;
}

export function sendUnauthorized(reply: FastifyReply) {
  return reply.status(401).send({ error: { code: 'UNAUTHORIZED', message: 'Unauthorized' } });
}

export function sendForbidden(reply: FastifyReply) {
  return reply.status(403).send({ error: { code: 'FORBIDDEN', message: 'Forbidden' } });
}
