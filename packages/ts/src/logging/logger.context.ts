import { AsyncLocalStorage } from 'async_hooks';

export interface LogContext {
  traceId?: string;
  userId?: string;
  method?: string;
  path?: string;
  [key: string]: any;
}

export class LoggerContext {
  private static storage = new AsyncLocalStorage<LogContext>();

  public static run(context: LogContext, callback: () => void) {
    this.storage.run(context, callback);
  }

  public static getStore(): LogContext | undefined {
    return this.storage.getStore();
  }

  public static setStore(store: LogContext) {
    const current = this.storage.getStore();
    if (current) {
      Object.assign(current, store);
    }
  }

  public static getTraceId(): string | undefined {
    return this.storage.getStore()?.traceId;
  }
}
