import { gql } from '@apollo/client';
import { client } from './client';
import type {
  AuthPayload,
  CompletedPart,
  UploadSession,
  User,
  PresignedPart,
} from '../upload/types';

const REGISTER = gql`
  mutation Register($input: RegisterInput!) {
    register(input: $input) {
      success
      statusCode
      message
      data {
        accessToken
        refreshToken
        user {
          id
          email
          firstName
          lastName
          role
          imageUrl
          phoneNumber
        }
      }
    }
  }
`;

const LOGIN = gql`
  mutation Login($input: LoginInput!) {
    login(input: $input) {
      success
      statusCode
      message
      data {
        accessToken
        refreshToken
        user {
          id
          email
          firstName
          lastName
          role
          imageUrl
        }
      }
    }
  }
`;

const CREATE_UPLOAD_SESSION = gql`
  mutation CreateSession($input: CreateUploadSessionInput!) {
    createUploadSession(input: $input) {
      success
      message
      data {
        mediaId
        uploadId
        s3UploadId
        partSize
        totalParts
        expiresAt
        presignedParts {
          partNumber
          presignedUrl
        }
      }
    }
  }
`;

const COMPLETE_UPLOAD = gql`
  mutation CompleteUpload($input: CompleteUploadInput!) {
    completeUpload(input: $input) {
      success
      message
      data {
        mediaId
        status
      }
    }
  }
`;

const UPDATE_PROFILE = gql`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input) {
      success
      statusCode
      message
      data {
        id
        email
        firstName
        lastName
        imageUrl
      }
    }
  }
`;

const MY_PROFILE = gql`
  query Me {
    myProfile {
      success
      message
      data {
        id
        email
        firstName
        lastName
        imageUrl
      }
    }
  }
`;

const ABORT_UPLOAD = gql`
  mutation AbortUpload($uploadId: String!) {
    abortUpload(uploadId: $uploadId) {
      success
    }
  }
`;

const DELETE_MEDIA = gql`
  mutation DeleteMedia($mediaId: String!, $idempotencyKey: String) {
    deleteMedia(mediaId: $mediaId, idempotencyKey: $idempotencyKey) {
      success
    }
  }
`;

const DOWNLOAD_URL = gql`
  query DownloadUrl($mediaId: String!, $versionType: String, $expirySeconds: Int) {
    downloadUrl(mediaId: $mediaId, versionType: $versionType, expirySeconds: $expirySeconds) {
      success
      message
      data {
        url
        expiresAt
        contentType
      }
    }
  }
`;

const UPLOAD_STATUS = gql`
  query UploadStatus($uploadId: String!) {
    uploadStatus(uploadId: $uploadId) {
      success
      message
      data {
        uploadId
        status
        totalParts
        completedParts
        missingParts
        expiresAt
      }
    }
  }
`;

export interface RegisterArgs {
  email: string;
  password: string;
  firstName: string;
  lastName: string;
  phoneNumber?: string;
}

export interface ApiErrorPayload {
  message: string;
}

function extractError(err: unknown): string {
  if (err instanceof Error) {
    const gqlErr = (err as any).graphQLErrors?.[0];
    const msg = gqlErr?.message || err.message;
    if (gqlErr?.extensions?.exception?.response?.message) {
      const inner = gqlErr.extensions.exception.response.message;
      return Array.isArray(inner) ? inner.join(', ') : String(inner);
    }
    return msg;
  }
  return 'Unknown error';
}

export async function register(input: RegisterArgs): Promise<AuthPayload> {
  try {
    const res = await client.mutate<{ register: { data: AuthPayload } }>({
      mutation: REGISTER,
      variables: { input },
    });
    if (!res.data?.register?.data) throw new Error('register returned no data');
    return res.data.register.data;
  } catch (err) {
    throw new Error(extractError(err));
  }
}

export async function login(input: {
  email: string;
  password: string;
}): Promise<AuthPayload> {
  try {
    const res = await client.mutate<{ login: { data: AuthPayload } }>({
      mutation: LOGIN,
      variables: { input },
    });
    if (!res.data?.login?.data) throw new Error('login returned no data');
    return res.data.login.data;
  } catch (err) {
    throw new Error(extractError(err));
  }
}

