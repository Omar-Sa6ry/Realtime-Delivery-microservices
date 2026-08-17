import {
  ClientMessageType,
  ClientMessage,
  ServerMessage,
  MessagePriority,
  WsErrorCode,
  WsException,
} from '@delivery/common';

export interface OutboundMessage<T = unknown> extends ServerMessage<T> {
  timestamp: string;
  priority?: MessagePriority;
}

export const buildServerMessage = <T>(
  type: ServerMessage['type'],
  data: T,
  requestId?: string,
  priority?: MessagePriority,
): OutboundMessage<T> => ({
  requestId,
  type,
  data,
  timestamp: new Date().toISOString(),
  priority,
});

/** Per-message-type shape validation (used by the guard chain). */
export const VALIDATION: Partial<
  Record<ClientMessageType, (data: any) => void>
> = {
  [ClientMessageType.SUBSCRIBE_DELIVERY]: (data) => {
    if (!data?.deliveryId || typeof data.deliveryId !== 'string')
      throw new WsException(WsErrorCode.INVALID_MESSAGE, 'deliveryId required');
  },
  [ClientMessageType.UNSUBSCRIBE_DELIVERY]: (data) => {
    if (!data?.deliveryId || typeof data.deliveryId !== 'string')
      throw new WsException(WsErrorCode.INVALID_MESSAGE, 'deliveryId required');
  },
  [ClientMessageType.ACCEPT_ASSIGNMENT]: (data) => {
    if (!data?.deliveryId)
      throw new WsException(WsErrorCode.INVALID_MESSAGE, 'deliveryId required');
    if (!data?.commandId || typeof data.commandId !== 'string')
      throw new WsException(WsErrorCode.INVALID_MESSAGE, 'commandId required');
  },
  [ClientMessageType.REJECT_ASSIGNMENT]: (data) => {
    if (!data?.deliveryId)
      throw new WsException(WsErrorCode.INVALID_MESSAGE, 'deliveryId required');
    if (!data?.commandId || typeof data.commandId !== 'string')
      throw new WsException(WsErrorCode.INVALID_MESSAGE, 'commandId required');
  },
  [ClientMessageType.COMPLETE_DELIVERY]: (data) => {
    if (!data?.deliveryId)
      throw new WsException(WsErrorCode.INVALID_MESSAGE, 'deliveryId required');
    if (!data?.commandId || typeof data.commandId !== 'string')
      throw new WsException(WsErrorCode.INVALID_MESSAGE, 'commandId required');
  },
};

/** Rate-limit actions applied per message type. */
export const RATE_ACTIONS: Partial<
  Record<ClientMessageType, 'subscribe' | 'command'>
> = {
  [ClientMessageType.SUBSCRIBE_DELIVERY]: 'subscribe',
  [ClientMessageType.UNSUBSCRIBE_DELIVERY]: 'subscribe',
  [ClientMessageType.ACCEPT_ASSIGNMENT]: 'command',
  [ClientMessageType.REJECT_ASSIGNMENT]: 'command',
  [ClientMessageType.COMPLETE_DELIVERY]: 'command',
};