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
import { Role, rolePermissionsMap, KafkaService, UserKafkaTopics } from '@delivery/common';
import { UserFactory } from './user.factory';
import { I18nService } from 'nestjs-i18n';
import {
  RegisterInput,
  LoginInput,
  ForgetPasswordInput,
  ResetPasswordInput,
  RefreshTokenInput,
  AuthPayloadType,
} from './dto/auth.types';

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
    private readonly kafkaService: KafkaService,
    private readonly userFactory: UserFactory,
    private readonly i18n: I18nService,
  ) {}

  async register(input: RegisterInput): Promise<AuthPayloadType> {
    const email = input.email.toLowerCase().trim();
    const { password, firstName, lastName, phoneNumber, imageUrl } = input;

    const [existingPhone, existingEmail] = await Promise.all([
      phoneNumber ? this.userRepo.findOne({ where: { phoneNumber } }) : Promise.resolve(null),
      this.userRepo.findOne({ where: { email } }),
    ]);
    if (existingEmail) throw new BadRequestException(this.i18n.t('user.EMAIL_EXISTED'));
    if (existingPhone) throw new BadRequestException(this.i18n.t('user.PHONE_EXISTED'));

    const hashedPassword = await this.passwordHasher.hash(password);

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
      imageUrl,
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
        imageUrl: savedUser.imageUrl,
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

    // Publish user.created event for the notification service (event-driven).
    // Fire-and-forget; consumer dedupes via the inbox pattern.
    this.kafkaService
      .emit(UserKafkaTopics.USER_CREATED, UserKafkaTopics.USER_CREATED, {
        userId: savedUser.id,
        email: savedUser.email,
        firstName: savedUser.firstName,
        lastName: savedUser.lastName,
        role: savedUser.role,
      })
      .catch((err) => console.error('Failed to publish user.created event:', err));

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
        imageUrl: savedUser.imageUrl,
        addresses: savedUser.addresses || [],
      },
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }

  async login(input: LoginInput): Promise<AuthPayloadType> {
    const email = input.email.toLowerCase().trim();
    const user = await this.userRepo.findOne({ where: { email } });
    if (!user) {
      throw new BadRequestException(this.i18n.t('user.INVALID_CREDENTIALS'));
    }

    const isValidPassword = await this.passwordHasher.compare(
      input.password,
      user.passwordHash,
    );
    if (!isValidPassword) {
      throw new UnauthorizedException(this.i18n.t('user.INVALID_CREDENTIALS'));
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
        imageUrl: user.imageUrl,
        addresses: user.addresses || [],
      },
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }

  async forgetPassword(input: ForgetPasswordInput): Promise<void> {
    const email = input.email.toLowerCase().trim();
    const user = await this.userRepo.findOne({ where: { email } });
    if (!user) {
      // Return success silently for security (avoid user enumeration)
      return;
    }

    const token = Math.floor(100000 + Math.random() * 900000).toString(); // 6-digit code
    user.resetToken = token;
    user.resetTokenExpiry = new Date(Date.now() + 15 * 60 * 1000); // 15 mins expiry

    await this.userRepo.save(user);

    this.notificationService.send(ChannelType.EMAIL, {
      recipientId: user.email,
      subject: 'Password Reset Code',
      body: 'Hi {{name}}, your password reset code is {{token}}.',
      context: { name: user.firstName, token },
    }).catch(err => console.error('Failed to send reset password email:', err));

    // Publish audit event (no token in payload) so notification-service keeps an in-app record
    this.kafkaService
      .emit(
        UserKafkaTopics.PASSWORD_RESET_REQUESTED,
        UserKafkaTopics.PASSWORD_RESET_REQUESTED,
        {
          userId: user.id,
          email: user.email,
          firstName: user.firstName,
        },
      )
      .catch(err => console.error('Failed to publish password_reset event:', err));
  }

  async resetPassword(input: ResetPasswordInput): Promise<void> {
    const user = await this.userRepo.findOne({ where: { resetToken: input.token } });
    if (
      !user ||
      !user.resetTokenExpiry ||
      user.resetTokenExpiry.getTime() < Date.now()
    ) {
      throw new BadRequestException(this.i18n.t('user.INVALID_RESET_TOKEN'));
    }

    const hashedPassword = await this.passwordHasher.hash(input.passwordNew);
    user.passwordHash = hashedPassword;
    user.resetToken = undefined;
    user.resetTokenExpiry = undefined;

    await this.userRepo.save(user);
  }

  async logout(userId: string, sessionId: string): Promise<void> {
    await this.sessionRepo.revokeSession(userId, sessionId);
  }

  async refreshToken(input: RefreshTokenInput): Promise<AuthPayloadType> {
    const payload = await this.tokenProvider.verifyRefreshToken(input.refreshToken);
    if (!payload || !payload.sessionId) {
      throw new UnauthorizedException(this.i18n.t('user.INVALID_TOKEN'));
    }

    const session = await this.sessionRepo.getSession(
      payload.userId,
      payload.sessionId,
    );
    if (!session) {
      throw new UnauthorizedException(this.i18n.t('user.SESSION_EXPIRED'));
    }

    const user = await this.userRepo.findOne({ where: { id: payload.userId } });
    if (!user) {
      throw new UnauthorizedException(this.i18n.t('user.NOT_FOUND'));
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
        imageUrl: user.imageUrl,
        addresses: user.addresses || [],
      },
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }
}
