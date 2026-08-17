import { WebSocket } from 'ws';
import { IJwtPayload, Role } from '@delivery/common';

export interface SocketData {
  socketId: string;
  nodeId: string;
  userId: string;
  userRole: Role;
  sessionId?: string;
  email?: string;
  connectedAt: number;
  lastPongAt: number;
  dirty: boolean;
  inboundMessage?: unknown;
}

export type AuthenticatedSocket = WebSocket & { data: SocketData };

export interface ConnectionContext {
  socketId: string;
  nodeId: string;
  userId: string;
  userRole: Role;
  sessionId?: string;
  email?: string;
  connectedAt: number;
  lastHeartbeatAt: number;
}

export const toContext = (
  socketId: string,
  nodeId: string,
  payload: IJwtPayload,
): ConnectionContext => ({
  socketId,
  nodeId,
  userId: payload.userId || payload.sub || (payload.id as string),
  userRole: payload.role,
  sessionId: payload.sessionId,
  email: payload.email,
  connectedAt: Date.now(),
  lastHeartbeatAt: Date.now(),
});

export const toSocketData = (
  ctx: ConnectionContext,
): SocketData => ({
  socketId: ctx.socketId,
  nodeId: ctx.nodeId,
  userId: ctx.userId,
  userRole: ctx.userRole,
  sessionId: ctx.sessionId,
  email: ctx.email,
  connectedAt: ctx.connectedAt,
  lastPongAt: Date.now(),
  dirty: false,
});