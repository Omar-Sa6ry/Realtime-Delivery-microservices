import { Injectable, Logger } from '@nestjs/common';
import { IJwtPayload, Role } from '@delivery/common';
import { WebSocket } from 'ws';
import { ConnectionContext, SocketData } from './connection.types';
import { ConnectionRegistry } from './connection.registry';
import { ConnectionStateStore } from './connection-state.store';
import { PresenceService } from '../../features/presence/presence.service';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';

@Injectable()
export class ConnectionService {
  private readonly logger = new Logger(ConnectionService.name);

  constructor(
    private readonly registry: ConnectionRegistry,
    private readonly stateStore: ConnectionStateStore,
    private readonly presence: PresenceService,
    private readonly metrics: RealtimeMetricsService,
  ) {}

  /**
   * Register a new socket: attach identity, keep in the local Map,
   * persist metadata in Redis and flip presence to ONLINE.
   */
  async register(socket: WebSocket, nodeId: string, payload: IJwtPayload): Promise<ConnectionContext> {
    const context = this.registry.buildContext(nodeId, payload);
    this.registry.attach(socket, context);
    await this.stateStore.register(context);
    await this.presence.setOnline(context.userId);
    this.metrics.connectionCounter.inc({ result: 'accepted' });
    this.metrics.activeConnections.set(this.registry.size);
    return context;
  }

  /** Unregister a socket on disconnect: local + Redis cleanup + OFFLINE presence. */
  async unregister(socket: WebSocket & { data: SocketData }): Promise<void> {
    const ctx = this.toContext(socket);
    if (ctx) {
      await this.stateStore.unregister(ctx);
      await this.presence.setOffline(ctx.userId);
    }
    this.registry.remove(socket.data.socketId);
    this.metrics.connectionCounter.inc({ result: 'closed' });
    this.metrics.activeConnections.set(this.registry.size);
  }

  getLocalConnection(socketId: string) {
    return this.registry.get(socketId);
  }

  getLocalSockets(): Array<WebSocket & { data: SocketData }> {
    return this.registry.values();
  }

  getLocalSocketsByRole(role: Role): Array<WebSocket & { data: SocketData }> {
    return this.getLocalSockets().filter((s) => s.data.userRole === role);
  }

  getLocalSocketCount(): number {
    return this.registry.size;
  }

  /** GraphQL federation query: connection status for a user (distributed via Redis). */
  async getUserConnectionStatus(userId: string) {
    const [connectionCount, presence] = await Promise.all([
      this.stateStore.userConnectionCount(userId),
      this.presence.get(userId),
    ]);
    return {
      isConnected: connectionCount > 0,
      lastSeen: presence?.lastSeen ? new Date(presence.lastSeen).toISOString() : null,
      connectionCount,
    };
  }

  /** GraphQL federation query: active connection totals by role. */
  async getActiveConnectionCounts() {
    const byRole = await this.stateStore.userConnectionCountsByRole();
    return {
      total: (byRole.user || 0) + (byRole.driver || 0) + (byRole.admin || 0),
      byRole: {
        customers: byRole.user || 0,
        drivers: byRole.driver || 0,
        admins: byRole.admin || 0,
      },
    };
  }

  private toContext(socket: WebSocket & { data: SocketData }): ConnectionContext | null {
    const d = socket.data;
    return {
      socketId: d.socketId,
      nodeId: d.nodeId,
      userId: d.userId,
      userRole: d.userRole,
      sessionId: d.sessionId,
      email: d.email,
      connectedAt: d.connectedAt,
      lastHeartbeatAt: d.lastPongAt,
    };
  }
}