import { Resolver, Query, Args, Context } from '@nestjs/graphql';
import { I18nService } from 'nestjs-i18n';
import { ForbiddenException } from '@nestjs/common';
import { Auth } from '@delivery/common';
import { BooleanResponse, ConnectionStatusResponse, ConnectionsCountResponse } from './common/graphql/connection.types';
import { ConnectionService } from './modules/gateway/connection/connection.service';

@Resolver()
export class AppResolver {
  constructor(
    private readonly connectionService: ConnectionService,
    private readonly i18n: I18nService,
  ) {}

  @Query(() => BooleanResponse)
  async pingForRealtime(@Context() ctx?: any): Promise<BooleanResponse> {
    const lang = ctx?.language || ctx?.req?.headers?.['x-lang'] || 'en';
    const message = await this.i18n.translate('messages.service.running', { lang });
    return {
      success: true,
      statusCode: 200,
      message,
      data: true,
    };
  }

  @Auth()
  @Query(() => ConnectionStatusResponse)
  async getConnectionStatus(
    @Args('userId') userId: string,
    @Context() ctx: any,
  ): Promise<ConnectionStatusResponse> {
    const lang = ctx?.language || ctx?.req?.headers?.['x-lang'] || 'en';
    const tokenUserId = ctx?.req?.user?.id ?? ctx?.req?.headers?.['x-user-id'];
    const userRole = (ctx?.req?.user?.role ?? ctx?.req?.headers?.['x-user-role'])?.toLowerCase();
    const isAdmin = userRole === 'admin';

    if (!isAdmin && tokenUserId !== userId) {
      throw new ForbiddenException(
        await this.i18n.translate('messages.ws.unauthorized', { lang }),
      );
    }

    const status = await this.connectionService.getUserConnectionStatus(userId);
    const message = await this.i18n.translate('messages.service.statusRetrieved', { lang });
    return {
      success: true,
      statusCode: 200,
      message,
      data: {
        isConnected: status.isConnected,
        lastSeen: status.lastSeen || undefined,
        connectionCount: status.connectionCount,
      },
    };
  }

  @Auth()
  @Query(() => ConnectionsCountResponse)
  async getActiveConnections(@Context() ctx: any): Promise<ConnectionsCountResponse> {
    const lang = ctx?.language || ctx?.req?.headers?.['x-lang'] || 'en';
    const userRole = (ctx?.req?.user?.role ?? ctx?.req?.headers?.['x-user-role'])?.toLowerCase();
    const isAdmin = userRole === 'admin';

    if (!isAdmin) {
      throw new ForbiddenException(
        await this.i18n.translate('messages.ws.unauthorized', { lang }),
      );
    }

    const data = await this.connectionService.getActiveConnectionCounts();
    const message = await this.i18n.translate('messages.service.countsRetrieved', { lang });
    return {
      success: true,
      statusCode: 200,
      message,
      data,
    };
  }
}
