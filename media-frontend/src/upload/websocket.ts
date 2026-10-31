import type { MediaWsEvent, MediaStatus } from './types';
import { WS_EVENT_TO_STATUS } from './types';

function resolveWebSocketURL(): string {
  const configured = import.meta.env.VITE_MEDIA_WS_URL as string | undefined;
  if (configured) return configured;
  if (typeof window === 'undefined') return 'ws://localhost/ws';
  const scheme = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${scheme}//${window.location.host}/ws`;
}

const WS_URL = resolveWebSocketURL();

export type MediaWsListener = (event: MediaWsEvent) => void;
export type ConnectionState = 'CONNECTING' | 'CONNECTED' | 'DISCONNECTED';
export type ConnectionStateListener = (state: ConnectionState) => void;

export class MediaSocket {
  private socket: WebSocket | null = null;
  private token: string | null = null;
  private listeners = new Set<MediaWsListener>();
  private stateListeners = new Set<ConnectionStateListener>();
  private shouldReconnect = false;
  private reconnectAttempt = 0;
  private _state: ConnectionState = 'DISCONNECTED';

  get connectionState(): ConnectionState {
    return this._state;
  }

  connect(token: string): void {
    if (this.socket && this._state !== 'DISCONNECTED') return; // already open
    this.token = token;
    this.shouldReconnect = true;
    this.reconnectAttempt = 0;
    this.open();
  }

  disconnect(): void {
    this.shouldReconnect = false;
    this.socket?.close();
    this.socket = null;
    this.setState('DISCONNECTED');
  }

  onEvent(listener: MediaWsListener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  onStateChange(listener: ConnectionStateListener): () => void {
    this.stateListeners.add(listener);
    listener(this._state);
    return () => this.stateListeners.delete(listener);
  }

  private setState(s: ConnectionState): void {
    if (this._state === s) return;
    this._state = s;
    this.stateListeners.forEach((l) => l(s));
  }

  private open(): void {
    if (!this.token) return;
    this.setState('CONNECTING');
    const ws = new WebSocket(`${WS_URL}?token=${encodeURIComponent(this.token)}`);
    this.socket = ws;

    ws.onopen = () => {
      this.reconnectAttempt = 0;
      this.setState('CONNECTED');
    };

    ws.onmessage = (msg) => {
      try {
        const data = JSON.parse(String(msg.data)) as MediaWsEvent;
        this.emit(data);
      } catch {
        // ignore malformed frames
      }
    };

    ws.onclose = () => {
      this.socket = null;
      this.setState('DISCONNECTED');
      if (this.shouldReconnect) this.scheduleReconnect();
    };

    ws.onerror = () => {
      ws.close();
    };
  }

  private scheduleReconnect(): void {
    const base = Math.min(30000, 1000 * 2 ** this.reconnectAttempt);
    const delay = base / 2 + Math.random() * (base / 2); // jitter
    this.reconnectAttempt += 1;
    setTimeout(() => {
      if (this.shouldReconnect) this.open();
    }, delay);
  }

  private emit(event: MediaWsEvent): void {
    this.listeners.forEach((l) => l(event));
  }

  static statusOf(event: MediaWsEvent): MediaStatus | null {
    return WS_EVENT_TO_STATUS[event.event] ?? null;
  }
}

export const mediaSocket = new MediaSocket();