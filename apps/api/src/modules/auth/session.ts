import { createHmac, timingSafeEqual } from 'node:crypto';

export interface SessionClaims {
  userId: string;
  workspaceId: string;
  role: 'owner' | 'admin' | 'member' | 'viewer';
  exp: number;
}

const TOKEN_TTL_SECONDS = 60 * 60 * 24 * 7;
const DEVELOPMENT_SESSION_SECRET = 'development-session-secret-change-me';

function getSessionSecret(): string {
  const secret = process.env.SESSION_SECRET;
  if (secret) return secret;

  if (process.env.NODE_ENV === 'production') {
    throw new Error('SESSION_SECRET must be set in production.');
  }

  return DEVELOPMENT_SESSION_SECRET;
}

function encode(value: unknown): string {
  return Buffer.from(JSON.stringify(value)).toString('base64url');
}

function decode<T>(value: string): T {
  return JSON.parse(Buffer.from(value, 'base64url').toString('utf8')) as T;
}

function sign(payload: string): string {
  return createHmac('sha256', getSessionSecret()).update(payload).digest('base64url');
}

export function createSessionToken(
  claims: Omit<SessionClaims, 'exp'>,
  ttlSeconds = TOKEN_TTL_SECONDS
): string {
  const header = encode({ alg: 'HS256', typ: 'DST' });
  const payload = encode({ ...claims, exp: Math.floor(Date.now() / 1000) + ttlSeconds });
  const signature = sign(`${header}.${payload}`);
  return `${header}.${payload}.${signature}`;
}

export function verifySessionToken(token: string): SessionClaims | null {
  const [header, payload, signature, extra] = token.split('.');
  if (!header || !payload || !signature || extra) return null;

  try {
    const expected = sign(`${header}.${payload}`);
    const actualBuffer = Buffer.from(signature, 'base64url');
    const expectedBuffer = Buffer.from(expected, 'base64url');
    if (actualBuffer.length !== expectedBuffer.length) return null;
    if (!timingSafeEqual(actualBuffer, expectedBuffer)) return null;

    const claims = decode<SessionClaims>(payload);
    if (!claims.userId || !claims.workspaceId || !claims.role || !claims.exp) return null;
    if (claims.exp < Math.floor(Date.now() / 1000)) return null;

    return claims;
  } catch {
    return null;
  }
}

export function getBearerToken(authorizationHeader: string | undefined): string | null {
  if (!authorizationHeader) return null;
  const [scheme, token] = authorizationHeader.split(' ');
  if (scheme?.toLowerCase() !== 'bearer' || !token) return null;
  return token;
}