export interface CreateSessionArgs {
  fileName: string;
  contentType: string;
  size: number;
  checksum?: string;
}

export async function createUploadSession(
  input: CreateSessionArgs,
): Promise<UploadSession> {
  try {
    const res = await client.mutate<{
      createUploadSession: { data: UploadSession };
    }>({
      mutation: CREATE_UPLOAD_SESSION,
      variables: { input },
    });
    if (!res.data?.createUploadSession?.data) throw new Error('no upload session');
    return res.data.createUploadSession.data;
  } catch (err) {
    throw new Error(extractError(err));
  }
}

export async function completeUpload(
  uploadId: string,
  parts: CompletedPart[],
): Promise<{ mediaId: string; status: string }> {
  try {
    const res = await client.mutate<{
      completeUpload: { data: { mediaId: string; status: string } };
    }>({
      mutation: COMPLETE_UPLOAD,
      variables: { input: { uploadId, parts } },
    });
    if (!res.data?.completeUpload?.data) throw new Error('complete returned no data');
    return res.data.completeUpload.data;
  } catch (err) {
    throw new Error(extractError(err));
  }
}

export async function updateProfileAvatar(
  avatarMediaId: string,
): Promise<User> {
  try {
    const res = await client.mutate<{
      updateProfile: { message?: string; data: User };
    }>({
      mutation: UPDATE_PROFILE,
      variables: { input: { avatarMediaId } },
    });
    if (!res.data?.updateProfile?.data) {
      throw new Error(res.data?.updateProfile?.message || 'update failed');
    }
    return res.data.updateProfile.data;
  } catch (err) {
    throw new Error(extractError(err));
  }
}

export async function fetchMyProfile(): Promise<User> {
  try {
    const res = await client.query<{ myProfile: { data: User } }>({
      query: MY_PROFILE,
      fetchPolicy: 'network-only',
    });
    if (!res.data?.myProfile?.data) throw new Error('myProfile returned no data');
    return res.data.myProfile.data;
  } catch (err) {
    throw new Error(extractError(err));
  }
}

export async function abortUpload(uploadId: string): Promise<boolean> {
  try {
    const res = await client.mutate<{ abortUpload: { success: boolean } }>({
      mutation: ABORT_UPLOAD,
      variables: { uploadId },
    });
    return res.data?.abortUpload?.success || false;
  } catch (err) {
    throw new Error(extractError(err));
  }
}

export async function deleteMedia(mediaId: string, idempotencyKey?: string): Promise<boolean> {
  try {
    const res = await client.mutate<{ deleteMedia: { success: boolean } }>({
      mutation: DELETE_MEDIA,
      variables: { mediaId, idempotencyKey },
    });
    return res.data?.deleteMedia?.success || false;
  } catch (err) {
    throw new Error(extractError(err));
  }
}

export interface DownloadUrlData {
  url: string;
  expiresAt: number;
  contentType: string;
}

export async function getDownloadUrl(mediaId: string, versionType?: string, expirySeconds?: number): Promise<DownloadUrlData> {
  try {
    const res = await client.query<{ downloadUrl: { data: DownloadUrlData } }>({
      query: DOWNLOAD_URL,
      variables: { mediaId, versionType, expirySeconds },
      fetchPolicy: 'network-only',
    });
    if (!res.data?.downloadUrl?.data) throw new Error('downloadUrl query returned no data');
    return res.data.downloadUrl.data;
  } catch (err) {
    throw new Error(extractError(err));
  }
}

export interface BackendUploadStatus {
  uploadId: string;
  status: string;
  totalParts: number;
  completedParts: number;
  missingParts: number[];
  expiresAt: number;
}

export async function getUploadStatus(uploadId: string): Promise<BackendUploadStatus> {
  try {
    const res = await client.query<{ uploadStatus: { data: BackendUploadStatus } }>({
      query: UPLOAD_STATUS,
      variables: { uploadId },
      fetchPolicy: 'network-only',
    });
    if (!res.data?.uploadStatus?.data) throw new Error('uploadStatus query returned no data');
    return res.data.uploadStatus.data;
  } catch (err) {
    throw new Error(extractError(err));
  }
}


export type { PresignedPart };