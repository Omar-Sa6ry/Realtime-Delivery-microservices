import { Injectable, NestInterceptor, ExecutionContext, CallHandler } from '@nestjs/common';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { ResponseFormatter } from '@bts-soft/core';

@Injectable()
export class GraphqlResponseInterceptor implements NestInterceptor {
  intercept(context: ExecutionContext, next: CallHandler): Observable<unknown> {
    const isGraphQL = context.getType<string>() === 'graphql';

    return next.handle().pipe(
      map((data: unknown) => {
        if (!isGraphQL) return data;

        const body = data as { statusCode?: number; success?: boolean };
        if (body && body.statusCode !== undefined && body.success !== undefined) {
          return data;
        }

        return ResponseFormatter.formatSuccess(data);
      }),
    );
  }
}