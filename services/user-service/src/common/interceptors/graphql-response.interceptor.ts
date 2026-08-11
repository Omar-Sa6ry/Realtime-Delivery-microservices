import { Injectable, NestInterceptor, ExecutionContext, CallHandler } from '@nestjs/common';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { ResponseFormatter } from '@bts-soft/core';

@Injectable()
export class GraphqlResponseInterceptor implements NestInterceptor {
  intercept(context: ExecutionContext, next: CallHandler): Observable<any> {
    const isGraphQL = context.getType<any>() === 'graphql';
    
    return next.handle().pipe(
      map(data => {
        if (!isGraphQL) return data;
        
        // Wrap the response using the exact same formatter used by REST
        if (data && data.statusCode !== undefined && data.success !== undefined) {
          return data;
        }
        
        return ResponseFormatter.formatSuccess(data);
      }),
    );
  }
}
