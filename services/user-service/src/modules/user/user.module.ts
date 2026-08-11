import { Module } from '@nestjs/common';
import { UserService } from './user.service';
import { DbUserService } from './db-user.service';
import { UserResolver } from './user.resolver';
import { UserGrpcController } from './user-grpc.controller';
import { MetricsController } from './metrics.controller';
import { HealthController } from './health.controller';

@Module({
  controllers: [UserGrpcController, MetricsController, HealthController],
  providers: [
    DbUserService, 
    UserService, 
    UserResolver,
    {
      provide: 'USER_SERVICE',
      useFactory: (userService: UserService) => {
        return {
          findById: async (id: string) => {
            const user = await userService.findById(id);
            if (!user) return null;
            return {
              id: user.id,
              email: user.email,
              role: user.role,
            };
          },
        };
      },
      inject: [UserService],
    }
  ],
  exports: [UserService, 'USER_SERVICE'],
})
export class UserModule {}
