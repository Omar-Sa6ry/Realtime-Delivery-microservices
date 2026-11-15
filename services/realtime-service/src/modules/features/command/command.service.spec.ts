import { CommandService } from './command.service';
import { IdempotencyStore } from '../events/idempotency.store';
import { RealtimeAuthorizationService } from '../../gateway/authorization/realtime-authorization.service';
import { NatsPublisher } from '../../infrastructure/nats/nats.publisher';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
import {
  IJwtPayload,
  RealtimeNatsSubjects,
  Role,
  WsErrorCode,
} from '@delivery-micro/shard';
import { AssignmentCommandPayload } from '../../../common/common-types/ws-message.types';

describe('CommandService', () => {
  let service: CommandService;
  let idempotency: { claim: jest.Mock };
  let authorization: { canSendCommand: jest.Mock };
  let natsPublisher: { publish: jest.Mock };
  let metrics: { natsPublished: { inc: jest.Mock } };

  const user: IJwtPayload = {
    userId: 'driver-1',
    role: Role.DRIVER,
    email: 'd@x.com',
  };

  const payload = (overrides: Partial<AssignmentCommandPayload> = {}) => ({
    deliveryId: 'delivery_1234',
    commandId: 'command_12345',
    ...overrides,
  });

  beforeEach(() => {
    idempotency = { claim: jest.fn().mockResolvedValue('new') };
    authorization = { canSendCommand: jest.fn().mockResolvedValue(true) };
    natsPublisher = { publish: jest.fn().mockResolvedValue(true) };
    metrics = { natsPublished: { inc: jest.fn() } };

    service = new CommandService(
      idempotency as unknown as IdempotencyStore,
      authorization as unknown as RealtimeAuthorizationService,
      natsPublisher as unknown as NatsPublisher,
      metrics as unknown as RealtimeMetricsService,
    );
  });

  it('rejects an invalid deliveryId', async () => {
    await expect(
      service.execute(user, 'ACCEPT_ASSIGNMENT', payload({ deliveryId: 'x' })),
    ).rejects.toMatchObject({ code: WsErrorCode.INVALID_DELIVERY_ID });
    expect(idempotency.claim).not.toHaveBeenCalled();
  });

  it('rejects a missing or short commandId', async () => {
    await expect(
      service.execute(
        user,
        'ACCEPT_ASSIGNMENT',
        payload({ commandId: undefined as any }),
      ),
    ).rejects.toMatchObject({ code: WsErrorCode.INVALID_MESSAGE });
    await expect(
      service.execute(
        user,
        'ACCEPT_ASSIGNMENT',
        payload({ commandId: 'short' }),
      ),
    ).rejects.toMatchObject({ code: WsErrorCode.INVALID_MESSAGE });
  });

  it('rejects a missing deliveryId entirely', async () => {
    await expect(
      service.execute(
        user,
        'ACCEPT_ASSIGNMENT',
        {} as AssignmentCommandPayload,
      ),
    ).rejects.toMatchObject({ code: WsErrorCode.INVALID_DELIVERY_ID });
  });

  it('short-circuits duplicate commands without re-forwarding', async () => {
    idempotency.claim.mockResolvedValue('duplicate');

    const result = await service.execute(user, 'ACCEPT_ASSIGNMENT', payload());

    expect(result).toEqual({ accepted: false, duplicate: true });
    expect(authorization.canSendCommand).not.toHaveBeenCalled();
    expect(natsPublisher.publish).not.toHaveBeenCalled();
  });

  it('throws forbidden when the driver is not authorized', async () => {
    authorization.canSendCommand.mockResolvedValue(false);

    await expect(
      service.execute(user, 'ACCEPT_ASSIGNMENT', payload()),
    ).rejects.toMatchObject({ code: WsErrorCode.FORBIDDEN });
    expect(natsPublisher.publish).not.toHaveBeenCalled();
  });

  it('throws service unavailable when NATS publish fails', async () => {
    natsPublisher.publish.mockResolvedValue(false);

    await expect(
      service.execute(user, 'ACCEPT_ASSIGNMENT', payload()),
    ).rejects.toMatchObject({
      code: WsErrorCode.SERVICE_UNAVAILABLE,
      retryable: true,
    });
  });

  it('forwards ACCEPT_ASSIGNMENT to the driver command subject', async () => {
    const result = await service.execute(user, 'ACCEPT_ASSIGNMENT', payload());

    expect(result).toEqual({ accepted: true });
    expect(natsPublisher.publish).toHaveBeenCalledWith(
      RealtimeNatsSubjects.COMMAND_DRIVER,
      expect.objectContaining({
        command: 'ACCEPT_ASSIGNMENT',
        commandId: 'command_12345',
        deliveryId: 'delivery_1234',
        driverId: 'driver-1',
      }),
    );
  });

  it('forwards REJECT_ASSIGNMENT to the driver command subject with reason', async () => {
    await service.execute(
      user,
      'REJECT_ASSIGNMENT',
      payload({ reason: 'busy' }),
    );

    expect(natsPublisher.publish).toHaveBeenCalledWith(
      RealtimeNatsSubjects.COMMAND_DRIVER,
      expect.objectContaining({ command: 'REJECT_ASSIGNMENT', reason: 'busy' }),
    );
  });

  it('forwards COMPLETE_DELIVERY to the delivery command subject', async () => {
    await service.execute(user, 'COMPLETE_DELIVERY', payload());

    expect(natsPublisher.publish).toHaveBeenCalledWith(
      RealtimeNatsSubjects.COMMAND_DELIVERY,
      expect.objectContaining({ command: 'COMPLETE_DELIVERY' }),
    );
  });
});
