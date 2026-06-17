import type { FastifyInstance } from 'fastify';
import {
  requireAuthenticatedRequest,
  sendForbidden,
  sendUnauthorized,
} from '../auth/current-user.js';
import { createDrizzleAuthStore, type AuthStore } from '../auth/store.js';
import { createObjectStorageProvider, getStorageConfig } from './config.js';
import { isStorageError } from './errors.js';
import { createStorageProxyToken, getObjectForProxy } from './proxy.js';
import { createDrizzleStorageMetadataStore, type StorageMetadataStore } from './store.js';
import type { ObjectLocation, ObjectStorageProvider } from './types.js';

const defaultAuthStore = createDrizzleAuthStore();
const defaultMetadataStore = createDrizzleStorageMetadataStore();
const defaultStorageProvider = createObjectStorageProvider();

type ObjectParams = {
  bucket?: string;
  key?: string;
  '*'?: string;
};

type ProxyParams = {
  token?: string;
  '*'?: string;
};

export type StorageRoutesOptions = {
  authStore?: AuthStore;
  storageMetadataStore?: StorageMetadataStore;
  storageProvider?: ObjectStorageProvider;
};

function parseObjectParams(params: ObjectParams): ObjectLocation | null {
  const key = params.key ?? params['*'];
  if (!params.bucket || !key) return null;
  return { bucket: params.bucket, key };
}

function getErrorStatus(error: unknown): number {
  if (isStorageError(error)) return error.statusCode ?? 500;
  return 500;
}

function getErrorResponse(error: unknown) {
  if (isStorageError(error)) return { error: error.toResponse() };
  return {
    error: {
      code: 'STORAGE_ERROR',
      message: 'Storage request failed.',
      action: 'Retry the request or contact support if the problem continues.',
      retryable: true,
    },
  };
}

export async function registerStorageRoutes(
  app: FastifyInstance,
  options: StorageRoutesOptions = {}
): Promise<void> {
  const authStore = options.authStore ?? defaultAuthStore;
  const metadataStore = options.storageMetadataStore ?? defaultMetadataStore;
  const storageProvider = options.storageProvider ?? defaultStorageProvider;

  app.get('/storage/objects/:bucket/*', async (request, reply) => {
    const claims = await requireAuthenticatedRequest(request, reply, authStore);
    if (!claims) return;

    const location = parseObjectParams(request.params as ObjectParams);
    if (!location) {
      return reply.status(400).send({
        error: {
          code: 'INVALID_LOCATION',
          message: 'Bucket and key are required.',
          action: 'Provide a valid object bucket and key.',
          retryable: false,
        },
      });
    }

    const record = await metadataStore.getWorkspaceObject(
      claims.workspaceId,
      location.bucket,
      location.key
    );
    if (!record) return sendForbidden(reply);

    try {
      const object = await storageProvider.getStream(location);
      reply.header('content-type', object.contentType ?? 'application/octet-stream');
      if (object.contentLength !== undefined) reply.header('content-length', object.contentLength);
      reply.header('x-dealsignal-storage-bucket', object.bucket);
      reply.header('x-dealsignal-storage-key', object.key);
      if (object.checksumSha256)
        reply.header('x-dealsignal-checksum-sha256', object.checksumSha256);
      return reply.send(object.stream);
    } catch (error) {
      request.log.error({ error }, 'storage.proxy_failed');
      return reply.status(getErrorStatus(error)).send(getErrorResponse(error));
    }
  });

  app.post('/storage/object-proxy-tokens/:bucket/*', async (request, reply) => {
    const claims = await requireAuthenticatedRequest(request, reply, authStore);
    if (!claims) return;

    const location = parseObjectParams(request.params as ObjectParams);
    if (!location) {
      return reply.status(400).send({
        error: {
          code: 'INVALID_LOCATION',
          message: 'Bucket and key are required.',
          action: 'Provide a valid object bucket and key.',
          retryable: false,
        },
      });
    }

    const record = await metadataStore.getWorkspaceObject(
      claims.workspaceId,
      location.bucket,
      location.key
    );
    if (!record) return sendForbidden(reply);

    const token = createStorageProxyToken(location);
    return reply.send({ token, expiresInSeconds: 300 });
  });

  app.get('/storage/proxy/*', async (request, reply) => {
    const params = request.params as ProxyParams;
    const token = params.token ?? params['*'];
    if (!token) return sendUnauthorized(reply);

    try {
      const object = await getObjectForProxy(token, storageProvider);
      reply.header('content-type', object.contentType ?? 'application/octet-stream');
      if (object.contentLength !== undefined) reply.header('content-length', object.contentLength);
      return reply.send(object.stream);
    } catch (error) {
      return reply.status(getErrorStatus(error)).send(getErrorResponse(error));
    }
  });

  app.get('/storage/config', async (request, reply) => {
    const claims = await requireAuthenticatedRequest(request, reply, authStore);
    if (!claims) return;

    const config = getStorageConfig();
    return reply.send({ backend: config.backend, bucket: config.bucket });
  });
}
