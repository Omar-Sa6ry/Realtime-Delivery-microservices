import {
  Injectable,
  BadRequestException,
  UnauthorizedException,
} from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, DataSource } from 'typeorm';
import { User } from '../../common/database/entities/user.entity';
import { Outbox } from '../../common/database/entities/outbox.entity';
import { BcryptPasswordHasher } from '../../common/security/bcrypt-password.hasher';
import { JwtTokenProvider } from '../../common/security/jwt-token.provider';
import { RedisSessionRepository } from '../../common/database/repositories/redis-session.repository';
import { OutboxWorkerService } from '../../common/messaging/outbox-worker.service';
import { NotificationService, ChannelType } from '@bts-soft/core';
import { Role, rolePermissionsMap } from '@delivery/common';
import { UserFactory } from './user.factory';

@Injectable()
export class AuthService {
  constructor(
    @InjectRepository(User)
    private readonly userRepo: Repository<User>,
    private readonly passwordHasher: BcryptPasswordHasher,
    private readonly tokenProvider: JwtTokenProvider,
    private readonly sessionRepo: RedisSessionRepository,
    private readonly dataSource: DataSource,
    private readonly outboxWorkerService: OutboxWorkerService,
    private readonly notificationService: NotificationService,
    private readonly userFactory: UserFactory,
  ) {}

