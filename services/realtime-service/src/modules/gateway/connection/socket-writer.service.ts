import { Injectable, Logger, OnModuleDestroy } from '@nestjs/common';
import { MessagePriority, ServerMessageType, ServerMessage } from '@delivery/common';
import { WebSocket } from 'ws';
import { TIMINGS } from '../../../common/common-types/constants';
import { SocketData } from './connection.types';
import { ConnectionRegistry } from './connection.registry';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';

/**
 * Backpressure-aware socket writer.
 *  - HIGH_FREQUENCY_LOSSY: coalescing (latest value wins) per delivery, flushed periodically
 *  - NORMAL / CRITICAL: immediate send; slow consumers (buffer saturated) are terminated
 */
@Injectable()
export class SocketWriter implements OnModuleDestroy {
  private readonly logger = new Logger(SocketWriter.name);

  private readonly pendingLossy = new Map<string, Map<string, string>>();
  private readonly flushTimer = setInterval(
    () => this.flushLossy(),
    TIMINGS.LOCATION_COALESCE_MS,
  );

  constructor(
    private readonly registry: ConnectionRegistry,
    private readonly metrics: RealtimeMetricsService,
  ) {}

  onModuleDestroy() {
    clearInterval(this.flushTimer);
  }

  /** Serialize + send a server message to one socket, applying backpressure policy. */
  send(
    socket: WebSocket & { data: SocketData },
    message: ServerMessage,
    priority: MessagePriority = MessagePriority.NORMAL,
  ): void {
    if (socket.readyState !== socket.OPEN) return;

    if (priority === MessagePriority.HIGH_FREQUENCY_LOSSY) {
      this.queueLossy(socket, message);
      return;
    }

    if (socket.bufferedAmount > TIMINGS.MAX_BACKLOG * 1024) {
      this.metrics.backpressureSaturated.inc();
      this.metrics.messagesDropped.inc({ type: message.type, reason: 'slow_consumer' });
      this.logger.warn(
        `Slow consumer detected (socket=${socket.data.socketId}, buffered=${socket.bufferedAmount}); terminating`,
      );
      socket.close(1013, 'Try again later');
      return;
    }

    this.write(socket, message, priority);
  }

  /** Send the same message to many local sockets (NATS fan-out target). */
  sendMany(
    sockets: Array<WebSocket & { data: SocketData }>,
    message: ServerMessage,
    priority: MessagePriority = MessagePriority.NORMAL,
  ): void {
    for (const socket of sockets) this.send(socket, message, priority);
  }

  private queueLossy(socket: WebSocket & { data: SocketData }, message: ServerMessage): void {
    const socketId = socket.data.socketId;
    if (!this.pendingLossy.has(socketId)) this.pendingLossy.set(socketId, new Map());
    const queue = this.pendingLossy.get(socketId)!;
    const key = this.lossyKey(message);
    queue.set(key, JSON.stringify(message));
  }

  private lossyKey(message: ServerMessage): string {
    const deliveryId = (message.data as { deliveryId?: string })?.deliveryId;
    return deliveryId || message.type;
  }

  private flushLossy(): void {
    for (const [socketId, queue] of this.pendingLossy) {
      if (queue.size === 0) continue;
      const socket = this.registry.get(socketId);
      if (!socket || socket.readyState !== socket.OPEN) {
        this.pendingLossy.delete(socketId);
        continue;
      }
      for (const payload of queue.values()) {
        this.metrics.messagesSent.inc({
          type: ServerMessageType.DELIVERY_LOCATION_UPDATED,
          priority: MessagePriority.HIGH_FREQUENCY_LOSSY,
        });
        try {
          socket.send(payload);
        } catch (err) {
          this.metrics.messagesDropped.inc({
            type: ServerMessageType.DELIVERY_LOCATION_UPDATED,
            reason: 'send_error',
          });
        }
      }
      queue.clear();
    }
  }

  private write(
    socket: WebSocket & { data: SocketData },
    message: ServerMessage,
    priority: MessagePriority,
  ): void {
    this.metrics.messagesSent.inc({ type: message.type, priority });
    try {
      socket.send(JSON.stringify(message));
    } catch (err) {
      this.metrics.messagesDropped.inc({ type: message.type, reason: 'send_error' });
      this.logger.warn(`Send failed for socket ${socket.data.socketId}: ${err.message}`);
    }
  }
}