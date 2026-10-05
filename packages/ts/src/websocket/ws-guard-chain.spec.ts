import { Test, TestingModule } from '@nestjs/testing';
import {
  WS_GUARD_CHAIN_OPTIONS,
  WS_GUARD_CHAIN_RATE_LIMITER,
  WsGuardChain,
} from './ws-guard-chain';
import { ClientMessageType } from './realtime-message';
import { WsErrorCode, WsException } from './ws-errors';

describe('WsGuardChain', () => {
  let rateLimiter: { check: jest.Mock };
  let validator: jest.Mock;
  let options: {
    validators: Record<string, (data: unknown) => void>;
    rateActions: Record<string, string>;
  };
  let module: TestingModule;

  const buildChain = async (withOptions: boolean) => {
    const providers: any[] = [
      WsGuardChain,
      { provide: WS_GUARD_CHAIN_RATE_LIMITER, useValue: rateLimiter },
    ];
    if (withOptions) {
      providers.push({
        provide: WS_GUARD_CHAIN_OPTIONS,
        useValue: options,
      });
    }
    module = await Test.createTestingModule({ providers }).compile();
    return module.get(WsGuardChain);
  };

  const ctx = (
    message: Record<string, unknown>,
    socket: Record<string, unknown> = { data: { userId: 'u-1' } },
  ) =>
    ({ message, socket }) as Parameters<WsGuardChain['run']>[0];

  const capture = async (promise: Promise<void>): Promise<WsException> => {
    const error = await promise.then(
      () => null,
      (err) => err,
    );
    expect(error).toBeInstanceOf(WsException);
    return error as WsException;
  };

  beforeEach(() => {
    rateLimiter = { check: jest.fn().mockResolvedValue(true) };
    validator = jest.fn();
    options = {
      validators: { [ClientMessageType.LOCATION_UPDATE]: validator },
      rateActions: { [ClientMessageType.LOCATION_UPDATE]: 'location.update' },
    };
  });

  afterEach(async () => {
    await module?.close();
  });

  it('is defined', async () => {
    expect(await buildChain(true)).toBeInstanceOf(WsGuardChain);
  });

  it('runs all steps in order when everything passes', async () => {
    const chain = await buildChain(true);

    await chain.run(
      ctx({
        type: ClientMessageType.LOCATION_UPDATE,
        data: { lat: 1 },
      }),
    );

    expect(rateLimiter.check).toHaveBeenCalledWith('u-1', 'location.update');
    expect(rateLimiter.check.mock.invocationCallOrder[0]).toBeLessThan(
      validator.mock.invocationCallOrder[0],
    );
    expect(validator).toHaveBeenCalledWith({ lat: 1 });
  });

  describe('authenticate step', () => {
    it('rejects when the socket carries no data', async () => {
      const chain = await buildChain(true);

      const error = await capture(
        chain.run(ctx({ type: ClientMessageType.PING, data: null }, {})),
      );

      expect(error.code).toBe(WsErrorCode.UNAUTHENTICATED);
      expect(error.message).toBe('Unauthenticated');
      expect(error.retryable).toBe(false);
      expect(rateLimiter.check).not.toHaveBeenCalled();
      expect(validator).not.toHaveBeenCalled();
    });

    it('rejects when data exists but no userId is attached', async () => {
      const chain = await buildChain(true);

      const error = await capture(
        chain.run(
          ctx(
            { type: ClientMessageType.LOCATION_UPDATE, data: {} },
            { data: { userId: '' } },
          ),
        ),
      );

      expect(error.code).toBe(WsErrorCode.UNAUTHENTICATED);
      expect(rateLimiter.check).not.toHaveBeenCalled();
      expect(validator).not.toHaveBeenCalled();
    });

    it('short-circuits before rate limiting and validation', async () => {
      const chain = await buildChain(true);

      await capture(chain.run(ctx({ type: 'PING', data: null }, {})));

      expect(rateLimiter.check).not.toHaveBeenCalled();
      expect(validator).not.toHaveBeenCalled();
    });
  });

  describe('rate limit step', () => {
    it('skips the limiter when no rate action is mapped', async () => {
      const chain = await buildChain(true);

      await chain.run(ctx({ type: ClientMessageType.PING, data: null }));

      expect(rateLimiter.check).not.toHaveBeenCalled();
      expect(validator).not.toHaveBeenCalled();
    });

    it('passes when the limiter allows the action', async () => {
      const chain = await buildChain(true);

      await expect(
        chain.run(
          ctx({ type: ClientMessageType.LOCATION_UPDATE, data: { lat: 1 } }),
        ),
      ).resolves.toBeUndefined();
      expect(rateLimiter.check).toHaveBeenCalledTimes(1);
    });

    it('rejects with a retryable RATE_LIMITED error when the limiter denies', async () => {
      rateLimiter.check.mockResolvedValue(false);
      const chain = await buildChain(true);

      const error = await capture(
        chain.run(
          ctx({ type: ClientMessageType.LOCATION_UPDATE, data: { lat: 1 } }),
        ),
      );

      expect(rateLimiter.check).toHaveBeenCalledWith('u-1', 'location.update');
      expect(error.code).toBe(WsErrorCode.RATE_LIMITED);
      expect(error.message).toBe('Rate limit exceeded for action: location.update');
      expect(error.retryable).toBe(true);
      expect(validator).not.toHaveBeenCalled();
    });

    it('propagates limiter failures', async () => {
      rateLimiter.check.mockRejectedValue(new Error('redis down'));
      const chain = await buildChain(true);

      await expect(
        chain.run(ctx({ type: ClientMessageType.LOCATION_UPDATE, data: {} })),
      ).rejects.toThrow('redis down');
      expect(validator).not.toHaveBeenCalled();
    });
  });

  describe('validation step', () => {
    it('invokes the registered validator with the message data', async () => {
      const chain = await buildChain(true);

      await chain.run(
        ctx({ type: ClientMessageType.LOCATION_UPDATE, data: { lat: 9 } }),
      );

      expect(validator).toHaveBeenCalledWith({ lat: 9 });
    });

    it('skips validation when no validator is registered for the type', async () => {
      const chain = await buildChain(true);

      await chain.run(ctx({ type: ClientMessageType.ACK, data: 'ok' }));

      expect(validator).not.toHaveBeenCalled();
    });

    it('propagates validator errors', async () => {
      const validationError = new Error('lat must be a number');
      validator.mockImplementation(() => {
        throw validationError;
      });
      const chain = await buildChain(true);

      await expect(
        chain.run(
          ctx({ type: ClientMessageType.LOCATION_UPDATE, data: { lat: 'x' } }),
        ),
      ).rejects.toThrow('lat must be a number');
    });
  });

  describe('without optional options', () => {
    it('resolves when the optional options provider is absent', async () => {
      const chain = await buildChain(false);

      await expect(
        chain.run(
          ctx({ type: ClientMessageType.LOCATION_UPDATE, data: { lat: 1 } }),
        ),
      ).resolves.toBeUndefined();
      expect(rateLimiter.check).not.toHaveBeenCalled();
      expect(validator).not.toHaveBeenCalled();
    });

    it('still enforces authentication', async () => {
      const chain = await buildChain(false);

      const error = await capture(
        chain.run(ctx({ type: ClientMessageType.PING, data: null }, {})),
      );

      expect(error.code).toBe(WsErrorCode.UNAUTHENTICATED);
    });
  });
});
