import { Resolver, Query } from '@nestjs/graphql';
import { BooleanResponse } from './common/graphql/general-response.type';

@Resolver()
export class AppResolver {
  @Query(() => BooleanResponse)
  pingForNotification(): BooleanResponse {
    return {
      success: true,
      statusCode: 200,
      message: 'Notification service is running',
      data: true,
    };
  }
}