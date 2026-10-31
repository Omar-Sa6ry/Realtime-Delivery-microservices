import { useState } from 'react';
import { AuthProvider, useAuth } from './auth/AuthContext';
import { AuthFlow } from './components/AuthFlow';
import { Dashboard } from './components/Dashboard';

function SessionGate() {
  const [hasToken] = useState(() => !!localStorage.getItem('accessToken'));
  const { accessToken } = useAuth();
  const [stage, setStage] = useState<'auth' | 'dashboard'>(hasToken ? 'dashboard' : 'auth');
  const loggedIn = stage === 'dashboard' && !!accessToken;

  return loggedIn ? (
    <Dashboard onLogout={() => setStage('auth')} />
  ) : (
    <AuthFlow onComplete={() => setStage('dashboard')} />
  );
}

export default function App() {
  return (
    <AuthProvider>
      <div className="app">
        <SessionGate />
      </div>
    </AuthProvider>
  );
}