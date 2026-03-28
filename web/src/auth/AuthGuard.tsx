import { useEffect } from 'react';
import { Outlet } from 'react-router-dom';
import { useAuth } from './auth-context.ts';

export function AuthGuard() {
  const { isAuthenticated, isLoading, error, loginWithRedirect } = useAuth();

  useEffect(() => {
    if (!isLoading && !isAuthenticated && !error) {
      void loginWithRedirect();
    }
  }, [isLoading, isAuthenticated, error, loginWithRedirect]);

  if (isLoading || !isAuthenticated) {
    return null;
  }

  return <Outlet />;
}
