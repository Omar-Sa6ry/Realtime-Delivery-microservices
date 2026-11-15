import { PresenceService } from './presence.service';
import { PresenceStore, PresenceRecord } from './presence.store';
import { NatsPublisher } from '../../infrastructure/nats/nats.publisher';
import { RealtimeNatsSubjects } from '@delivery-micro/shard';

describe('PresenceService', () => {
  let service: PresenceService;
  let store: {
    get: jest.Mock;
    setState: jest.Mock;
  };
  let natsPublisher: { publish: jest.Mock };

  beforeEach(() => {
    store = { get: jest.fn(), setState: jest.fn() };
    natsPublisher = { publish: jest.fn().mockResolvedValue(true) };
    service = new PresenceService(
      store as unknown as PresenceStore,
      natsPublisher as unknown as NatsPublisher,
    );
    jest.restoreAllMocks();
  });

  describe('setOnline', () => {
    it('sets state and broadcasts when there was no previous state', async () => {
      store.get.mockResolvedValue(null);

      await service.setOnline('u1');

      expect(store.setState).toHaveBeenCalledWith('u1', 'ONLINE');
      expect(natsPublisher.publish).toHaveBeenCalledWith(
        RealtimeNatsSubjects.DRIVER_PRESENCE_UPDATED,
        expect.objectContaining({ driverId: 'u1', status: 'ONLINE' }),
      );
    });

    it('broadcasts on transition from OFFLINE to ONLINE', async () => {
      store.get.mockResolvedValue({ state: 'OFFLINE', lastSeen: 1 });

      await service.setOnline('u1');

      expect(natsPublisher.publish).toHaveBeenCalledTimes(1);
    });

    it('does not rebroadcast when already ONLINE', async () => {
      store.get.mockResolvedValue({ state: 'ONLINE', lastSeen: 1 });

      await service.setOnline('u1');

      expect(store.setState).toHaveBeenCalledWith('u1', 'ONLINE');
      expect(natsPublisher.publish).not.toHaveBeenCalled();
    });
  });

  it('setIdle broadcasts only when the previous state differs', async () => {
    store.get.mockResolvedValue({ state: 'IDLE', lastSeen: 1 });
    await service.setIdle('u1');
    expect(natsPublisher.publish).not.toHaveBeenCalled();

    store.get.mockResolvedValue({
      state: 'ONLINE',
      lastSeen: 1,
    } as PresenceRecord);
    await service.setIdle('u1');
    expect(store.setState).toHaveBeenCalledWith('u1', 'IDLE');
    expect(natsPublisher.publish).toHaveBeenCalledTimes(1);
  });

  describe('setOffline', () => {
    it('broadcasts on transition to OFFLINE', async () => {
      store.get.mockResolvedValue({ state: 'ONLINE', lastSeen: 1 });

      await service.setOffline('u1');

      expect(store.setState).toHaveBeenCalledWith('u1', 'OFFLINE');
      expect(natsPublisher.publish).toHaveBeenCalledWith(
        RealtimeNatsSubjects.DRIVER_PRESENCE_UPDATED,
        expect.objectContaining({ status: 'OFFLINE' }),
      );
    });

    it('does not rebroadcast when already OFFLINE', async () => {
      store.get.mockResolvedValue({ state: 'OFFLINE', lastSeen: 1 });

      await service.setOffline('u1');

      expect(natsPublisher.publish).not.toHaveBeenCalled();
    });
  });

  it('get delegates to the store', async () => {
    const record = { state: 'ONLINE', lastSeen: 42 };
    store.get.mockResolvedValue(record);
    await expect(service.get('u1')).resolves.toEqual(record);
  });

  it('swallows NATS publish failures', async () => {
    jest.restoreAllMocks();
    const warn = jest.spyOn((service as any).logger, 'warn');
    store.get.mockResolvedValue(null);
    natsPublisher.publish.mockRejectedValue(new Error('nats down'));

    await expect(service.setOnline('u1')).resolves.toBeUndefined();
    expect(warn).toHaveBeenCalled();
  });
});
