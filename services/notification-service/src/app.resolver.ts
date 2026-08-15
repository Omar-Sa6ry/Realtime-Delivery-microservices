import { Query, Resolver } from '@nestjs/graphql';

@Resolver()
export class AppResolver {
  @Query(() => String)
  sayHelloinNotificationService(): string {
    return 'Hello, in Notification Service';
  }
}
