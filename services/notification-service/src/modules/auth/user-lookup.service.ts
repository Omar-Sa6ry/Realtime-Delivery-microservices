import { Inject, Injectable, Logger, NotFoundException, OnModuleInit } from '@nestjs/common';
import type { ClientGrpc } from '@nestjs/microservices';
import {
  USER_SERVICE_NAME,
  GetUserResponse,
  Role,
  IUser,
} from '@delivery/common';

interface UserGrpcClient {
  getUser(request: { id: string }): Promise<Partial<GetUserResponse>>;
}


@Injectable()
export class UserLookupService implements OnModuleInit {
  private readonly logger = new Logger(UserLookupService.name);
  private userGrpcClient: UserGrpcClient;

  constructor(
    @Inject('GRPC_USER_SERVICE') private readonly grpcClient: ClientGrpc,
  ) {}

  onModuleInit() {
    this.userGrpcClient = this.grpcClient.getService<UserGrpcClient>(USER_SERVICE_NAME);
  }

  async findById(id: string): Promise<IUser> {
    let user: Partial<GetUserResponse>;
    try {
      user = await this.userGrpcClient.getUser({ id });
    } catch (error) {
      this.logger.error(`Failed to resolve user ${id} via gRPC`, error);
      throw new NotFoundException('user.USER_NOT_FOUND');
    }

    if (!user || !user.id) {
      throw new NotFoundException('user.USER_NOT_FOUND');
    }

    const resolved: IUser = {
      id: user.id,
      email: user.email || '',
      role: this.normalizeRole(user.role),
      firstName: user.first_name || '',
      lastName: user.last_name || '',
      isActive: user.is_active ?? true,
      isVerified: true,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    };

    return resolved;
  }

  private normalizeRole(role?: string): Role {
    if (role === 'customer') return Role.USER;
    return (role as Role) || Role.USER;
  }
}