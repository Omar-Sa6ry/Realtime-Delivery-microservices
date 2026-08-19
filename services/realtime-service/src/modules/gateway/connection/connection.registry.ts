import { Injectable, Logger } from '@nestjs/common';
import type { WebSocket } from 'ws';
import {
  SocketData,
  toSocketData,
  toContext,
  ConnectionContext,
} from './connection.types';
import { IJwtPayload } from '@delivery/common';

@Injectable()
export class ConnectionRegistry {
  private readonly logger = new Logger(ConnectionRegistry.name);
  private readonly sockets = new Map<
    string,
    WebSocket & { data: SocketData }
  >();

  get size(): number {
    return this.sockets.size;
  }

  register(socket: WebSocket & { data: SocketData }): void {
    this.sockets.set(socket.data.socketId, socket);
  }

  get(socketId: string): (WebSocket & { data: SocketData }) | undefined {
    return this.sockets.get(socketId);
  }

  has(socketId: string): boolean {
    return this.sockets.has(socketId);
  }

  values(): Array<WebSocket & { data: SocketData }> {
    return Array.from(this.sockets.values());
  }

  remove(socketId: string): void {
    this.sockets.delete(socketId);
  }

  buildContext(nodeId: string, payload: IJwtPayload): ConnectionContext {
    return toContext(this.newSocketId(), nodeId, payload);
  }

  attach(socket: WebSocket, context: ConnectionContext): void {
    (socket as WebSocket & { data: SocketData }).data = toSocketData(context);
    this.register(socket as WebSocket & { data: SocketData });
  }

  private newSocketId(): string {
    return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
  }
}
