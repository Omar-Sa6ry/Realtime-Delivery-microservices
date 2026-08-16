import { Resolver, Query } from '@nestjs/graphql';

@Resolver()
export class AppResolver {
  @Query()
  pingForRealtime() {
    return {
      success: true,
      statusCode: 200,
      message: 'Realtmime service is running',
      data: true,
    };
  }
}
