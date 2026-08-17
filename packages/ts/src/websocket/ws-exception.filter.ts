import { ArgumentsHost, Catch, Logger } from '@nestjs/common';
import { BaseWsExceptionFilter } from '@nestjs/websockets';
import { ServerMessageType } from './realtime-message';
import { WsCloseCode, WsErrorCode, WsException } from './ws-errors';

/**
 * Converts WsException thrown by guards / services into the wire-protocol
 * ERROR envelope and closes the socket for terminal error codes.
 */
@Catch(WsException)
export class WsExceptionFilter extends BaseWsExceptionFilter {
  private readonly logger = new Logger(WsExceptionFilter.name);

  catch(exception: WsException, host: ArgumentsHost) {
    const ctx = host.switchToWs();
    const client = ctx.getClient();
    const requestId = (ctx.getData() as { requestId?: string } | undefined)?.requestId;

    if (!client || typeof client.send !== 'function') {
      return super.catch(exception, host);
    }

    try {
      client.send(
        JSON.stringify({
          type: ServerMessageType.ERROR,
          timestamp: new Date().toISOString(),
          requestId,
          data: exception.toPayload(requestId),
        }),
      );
    } catch (err) {
      this.logger.warn(`Failed to send WsException envelope: ${(err as Error).message}`);
      return super.catch(exception, host);
    }

    switch (exception.code) {
      case WsErrorCode.UNAUTHENTICATED:
        client.close?.(WsCloseCode.UNAUTHORIZED, exception.code);
        break;
      case WsErrorCode.FORBIDDEN:
        client.close?.(WsCloseCode.FORBIDDEN, exception.code);
        break;
      case WsErrorCode.RATE_LIMITED:
        client.close?.(WsCloseCode.RATE_LIMITED, exception.code);
        break;
      case WsErrorCode.TOO_LARGE:
        client.close?.(WsCloseCode.PAYLOAD_TOO_LARGE, exception.code);
        break;
      default:
        break;
    }
  }
}