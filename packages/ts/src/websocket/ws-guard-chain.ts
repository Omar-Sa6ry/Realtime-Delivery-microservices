import { Inject, Injectable, Optional } from "@nestjs/common";
import { ClientMessage } from "./realtime-message";
import { WsErrorCode, WsException } from "./ws-errors";

// Token used to inject the rate limiter into the guard chain.
export const WS_GUARD_CHAIN_RATE_LIMITER = "WS_GUARD_CHAIN_RATE_LIMITER";


// Token used to inject per-service guard options (validators + rate actions).
 export const WS_GUARD_CHAIN_OPTIONS = "WS_GUARD_CHAIN_OPTIONS";

/**
 * Rate limiter contract the guard chain depends on. Implementations stay in
 * the owning service (Redis-backed store lives in this package as
 * RedisRateLimitStore / RateLimitStore).
 */
export interface WsRateLimiter {
  check(userId: string, action: string): Promise<boolean>;
}

export type WsMessageValidator = (data: any) => void;

export interface WsGuardChainOptions {
  validators?: Record<string, WsMessageValidator>;
  rateActions?: Record<string, string>;
}

/**
 * Minimal socket surface the guard chain needs (client-side attach point).
 */
export interface WsSocketLike {
  data?: { userId?: string };
}

export interface WsGuardChainContext<T = unknown> {
  message: ClientMessage<T>;
  socket: WsSocketLike;
}

/**
 * Shared per-message guard pipeline: authenticate -> rate limit -> validate.
 * Domain-specific validators / rate actions are injected via WS_GUARD_CHAIN_OPTIONS.
 */
@Injectable()
export class WsGuardChain {
  constructor(
    @Inject(WS_GUARD_CHAIN_RATE_LIMITER)
    private readonly rateLimiter: WsRateLimiter,
    @Optional()
    @Inject(WS_GUARD_CHAIN_OPTIONS)
    private readonly options?: WsGuardChainOptions,
  ) {}

  async run<T = unknown>(ctx: WsGuardChainContext<T>): Promise<void> {
    await this.authenticateStep(ctx.socket);
    await this.rateLimitStep(ctx.socket, ctx.message);
    this.validationStep(ctx.message);
  }

  private authenticateStep(socket: WsSocketLike): void {
    if (!socket.data?.userId) {
      throw new WsException(WsErrorCode.UNAUTHENTICATED, "Unauthenticated");
    }
  }

  private async rateLimitStep(
    socket: WsSocketLike,
    message: ClientMessage,
  ): Promise<void> {
    const action = this.options?.rateActions?.[message.type];
    if (!action) return;
    const allowed = await this.rateLimiter.check(socket.data!.userId!, action);
    if (!allowed) {
      throw new WsException(
        WsErrorCode.RATE_LIMITED,
        `Rate limit exceeded for action: ${action}`,
        true,
      );
    }
  }

  private validationStep(message: ClientMessage): void {
    const validator = this.options?.validators?.[message.type];
    if (validator) validator(message.data);
  }
}
