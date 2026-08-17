import { Logger } from "@nestjs/common";
import { WsAdapter } from "@nestjs/platform-ws";
import { CLOSE_EVENT } from "@nestjs/websockets/constants";
import { isNil } from "@nestjs/common/utils/shared.utils";
import {
  from,
  fromEvent,
  Observable,
  ObservableInput,
  of,
} from "rxjs";
import { filter, first, mergeMap, share, takeUntil } from "rxjs/operators";
import { WsErrorCode, WsException } from "./ws-errors";


export class RealtimeWsAdapter extends WsAdapter {
  public readonly logger = new Logger(RealtimeWsAdapter.name);
  private readonly maxPayload: number;

  constructor(appOrHttpServer: any, options?: { maxPayload?: number }) {
    super(appOrHttpServer);
    this.maxPayload =
      options?.maxPayload || Number(process.env.WS_MAX_PAYLOAD) || 16384;
  }

  create(port: number, options: Record<string, any>) {
    return super.create(port, {
      ...options,
      maxPayload: this.maxPayload,
      perMessageDeflate: false,
    });
  }

  bindMessageHandlers(client: any, handlers: any[], transform: any) {
    const handlersMap = new Map(
      handlers.map((handler) => [handler.message, handler]),
    );
    const close$ = fromEvent(client, CLOSE_EVENT).pipe(share(), first());

    const source$ = fromEvent(client, "message").pipe(
      mergeMap((buffer: any) =>
        this.routeMessage(buffer, handlersMap, transform, client).pipe(
          filter((result) => !isNil(result)),
        ),
      ),
      takeUntil(close$),
    );

    source$.subscribe({
      next: (response: any) => {
        if (client.readyState === client.OPEN) {
          client.send(JSON.stringify(response));
        }
      },
      error: (err: unknown) => this.dispatchError(client, err),
    });
  }

  private routeMessage(
    buffer: any,
    handlersMap: Map<string, any>,
    transform: any,
    client: any,
  ) {
    const raw = Array.isArray(buffer) ? buffer[0] : buffer?.data ?? buffer;
    let message: any;
    try {
      message = JSON.parse(raw.toString());
    } catch {
      return of(
        this.errorEnvelope(WsErrorCode.INVALID_MESSAGE, "Invalid JSON message"),
      );
    }

    if (!message || typeof message !== "object") {
      return of(
        this.errorEnvelope(WsErrorCode.INVALID_MESSAGE, "Message must be an object"),
      );
    }

    const type = message.type ?? message.event;
    if (!type) {
      return of(
        this.errorEnvelope(WsErrorCode.INVALID_MESSAGE, "Missing message type"),
      );
    }

    const handler = handlersMap.get(type);
    if (!handler) {
      this.logger.warn(`Unknown WebSocket message type: ${type}`);
      return of(
        this.errorEnvelope(
          WsErrorCode.INVALID_MESSAGE,
          `Unknown message type: ${type}`,
          message.requestId,
        ),
      );
    }

    try {
      // Nest's WsParamsFactory reads args[0] (socket) and args[1] (payload).
      // The handlers passed by WebSocketsController are already bound with the
      // client (`callback.bind(instance, client)`), so calling with (message)
      // yields args = [client, message] for the param factories.
      const result = handler.callback(message);
      return from(result as ObservableInput<unknown>).pipe(
        mergeMap((value) =>
          value instanceof Observable ? value : of(value),
        ),
      );
    } catch (err) {
      const error = this.normalizeError(err);
      if (error) {
        return of(
          this.errorEnvelope(
            error.code,
            error.message,
            message.requestId,
            error.retryable,
          ),
        );
      }
      return of(undefined);
    }
  }

  private dispatchError(client: any, err: unknown): void {
    if (client.readyState !== client.OPEN) return;
    const error = this.normalizeError(err);
    const envelope = error
      ? this.errorEnvelope(error.code, error.message, undefined, error.retryable)
      : this.errorEnvelope(WsErrorCode.INTERNAL_ERROR, "Internal server error", undefined, true);

    try {
      client.send(JSON.stringify(envelope));
    } catch (sendErr) {
      this.logger.warn(`Failed to send error envelope: ${sendErr.message}`);
    }
  }

  private normalizeError(err: unknown): WsException | null {
    if (err instanceof WsException) return err;
    this.logger.error(
      `Unhandled WS handler error: ${(err as any)?.stack || err}`,
      (err as any)?.stack,
    );
    return null;
  }

  private errorEnvelope(
    code: WsErrorCode,
    message: string,
    requestId?: string,
    retryable = false,
  ) {
    return {
      type: "ERROR",
      timestamp: new Date().toISOString(),
      requestId,
      data: { code, message, retryable },
    };
  }
}
