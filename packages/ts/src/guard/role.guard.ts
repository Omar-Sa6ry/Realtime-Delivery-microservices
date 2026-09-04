import {
  CanActivate,
  ExecutionContext,
  Injectable,
  UnauthorizedException,
  Inject,
  Optional,
} from '@nestjs/common';
import { Reflector } from '@nestjs/core';
import { GqlExecutionContext } from '@nestjs/graphql';
import { JwtService } from '@nestjs/jwt';
import { I18nService } from 'nestjs-i18n';
import { Request } from 'express';
import { rolePermissionsMap } from '../constants/rolePermissionsMap.constant';
import { Permission, Role } from '../constants/enum.constant';
import { IJwtPayload, IUser } from '../interfaces/user.interface';

export interface IUserService {
  findById(id: string): Promise<IUser>;
}

@Injectable()
export class RoleGuard implements CanActivate {
  constructor(
    private readonly i18n: I18nService,
    private readonly jwtService: JwtService,
    private readonly reflector: Reflector,
    @Optional()
    @Inject('USER_SERVICE')
    private readonly userService?: IUserService,
  ) {}

  async canActivate(context: ExecutionContext): Promise<boolean> {
    if (!this.userService) {
      throw new Error('USER_SERVICE not provided in AuthCommonModule context');
    }

    const ctx = GqlExecutionContext.create(context);
    const request = ctx.getContext().req;

    const token = await this.extractTokenFromHeader(request);

    if (!token) {
      throw new UnauthorizedException(await this.i18n.t('user.NO_TOKEN'));
    }

    const requiredRoles = this.getRequiredRoles(context);
    const requiredPermissions = this.getRequiredPermissions(context);

    const payload = (await this.verifyToken(token)) as IJwtPayload;
    const userId = payload.sub || payload.id;

    if (!userId) {
      throw new UnauthorizedException(await this.i18n.t('user.INVALID_TOKEN'));
    }

    // Prefer JWT payload role if available to avoid unnecessary DB calls in microservices
    let userRole = payload.role as Role;
    
    // Fallback to user service if role is missing in token
    if (!userRole && this.userService) {
      try {
        const userResponse = await this.userService.findById(userId);
        const user = (userResponse as any).data || userResponse;
        userRole = user.role as Role;
      } catch (err) {
        // ignore
      }
    }

    const hasRole = this.validateRole(userRole, requiredRoles);
    const userPermissions = rolePermissionsMap[userRole] ?? [];
    const hasPermissions = this.validatePermissions(
      userPermissions,
      requiredPermissions,
    );

    if (!hasRole || !hasPermissions)
      throw new UnauthorizedException(
        await this.i18n.t('user.INSUFFICIENT_PERMISSIONS'),
      );

    request['user'] = {
      id: userId,
      email: payload.email,
      role: userRole,
      permissions: userPermissions,
      sessionId: payload.sessionId,
    };

    return true;
  }

  private extractTokenFromHeader(request: Request): string | null {
    const [type, token] = request.headers['authorization']?.split(' ') ?? [];
    return type === 'Bearer' ? token : null;
  }

  private async verifyToken(token: string): Promise<IJwtPayload> {
    try {
      return await this.jwtService.verifyAsync(token, {
        secret: process.env.JWT_SECRET,
      });
    } catch {
      throw new UnauthorizedException(await this.i18n.t('user.INVALID_TOKEN'));
    }
  }

  private getRequiredRoles(context: ExecutionContext): Role[] {
    return (
      this.reflector.getAllAndOverride<Role[]>('roles', [
        context.getHandler(),
        context.getClass(),
      ]) ?? []
    );
  }

  private getRequiredPermissions(context: ExecutionContext): Permission[] {
    return (
      this.reflector.getAllAndOverride<Permission[]>('permissions', [
        context.getHandler(),
        context.getClass(),
      ]) ?? []
    );
  }

  private validateRole(userRole: Role, requiredRoles: Role[]): boolean {
    return requiredRoles.length === 0 || requiredRoles.includes(userRole);
  }

  private validatePermissions(
    userPermissions: Permission[],
    required: Permission[],
  ): boolean {
    return (
      required.length === 0 ||
      required.every((p) => userPermissions.includes(p))
    );
  }
}
