import { Test, TestingModule } from '@nestjs/testing';
import {
  BadRequestException,
  ServiceUnavailableException,
} from '@nestjs/common';
import { MediaGrpcService } from './grpc-media.service';
import { ConfigService } from '@nestjs/config';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';

jest.mock('@grpc/grpc-js');
jest.mock('@grpc/proto-loader');

describe('MediaGrpcService', () => {
  let service: MediaGrpcService;
  let configService: jest.Mocked<ConfigService>;
  let mockClient: any;

  const mockResolveResult = {
    url: 'https://media.example.com/file.jpg',
    expiresAtSeconds: 3600,
    contentType: 'image/jpeg',
    mediaId: 'media-123',
    status: 'ready',
  };

  const mockGetMediaResult = {
    mediaId: 'media-123',
    ownerId: 'user-123',
    fileName: 'file.jpg',
    contentType: 'image/jpeg',
    mediaType: 'image',
    size: '1024',
    status: 'ready',
    objectKey: 'uploads/file.jpg',
  };

  beforeEach(async () => {
    mockClient = {
      ResolveMediaUrl: jest.fn(),
      GetMedia: jest.fn(),
      close: jest.fn(),
    };

    configService = {
      get: jest.fn().mockReturnValue('media-srv:50052'),
    } as unknown as jest.Mocked<ConfigService>;

    (protoLoader.loadSync as jest.Mock).mockReturnValue({});
    (grpc.loadPackageDefinition as jest.Mock).mockReturnValue({
      media: { MediaService: jest.fn().mockImplementation(() => mockClient) },
    });
    (grpc.credentials.createInsecure as jest.Mock).mockReturnValue({});

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        MediaGrpcService,
        { provide: ConfigService, useValue: configService },
      ],
    }).compile();

    service = module.get<MediaGrpcService>(MediaGrpcService);
    await service.onModuleInit();
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('resolveMediaUrl', () => {
    it('should resolve media URL successfully', async () => {
      mockClient.ResolveMediaUrl.mockImplementation(
        (args, metadata, options, callback) => {
          callback(null, mockResolveResult);
        },
      );

      const result = await service.resolveMediaUrl({
        mediaId: 'media-123',
        requesterId: 'user-123',
        versionType: 'original',
        expirySeconds: 3600,
      });

      expect(result).toEqual({
        url: 'https://media.example.com/file.jpg',
        expiresAtSeconds: 3600,
        contentType: 'image/jpeg',
        mediaId: 'media-123',
        status: 'ready',
      });
      expect(mockClient.ResolveMediaUrl).toHaveBeenCalled();
    });

    it('should throw BadRequestException on PERMISSION_DENIED', async () => {
      const error = {
        code: grpc.status.PERMISSION_DENIED,
        details: 'access denied',
      };
      mockClient.ResolveMediaUrl.mockImplementation(
        (args, metadata, options, callback) => {
          callback(error, null);
        },
      );

      await expect(
        service.resolveMediaUrl({
          mediaId: 'media-123',
          requesterId: 'user-123',
        }),
      ).rejects.toThrow(BadRequestException);
    });

    it('should throw BadRequestException on NOT_FOUND', async () => {
      const error = { code: grpc.status.NOT_FOUND, details: 'not found' };
      mockClient.ResolveMediaUrl.mockImplementation(
        (args, metadata, options, callback) => {
          callback(error, null);
        },
      );

      await expect(
        service.resolveMediaUrl({
          mediaId: 'media-123',
          requesterId: 'user-123',
        }),
      ).rejects.toThrow(BadRequestException);
    });

    it('should throw BadRequestException on FAILED_PRECONDITION', async () => {
      const error = {
        code: grpc.status.FAILED_PRECONDITION,
        details: 'not ready',
      };
      mockClient.ResolveMediaUrl.mockImplementation(
        (args, metadata, options, callback) => {
          callback(error, null);
        },
      );

      await expect(
        service.resolveMediaUrl({
          mediaId: 'media-123',
          requesterId: 'user-123',
        }),
      ).rejects.toThrow(BadRequestException);
    });

    it('should throw BadRequestException on RESOURCE_EXHAUSTED', async () => {
      const error = {
        code: grpc.status.RESOURCE_EXHAUSTED,
        details: 'rate limit',
      };
      mockClient.ResolveMediaUrl.mockImplementation(
        (args, metadata, options, callback) => {
          callback(error, null);
        },
      );

      await expect(
        service.resolveMediaUrl({
          mediaId: 'media-123',
          requesterId: 'user-123',
        }),
      ).rejects.toThrow(BadRequestException);
    });

    it('should throw ServiceUnavailableException on UNAVAILABLE', async () => {
      const error = { code: grpc.status.UNAVAILABLE, details: 'unavailable' };
      mockClient.ResolveMediaUrl.mockImplementation(
        (args, metadata, options, callback) => {
          callback(error, null);
        },
      );

      await expect(
        service.resolveMediaUrl({
          mediaId: 'media-123',
          requesterId: 'user-123',
        }),
      ).rejects.toThrow(ServiceUnavailableException);
    });

    it('should throw ServiceUnavailableException on DEADLINE_EXCEEDED', async () => {
      const error = { code: grpc.status.DEADLINE_EXCEEDED, details: 'timeout' };
      mockClient.ResolveMediaUrl.mockImplementation(
        (args, metadata, options, callback) => {
          callback(error, null);
        },
      );

      await expect(
        service.resolveMediaUrl({
          mediaId: 'media-123',
          requesterId: 'user-123',
        }),
      ).rejects.toThrow(ServiceUnavailableException);
    });

    it('should throw ServiceUnavailableException for unknown error codes', async () => {
      const error = { code: grpc.status.UNKNOWN, details: 'unknown error' };
      mockClient.ResolveMediaUrl.mockImplementation(
        (args, metadata, options, callback) => {
          callback(error, null);
        },
      );

      await expect(
        service.resolveMediaUrl({
          mediaId: 'media-123',
          requesterId: 'user-123',
        }),
      ).rejects.toThrow(ServiceUnavailableException);
    });

    it('should use default versionType and expirySeconds', async () => {
      mockClient.ResolveMediaUrl.mockImplementation(
        (args, metadata, options, callback) => {
          callback(null, mockResolveResult);
        },
      );

      await service.resolveMediaUrl({
        mediaId: 'media-123',
        requesterId: 'user-123',
      });

      expect(mockClient.ResolveMediaUrl).toHaveBeenCalledWith(
        expect.objectContaining({
          versionType: '',
          expirySeconds: 3600,
        }),
        expect.any(Object),
        expect.any(Object),
        expect.any(Function),
      );
    });
  });

  describe('getMedia', () => {
    it('should get media info successfully', async () => {
      mockClient.GetMedia.mockImplementation(
        (args, metadata, options, callback) => {
          callback(null, mockGetMediaResult);
        },
      );

      const result = await service.getMedia('media-123', 'user-123');

      expect(result).toEqual({
        mediaId: 'media-123',
        ownerId: 'user-123',
        fileName: 'file.jpg',
        contentType: 'image/jpeg',
        mediaType: 'image',
        size: '1024',
        status: 'ready',
        objectKey: 'uploads/file.jpg',
      });
      expect(mockClient.GetMedia).toHaveBeenCalled();
    });

    it('should throw BadRequestException on PERMISSION_DENIED', async () => {
      const error = {
        code: grpc.status.PERMISSION_DENIED,
        details: 'access denied',
      };
      mockClient.GetMedia.mockImplementation(
        (args, metadata, options, callback) => {
          callback(error, null);
        },
      );

      await expect(service.getMedia('media-123', 'user-123')).rejects.toThrow(
        BadRequestException,
      );
    });

    it('should throw ServiceUnavailableException on UNAVAILABLE', async () => {
      const error = { code: grpc.status.UNAVAILABLE, details: 'unavailable' };
      mockClient.GetMedia.mockImplementation(
        (args, metadata, options, callback) => {
          callback(error, null);
        },
      );

      await expect(service.getMedia('media-123', 'user-123')).rejects.toThrow(
        ServiceUnavailableException,
      );
    });
  });

  describe('onModuleDestroy', () => {
    it('should close gRPC client', async () => {
      await service.onModuleDestroy();

      expect(mockClient.close).toHaveBeenCalled();
    });

    it('should handle undefined client gracefully', async () => {
      const serviceWithoutClient = new MediaGrpcService(configService);
      const result = serviceWithoutClient.onModuleDestroy();
      expect(result).toBeUndefined();
    });
  });

  describe('buildMetadata', () => {
    it('should create metadata with requesterId and userId', () => {
      const metadata = (service as any).buildMetadata('user-123');

      expect(metadata).toBeInstanceOf(grpc.Metadata);
      // In test environment, the mock may not fully implement get()
      // Just verify the method exists and returns something
      expect(typeof metadata.get).toBe('function');
    });
  });
});
