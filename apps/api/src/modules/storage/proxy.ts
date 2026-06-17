import {
  createCipheriv,
  createDecipheriv,
  createHash,
  createHmac,
  randomBytes,
  timingSafeEqual,
} from 'node:crypto';
import { StorageError } from './errors.js';
import type { GetObjectResult, ObjectLocation, ObjectStorageProvider } from './types.js';

export type ProxyTokenClaims = ObjectLocation & {
  exp: number;
};

const TOKEN_TTL_SECONDS = 60 * 5;
const DEVELOPMENT_PROXY_SECRET = 'development-storage-proxy-secret-change-me';

function getProxySecret(): string {
  const secret = process.env.STORAGE_PROXY_SECRET ?? process.env.SESSION_SECRET;
  if (secret) return secret;

  if (process.env.NODE_ENV === 'production') {
    throw new StorageError(
      'CONFIGURATION_ERROR',
      'STORAGE_PROXY_SECRET must be set in production.',
      {
        action: 'Set STORAGE_PROXY_SECRET to a strong random value.',
      }
    );
  }

  return DEVELOPMENT_PROXY_SECRET;
}

function encryptionKey(): Buffer {
  return createHash('sha256').update(getProxySecret()).digest();
}

function encodeEncrypted(value: unknown): string {
  const iv = randomBytes(12);
  const cipher = createCipheriv('aes-256-gcm', encryptionKey(), iv);
  const encrypted = Buffer.concat([cipher.update(JSON.stringify(value), 'utf8'), cipher.final()]);
  const tag = cipher.getAuthTag();
  return `${iv.toString('base64url')}.${encrypted.toString('base64url')}.${tag.toString('base64url')}`;
}

function decodeEncrypted<T>(value: string): T {
  const [iv, encrypted, tag, extra] = value.split('.');
  if (!iv || !encrypted || !tag || extra) throw new Error('Invalid encrypted storage token.');

  const decipher = createDecipheriv('aes-256-gcm', encryptionKey(), Buffer.from(iv, 'base64url'));
  decipher.setAuthTag(Buffer.from(tag, 'base64url'));
  const decrypted = Buffer.concat([
    decipher.update(Buffer.from(encrypted, 'base64url')),
    decipher.final(),
  ]);
  return JSON.parse(decrypted.toString('utf8')) as T;
}

function sign(payload: string): string {
  return createHmac('sha256', getProxySecret()).update(payload).digest('base64url');
}

export function createStorageProxyToken(
  location: ObjectLocation,
  ttlSeconds = TOKEN_TTL_SECONDS
): string {
  const claims = encodeEncrypted({ ...location, exp: Math.floor(Date.now() / 1000) + ttlSeconds });
  return `${claims}.${sign(claims)}`;
}

export function verifyStorageProxyToken(token: string): ProxyTokenClaims | null {
  const parts = token.split('.');
  if (parts.length !== 4) return null;
  const payload = parts.slice(0, 3).join('.');
  const signature = parts[3];
  if (!signature) return null;

  try {
    const expected = sign(payload);
    const actualBuffer = Buffer.from(signature, 'base64url');
    const expectedBuffer = Buffer.from(expected, 'base64url');
    if (actualBuffer.length !== expectedBuffer.length) return null;
    if (!timingSafeEqual(actualBuffer, expectedBuffer)) return null;

    const claims = decodeEncrypted<ProxyTokenClaims>(payload);
    if (!claims.bucket || !claims.key || !claims.exp) return null;
    if (claims.exp < Math.floor(Date.now() / 1000)) return null;
    return claims;
  } catch {
    return null;
  }
}

export async function getObjectForProxy(
  token: string,
  storageProvider: ObjectStorageProvider
): Promise<GetObjectResult> {
  const claims = verifyStorageProxyToken(token);
  if (!claims) {
    throw new StorageError('INVALID_LOCATION', 'Storage proxy token is invalid or expired.', {
      action: 'Request a new storage proxy token after access control passes.',
      statusCode: 401,
    });
  }

  return await storageProvider.getStream({ bucket: claims.bucket, key: claims.key });
}
