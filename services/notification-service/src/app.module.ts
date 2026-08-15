import { Module } from '@nestjs/common';
import { AppResolver } from './app.resolver';

@Module({
  imports: [  ConfigModule.forRoot({
      isGlobal: true,
      envFilePath: [
        join(process.cwd(), '.env'),
        join(process.cwd(), '../../.env'),
      ],
    }),

    TranslationModule,
    RedisModule,
    
    TypeOrmModule.forRootAsync({
        imports: [ConfigModule],
        inject: [ConfigService],
        useFactory: (config: ConfigService) => ({
          type: 'postgres',
          host:
            config.get<string>('DB_HOST', 'localhost') === 'user-db-srv' &&
            process.env.NODE_ENV !== 'production' &&
            !process.env.KUBERNETES_SERVICE_HOST
              ? 'localhost'
              : config.get<string>('DB_HOST', 'localhost'),
          port: Number(config.get<number>('DB_PORT', 5433)) || 5433,
          username:
            config.get<string>('POSTGRES_USER') ||
            config.get<string>('DB_USERNAME', 'postgres'),
          password: config.get<string>('POSTGRES_PASSWORD'),
          database: config.get<string>('DB_NAME', 'delivery_notification_db'),
          entities: [],
          synchronize: true, // For development mode
          // Survive concurrent boot: retry for ~3 minutes before giving up.
          retryAttempts: 60,
          retryDelay: 3000,
        }),
      }),
  
      BullModule.forRootAsync({
        imports: [ConfigModule],
        inject: [ConfigService],
        useFactory: (config: ConfigService) => ({
          connection: {
            host: config.get<string>('REDIS_HOST', 'localhost'),
            port: Number(config.get<number>('REDIS_PORT', 6379)) || 6379,
          },
        }),
      }),
  
      GraphQLModule.forRoot<ApolloFederationDriverConfig>({
        driver: ApolloFederationDriver,
        path: '/user/graphql',
        autoSchemaFile: {
          path: join(process.cwd(), 'src/schema.gql'),
          federation: 2,
        },
        context: ({ req }) => ({
          req,
          user: req.user,
          language: req.headers['accept-language'] || 'en',
        }),
        playground: true,
        debug: false,
      }),
  ],
  providers: [AppResolver],
})
export class AppModule { }
