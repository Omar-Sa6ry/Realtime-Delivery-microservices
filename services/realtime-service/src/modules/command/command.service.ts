import { Injectable, Logger } from '@nestjs/common';
import { ServerMessageType, RealtimeNatsSubjects, MessagePriority } from '@delivery/common';
import { IdempotencyStore } from '../events/idempotency.store';
import { RealtimeAuthorizationService } from '../authorization/realtime-authorization.service';
import { NatsPublisher } from '../nats/nats.publisher';
import { WsErrorCode, WsException } from '@delivery/common';
import { IJwtPayload, Role } from '@delivery/common';
import { AssignmentCommandPayload } from '../../common/common-types/ws-message.types';
import { RealtimeMetricsService } from '../../common/metrics/realtime-metrics.service';

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export type CommandKind = 'ACCEPT_ASSIGNMENT' | 'REJECT_ASSIGNMENT' | 'COMPLETE_DELIVERY';

/**
 * Command pattern: each WebSocket command is mapped to a NATS command subject
 * forwarded to the delivery domain service. Commands are idempotent
 * (Redis `realtime:idempotency:{commandId}`) and authorization-checked.
 */
@Injectable()
export class CommandService {
  private readonly logger = new Logger(CommandService.name);

  private readonly subjects: Record<CommandKind, RealtimeNatsSubjects> = {
    ACCEPT_ASSIGNMENT: RealtimeNatsSubjects.COMMAND_DRIVER,
    REJECT_ASSIGNMENT: RealtimeNatsSubjects.COMMAND_DRIVER,
    COMPLETE_DELIVERY: RealtimeNatsSubjects.COMMAND_DELIVERY,
  };

  constructor(
    private readonly idempotency: IdempotencyStore,
    private readonly authorization: RealtimeAuthorizationService,
    private readonly natsPublisher: NatsPublisher,
    private readonly metrics: RealtimeMetricsService,
  ) {}

  async execute(
    user: IJwtPayload,
    kind: CommandKind,
    payload: AssignmentCommandPayload,
  ): Promise<{ accepted: boolean; duplicate?: boolean; rejected?: boolean }> {
    // 1. Validate shape
    if (!payload?.deliveryId || !UUID_RE.test(payload.deliveryId)) {
      throw new WsException(WsErrorCode.INVALID_DELIVERY_ID, 'Invalid deliveryId', false);
    }
    if (typeof payload.commandId !== 'string' || payload.commandId.length < 8) {
      throw new WsException(WsErrorCode.INVALID_MESSAGE, 'Missing or invalid commandId', false);
    }

    // 2. Idempotency (SET NX) — duplicate commands are acknowledged, not re-forwarded
    const claim = await this.idempotency.claim(payload.commandId);
    if (claim === 'duplicate') {
      this.logger.debug(`Duplicate command ignored (commandId=${payload.commandId})`);
      return { accepted: false, duplicate: true };
    }

    // 3. Authorization (driver-only commands)
    const authorized = await this.authorization.canSendCommand(user, kind, payload.deliveryId);
    if (!authorized) {
      throw new WsException(WsErrorCode.FORBIDDEN, 'Not the assigned driver', false);
    }

    // 4. Forward to the domain service via NATS
    const published = await this.natsPublisher.publish(this.subjects[kind], {
      command: kind,
      commandId: payload.commandId,
      deliveryId: payload.deliveryId,
      driverId: user.userId,
      reason: payload.reason,
      timestamp: new Date().toISOString(),
    });

    if (!published) {
      throw new WsException(
        WsErrorCode.SERVICE_UNAVAILABLE,
        'Command transport unavailable',
        true,
      );
    }

    this.metrics.natsPublished.inc({ subject: this.subjects[kind] });
    return { accepted: true };
  }
}