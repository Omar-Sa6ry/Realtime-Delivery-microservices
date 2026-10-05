import { Logger } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import type { IncomingMessage } from 'http';
import type { WebSocket } from 'ws';
import { WsAuthGuard } from './ws-auth.guard';
import { WsJwtStrategy } from './ws-jwt.strategy';

describe('WsAuthGuard', () => {
  let strategy: { authenticate: jest.Mock };
  let guard: WsAuthGuard;
  let socket: { close: jest.Mock };
  let request: IncomingMessage;
  let warnSpy: jest.SpyInstance;

  beforeEach(async () => {
    strategy = { authenticate: jest.fn() };
    socket = { close: jest.fn() };
    request = { headers: {}, url: '/ws' } as unknown as IncomingMessage;
    warnSpy = jest.spyOn(Logger.prototype, 'warn').mockImplementation();

    const module = await Test.createTestingModule({
      providers: [
        WsAuthGuard,
        { provide: WsJwtStrategy, useValue: strategy },
      ],
    }).compile();

    guard = module.get(WsAuthGuard);
  });

  afterEach(() => {
    warnSpy.mockRestore();
  });

  it('is defined', () => {
    expect(guard).toBeInstanceOf(WsAuthGuard);
  });

  it('returns the payload when the strategy succeeds', async () => {
    const payload = { userId: 'u-1', role: 'DRIVER' };
    strategy.authenticate.mockResolvedValue(payload);

    const result = await guard.authenticate(
      socket as unknown as WebSocket,
      request,
    );

    expect(strategy.authenticate).toHaveBeenCalledWith(request);
    expect(result).toEqual(payload);
    expect(socket.close).not.toHaveBeenCalled();
    expect(warnSpy).not.toHaveBeenCalled();
  });

  it('returns null, closes the socket and logs when verification fails', async () => {
    strategy.authenticate.mockRejectedValue(
      new Error('Invalid or expired token'),
    );

    const result = await guard.authenticate(
      socket as unknown as WebSocket,
      request,
    );

    expect(result).toBeNull();
    expect(socket.close).toHaveBeenCalledWith(4401, 'UNAUTHENTICATED');
    expect(warnSpy).toHaveBeenCalledWith(
      'WebSocket handshake rejected: Invalid or expired token',
    );
  });

  it('closes the socket even when the rejection has no message', async () => {
    strategy.authenticate.mockRejectedValue(new Error());

    const result = await guard.authenticate(
      socket as unknown as WebSocket,
      request,
    );

    expect(result).toBeNull();
    expect(socket.close).toHaveBeenCalledWith(4401, 'UNAUTHENTICATED');
    expect(warnSpy).toHaveBeenCalledWith('WebSocket handshake rejected: ');
  });

  it('handles a non-Error rejection', async () => {
    strategy.authenticate.mockRejectedValue('plain string');

    const result = await guard.authenticate(
      socket as unknown as WebSocket,
      request,
    );

    expect(result).toBeNull();
    expect(socket.close).toHaveBeenCalledWith(4401, 'UNAUTHENTICATED');
    expect(warnSpy).toHaveBeenCalledWith(
      'WebSocket handshake rejected: undefined',
    );
  });

  it('does not close the socket when no strategy is reached', async () => {
    strategy.authenticate.mockResolvedValue(undefined);

    const result = await guard.authenticate(
      socket as unknown as WebSocket,
      request,
    );

    expect(result).toBeUndefined();
    expect(socket.close).not.toHaveBeenCalled();
  });
});
