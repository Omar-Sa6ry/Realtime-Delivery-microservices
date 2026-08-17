import { Logger, UseFilters } from '@nestjs/common';
import {
  ConnectedSocket,
  MessageBody,
  OnGatewayConnection,
  OnGatewayDisconnect,
  SubscribeMessage,
  WebSocketGateway,
} from '@nestjs/websockets';
import type { IncomingMessage } from 'http';
import { ClientMessageType, ServerMessageType } from '@delivery/common';
import type { ClientMessage, IJwtPayload } from '@delivery/common';
import { WsAuthGuard } from '@delivery/common';
import { ConnectionService } from '../connection/connection.service';
import { SubscriptionService } from '../subscription/subscription.service';
import { HeartbeatService } from '../heartbeat/heartbeat.service';
import { LocationService } from '../location/location.service';
import { CommandService } from '../command/command.service';
import { WsGuardChain } from '@delivery/common';
import type { AuthenticatedSocket } from '../connection/connection.types';
import { ConfigService } from '@nestjs/config';
import type {
  AssignmentCommandPayload,
  DeliverySubscriptionPayload,
  LocationUpdatePayload,
} from '../../common/common-types/ws-message.types';
import { RealtimeMetricsService } from '../../common/metrics/realtime-metrics.service';
import { WsExceptionFilter } from '@delivery/common';

