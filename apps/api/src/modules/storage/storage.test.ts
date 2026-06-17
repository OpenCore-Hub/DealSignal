import assert from 'node:assert/strict';
import { createHash, randomUUID } from 'node:crypto';
import { mkdtemp, rm, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { Readable } from 'node:stream';
import { StorageError } from './errors.js';
import { LocalObjectStorageProvider } from './local-provider.js';
import { createStorageProxyToken, getObjectForProxy, verifyStorageProxyToken } from './proxy.js';

async function streamToBuffer(stream: Readable): Promise<Buffer> {
  const chunks: Buffer[] = [];
  for await (const chunk of stream) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk as Uint8Array));
  }
  return Buffer.concat(chunks);
}

function sha256(value: string | Buffer): string {
  return createHash('sha256').update(value).digest('hex');
}

test('local provider stores private objects by bucket/key with checksum metadata', async () => {
  const rootDir = await mkdtemp(join(tmpdir(), 'dealsignal-storage-'));
  test.after(async () => {
    await rm(rootDir, { recursive: true, force: true });
  });

  const provider = new LocalObjectStorageProvider({ rootDir });
  const bucket = 'dealsignal-private';
  const key = `documents/${randomUUID()}/source.pdf`;
  const body = Buffer.from('private deck bytes');

  const putResult = await provider.putObject({
    bucket,
    key,
    body,
    contentType: 'application/pdf',
  });

  assert.equal(putResult.bucket, bucket);
  assert.equal(putResult.key, key);
  assert.equal(putResult.checksumSha256, sha256(body));
  assert.equal(putResult.sizeBytes, body.length);
  assert.equal('url' in putResult, false);

  const object = await provider.getStream({ bucket, key });
  assert.equal(object.bucket, bucket);
  assert.equal(object.key, key);
  assert.equal(object.contentType, 'application/pdf');
  assert.equal(object.contentLength, body.length);
  assert.equal(object.checksumSha256, sha256(body));
  assert.deepEqual(await streamToBuffer(object.stream), body);

  await assert.rejects(access(join(rootDir, bucket, key)));
});

test('local provider rejects checksum mismatches with actionable storage error', async () => {
  const rootDir = await mkdtemp(join(tmpdir(), 'dealsignal-storage-'));
  test.after(async () => {
    await rm(rootDir, { recursive: true, force: true });
  });

  const provider = new LocalObjectStorageProvider({ rootDir });

  await assert.rejects(
    provider.putObject({
      bucket: 'dealsignal-private',
      key: 'documents/mismatch/source.pdf',
      body: 'changed bytes',
      checksumSha256: sha256('original bytes'),
    }),
    (error: unknown) => {
      assert.ok(error instanceof StorageError);
      assert.equal(error.code, 'WRITE_FAILED');
      assert.match(error.action, /Retry the upload/);
      assert.equal(error.retryable, true);
      return true;
    }
  );
});

test('local provider deletes objects and reports missing reads as actionable errors', async () => {
  const rootDir = await mkdtemp(join(tmpdir(), 'dealsignal-storage-'));
  test.after(async () => {
    await rm(rootDir, { recursive: true, force: true });
  });

  const provider = new LocalObjectStorageProvider({ rootDir });
  const location = { bucket: 'dealsignal-private', key: 'documents/delete-me/source.pdf' };
  await provider.putObject({ ...location, body: Readable.from(['temporary bytes']) });
  await provider.deleteObject(location);

  await assert.rejects(provider.getStream(location), (error: unknown) => {
    assert.ok(error instanceof StorageError);
    assert.equal(error.code, 'NOT_FOUND');
    assert.equal(error.statusCode, 404);
    assert.match(error.action, /Verify the bucket\/key pair/);
    return true;
  });
});

test('storage proxy token gates provider reads and rejects tampered tokens', async () => {
  process.env.STORAGE_PROXY_SECRET = 'storage-proxy-test-secret';
  const rootDir = await mkdtemp(join(tmpdir(), 'dealsignal-storage-'));
  test.after(async () => {
    delete process.env.STORAGE_PROXY_SECRET;
    await rm(rootDir, { recursive: true, force: true });
  });

  const provider = new LocalObjectStorageProvider({ rootDir });
  const location = { bucket: 'dealsignal-private', key: 'documents/proxy/source.pdf' };
  await provider.putObject({ ...location, body: 'proxied bytes' });

  const token = createStorageProxyToken(location);
  assert.deepEqual(verifyStorageProxyToken(token), {
    ...location,
    exp: verifyStorageProxyToken(token)?.exp,
  });
  assert.equal(token.includes(location.key), false);
  assert.equal(token.includes(location.bucket), false);

  const object = await getObjectForProxy(token, provider);
  assert.deepEqual(await streamToBuffer(object.stream), Buffer.from('proxied bytes'));
  assert.equal(verifyStorageProxyToken(`${token}tampered`), null);
});
