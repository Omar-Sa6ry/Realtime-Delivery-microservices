import { Injectable, Logger } from '@nestjs/common';
import { Cron } from '@nestjs/schedule';
import { ServerMessageType, MessagePriority } from '@delivery/common';
import { ConnectionRegistry } from '../connection/connection.registry';
import { ConnectionService } from '../connection/connection.service';
import { ConnectionStateStore } from '../connection/connection-state.store';
import { SocketWriter } from '../connection/socket-writer.service';
import { TIMINGS } from '../../common/common-types/constants';
import { RealtimeMetricsService } from '../../common/metrics/realtime-metrics.service';
import { AuthenticatedSocket } from '../connection/connection.types';

/**
 * Heartbeat management:
 *  - PING -> refreshes Redis connection TTL + updates lastPongAt
 *  - periodic sweep closes stale connections (no heartbeat within timeout)
 *  - every connection has a Redis TTL of 60s renewed on each heartbeat —
 *    nodes that crash leave no dangling metadata (TTL-based cleanup).
 */
@Injectable()
export class HeartbeatService {
  private readonly logger = new Logger(HeartbeatService.name);

  constructor(
    private readonly registry: ConnectionRegistry,
    private readonly connectionService: ConnectionService,
    private readonly stateStore: ConnectionStateStore,
    private readonly writer: SocketWriter,
    private readonly metrics: RealtimeMetricsService,
  ) {}

  /** Handle a PING: refresh Redis TTL + local lastPongAt, return the PONG payload. */
  async handlePing(socket: AuthenticatedSocket): Promise<any> {
    socket.data.lastPongAt = Date.now();
    await this.stateStore.refreshHeartbeat(socket.data.socketId);
    return {
      type: ServerMessageType.PONG,
      data: { timestamp: new Date().toISOString() },
    };
  }

  @Cron('*/20 * * * * *')
  async checkStaleConnections(): Promise<void> {
    const now = Date.now();
    for (const socket of this.registry.values()) {
      if (now - socket.data.lastPongAt > TIMINGS.STALE_CONNECTION_MS) {
        const socketId = socket.data.socketId;
        this.logger.warn(`Closing stale connection (socket=${socketId})`);
        this.metrics.staleConnections.inc();
        socket.close(4001, 'Heartbeat timeout');
      }
    }
  }
}