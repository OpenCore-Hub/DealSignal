import { and, eq } from 'drizzle-orm';
import { db as defaultDb, type Database } from '../../db/index.js';
import { documentVersions } from '../../db/schema.js';

export type StoredObjectRecord = {
  id: string;
  workspaceId: string;
  bucket: string;
  key: string;
  checksumSha256: string | null;
};

export interface StorageMetadataStore {
  getWorkspaceObject(
    workspaceId: string,
    bucket: string,
    key: string
  ): Promise<StoredObjectRecord | null>;
}

export function createDrizzleStorageMetadataStore(
  database: Database = defaultDb
): StorageMetadataStore {
  return {
    async getWorkspaceObject(workspaceId, bucket, key) {
      const [version] = await database
        .select({
          id: documentVersions.id,
          workspaceId: documentVersions.workspaceId,
          bucket: documentVersions.storageBucket,
          key: documentVersions.storageKey,
          checksumSha256: documentVersions.checksumSha256,
        })
        .from(documentVersions)
        .where(
          and(
            eq(documentVersions.workspaceId, workspaceId),
            eq(documentVersions.storageBucket, bucket),
            eq(documentVersions.storageKey, key)
          )
        );

      return version ?? null;
    },
  };
}
