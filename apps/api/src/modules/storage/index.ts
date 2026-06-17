export { getStorageConfig, createObjectStorageProvider, type StorageConfig } from './config.js';
export { StorageError, isStorageError, type StorageErrorCode } from './errors.js';
export { LocalObjectStorageProvider, type LocalObjectStorageOptions } from './local-provider.js';
export {
  createStorageProxyToken,
  getObjectForProxy,
  verifyStorageProxyToken,
  type ProxyTokenClaims,
} from './proxy.js';
export {
  S3CompatibleObjectStorageProvider,
  type S3CompatibleObjectStorageOptions,
} from './s3-provider.js';
export type {
  GetObjectResult,
  ObjectLocation,
  ObjectStorageProvider,
  PutObjectInput,
  PutObjectResult,
  SignedAccessOptions,
  SignedAccessResult,
} from './types.js';
