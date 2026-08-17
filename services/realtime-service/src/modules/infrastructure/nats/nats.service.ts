import { Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { connect, NatsConnection, JSONCodec, Codec, ConnectionOptions } from 'nats';
import { ConfigService } from '@nestjs/config';
import type { RealtimeConfig } from '../../../common/config/realtime.config';

@Injectable()
export class RealtimeNatsService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(RealtimeNatsService.name);
  private client: NatsConnection | null = null;
  private readonly codec: Codec<unknown> = JSONCodec();
  private connected = false;

  constructor(private readonly config: ConfigService) {}

  async onModuleInit(): Promise<void> {
    await this.connect();
  }

  private async connect(): Promise<void> {
    const cfg = this.config.get<RealtimeConfig>('realtime')!;
    const options: ConnectionOptions = {
      servers: cfg.natsUrl,
      name: cfg.instanceId,
      maxReconnectAttempts: -1,
      reconnectTimeWait: 2_000,
      timeout: 5_000,
      waitOnFirstConnect: true,
    };

    this.client = await connect(options);
    this.connected = this.clientIsConnected();
    this.logger.log(`Connected to NATS at ${cfg.natsUrl}`);

    // Refresh connectivity flags on status changes.
    (async () => {
      for await (const status of this.client!.status()) {
        this.connected = status.type === 'update' && this.clientIsConnected();
        if (!this.connected) {
          this.logger.warn(`NATS status changed: ${status.type}`);
        } else if (status.type === 'update') {
          this.logger.log('NATS reconnected');
        }
      }
    })().catch((err) => this.logger.error(`NATS status loop error: ${err.message}`));
  }

  private clientIsConnected(): boolean {
    return this.client !== null && !this.client.isClosed();
  }

  getClient(): NatsConnection | null {
    return this.client;
  }

  getCodec(): Codec<unknown> {
    return this.codec;
  }

  isConnected(): boolean {
    return this.clientIsConnected();
  }

  getLocalConnectionCount(): number {
    // Provided by the NATS server on a connected client is not trivial;
    // this returns the client's current cached connections (0 when not connected).
    return this.isConnected() ? 1 : 0;
  }

  async onModuleDestroy(): Promise<void> {
    if (this.client) {
      await this.client.drain();
      await this.client.close();
      this.client = null;
      this.connected = false;
      this.logger.log('NATS connection drained and closed');
    }
  }
}