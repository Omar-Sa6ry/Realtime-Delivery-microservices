import { Inject, Injectable, Optional } from "@nestjs/common";
import { ClientMessage } from "./realtime-message";
import { WsErrorCode, WsException } from "./ws-errors";

export const WS_GUARD_CHAIN_RATE_LIMITER = "WS_GUARD_CHAIN_RATE_LIMITER";
export const WS_GUARD_CHAIN_OPTIONS = "WS_GUARD_CHAIN_OPTIONS";

export interface WsRateLimiter {
  check(userId: string, action: string): Promise<boolean>;
}

export type WsMessageValidator = (data: any) => void;

export interface WsGuardChainOptions {
  validators?: Record<string, WsMessageValidator>;
  rateActions?: Record<string, string>;
}

export interface WsSocketLike {
  data?: { userId?: string };
}

export interface WsGuardChainContext<T = unknown> {
  message: ClientMessage<T>;
  socket: WsSocketLike;
}

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
