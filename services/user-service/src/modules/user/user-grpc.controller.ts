import { Controller } from '@nestjs/common';
import { GrpcMethod, RpcException } from '@nestjs/microservices';
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
  UpdateUserRoleRequest,
  UpdateUserRoleResponse,
} from '@delivery/common';

@Controller()
export class UserGrpcController {
  constructor(
    private readonly userService: UserService,
    private readonly tokenProvider: JwtTokenProvider,
  ) {}

  @GrpcMethod(USER_SERVICE_NAME, 'GetUser')
  async getUser(data: GetUserRequest): Promise<Partial<GetUserResponse>> {
    const user = await this.userService.findById(data.id);
    if (!user) {
      throw new RpcException({
        code: 5, // NOT_FOUND
        message: `User with ID ${data.id} not found`,
      });
    }
    return {
      id: user.id,
      email: user.email,
      role: user.role,
      first_name: user.firstName,
      last_name: user.lastName,
      is_active: user.isActive,
    };
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

  @GrpcMethod(USER_SERVICE_NAME, 'UpdateUserRole')
  async updateUserRole(data: UpdateUserRoleRequest): Promise<UpdateUserRoleResponse> {
    try {
      await this.userService.updateUserRole(data.user_id, data.role);
      return {
        success: true,
        message: `Role updated to ${data.role}`,
      };
    } catch (err: any) {
      return {
        success: false,
        message: err?.message || 'Failed to update user role',
      };
    }
  }
}

