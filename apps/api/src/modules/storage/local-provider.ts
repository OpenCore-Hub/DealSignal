import { createHash } from 'node:crypto';
import { constants, createReadStream } from 'node:fs';
import { mkdir, readFile, rm, stat, writeFile, access } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { StorageError } from './errors.js';
import type {
  GetObjectResult,
  ObjectLocation,
  ObjectStorageProvider,
  PutObjectInput,
  PutObjectResult,
  SignedAccessOptions,
  SignedAccessResult,
} from './types.js';
import { collectBody, validateObjectLocation, verifyChecksum } from './utils.js';

type MetadataFile = {
  contentType?: string;
  checksumSha256: string;
  sizeBytes: number;
  metadata?: Record<string, string>;
};

function storagePathId(key: string): string {
  return createHash('sha256').update(key).digest('hex');
}

export type LocalObjectStorageOptions = {
  rootDir: string;
  signingBaseUrl?: string;
};

export class LocalObjectStorageProvider implements ObjectStorageProvider {
  readonly #rootDir: string;
  readonly #signingBaseUrl: string;

  constructor(options: LocalObjectStorageOptions) {
    if (!options.rootDir) {
      throw new StorageError('CONFIGURATION_ERROR', 'Local storage root directory is required.', {
        action: 'Set STORAGE_LOCAL_DIR or pass a local storage root directory.',
      });
    }

    this.#rootDir = resolve(options.rootDir);
    this.#signingBaseUrl = options.signingBaseUrl ?? 'storage://local';
  }

  async putObject(input: PutObjectInput): Promise<PutObjectResult> {
    validateObjectLocation(input.bucket, input.key);

    const objectPath = this.#objectPath(input);
    const metadataPath = this.#metadataPath(input);
    const collected = await collectBody(input.body);
    verifyChecksum(input.checksumSha256, collected.checksumSha256);

    try {
      await Promise.all([
        mkdir(dirname(objectPath), { recursive: true }),
        mkdir(dirname(metadataPath), { recursive: true }),
      ]);
      await writeFile(objectPath, collected.buffer, { mode: 0o600 });
      const metadata: MetadataFile = {
        contentType: input.contentType,
        checksumSha256: collected.checksumSha256,
        sizeBytes: collected.sizeBytes,
        metadata: input.metadata,
      };
      await writeFile(metadataPath, JSON.stringify(metadata), { mode: 0o600 });
    } catch (error) {
      throw new StorageError('WRITE_FAILED', 'Failed to store object bytes.', {
        action: 'Verify local storage permissions and retry the upload.',
        retryable: true,
        cause: error,
      });
    }

    return {
      bucket: input.bucket,
      key: input.key,
      checksumSha256: collected.checksumSha256,
      sizeBytes: collected.sizeBytes,
    };
  }

  async getStream(location: ObjectLocation): Promise<GetObjectResult> {
    validateObjectLocation(location.bucket, location.key);

    const objectPath = this.#objectPath(location);
    const metadata = await this.#readMetadata(location);

    try {
      await access(objectPath, constants.R_OK);
      return {
        ...location,
        stream: createReadStream(objectPath),
        contentType: metadata.contentType,
        contentLength: metadata.sizeBytes,
        checksumSha256: metadata.checksumSha256,
        metadata: metadata.metadata,
      };
    } catch (error) {
      throw new StorageError('NOT_FOUND', 'Storage object was not found.', {
        action: 'Verify the bucket/key pair before requesting the object.',
        statusCode: 404,
        cause: error,
      });
    }
  }

  async deleteObject(location: ObjectLocation): Promise<void> {
    validateObjectLocation(location.bucket, location.key);

    try {
      await rm(this.#objectPath(location), { force: true });
      await rm(this.#metadataPath(location), { force: true });
    } catch (error) {
      throw new StorageError('DELETE_FAILED', 'Failed to delete storage object.', {
        action: 'Retry deletion or inspect storage permissions.',
        retryable: true,
        cause: error,
      });
    }
  }

  async createSignedAccess(
    location: ObjectLocation,
    options: SignedAccessOptions = {}
  ): Promise<SignedAccessResult> {
    validateObjectLocation(location.bucket, location.key);

    const expiresInSeconds = options.expiresInSeconds ?? 300;
    const expiresAt = new Date(Date.now() + expiresInSeconds * 1000);
    const method = options.method ?? 'GET';
    const url = new URL(this.#signingBaseUrl);
    url.pathname = `/${location.bucket}/${location.key}`;
    url.searchParams.set('method', method);
    url.searchParams.set('expiresAt', expiresAt.toISOString());

    return { ...location, url: url.toString(), expiresAt, method };
  }

  #objectPath(location: ObjectLocation): string {
    const pathId = storagePathId(location.key);
    return resolve(this.#rootDir, 'objects', location.bucket, pathId);
  }

  #metadataPath(location: ObjectLocation): string {
    const pathId = storagePathId(location.key);
    return resolve(this.#rootDir, 'metadata', location.bucket, `${pathId}.json`);
  }

  async #readMetadata(location: ObjectLocation): Promise<MetadataFile> {
    try {
      const metadataPath = this.#metadataPath(location);
      await stat(metadataPath);
      return JSON.parse(await readFile(metadataPath, 'utf8')) as MetadataFile;
    } catch {
      throw new StorageError('NOT_FOUND', 'Storage object metadata was not found.', {
        action: 'Verify the bucket/key pair before requesting the object.',
        statusCode: 404,
      });
    }
  }
}
