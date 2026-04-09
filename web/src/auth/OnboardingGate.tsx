import { useState, useEffect, useCallback } from 'react';
import { Outlet, Navigate } from 'react-router-dom';
import { FullPageLoader } from '../components/FullPageLoader/FullPageLoader.tsx';
import { FullPageError } from '../components/FullPageError/FullPageError.tsx';
import { useProfileRepository } from './profile-context.ts';
import { useAuth } from './auth-context.ts';

type GateState = 'loading' | 'has-profile' | 'needs-onboarding' | 'error';

export function OnboardingGate() {
  const repository = useProfileRepository();
  const { logout } = useAuth();
  const [state, setState] = useState<GateState>('loading');

  const loadProfile = useCallback(async () => {
    setState('loading');
    try {
      const profile = await repository.fetchProfile();
      setState(profile ? 'has-profile' : 'needs-onboarding');
    } catch {
      setState('error');
    }
  }, [repository]);

  useEffect(() => {
    let cancelled = false;

    async function check() {
      try {
        const profile = await repository.fetchProfile();
        if (!cancelled) {
          setState(profile ? 'has-profile' : 'needs-onboarding');
        }
      } catch {
        if (!cancelled) {
          setState('error');
        }
      }
    }

    void check();
    return () => { cancelled = true; };
  }, [repository]);

  if (state === 'loading') {
    return <FullPageLoader message="Loading your profile…" />;
  }

  if (state === 'error') {
    return (
      <FullPageError
        message="We couldn't load your profile. This can happen when your session has expired."
        onRetry={loadProfile}
        onSignOut={logout}
      />
    );
  }

  if (state === 'needs-onboarding') {
    return <Navigate to="/onboarding" replace />;
  }

  return <Outlet />;
}
