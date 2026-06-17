export type StorageErrorCode =
  | 'CONFIGURATION_ERROR'
  | 'INVALID_LOCATION'
  | 'NOT_FOUND'
  | 'READ_FAILED'
  | 'WRITE_FAILED'
  | 'DELETE_FAILED'
  | 'PROVIDER_ERROR';

export type StorageErrorOptions = {
  action: string;
  retryable?: boolean;
  statusCode?: number;
  cause?: unknown;
};

export class StorageError extends Error {
  readonly code: StorageErrorCode;
  readonly action: string;
  readonly retryable: boolean;
  readonly statusCode?: number;

  constructor(code: StorageErrorCode, message: string, options: StorageErrorOptions) {
    super(message);
    this.name = 'StorageError';
    this.code = code;
    this.action = options.action;
    this.retryable = options.retryable ?? false;
    this.statusCode = options.statusCode;
    if (options.cause !== undefined) this.cause = options.cause;
  }

  toResponse() {
    return {
      code: this.code,
      message: this.message,
      action: this.action,
      retryable: this.retryable,
    };
  }
}

export function isStorageError(error: unknown): error is StorageError {
  return error instanceof StorageError;
}
