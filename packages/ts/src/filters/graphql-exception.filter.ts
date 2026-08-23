import { ArgumentsHost, Catch, Logger } from '@nestjs/common';
import { GqlExceptionFilter } from '@nestjs/graphql';
import { GraphQLError } from 'graphql';
import { ResponseFormatter } from '../interceptors/response-formatter';

@Catch()
export class GraphQLExceptionFilter implements GqlExceptionFilter {
  private readonly logger = new Logger(GraphQLExceptionFilter.name);

  catch(exception: any, host: ArgumentsHost): GraphQLError {
    if (exception instanceof Error) {
      this.logger.error(`GraphQL Exception: ${exception.message}`, exception.stack);
    } else {
      this.logger.error('GraphQL Exception', JSON.stringify(exception));
    }

    let statusCode = exception.extensions?.statusCode || 500;
    let message = exception.message;
    let response = null;

    if (exception instanceof Error && 'getStatus' in exception) {
      const httpException = exception as any;
      statusCode = httpException.getStatus();
      response = httpException.getResponse();
      if (response && typeof response === 'object') {
        message = response.message || exception.message;
      }
    } else if (exception && typeof exception === 'object') {
      const status = exception.status || exception.statusCode || exception.extensions?.statusCode;
      if (status) {
        statusCode = status;
      }
      if (typeof exception.getResponse === 'function') {
        response = exception.getResponse();
      } else if (exception.response) {
        response = exception.response;
      }
      if (response && typeof response === 'object') {
        message = response.message || exception.message;
      }
    }

    const formattedError = ResponseFormatter.formatError({
      message,
      response,
      statusCode,
      extensions: exception.extensions,
    });

    return new GraphQLError(formattedError.message, {
      extensions: {
        ...exception.extensions,
        success: false,
        statusCode: formattedError.statusCode,
        timeStamp: formattedError.timeStamp,
        code: exception.extensions?.code || (statusCode === 400 ? 'BAD_REQUEST' : 'INTERNAL_SERVER_ERROR'),
        error: formattedError.error,
      },
    });
  }
}