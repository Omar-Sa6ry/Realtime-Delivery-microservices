import { Resolver, Query } from '@nestjs/graphql';
import { BooleanResponse } from '@delivery-micro/shard';

@Resolver()
export class AppResolver {
  @Query(() => BooleanResponse)
  pingForUser(): BooleanResponse {
    return {
      success: true,
      statusCode: 200,
      message: 'User service is running',
      data: true,
    };
  }
}
