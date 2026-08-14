import {
  Injectable,
  OnModuleInit,
  OnModuleDestroy,
  BadRequestException,
  ServiceUnavailableException,
} from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import { join } from 'path';
import { promisify } from 'util';

export interface ResolveMediaUrlArgs {
  mediaId: string;
  requesterId: string;
  versionType?: string;
  expirySeconds?: number;
}

export interface ResolveMediaUrlResult {
  url: string;
  expiresAtSeconds: number;
  contentType: string;
  mediaId: string;
  status: string;
}

export interface GetMediaResult {
  mediaId: string;
  ownerId: string;
  fileName: string;
  contentType: string;
  mediaType: string;
  size: string;
  status: string;
  objectKey: string;
}

const GRPC_METADATA_KEYS = {
  requesterId: 'x-requester-id',
  userId: 'x-user-id',
  correlationId: 'x-correlation-id',
} as const;

@Injectable()
export class MediaGrpcService implements OnModuleInit, OnModuleDestroy {
  private client: any;
  private readonly address: string;
  private readonly protoPath: string;

  constructor(configService: ConfigService) {
    this.address = configService.get<string>('MEDIA_SERVICE_GRPC_URL') || 'media-srv:50052';
    this.protoPath = join(process.cwd(), '../../protos/media.proto');
  }

  onModuleInit(): void {
    const packageDefinition = protoLoader.loadSync(this.protoPath, {
      keepCase: false,
      longs: String,
      enums: String,
      defaults: true,
      oneofs: true,
    });
    const proto = grpc.loadPackageDefinition(packageDefinition) as any;
    const MediaServiceCtor = proto.media?.MediaService;
    if (!MediaServiceCtor) {
      throw new Error('media.proto must define package "media" service "MediaService"');
    }
    this.client = new MediaServiceCtor(this.address, grpc.credentials.createInsecure());
  }

  onModuleDestroy(): void {
    this.client?.close();
  }

  async resolveMediaUrl(args: ResolveMediaUrlArgs): Promise<ResolveMediaUrlResult> {
    const call = promisify(this.client.ResolveMediaUrl.bind(this.client));
    const deadline = new Date(Date.now() + 5000);

    try {
      const res = await call(
        {
          mediaId: args.mediaId,
          requesterId: args.requesterId,
          versionType: args.versionType || '',
          expirySeconds: args.expirySeconds || 3600,
        },
        this.buildMetadata(args.requesterId),
        { deadline },
      );
      return {
        url: String(res.url),
        expiresAtSeconds: Number(res.expiresAtSeconds),
        contentType: String(res.contentType || ''),
        mediaId: String(res.mediaId || args.mediaId),
        status: String(res.status || ''),
      };
    } catch (err) {
      this.mapError(err, 'resolve media url');
    }
  }

  async getMedia(mediaId: string, requesterId: string): Promise<GetMediaResult> {
    const call = promisify(this.client.GetMedia.bind(this.client));
    const deadline = new Date(Date.now() + 5000);

    try {
      const res = await call(
        { mediaId, requesterId },
        this.buildMetadata(requesterId),
        { deadline },
      );
      return {
        mediaId: String(res.mediaId),
        ownerId: String(res.ownerId),
        fileName: String(res.fileName),
        contentType: String(res.contentType),
        mediaType: String(res.mediaType),
        size: String(res.size),
        status: String(res.status),
        objectKey: String(res.objectKey),
      };
    } catch (err) {
      this.mapError(err, 'get media');
    }
  }

  private buildMetadata(requesterId: string): grpc.Metadata {
    const metadata = new grpc.Metadata();
    metadata.set(GRPC_METADATA_KEYS.requesterId, requesterId);
    metadata.set(GRPC_METADATA_KEYS.userId, requesterId);
    return metadata;
  }

  private mapError(err: any, operation: string): never {
    const code = err?.code;
    const message = String(err?.details || err?.message || 'unknown');
    if (code === grpc.status.PERMISSION_DENIED) {
      throw new BadRequestException('media link rejected: access denied on media-service');
    }
    if (code === grpc.status.NOT_FOUND) {
      throw new BadRequestException('media link rejected: media not found');
    }
    if (code === grpc.status.FAILED_PRECONDITION) {
      throw new BadRequestException(`media not ready: ${message}`);
    }
    if (code === grpc.status.RESOURCE_EXHAUSTED) {
      throw new BadRequestException('media rate limit exceeded');
    }
    if (code === grpc.status.UNAVAILABLE || code === grpc.status.DEADLINE_EXCEEDED) {
      throw new ServiceUnavailableException(`media-service unreachable (${operation})`);
    }
    throw new ServiceUnavailableException(`media-service call failed (${operation})`);
  }
}
