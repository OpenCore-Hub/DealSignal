import { resolve } from 'node:path';
import { loadEnvFromProjectRoot } from '../../lib/env.js';
import { LocalObjectStorageProvider } from './local-provider.js';
import { S3CompatibleObjectStorageProvider } from './s3-provider.js';
import type { ObjectStorageProvider } from './types.js';

export type StorageBackend = 'local' | 's3';

export type StorageConfig = {
  backend: StorageBackend;
  bucket: string;
  localDir: string;
  endpoint?: string;
  region?: string;
  accessKeyId?: string;
  secretAccessKey?: string;
  forcePathStyle: boolean;
};

const defaultLocalDir = resolve(process.cwd(), '.dealsignal-storage');

function parseBackend(value: string | undefined): StorageBackend {
  return value === 's3' ? 's3' : 'local';
}

function parseBoolean(value: string | undefined, defaultValue: boolean): boolean {
  if (value === undefined) return defaultValue;
  return ['1', 'true', 'yes'].includes(value.toLowerCase());
}

export function getStorageConfig(env: NodeJS.ProcessEnv = process.env): StorageConfig {
  loadEnvFromProjectRoot();

  return {
    backend: parseBackend(env.STORAGE_BACKEND),
    bucket: env.STORAGE_BUCKET ?? 'dealsignal-private',
    localDir: env.STORAGE_LOCAL_DIR ?? defaultLocalDir,
    endpoint: env.STORAGE_ENDPOINT,
    region: env.STORAGE_REGION,
    accessKeyId: env.STORAGE_ACCESS_KEY_ID,
    secretAccessKey: env.STORAGE_SECRET_ACCESS_KEY,
    forcePathStyle: parseBoolean(env.STORAGE_FORCE_PATH_STYLE, true),
  };
}

export function createObjectStorageProvider(config = getStorageConfig()): ObjectStorageProvider {
  if (config.backend === 's3') {
    return new S3CompatibleObjectStorageProvider({
      endpoint: config.endpoint ?? '',
      region: config.region ?? '',
      accessKeyId: config.accessKeyId ?? '',
      secretAccessKey: config.secretAccessKey ?? '',
      forcePathStyle: config.forcePathStyle,
    });
  }

  return new LocalObjectStorageProvider({ rootDir: config.localDir });
}
