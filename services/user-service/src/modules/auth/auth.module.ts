import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { KafkaModule } from '@delivery/common';
import { AuthService } from './auth.service';
import { AuthResolver } from './auth.resolver';
import { UserFactory } from './user.factory';
import { UserModule } from '../user/user.module';

@Module({
  imports: [
    UserModule,
    KafkaModule.registerAsync({
      imports: [ConfigModule],
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        clientId: 'user-service',
        brokers: (config.get<string>('KAFKA_BROKERS', 'kafka-srv:9092') || '')
          .split(',').map(b => b.trim()).filter(Boolean),
      }),
    }),
  ],
  providers: [AuthService, AuthResolver, UserFactory],
  exports: [AuthService],
})
export class AuthModule {}