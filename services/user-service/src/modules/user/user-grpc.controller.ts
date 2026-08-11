import { Controller } from '@nestjs/common';
import { GrpcMethod } from '@nestjs/microservices';
import { UserService } from './user.service';
import { JwtTokenProvider } from '../../common/security/jwt-token.provider';
import { USER_SERVICE_NAME } from '@delivery/common';
import type {
  GetUserRequest,
  GetUserResponse,
  ValidateTokenRequest,
  ValidateTokenResponse,
  GetUserPermissionsRequest,
  GetUserPermissionsResponse,
} from '@delivery/common';

@Controller()
export class UserGrpcController {
  constructor(
    private readonly userService: UserService,
    private readonly tokenProvider: JwtTokenProvider,
  ) {}

  @GrpcMethod(USER_SERVICE_NAME, 'GetUser')
  async getUser(data: GetUserRequest): Promise<Partial<GetUserResponse>> {
    try {
      const user = await this.userService.findById(data.id);
      if (!user) return {};
      return {
        id: user.id,
        email: user.email,
        role: user.role,
        first_name: user.firstName,
        last_name: user.lastName,
        is_active: user.isActive,
      };
    } catch {
      return {};
    }
  }

  @GrpcMethod(USER_SERVICE_NAME, 'ValidateToken')
  async validateToken(data: ValidateTokenRequest): Promise<ValidateTokenResponse> {
    const payload = await this.tokenProvider.verifyAccessToken(data.token);
    if (!payload) {
      return { valid: false, user_id: '', role: '' };
    }
    return {
      valid: true,
      user_id: payload.userId,
      role: payload.role,
    };
  }

  @GrpcMethod(USER_SERVICE_NAME, 'GetUserPermissions')
  async getUserPermissions(data: GetUserPermissionsRequest): Promise<GetUserPermissionsResponse> {
    try {
      await this.userService.findById(data.user_id);
      return { permissions: [] };
    } catch {
      return { permissions: [] };
    }
  }
}
