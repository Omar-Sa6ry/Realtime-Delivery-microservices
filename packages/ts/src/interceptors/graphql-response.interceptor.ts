import { Injectable, NestInterceptor, ExecutionContext, CallHandler } from '@nestjs/common';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { ResponseFormatter } from '../interceptors/response-formatter';

@Injectable()
export class GraphQLResponseInterceptor implements NestInterceptor {
  intercept(context: ExecutionContext, next: CallHandler): Observable<any> {
    const isGraphQL = context.getType<any>() === 'graphql';

    return next.handle().pipe(
      map((data) => {
        if (!isGraphQL) return data;

        if (data && data.statusCode !== undefined && data.success !== undefined) {
          return data;
        }

        return ResponseFormatter.formatSuccess(data);
      }),
    );
  }
}