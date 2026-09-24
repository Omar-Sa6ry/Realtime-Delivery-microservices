import { Resolver, Query, Args } from '@nestjs/graphql';
import { I18nService, I18nContext } from 'nestjs-i18n';
import { BooleanResponse, ConnectionStatusResponse, ConnectionsCountResponse } from './common/graphql/connection.types';
import { ConnectionService } from './modules/gateway/connection/connection.service';

@Resolver()
export class AppResolver {
  constructor(
    private readonly connectionService: ConnectionService,
    private readonly i18n: I18nService,
  ) {}

  @Query(() => BooleanResponse)
  async pingForRealtime(): Promise<BooleanResponse> {
    const lang = I18nContext.current()?.lang || 'en';
    const message = await this.i18n.translate('messages.service.running', { lang });
    return {
      success: true,
      statusCode: 200,
      message,
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