  async register(
    emailInput: string,
    passwordInput: string,
    firstName: string,
    lastName: string,
    phoneNumber?: string,
  ): Promise<any> {
    const email = emailInput.toLowerCase().trim();

    const [existingPhone, existingEmail] = await Promise.all([
      this.userRepo.findOne({ where: { phoneNumber } }),
      this.userRepo.findOne({ where: { email } }),
    ]);
    if (existingPhone) throw new BadRequestException('Email already exists');
    if (existingPhone) throw new BadRequestException('Phone already exists');

    const hashedPassword = await this.passwordHasher.hash(passwordInput);

    // Check if it's the first user to make them Admin
    const totalUsers = await this.userRepo.count();
    const role = totalUsers === 0 ? Role.ADMIN : Role.USER;

    const user = this.userFactory.createUser(
      email,
      hashedPassword,
      firstName,
      lastName,
      role,
      phoneNumber,
    );

    const queryRunner = this.dataSource.createQueryRunner();
    await queryRunner.connect();
    await queryRunner.startTransaction();

    let savedUser: User;
    let outboxId: string;

    try {
      savedUser = await queryRunner.manager.save(user);

      // Create outbox record
      const outbox = new Outbox();
      outbox.id = crypto.randomUUID();
      outbox.aggregateType = 'User';
      outbox.aggregateId = savedUser.id;
      outbox.eventType = 'user.registered';
      outbox.payload = {
        userId: savedUser.id,
        email: savedUser.email,
        firstName: savedUser.firstName,
        lastName: savedUser.lastName,
        role: savedUser.role,
      };
      outbox.processed = false;

      const savedOutbox = await queryRunner.manager.save(outbox);
      outboxId = savedOutbox.id;

      await queryRunner.commitTransaction();
    } catch (err) {
      await queryRunner.rollbackTransaction();
      throw err;
    } finally {
      await queryRunner.release();
    }

    // Enqueue event to Redis BullMQ
    await this.outboxWorkerService.enqueueEvent(outboxId);

    const sessionId = crypto.randomUUID();
    const permissions = rolePermissionsMap[savedUser.role] || [];

    const tokens = await this.tokenProvider.generateTokens({
      userId: savedUser.id,
      email: savedUser.email,
      role: savedUser.role,
      permissions,
      sessionId,
    });

    await this.sessionRepo.createSession(
      savedUser.id,
      sessionId,
      {
        userId: savedUser.id,
        email: savedUser.email,
        role: savedUser.role,
        sessionId,
        createdAt: new Date(),
      },
      tokens.expiresIn,
    );

    return {
      user: {
        id: savedUser.id,
        email: savedUser.email,
        firstName: savedUser.firstName,
        lastName: savedUser.lastName,
        role: savedUser.role,
        isActive: savedUser.isActive,
        createdAt: savedUser.createdAt,
      },
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }

  async login(emailInput: string, passwordInput: string): Promise<any> {
    const email = emailInput.toLowerCase().trim();
    const user = await this.userRepo.findOne({ where: { email } });
    if (!user) {
      throw new BadRequestException('Invalid credentials');
    }

    const isValidPassword = await this.passwordHasher.compare(
      passwordInput,
      user.passwordHash,
    );
    if (!isValidPassword) {
      throw new UnauthorizedException('Invalid credentials');
    }

    const sessionId = crypto.randomUUID();
    const permissions = rolePermissionsMap[user.role] || [];

    const tokens = await this.tokenProvider.generateTokens({
      userId: user.id,
      email: user.email,
      role: user.role,
      permissions,
      sessionId,
    });

    await this.sessionRepo.createSession(
      user.id,
      sessionId,
      {
        userId: user.id,
        email: user.email,
        role: user.role,
        sessionId,
        createdAt: new Date(),
      },
      tokens.expiresIn,
    );

    return {
      user: {
        id: user.id,
        email: user.email,
        firstName: user.firstName,
        lastName: user.lastName,
        role: user.role,
        isActive: user.isActive,
        createdAt: user.createdAt,
      },
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }

  async forgetPassword(emailInput: string): Promise<void> {
    const email = emailInput.toLowerCase().trim();
    const user = await this.userRepo.findOne({ where: { email } });
    if (!user) {
      // Return success silently for security (avoid user enumeration)
      return;
    }

    const token = Math.floor(100000 + Math.random() * 900000).toString(); // 6-digit code
    user.resetToken = token;
    user.resetTokenExpiry = new Date(Date.now() + 15 * 60 * 1000); // 15 mins expiry

    await this.userRepo.save(user);

    // Send email using @bts-soft/notifications from @bts-soft/core
    await this.notificationService.send(ChannelType.EMAIL, {
      recipientId: user.email,
      subject: 'Password Reset Code',
      body: 'Hi {{name}}, your password reset code is {{token}}.',
      context: { name: user.firstName, token },
    });
  }

  async resetPassword(token: string, passwordNew: string): Promise<void> {
    const user = await this.userRepo.findOne({ where: { resetToken: token } });
    if (
      !user ||
      !user.resetTokenExpiry ||
      user.resetTokenExpiry.getTime() < Date.now()
    ) {
      throw new BadRequestException('Invalid or expired reset token');
    }

    const hashedPassword = await this.passwordHasher.hash(passwordNew);
    user.passwordHash = hashedPassword;
    user.resetToken = undefined;
    user.resetTokenExpiry = undefined;

    await this.userRepo.save(user);
  }

  async logout(userId: string, sessionId: string): Promise<void> {
    await this.sessionRepo.revokeSession(userId, sessionId);
  }

  async refreshToken(refreshToken: string): Promise<any> {
    const payload = await this.tokenProvider.verifyRefreshToken(refreshToken);
    if (!payload || !payload.sessionId) {
      throw new UnauthorizedException('Invalid or expired refresh token');
    }

    const session = await this.sessionRepo.getSession(
      payload.userId,
      payload.sessionId,
    );
    if (!session) {
      throw new UnauthorizedException('Session expired or logged out');
    }

    const user = await this.userRepo.findOne({ where: { id: payload.userId } });
    if (!user) {
      throw new UnauthorizedException('User not found');
    }

    const permissions = rolePermissionsMap[user.role] || [];

    const tokens = await this.tokenProvider.generateTokens({
      userId: user.id,
      email: user.email,
      role: user.role,
      permissions,
      sessionId: payload.sessionId,
    });

    // Refresh the session in Redis with same creation timestamp and updated expiry
    await this.sessionRepo.createSession(
      user.id,
      payload.sessionId,
      {
        userId: user.id,
        email: user.email,
        role: user.role,
        sessionId: payload.sessionId,
        createdAt: session.createdAt,
      },
      tokens.expiresIn,
    );

    return {
      user: {
        id: user.id,
        email: user.email,
        firstName: user.firstName,
        lastName: user.lastName,
        role: user.role,
        isActive: user.isActive,
        createdAt: user.createdAt,
      },
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }
}
