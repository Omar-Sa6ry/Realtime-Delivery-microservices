/**
 * Media service operations — mirrors graphql-docs/media.graphql (uploads,
 * presigned URLs, quota and DLQ administration).
 */
import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

const MEDIA_FIELDS = `
  mediaId
  ownerId
  fileName
  contentType
  mediaType
  size
  objectKey
  versions { versionType objectKey contentType size width height durationMs }
  status
  createdAt
`;

describe('Media service (graphql-docs/media.graphql)', () => {
  let h: GatewayHarness;
  let token: string;

  beforeAll(async () => {
    h = await createGatewayHarness();
    token = h.signToken({ sub: 'user-1', role: 'USER', sessionId: 'sess-1' });
  });

  afterAll(async () => {
    await h.close();
  });

  describe('listMedia (User / Driver / Admin)', () => {
    it('lists media with versions and pagination', async () => {
      const res = await h
        .gql(
          `query ListMedia {
            listMedia(ownerId: "364829287083151360", limit: 10, cursor: "") {
              success statusCode message timeStamp
              data {
                paginationInfo { totalItems currentPage nextPage }
                items { ${MEDIA_FIELDS} }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.listMedia.data;
      expect(data.paginationInfo.totalItems).toBe(1);
      expect(data.items[0]).toMatchObject({
        mediaId: 'media-1',
        ownerId: 'owner-1',
        fileName: 'sample.mp4',
        contentType: 'video/mp4',
        mediaType: 'video',
        size: 104857600,
        objectKey: 'media/media-1/original.mp4',
        status: 'READY',
      });
      expect(data.items[0].versions[0]).toMatchObject({
        versionType: 'thumbnail',
        contentType: 'image/jpeg',
        width: 320,
        height: 180,
      });
      expect(h.subgraphs.last('media')?.headers['x-user-id']).toBe('user-1');
    });
  });

  describe('media(mediaId) (User / Driver / Admin)', () => {
    it('fetches one media item', async () => {
      const res = await h
        .gql(
          `query GetMedia {
            media(mediaId: "66354b8c-4cec-45c8-afc0-46254e276663") {
              success statusCode message timeStamp data { ${MEDIA_FIELDS} }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.media.data.mediaId).toBe(
        '66354b8c-4cec-45c8-afc0-46254e276663',
      );
      expect(res.body.data.media.data.versions).toHaveLength(1);
    });
  });

  describe('upload lifecycle', () => {
    it('createUploadSession returns presigned parts', async () => {
      const res = await h
        .gql(
          `mutation CreateUploadSession {
            createUploadSession(input: {
              fileName: "sample.mp4"
              contentType: "video/mp4"
              size: 104857600
              checksum: "sha256-xyz"
              deliveryId: "delivery-123"
              idempotencyKey: "upload-key-123"
            }) {
              success statusCode message timeStamp
              data {
                mediaId uploadId s3UploadId
                presignedParts { partNumber presignedUrl }
                partSize totalParts expiresAt
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.createUploadSession.data).toMatchObject({
        mediaId: 'media-1',
        uploadId: 'upload-001',
        s3UploadId: 's3-upload-1',
        partSize: 5242880,
        totalParts: 1,
      });
      expect(res.body.data.createUploadSession.data.presignedParts[0]).toEqual({
        partNumber: 1,
        presignedUrl: 'https://s3.example.com/part1',
      });
    });

    it('uploadStatus reports missing parts', async () => {
      const res = await h
        .gql(
          `query GetUploadStatus {
            uploadStatus(uploadId: "upload-001") {
              success statusCode message timeStamp
              data { uploadId status totalParts completedParts missingParts expiresAt }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.uploadStatus.data).toMatchObject({
        uploadId: 'upload-001',
        status: 'IN_PROGRESS',
        totalParts: 3,
        completedParts: 1,
        missingParts: [2, 3],
      });
    });

    it('completeUpload finalizes the session', async () => {
      const res = await h
        .gql(
          `mutation CompleteUpload {
            completeUpload(input: {
              uploadId: "upload-001"
              parts: [{ partNumber: 1, eTag: "etag-001" }]
              idempotencyKey: "complete-key-123"
            }) {
              success statusCode message timeStamp data { mediaId status }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.completeUpload.data).toEqual({
        mediaId: 'media-1',
        status: 'READY',
      });
    });

    it('renewPresigned refreshes upload URLs', async () => {
      const res = await h
        .gql(
          `mutation RenewPresigned {
            renewPresigned(input: { uploadId: "upload-001", expirySeconds: 3600 }) {
              success statusCode message timeStamp
              data { uploadId s3UploadId presignedParts { partNumber presignedUrl } partSize totalParts expiresAt }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.renewPresigned.data.uploadId).toBe('upload-001');
    });

    it('abortUpload cancels the session', async () => {
      const res = await h
        .gql(
          `mutation AbortUpload { abortUpload(uploadId: "upload-001") { success statusCode message timeStamp data } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.abortUpload.data).toBe(true);
    });

    it('deleteMedia removes a media file', async () => {
      const res = await h
        .gql(
          `mutation DeleteMedia {
            deleteMedia(mediaId: "66354b8c-4cec-45c8-afc0-46254e276663", idempotencyKey: "del-001") {
              success statusCode message timeStamp data
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.deleteMedia).toMatchObject({
        success: true,
        statusCode: 200,
        data: true,
      });
    });

    it('requires fileName/contentType/size for an upload session', async () => {
      const res = await h
        .gql(
          `mutation { createUploadSession(input: { fileName: "a" }) { statusCode } }`,
        )
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });
  });

  describe('download & quota (User / Driver / Admin)', () => {
    it('downloadUrl returns a presigned URL', async () => {
      const res = await h
        .gql(
          `query GetDownloadUrl {
            downloadUrl(mediaId: "66354b8c-4cec-45c8-afc0-46254e276663", versionType: "original", expirySeconds: 3600) {
              success statusCode message timeStamp data { url expiresAt contentType }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.downloadUrl.data).toMatchObject({
        url: 'https://s3.example.com/download',
        contentType: 'video/mp4',
      });
    });

    it('quota reports usage against the limit', async () => {
      const res = await h
        .gql(
          `query GetQuota {
            quota { success statusCode message timeStamp
              data { usedBytes quotaBytes remainingBytes activeUploads maxConcurrentUploads } }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.quota.data).toEqual({
        usedBytes: 1024,
        quotaBytes: 10240,
        remainingBytes: 9216,
        activeUploads: 1,
        maxConcurrentUploads: 3,
      });
    });
  });

  describe('DLQ administration (Admin)', () => {
    it('dlqTopics returns the topic list (bare list field)', async () => {
      const res = await h.gql(`{ dlqTopics }`).expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.dlqTopics).toEqual(['media-events-dlq']);
    });

    it('dlqStats reports message counts', async () => {
      const res = await h
        .gql(
          `query GetDLQStats {
            dlqStats(topics: ["media-events-dlq"]) {
              success statusCode message timeStamp data { topic messageCount }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.dlqStats.data).toEqual({
        topic: 'media-events-dlq',
        messageCount: 2,
      });
    });

    it('dlqReplay replays messages', async () => {
      const res = await h
        .gql(
          `mutation ReplayDLQ {
            dlqReplay(topic: "media-events-dlq", maxMessages: 50) {
              success statusCode message timeStamp data { success replayedCount errors }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.dlqReplay.data).toMatchObject({
        success: true,
        replayedCount: 5,
        errors: [],
      });
    });

    it('requires the topic for dlqReplay', async () => {
      const res = await h
        .gql(`mutation { dlqReplay { statusCode } }`)
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });
  });

  it('routes every media operation to the media subgraph', async () => {
    h.subgraphs.clear();

    await h.gql(`{ listMedia { success } }`).expect(200);
    await h.gql(`{ media(mediaId: "1") { success } }`).expect(200);
    await h.gql(`{ uploadStatus(uploadId: "1") { success } }`).expect(200);
    await h.gql(`{ downloadUrl(mediaId: "1") { success } }`).expect(200);
    await h.gql(`{ quota { success } }`).expect(200);
    await h.gql(`{ dlqTopics }`).expect(200);
    await h.gql(`{ dlqStats(topics: ["t"]) { success } }`).expect(200);
    await h
      .gql(`mutation { deleteMedia(mediaId: "1") { success } }`)
      .expect(200);
    await h
      .gql(`mutation { abortUpload(uploadId: "1") { success } }`)
      .expect(200);
    await h
      .gql(
        `mutation { createUploadSession(input: { fileName: "f", contentType: "t", size: 1 }) { success } }`,
      )
      .expect(200);
    await h
      .gql(
        `mutation { completeUpload(input: { uploadId: "1", parts: [{ partNumber: 1 }] }) { success } }`,
      )
      .expect(200);
    await h.gql(`mutation { dlqReplay(topic: "t") { success } }`).expect(200);
    await h
      .gql(`mutation { renewPresigned(input: { uploadId: "1" }) { success } }`)
      .expect(200);

    const fields = new Set(
      h.subgraphs.forSubgraph('media').flatMap((r) => r.fields),
    );
    for (const f of [
      'listMedia',
      'media',
      'uploadStatus',
      'downloadUrl',
      'quota',
      'dlqTopics',
      'dlqStats',
      'deleteMedia',
      'abortUpload',
      'createUploadSession',
      'completeUpload',
      'dlqReplay',
      'renewPresigned',
    ]) {
      expect(fields).toContain(f);
    }
  });
});
