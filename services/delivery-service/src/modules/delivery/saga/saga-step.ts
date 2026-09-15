import { Delivery } from '../entities/delivery.entity';

export interface DeliverySagaContext {
  delivery: Delivery;
  paymentId?: string;
  authorizationId?: string;
}

export interface DeliverySagaStep {
  readonly name: string;
  execute(context: DeliverySagaContext): Promise<DeliverySagaContext>;
  compensate(context: DeliverySagaContext): Promise<void>;
}
