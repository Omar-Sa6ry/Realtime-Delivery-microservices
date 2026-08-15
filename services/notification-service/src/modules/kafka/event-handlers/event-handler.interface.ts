export interface IEventHandler {
  handle(payload: any): Promise<void>;
}
