import { Test } from '@nestjs/testing';
import {
  GraphQLSchemaBuilderModule,
  GraphQLSchemaFactory,
} from '@nestjs/graphql';
import { JwtService } from '@nestjs/jwt';
import { printSchema } from 'graphql';
import { I18nService } from 'nestjs-i18n';
import { AppResolver } from './app.resolver';
import { ConnectionService } from './modules/gateway/connection/connection.service';

describe('GraphQL schema build', () => {
  it('builds the realtime schema from the resolver and its DTOs', async () => {
    const moduleRef = await Test.createTestingModule({
      imports: [GraphQLSchemaBuilderModule],
      providers: [
        AppResolver,
        { provide: ConnectionService, useValue: {} },
        {
          provide: I18nService,
          useValue: { translate: jest.fn(), t: jest.fn() },
        },
        { provide: JwtService, useValue: { verifyAsync: jest.fn() } },
        { provide: 'USER_SERVICE', useValue: {} },
      ],
    }).compile();

    const factory = moduleRef.get(GraphQLSchemaFactory);
    const schema = await factory.create([AppResolver]);
    const printed = printSchema(schema);

    expect(printed).toContain('type BooleanResponse');
    expect(printed).toContain('type ConnectionStatusResponse');
    expect(printed).toContain('type ConnectionsCountResponse');
    expect(printed).toContain('pingForRealtime');
    expect(printed).toContain('getConnectionStatus');
    expect(printed).toContain('getActiveConnections');

    await moduleRef.close();
  });
});
