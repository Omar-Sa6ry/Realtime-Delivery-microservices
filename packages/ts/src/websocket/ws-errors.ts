export enum WsErrorCode {
  UNAUTHENTICATED = "UNAUTHENTICATED",
  FORBIDDEN = "FORBIDDEN",
  INVALID_MESSAGE = "INVALID_MESSAGE",
  INVALID_DELIVERY_ID = "INVALID_DELIVERY_ID",
  NOT_FOUND = "NOT_FOUND",
  RATE_LIMITED = "RATE_LIMITED",
  TOO_LARGE = "TOO_LARGE",
  STALE_COMMAND = "STALE_COMMAND",
  SERVICE_UNAVAILABLE = "SERVICE_UNAVAILABLE",
  INTERNAL_ERROR = "INTERNAL_ERROR",
}

export interface WsErrorPayload {
  code: WsErrorCode;
  message: string;
  retryable: boolean;
  requestId?: string;
}

export class WsException extends Error {
  constructor(
    public readonly code: WsErrorCode,
    message?: string,
    public readonly retryable = false,
  ) {
    super(message || code);
    this.name = "WsException";
  }

  toPayload(requestId?: string): WsErrorPayload {
    return {
      code: this.code,
      message: this.message,
      retryable: this.retryable,
      requestId,
    };
  }
}

export enum WsCloseCode {
  UNAUTHORIZED = 4401,
  FORBIDDEN = 4403,
  RATE_LIMITED = 4408,
  PAYLOAD_TOO_LARGE = 4413,
  INTERNAL_ERROR = 4500,
}