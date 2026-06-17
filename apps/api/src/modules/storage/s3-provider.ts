import { createHmac, createHash } from 'node:crypto';
import { Readable } from 'node:stream';
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

export type S3CompatibleObjectStorageOptions = {
  endpoint: string;
  region: string;
  accessKeyId: string;
  secretAccessKey: string;
  forcePathStyle?: boolean;
};

type RequestMethod = 'GET' | 'PUT' | 'DELETE';

type SignedRequest = {
  url: URL;
  headers: Record<string, string>;
};

const service = 's3';
const unsignedPayload = 'UNSIGNED-PAYLOAD';

function hashHex(value: string | Buffer): string {
  return createHash('sha256').update(value).digest('hex');
}

function hmac(key: Buffer | string, value: string): Buffer {
  return createHmac('sha256', key).update(value).digest();
}

function hmacHex(key: Buffer | string, value: string): string {
  return createHmac('sha256', key).update(value).digest('hex');
}

function formatAmzDate(date: Date): string {
  return date.toISOString().replace(/[:-]|\.\d{3}/g, '');
}

function credentialDate(amzDate: string): string {
  return amzDate.slice(0, 8);
}

function encodePathSegment(value: string): string {
  return encodeURIComponent(value).replace(
    /[!'()*]/g,
    (char) => `%${char.charCodeAt(0).toString(16).toUpperCase()}`
  );
}

function canonicalUri(bucket: string, key: string, forcePathStyle: boolean): string {
  const encodedKey = key.split('/').map(encodePathSegment).join('/');
  if (forcePathStyle) return `/${encodePathSegment(bucket)}/${encodedKey}`;
  return `/${encodedKey}`;
}

function canonicalQuery(params: URLSearchParams): string {
  return [...params.entries()]
    .sort(([leftKey, leftValue], [rightKey, rightValue]) =>
      leftKey === rightKey ? leftValue.localeCompare(rightValue) : leftKey.localeCompare(rightKey)
    )
    .map(([key, value]) => `${encodePathSegment(key)}=${encodePathSegment(value)}`)
    .join('&');
}

function canonicalHeaders(headers: Record<string, string>, signedHeaders: string[]): string {
  return signedHeaders
    .map((header) => `${header}:${headers[header].trim().replace(/\s+/g, ' ')}`)
    .join('\n');
}

function signingKey(secretAccessKey: string, date: string, region: string): Buffer {
  const dateKey = hmac(`AWS4${secretAccessKey}`, date);
  const regionKey = hmac(dateKey, region);
  const serviceKey = hmac(regionKey, service);
  return hmac(serviceKey, 'aws4_request');
}

function bufferToArrayBuffer(buffer: Buffer): ArrayBuffer {
  // fetch BodyInit types accept ArrayBuffer more reliably than Buffer/Uint8Array
  // across the current DOM lib combination. This copies the buffer; replace with
  // streaming upload once the interface supports Readable bodies without buffering.
  const arrayBuffer = new ArrayBuffer(buffer.byteLength);
  new Uint8Array(arrayBuffer).set(buffer);
  return arrayBuffer;
}

function extractResponseHeaders(response: Response) {
  const contentLength = response.headers.get('content-length');
  const checksumSha256 = response.headers.get('x-amz-meta-checksum-sha256') ?? undefined;

  return {
    contentType: response.headers.get('content-type') ?? undefined,
    contentLength: contentLength ? Number(contentLength) : undefined,
    checksumSha256,
  };
}


async function storageErrorFromResponse(
  response: Response,
  code: 'NOT_FOUND' | 'READ_FAILED' | 'WRITE_FAILED' | 'DELETE_FAILED',
  message: string,
  action: string
): Promise<StorageError> {
  const responseBody = await response.text().catch(() => '');
  return new StorageError(code, message, {
    action: responseBody ? `${action} Provider response: ${responseBody}` : action,
    retryable: response.status >= 500,
    statusCode: response.status,
  });
}

export class S3CompatibleObjectStorageProvider implements ObjectStorageProvider {
  readonly #endpoint: URL;
  readonly #region: string;
  readonly #accessKeyId: string;
  readonly #secretAccessKey: string;
  readonly #forcePathStyle: boolean;

  constructor(options: S3CompatibleObjectStorageOptions) {
    if (!options.endpoint || !options.region || !options.accessKeyId || !options.secretAccessKey) {
      throw new StorageError(
        'CONFIGURATION_ERROR',
        'S3-compatible storage is not fully configured.',
        {
          action:
            'Set STORAGE_ENDPOINT, STORAGE_REGION, STORAGE_ACCESS_KEY_ID, and STORAGE_SECRET_ACCESS_KEY.',
        }
      );
    }

    this.#endpoint = new URL(options.endpoint);
    this.#region = options.region;
    this.#accessKeyId = options.accessKeyId;
    this.#secretAccessKey = options.secretAccessKey;
    this.#forcePathStyle = options.forcePathStyle ?? true;
  }

  async putObject(input: PutObjectInput): Promise<PutObjectResult> {
    validateObjectLocation(input.bucket, input.key);
    const collected = await collectBody(input.body);
    verifyChecksum(input.checksumSha256, collected.checksumSha256);

    const headers: Record<string, string> = {
      'content-length': String(collected.sizeBytes),
      'content-type': input.contentType ?? 'application/octet-stream',
      'x-amz-meta-checksum-sha256': collected.checksumSha256,
    };

    for (const [key, value] of Object.entries(input.metadata ?? {})) {
      headers[`x-amz-meta-${key.toLowerCase()}`] = value;
    }

    const signed = this.#signRequest('PUT', input, collected.buffer, headers);
    const response = await fetch(signed.url, {
      method: 'PUT',
      headers: signed.headers,
      body: bufferToArrayBuffer(collected.buffer),
    });

    if (!response.ok) {
      throw await storageErrorFromResponse(
        response,
        'WRITE_FAILED',
        'Failed to store object bytes.',
        'Verify object storage credentials, bucket permissions, and object key.'
      );
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

    const signed = this.#signRequest('GET', location);
    const response = await fetch(signed.url, { method: 'GET', headers: signed.headers });

    if (!response.ok || !response.body) {
      throw await storageErrorFromResponse(
        response,
        response.status === 404 ? 'NOT_FOUND' : 'READ_FAILED',
        response.status === 404
          ? 'Storage object was not found.'
          : 'Failed to retrieve object bytes.',
        'Verify the bucket/key pair and storage permissions.'
      );
    }

    return {
      ...location,
      stream: Readable.fromWeb(response.body as import('node:stream/web').ReadableStream),
      ...extractResponseHeaders(response),
    };
  }

  async deleteObject(location: ObjectLocation): Promise<void> {
    validateObjectLocation(location.bucket, location.key);

    const signed = this.#signRequest('DELETE', location);
    const response = await fetch(signed.url, { method: 'DELETE', headers: signed.headers });

    if (!response.ok && response.status !== 404) {
      throw await storageErrorFromResponse(
        response,
        'DELETE_FAILED',
        'Failed to delete storage object.',
        'Verify object storage credentials, bucket permissions, and object key.'
      );
    }
  }

  async createSignedAccess(
    location: ObjectLocation,
    options: SignedAccessOptions = {}
  ): Promise<SignedAccessResult> {
    validateObjectLocation(location.bucket, location.key);

    const method = options.method ?? 'GET';
    const expiresInSeconds = options.expiresInSeconds ?? 300;
    if (expiresInSeconds < 1 || expiresInSeconds > 604800) {
      throw new StorageError('INVALID_LOCATION', 'Signed access expiration is invalid.', {
        action: 'Choose an expiration between 1 second and 7 days.',
      });
    }

    const now = new Date();
    const amzDate = formatAmzDate(now);
    const shortDate = credentialDate(amzDate);
    const scope = `${shortDate}/${this.#region}/${service}/aws4_request`;
    const signedHeaders = 'host';
    const url = this.#urlFor(location);
    url.searchParams.set('X-Amz-Algorithm', 'AWS4-HMAC-SHA256');
    url.searchParams.set('X-Amz-Credential', `${this.#accessKeyId}/${scope}`);
    url.searchParams.set('X-Amz-Date', amzDate);
    url.searchParams.set('X-Amz-Expires', String(expiresInSeconds));
    url.searchParams.set('X-Amz-SignedHeaders', signedHeaders);

    const canonicalRequest = [
      method,
      canonicalUri(location.bucket, location.key, this.#forcePathStyle),
      canonicalQuery(url.searchParams),
      `host:${url.host}\n`,
      signedHeaders,
      unsignedPayload,
    ].join('\n');
    const stringToSign = ['AWS4-HMAC-SHA256', amzDate, scope, hashHex(canonicalRequest)].join('\n');
    const signature = hmacHex(
      signingKey(this.#secretAccessKey, shortDate, this.#region),
      stringToSign
    );
    url.searchParams.set('X-Amz-Signature', signature);

    return {
      ...location,
      url: url.toString(),
      expiresAt: new Date(now.getTime() + expiresInSeconds * 1000),
      method,
    };
  }

  #urlFor(location: ObjectLocation): URL {
    const url = new URL(this.#endpoint);
    if (this.#forcePathStyle) {
      url.pathname = canonicalUri(location.bucket, location.key, true);
      return url;
    }

    url.hostname = `${location.bucket}.${url.hostname}`;
    url.pathname = canonicalUri(location.bucket, location.key, false);
    return url;
  }

  #signRequest(
    method: RequestMethod,
    location: ObjectLocation,
    body: Buffer = Buffer.alloc(0),
    inputHeaders: Record<string, string> = {}
  ): SignedRequest {
    const url = this.#urlFor(location);
    const amzDate = formatAmzDate(new Date());
    const shortDate = credentialDate(amzDate);
    const payloadHash = hashHex(body);
    const headers: Record<string, string> = {
      host: url.host,
      'x-amz-content-sha256': payloadHash,
      'x-amz-date': amzDate,
      ...Object.fromEntries(
        Object.entries(inputHeaders).map(([key, value]) => [key.toLowerCase(), value])
      ),
    };
    const signedHeaders = Object.keys(headers).sort();
    const scope = `${shortDate}/${this.#region}/${service}/aws4_request`;
    const canonicalRequest = [
      method,
      canonicalUri(location.bucket, location.key, this.#forcePathStyle),
      canonicalQuery(url.searchParams),
      `${canonicalHeaders(headers, signedHeaders)}\n`,
      signedHeaders.join(';'),
      payloadHash,
    ].join('\n');
    const stringToSign = ['AWS4-HMAC-SHA256', amzDate, scope, hashHex(canonicalRequest)].join('\n');
    const signature = hmacHex(
      signingKey(this.#secretAccessKey, shortDate, this.#region),
      stringToSign
    );

    return {
      url,
      headers: {
        ...headers,
        authorization: `AWS4-HMAC-SHA256 Credential=${this.#accessKeyId}/${scope}, SignedHeaders=${signedHeaders.join(
          ';'
        )}, Signature=${signature}`,
      },
    };
  }
}
