export type ApiErrorKind =
  | 'unauthorized'
  | 'forbidden'
  | 'validation'
  | 'server'
  | 'network';

export type ApiErrorDetails = {
	status?: number;
	code?: number;
	machineCode?: string;
	cause?: unknown;
};

export class ApiError extends Error {
	readonly kind: ApiErrorKind;
	readonly status?: number;
	readonly code?: number;
	readonly machineCode?: string;

  constructor(kind: ApiErrorKind, message: string, details: ApiErrorDetails = {}) {
    super(message, details.cause === undefined ? undefined : { cause: details.cause });
    this.name = 'ApiError';
    this.kind = kind;
    if (details.status !== undefined) {
      this.status = details.status;
    }
    if (details.code !== undefined) {
      this.code = details.code;
    }
    if (details.machineCode !== undefined) {
      this.machineCode = details.machineCode;
    }
  }
}
