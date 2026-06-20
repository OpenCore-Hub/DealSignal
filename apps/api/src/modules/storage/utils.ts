import { createHash } from 'node:crypto';
import { Readable } from 'node:stream';
import { StorageError } from './errors.js';

const bucketPattern = /^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$/;
const keyPattern = /^(?!\/)(?!.*\.\.)(?!.*\/\/)[\x20-\x7e]{1,1024}$/;

export function validateObjectLocation(bucket: string, key: string): void {
  if (!bucketPattern.test(bucket)) {
    throw new StorageError('INVALID_LOCATION', 'Storage bucket name is invalid.', {
      action: 'Use a lowercase S3/R2 bucket name and retry.',
    });
  }

  if (!keyPattern.test(key)) {
    throw new StorageError('INVALID_LOCATION', 'Storage object key is invalid.', {
      action: 'Use a non-empty relative object key without traversal segments.',
    });
  }
}

export function createSha256Hash(): ReturnType<typeof createHash> {
  return createHash('sha256');
}

export function normalizeChecksum(checksum: string): string {
  return checksum.trim().toLowerCase();
}

export function verifyChecksum(expected: string | undefined, actual: string): void {
  if (!expected) return;
  if (normalizeChecksum(expected) === normalizeChecksum(actual)) return;

  throw new StorageError('WRITE_FAILED', 'Uploaded object checksum did not match.', {
    action: 'Retry the upload with the original file bytes.',
    retryable: true,
  });
}

export function bufferToStream(buffer: Buffer): Readable {
  return Readable.from(buffer);
}

export async function collectBody(
  body: Buffer | Uint8Array | string | Readable
): Promise<{ buffer: Buffer; checksumSha256: string; sizeBytes: number }> {
  const chunks: Buffer[] = [];
  const hash = createSha256Hash();
  let sizeBytes = 0;

  if (typeof body === 'string' || body instanceof Uint8Array) {
    const buffer = Buffer.from(body);
    hash.update(buffer);
    return {
      buffer,
      checksumSha256: hash.digest('hex'),
      sizeBytes: buffer.length,
    };
  }

  try {
    for await (const chunk of body) {
      const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk as Uint8Array);
      chunks.push(buffer);
      hash.update(buffer);
      sizeBytes += buffer.length;
    }
  } catch (error) {
    throw new StorageError('READ_FAILED', 'Failed to read object bytes.', {
      action: 'Retry the upload or inspect the source stream.',
      retryable: true,
      cause: error,
    });
  }

  return {
    buffer: Buffer.concat(chunks, sizeBytes),
    checksumSha256: hash.digest('hex'),
    sizeBytes,
  };
}
