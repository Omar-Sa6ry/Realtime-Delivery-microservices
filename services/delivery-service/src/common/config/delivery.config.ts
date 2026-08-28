import { registerAs } from '@nestjs/config';

export default registerAs('delivery', () => ({
  httpPort: Number(process.env.PORT_DELIVERY ?? 4003),
  grpcPort: Number(process.env.PORT_DELIVERY_GRPC ?? 50054),
  metricsPort: Number(process.env.PORT_METRICS ?? 9104),
  serviceName: process.env.SERVICE_NAME ?? 'delivery-service',
  nodeEnv: process.env.NODE_ENV ?? 'development',
}));
