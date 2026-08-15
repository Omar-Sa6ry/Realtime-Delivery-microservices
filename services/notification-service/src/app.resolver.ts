import { Resolver, Query } from '@nestjs/graphql';

@Resolver()
export class AppResolver {
  @Query(() => String)
  ping(): string {
    return 'Notification service is running';
  }
}
