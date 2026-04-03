import { useState, useEffect } from 'react';
import { Outlet, Navigate } from 'react-router-dom';
import { FullPageLoader } from '../components/FullPageLoader/FullPageLoader.tsx';
import { useProfileRepository } from './profile-context.ts';

type GateState = 'loading' | 'has-profile' | 'needs-onboarding';

export function OnboardingGate() {
  const repository = useProfileRepository();
  const [state, setState] = useState<GateState>('loading');

  useEffect(() => {
    let cancelled = false;

    async function check() {
      const profile = await repository.fetchProfile();
      if (!cancelled) {
        setState(profile ? 'has-profile' : 'needs-onboarding');
      }
    }

    void check();
    return () => { cancelled = true; };
  }, [repository]);

  if (state === 'loading') {
    return <FullPageLoader message="Loading your profile…" />;
  }

  if (state === 'needs-onboarding') {
    return <Navigate to="/onboarding" replace />;
  }

  return <Outlet />;
}
