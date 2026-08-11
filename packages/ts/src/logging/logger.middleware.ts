import { Injectable, NestMiddleware } from '@nestjs/common';
import { Request, Response, NextFunction } from 'express';
import { LoggerContext } from './logger.context';

@Injectable()
export class LoggerMiddleware implements NestMiddleware {
  use(req: Request, res: Response, next: NextFunction) {
    const traceId = (req.headers['x-trace-id'] || req.headers['x-request-id'] || crypto.randomUUID()) as string;
    const userId = req.headers['x-user-id'] as string;
    
    res.setHeader('x-trace-id', traceId);

    const contextStore = {
      traceId,
      userId,
      method: req.method,
      path: req.baseUrl + req.path,
    };

    LoggerContext.run(contextStore, () => {
      next();
    });
  }
}
