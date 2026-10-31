import { useEffect, useState, useRef } from 'react';
import type { RunSummary } from '../api/testingApi';

export interface TestResult {
  suiteName: string;
  caseName: string;
  status: 'PASSED' | 'FAILED' | 'SKIPPED' | 'RUNNING';
  durationMs: number;
  error?: string;
  details?: string;
  timestamp: string;
}

export function useTestingWS(runId: string | null) {
  const [results, setResults] = useState<TestResult[]>([]);
  const [summary, setSummary] = useState<RunSummary | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    // We connect generally, or you could connect with runId if supported
    const ws = new WebSocket('ws://localhost:8090/ws');
    wsRef.current = ws;

    ws.onopen = () => setIsConnected(true);
    ws.onclose = () => setIsConnected(false);

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      if (data.type === 'TEST_RESULT') {
        setResults((prev) => {
          // If the case already exists (e.g. going from RUNNING to PASSED), replace it
          const existingIndex = prev.findIndex(r => r.suiteName === data.suiteName && r.caseName === data.caseName);
          if (existingIndex >= 0) {
            const newResults = [...prev];
            newResults[existingIndex] = data;
            return newResults;
          }
          return [...prev, data];
        });
      } else if (data.type === 'RUN_COMPLETE') {
        setSummary(data.summary);
      }
    };

    return () => {
      ws.close();
    };
  }, [runId]);

  const clearResults = () => {
    setResults([]);
    setSummary(null);
  };

  return { results, summary, isConnected, clearResults };
}
