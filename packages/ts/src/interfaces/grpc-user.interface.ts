export const USER_PACKAGE_NAME = 'user';
export const USER_SERVICE_NAME = 'UserService';

export interface GetUserRequest {
  id: string;
}

export interface GetUserByEmailRequest {
  email: string;
}

export interface GetUserResponse {
  id: string;
  email: string;
  role: string;
  first_name: string;
  last_name: string;
  is_active: boolean;
}

export interface ValidateTokenRequest {
  token: string;
}

export interface ValidateTokenResponse {
  valid: boolean;
  user_id: string;
  role: string;
}

export interface GetUserPermissionsRequest {
  user_id: string;
}

export interface GetUserPermissionsResponse {
  permissions: string[];
}
