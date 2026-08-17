import { registerAs } from '@nestjs/config';

export interface RealtimeConfig {
  natsUrl: string;
  instanceId: string;
  authzDeliveryServiceUrl?: string;
  authzDriverServiceUrl?: string;
}

export default registerAs('realtime', (): RealtimeConfig => ({
  natsUrl: process.env.NATS_URL || 'nats://localhost:4222',
  instanceId: process.env.INSTANCE_ID || `realtime-${process.pid}`,
  authzDeliveryServiceUrl: process.env.DELIVERY_SERVICE_GRPC_URL || 'delivery-srv:50051',
  authzDriverServiceUrl: process.env.DRIVER_SERVICE_GRPC_URL || 'driver-srv:50053',
}));
