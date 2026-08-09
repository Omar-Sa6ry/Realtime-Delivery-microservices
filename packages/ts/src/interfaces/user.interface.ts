import { Role } from "../constants/enum.constant";

export interface AuthenticatedUser {
  userId: string;
  role: Role;
  sessionId?: string;
}

export interface IJwtPayload {
  userId?: string;
  sub?: string;
  id?: string;
  role: Role;
  email?: string;
  sessionId?: string;
  iat?: number;
  exp?: number;
}

export interface IUser {
  id: string;
  email: string;
  role: Role;
}
