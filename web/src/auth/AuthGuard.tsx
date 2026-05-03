import { useEffect } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import { FullPageLoader } from '../components/FullPageLoader/FullPageLoader.tsx';
import { useAuth } from './auth-context.ts';

export function AuthGuard() {
  const { isAuthenticated, isLoading, error, loginWithRedirect } = useAuth();
  const location = useLocation();

  useEffect(() => {
    if (!isLoading && !isAuthenticated && !error) {
      const returnTo = location.pathname + location.search;
      void loginWithRedirect({ appState: { returnTo } });
    }
  }, [isLoading, isAuthenticated, error, loginWithRedirect, location.pathname, location.search]);

  if (isLoading || !isAuthenticated) {
    return <FullPageLoader message="Signing you in…" />;
  }

  return <Outlet />;
}
