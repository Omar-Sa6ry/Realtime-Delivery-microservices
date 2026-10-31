export interface RunSummary {
  runId: string;
  total: number;
  passed: number;
  failed: number;
  skipped: number;
  durationMs: number;
  startedAt: string;
  finishedAt: string;
}

const API_BASE = 'http://localhost:8090/api';

export const triggerRun = async (suiteFilter?: string): Promise<{ runId: string }> => {
  const url = suiteFilter ? `${API_BASE}/runs?suite=${encodeURIComponent(suiteFilter)}` : `${API_BASE}/runs`;
  const res = await fetch(url, { method: 'POST' });
  if (!res.ok) throw new Error('Failed to trigger test run');
  return res.json();
};

export const listRuns = async (): Promise<RunSummary[]> => {
  const res = await fetch(`${API_BASE}/runs`);
  if (!res.ok) throw new Error('Failed to fetch runs');
  return res.json();
};

export const listSuites = async (): Promise<string[]> => {
  const res = await fetch(`${API_BASE}/suites`);
  if (!res.ok) throw new Error('Failed to fetch suites');
  return res.json();
};
