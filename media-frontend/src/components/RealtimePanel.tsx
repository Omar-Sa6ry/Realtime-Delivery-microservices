import { useEffect, useRef, useState } from 'react';
import type { ConnectionState } from '../upload/websocket';
import { mediaSocket } from '../upload/websocket';
import type { MediaWsEvent } from '../upload/types';

interface LogEntry {
  id: string;
  ts: Date;
  event: MediaWsEvent;
}

const EVENT_COLOR: { [key: string]: string } = {
  SCAN_STARTED:          'ws-evt--scan',
  SCAN_COMPLETED:        'ws-evt--scan',
  SCAN_FAILED:           'ws-evt--fail',
  PROCESSING_STARTED:    'ws-evt--process',
  PROCESSING_PROGRESS:   'ws-evt--process',
  PROCESSING_COMPLETED:  'ws-evt--process',
  PROCESSING_FAILED:     'ws-evt--fail',
  READY:                 'ws-evt--ready',
  DELETE_STARTED:        'ws-evt--delete',
  DELETE_COMPLETED:      'ws-evt--delete',
  DELETE_FAILED:         'ws-evt--fail',
};

const STATE_LABEL: Record<ConnectionState, string> = {
  CONNECTED:    '● متصل',
  CONNECTING:   '◌ يتصل…',
  DISCONNECTED: '○ غير متصل',
};

const STATE_CLASS: Record<ConnectionState, string> = {
  CONNECTED:    'ws-dot--connected',
  CONNECTING:   'ws-dot--connecting',
  DISCONNECTED: 'ws-dot--disconnected',
};

function formatTime(d: Date): string {
  return d.toLocaleTimeString('ar-EG', { hour12: false });
}

function eventLabel(evt: MediaWsEvent): string {
  const parts: string[] = [evt.event];
  if (evt.stage) parts.push(`stage: ${evt.stage}`);
  if (evt.progress !== undefined) parts.push(`${evt.progress}%`);
  return parts.join(' · ');
}

export function RealtimePanel({ accessToken }: { accessToken: string | null }) {
  const [connState, setConnState] = useState<ConnectionState>('DISCONNECTED');
  const [log, setLog] = useState<LogEntry[]>([]);
  const [filter, setFilter] = useState('');
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!accessToken) return;

    const offState = mediaSocket.onStateChange(setConnState);
    const offEvent = mediaSocket.onEvent((evt) => {
      setLog((prev) => [
        ...prev.slice(-199), // keep last 200
        { id: crypto.randomUUID(), ts: new Date(), event: evt },
      ]);
    });

    mediaSocket.connect(accessToken);

    return () => {
      offState();
      offEvent();
    };
  }, [accessToken]);

  // Auto-scroll to bottom on new events
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [log.length]);

  const visible = filter
    ? log.filter(
        (e) =>
          e.event.mediaId?.includes(filter) ||
          e.event.event.toLowerCase().includes(filter.toLowerCase()),
      )
    : log;

  const clearLog = () => setLog([]);

  return (
    <section className="card realtime-panel">
      <div className="realtime-header">
        <h2>
          أحداث Realtime
          <span className={`ws-dot ${STATE_CLASS[connState]}`}>
            {STATE_LABEL[connState]}
          </span>
        </h2>
        <div className="realtime-controls">
          <input
            className="ws-filter-input"
            type="text"
            placeholder="فلتر بـ mediaId أو نوع الحدث…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
          <button className="btn btn-ghost btn-sm" onClick={clearLog}>
            مسح
          </button>
        </div>
      </div>

      <div className="ws-log">
        {visible.length === 0 ? (
          <p className="muted ws-empty">
            {connState === 'DISCONNECTED'
              ? 'الاتصال مغلق — ارفع ملفاً لبدء الجلسة'
              : connState === 'CONNECTING'
              ? 'جارٍ الاتصال…'
              : 'في انتظار الأحداث…'}
          </p>
        ) : (
          visible.map((entry) => {
            const cls = EVENT_COLOR[entry.event.event] ?? '';
            const pct = entry.event.progress;
            return (
              <div key={entry.id} className={`ws-evt ${cls}`}>
                <span className="ws-ts">{formatTime(entry.ts)}</span>
                <span className="ws-badge">{entry.event.event}</span>
                <span className="ws-meta">{eventLabel(entry.event)}</span>
                {entry.event.mediaId && (
                  <span className="ws-id" title={entry.event.mediaId}>
                    {entry.event.mediaId.slice(0, 8)}…
                  </span>
                )}
                {pct !== undefined && (
                  <div className="ws-mini-track">
                    <div className="ws-mini-fill" style={{ width: `${pct}%` }} />
                  </div>
                )}
              </div>
            );
          })
        )}
        <div ref={bottomRef} />
      </div>
    </section>
  );
}
