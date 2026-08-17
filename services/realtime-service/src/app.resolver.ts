import { Resolver, Query, Args } from '@nestjs/graphql';
import { BooleanResponse, ConnectionStatusResponse, ConnectionsCountResponse } from './common/graphql/connection.types';
import { ConnectionService } from './modules/gateway/connection/connection.service';

@Resolver()
export class AppResolver {
  constructor(private readonly connectionService: ConnectionService) {}

  @Query(() => BooleanResponse)
  pingForRealtime(): BooleanResponse {
    return {
      success: true,
      statusCode: 200,
      message: 'Realtime service is running',
      data: true,
    };
  }

  @Query(() => ConnectionStatusResponse)
  async getConnectionStatus(
    @Args('userId') userId: string,
  ): Promise<ConnectionStatusResponse> {
    const status = await this.connectionService.getUserConnectionStatus(userId);
    return {
      success: true,
      statusCode: 200,
      data: {
        isConnected: status.isConnected,
        lastSeen: status.lastSeen || undefined,
        connectionCount: status.connectionCount,
      },
    };
  }

  @Query(() => ConnectionsCountResponse)
  async getActiveConnections(): Promise<ConnectionsCountResponse> {
    const data = await this.connectionService.getActiveConnectionCounts();
    return {
      success: true,
      statusCode: 200,
      data,
    };
  }
}