@WebSocketGateway({
  path: '/ws',
  cors: { origin: '*' },
})
@UseFilters(WsExceptionFilter)
export class RealtimeGateway
  implements OnGatewayConnection, OnGatewayDisconnect
{
  private readonly logger = new Logger(RealtimeGateway.name);
  private readonly allowedOrigins: string[];

  constructor(
    private readonly config: ConfigService,
    private readonly authGuard: WsAuthGuard,
    private readonly connectionService: ConnectionService,
    private readonly subscriptionService: SubscriptionService,
    private readonly heartbeatService: HeartbeatService,
    private readonly locationService: LocationService,
    private readonly commandService: CommandService,
    private readonly guardChain: WsGuardChain,
    private readonly metrics: RealtimeMetricsService,
  ) {
    this.allowedOrigins = (config.get<string>('WS_ALLOWED_ORIGINS') || '*')
      .split(',')
      .map((o) => o.trim())
      .filter(Boolean);
  }

  // ===== Connection lifecycle =====

  async handleConnection(
    client: AuthenticatedSocket | any,
    request: IncomingMessage,
  ): Promise<void> {
    if (!this.isOriginAllowed(request)) {
      this.logger.warn(
        `WebSocket origin rejected: ${request.headers.origin}`,
      );
      client.close(4403, 'FORBIDDEN');
      return;
    }

    const payload = await this.authGuard.authenticate(client, request);
    if (!payload) {
      this.metrics.connectionCounter.inc({ result: 'rejected' });
      return;
    }

    try {
      const nodeId =
        this.config.get<string>('INSTANCE_ID') || `realtime-${process.pid}`;
      const context = await this.connectionService.register(
        client,
        nodeId,
        payload,
      );

      client.data.user = {
        userId: context.userId,
        role: context.userRole,
        sessionId: context.sessionId,
        email: context.email,
      } as IJwtPayload;

      this.logger.log(
        `Socket connected: ${context.socketId} (user=${context.userId}, role=${context.userRole})`,
      );
      client.send(
        JSON.stringify({
          type: ServerMessageType.CONNECTED,
          timestamp: new Date().toISOString(),
          data: {
            socketId: context.socketId,
            nodeId,
            timestamp: new Date().toISOString(),
          },
        }),
      );
    } catch (err) {
      this.logger.error(
        `Connection registration failed: ${(err as Error).message}`,
      );
      client.close(1011, 'INTERNAL_ERROR');
    }
  }

  async handleDisconnect(client: AuthenticatedSocket | any): Promise<void> {
    const socket = client as AuthenticatedSocket;
    if (!socket.data?.socketId) return;

    const socketId = socket.data.socketId;
    this.logger.log(`Socket disconnected: ${socketId}`);
    await this.subscriptionService.cleanupSocket(socketId);
    await this.connectionService.unregister(socket);
  }

  // ===== Message handlers (thin gateway -> services) =====

  @SubscribeMessage(ClientMessageType.PING)
  async handlePing(
    @ConnectedSocket() client: AuthenticatedSocket,
    @MessageBody() message?: ClientMessage,
  ) {
    await this.guardChain.run({ message: message!, socket: client });
    const pong = await this.heartbeatService.handlePing(client);
    return {
      ...pong,
      type: ServerMessageType.PONG,
      requestId: message?.requestId,
    };
  }

  @SubscribeMessage(ClientMessageType.SUBSCRIBE_DELIVERY)
  async handleSubscribe(
    @ConnectedSocket() client: AuthenticatedSocket,
    @MessageBody() message?: ClientMessage<DeliverySubscriptionPayload>,
  ) {
    this.metrics.messagesReceived.inc({
      type: ClientMessageType.SUBSCRIBE_DELIVERY,
    });
    await this.guardChain.run({ message: message!, socket: client });
    const deliveryId = message!.data!.deliveryId;
    await this.subscriptionService.subscribeToDelivery(
      client.data.socketId,
      this.userOf(client),
      deliveryId,
    );
    return {
      type: ServerMessageType.SUBSCRIBED,
      requestId: message?.requestId,
      timestamp: new Date().toISOString(),
      data: { deliveryId, requestId: message?.requestId },
    };
  }

  @SubscribeMessage(ClientMessageType.UNSUBSCRIBE_DELIVERY)
  async handleUnsubscribe(
    @ConnectedSocket() client: AuthenticatedSocket,
    @MessageBody() message?: ClientMessage<DeliverySubscriptionPayload>,
  ) {
    await this.guardChain.run({ message: message!, socket: client });
    const deliveryId = message!.data!.deliveryId;
    await this.subscriptionService.unsubscribeFromDelivery(
      client.data.socketId,
      deliveryId,
    );
    return {
      type: ServerMessageType.UNSUBSCRIBED,
      requestId: message?.requestId,
      timestamp: new Date().toISOString(),
      data: { deliveryId, requestId: message?.requestId },
    };
  }

  @SubscribeMessage(ClientMessageType.LOCATION_UPDATE)
  async handleLocationUpdate(
    @ConnectedSocket() client: AuthenticatedSocket,
    @MessageBody() message?: ClientMessage<LocationUpdatePayload>,
  ) {
    this.metrics.messagesReceived.inc({
      type: ClientMessageType.LOCATION_UPDATE,
    });
    await this.guardChain.run({ message: message!, socket: client });
    await this.locationService.handle(client, message!.data!, message?.requestId);
    return undefined;
  }

  @SubscribeMessage(ClientMessageType.ACCEPT_ASSIGNMENT)
  async handleAcceptAssignment(
    @ConnectedSocket() client: AuthenticatedSocket,
    @MessageBody() message?: ClientMessage<AssignmentCommandPayload>,
  ) {
    this.metrics.messagesReceived.inc({
      type: ClientMessageType.ACCEPT_ASSIGNMENT,
    });
    await this.guardChain.run({ message: message!, socket: client });
    const result = await this.commandService.execute(
      this.userOf(client),
      'ACCEPT_ASSIGNMENT',
      message!.data!,
    );
    return this.commandAck(ServerMessageType.ACK, message?.requestId, result);
  }

  @SubscribeMessage(ClientMessageType.REJECT_ASSIGNMENT)
  async handleRejectAssignment(
    @ConnectedSocket() client: AuthenticatedSocket,
    @MessageBody() message?: ClientMessage<AssignmentCommandPayload>,
  ) {
    this.metrics.messagesReceived.inc({
      type: ClientMessageType.REJECT_ASSIGNMENT,
    });
    await this.guardChain.run({ message: message!, socket: client });
    const result = await this.commandService.execute(
      this.userOf(client),
      'REJECT_ASSIGNMENT',
      message!.data!,
    );
    return this.commandAck(ServerMessageType.ACK, message?.requestId, result);
  }

  @SubscribeMessage(ClientMessageType.COMPLETE_DELIVERY)
  async handleCompleteDelivery(
    @ConnectedSocket() client: AuthenticatedSocket,
    @MessageBody() message?: ClientMessage<AssignmentCommandPayload>,
  ) {
    this.metrics.messagesReceived.inc({
      type: ClientMessageType.COMPLETE_DELIVERY,
    });
    await this.guardChain.run({ message: message!, socket: client });
    const result = await this.commandService.execute(
      this.userOf(client),
      'COMPLETE_DELIVERY',
      message!.data!,
    );
    return this.commandAck(ServerMessageType.ACK, message?.requestId, result);
  }

  @SubscribeMessage(ClientMessageType.ACK)
  async handleAck(
    @MessageBody() message?: ClientMessage<{ messageId: string }>,
  ) {
    this.metrics.messagesReceived.inc({ type: ClientMessageType.ACK });
    this.logger.debug(
      `Client ACK: ${message?.data?.messageId || message?.requestId}`,
    );
    return undefined;
  }

  // ===== helpers =====

  private commandAck(
    type: ServerMessageType.ACK,
    requestId: string | undefined,
    result: { accepted: boolean; duplicate?: boolean; rejected?: boolean },
  ) {
    return {
      type,
      requestId,
      timestamp: new Date().toISOString(),
      data: result,
    };
  }

  private userOf(client: AuthenticatedSocket): IJwtPayload {
    return {
      userId: client.data.userId,
      role: client.data.userRole,
      sessionId: client.data.sessionId,
      email: client.data.email,
    };
  }

  private isOriginAllowed(request: IncomingMessage): boolean {
    if (this.allowedOrigins.includes('*')) return true;
    const origin = request.headers.origin;
    if (!origin) return true; // non-browser clients send no Origin header
    return this.allowedOrigins.some(
      (o) => origin === o || origin.endsWith(`://${o}`) || origin.endsWith(`.${o}`),
    );
  }
}