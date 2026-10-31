import { useState } from 'react';
import { triggerRun } from '../api/testingApi';
import { useTestingWS, type TestResult } from '../hooks/useTestingWS';

export function TestingDashboard() {
  const [activeRunId, setActiveRunId] = useState<string | null>(null);
  const { results, summary, isConnected, clearResults } = useTestingWS(activeRunId);
  const [isStarting, setIsStarting] = useState(false);

  const startTests = async () => {
    try {
      setIsStarting(true);
      clearResults();
      const res = await triggerRun();
      setActiveRunId(res.runId);
    } catch (e) {
      console.error(e);
    } finally {
      setIsStarting(false);
    }
  };

  // Group results by suite
  const suitesMap: Record<string, TestResult[]> = {};
  for (const r of results) {
    if (!suitesMap[r.suiteName]) suitesMap[r.suiteName] = [];
    suitesMap[r.suiteName].push(r);
  }

  return (
    <section className="card testing-panel" style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div className="lab-header">
        <h2>اختبارات السيرفيس (Media)</h2>
        <div className="lab-header-actions">
          <span className={`status-dot ${isConnected ? 'connected' : 'disconnected'}`} style={{
            display: 'inline-block',
            width: 10,
            height: 10,
            borderRadius: '50%',
            backgroundColor: isConnected ? 'var(--success)' : 'var(--error)',
            marginRight: 8
          }} title={isConnected ? 'متصل' : 'غير متصل'}></span>
          <button 
            className="btn btn-primary btn-sm" 
            onClick={startTests}
            disabled={isStarting}
          >
            {isStarting ? 'جاري التشغيل...' : 'تشغيل الاختبارات ▶'}
          </button>
        </div>
      </div>

      {summary && (
        <div style={{ marginBottom: 16, padding: 12, backgroundColor: 'var(--surface-50)', borderRadius: 8 }}>
          <h3 style={{ margin: '0 0 8px 0', fontSize: '1rem' }}>ملخص الاختبار (Run: {summary.runId.substring(0, 8)})</h3>
          <div style={{ display: 'flex', gap: 16, fontSize: '0.9rem' }}>
            <span><strong>إجمالي:</strong> {summary.total}</span>
            <span style={{ color: 'var(--success)' }}><strong>نجاح:</strong> {summary.passed}</span>
            <span style={{ color: 'var(--error)' }}><strong>فشل:</strong> {summary.failed}</span>
            <span><strong>تخطي:</strong> {summary.skipped}</span>
            <span><strong>المدة:</strong> {Math.round(summary.durationMs / 1000000)}ms</span>
          </div>
        </div>
      )}

      <div className="test-suites" style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 12 }}>
        {Object.keys(suitesMap).length === 0 ? (
          <p className="muted" style={{ textAlign: 'center', marginTop: 32 }}>اضغط تشغيل لبدء الاختبارات</p>
        ) : (
          Object.keys(suitesMap).map((suiteName) => {
            const cases = suitesMap[suiteName];
            const hasFailures = cases.some(c => c.status === 'FAILED');
            return (
              <div key={suiteName} className="test-suite-card" style={{
                border: `1px solid ${hasFailures ? 'var(--error-alpha)' : 'var(--border)'}`,
                borderRadius: 8,
                overflow: 'hidden'
              }}>
                <div style={{
                  padding: '8px 12px',
                  backgroundColor: hasFailures ? 'rgba(239, 68, 68, 0.1)' : 'var(--surface-100)',
                  fontWeight: 600,
                  borderBottom: '1px solid var(--border)'
                }}>
                  {suiteName}
                </div>
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                  {cases.map((c) => (
                    <div key={c.caseName} style={{
                      padding: '8px 12px',
                      borderBottom: '1px solid var(--border)',
                      fontSize: '0.9rem',
                      display: 'flex',
                      alignItems: 'flex-start',
                      justifyContent: 'space-between',
                      backgroundColor: c.status === 'FAILED' ? 'rgba(239, 68, 68, 0.05)' : 'transparent'
                    }}>
                      <div style={{ flex: 1 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          {c.status === 'RUNNING' && <span style={{ color: 'var(--warning)' }}>↻</span>}
                          {c.status === 'PASSED' && <span style={{ color: 'var(--success)' }}>✓</span>}
                          {c.status === 'FAILED' && <span style={{ color: 'var(--error)' }}>✗</span>}
                          <span>{c.caseName}</span>
                        </div>
                        {c.error && (
                          <div style={{ color: 'var(--error)', marginTop: 4, fontSize: '0.85rem' }}>
                            {c.error}
                          </div>
                        )}
                      </div>
                      {c.durationMs > 0 && (
                        <span className="muted" style={{ fontSize: '0.8rem' }}>
                          {Math.round(c.durationMs / 1000000)}ms
                        </span>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            );
          })
        )}
      </div>
    </section>
  );
}
