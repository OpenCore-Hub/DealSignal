import type { Readable } from 'node:stream';

export type ObjectLocation = {
  bucket: string;
  key: string;
};

export type PutObjectInput = ObjectLocation & {
  body: Buffer | Uint8Array | string | Readable;
  contentType?: string;
  contentLength?: number;
  checksumSha256?: string;
  metadata?: Record<string, string>;
};

export type PutObjectResult = ObjectLocation & {
  checksumSha256: string;
  sizeBytes: number;
};

export type GetObjectResult = ObjectLocation & {
  stream: Readable;
  contentType?: string;
  contentLength?: number;
  checksumSha256?: string;
  metadata?: Record<string, string>;
};

export type SignedAccessOptions = {
  expiresInSeconds?: number;
  method?: 'GET' | 'PUT';
};

export type SignedAccessResult = ObjectLocation & {
  url: string;
  expiresAt: Date;
  method: 'GET' | 'PUT';
};

export interface ObjectStorageProvider {
  putObject(input: PutObjectInput): Promise<PutObjectResult>;
  getStream(location: ObjectLocation): Promise<GetObjectResult>;
  deleteObject(location: ObjectLocation): Promise<void>;
  createSignedAccess(
    location: ObjectLocation,
    options?: SignedAccessOptions
  ): Promise<SignedAccessResult>;
}
