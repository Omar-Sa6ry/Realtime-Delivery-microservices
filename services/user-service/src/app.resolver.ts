import { Query, Resolver } from '@nestjs/graphql';

@Resolver()
export class AppResolver {
  @Query(() => String)
  sayHelloInUserService(): string {
    return 'Hello, in User Service';
  }
}
