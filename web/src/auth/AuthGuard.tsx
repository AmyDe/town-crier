import { useEffect } from 'react';
import { Outlet } from 'react-router-dom';
import { FullPageLoader } from '../components/FullPageLoader/FullPageLoader.tsx';
import { useAuth } from './auth-context.ts';

export function AuthGuard() {
  const { isAuthenticated, isLoading, error, loginWithRedirect } = useAuth();

  useEffect(() => {
    if (!isLoading && !isAuthenticated && !error) {
      void loginWithRedirect();
    }
  }, [isLoading, isAuthenticated, error, loginWithRedirect]);

  if (isLoading || !isAuthenticated) {
    return <FullPageLoader message="Signing you in\u2026" />;
  }

  return <Outlet />;
}
