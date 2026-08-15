import { Injectable, NestMiddleware } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { Request, Response, NextFunction } from 'express';

@Injectable()
export class JwtAuthMiddleware implements NestMiddleware {
  constructor(private readonly jwtService: JwtService) {}

  async use(req: Request, _res: Response, next: NextFunction): Promise<void> {
    const authHeader = req.headers.authorization;

    if (authHeader && authHeader.startsWith('Bearer ')) {
      const token = authHeader.split(' ')[1];
      try {
        const secret = process.env.JWT_SECRET || 'super-secret-jwt-key';
        const payload = await this.jwtService.verifyAsync(token, { secret });

        (req as Request & { user?: any }).user = {
          userId: payload.sub || payload.userId || payload.id,
          role: payload.role || 'USER',
          sessionId: payload.sessionId,
        };
      } catch {
        // Invalid/expired token: treat the request as anonymous and let the
        // downstream subgraph enforce authentication (public ops like
        // register/login must keep working even if a stale token is attached).
        (req as Request & { user?: any }).user = undefined;
      }
    }

    next();
  }
}
