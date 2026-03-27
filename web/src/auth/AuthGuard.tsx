import { useEffect } from 'react';
import { Outlet } from 'react-router-dom';
import { useAuth } from './auth-context.ts';

export function AuthGuard() {
  const { isAuthenticated, isLoading, loginWithRedirect } = useAuth();

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      void loginWithRedirect();
    }
  }, [isLoading, isAuthenticated, loginWithRedirect]);

  if (isLoading || !isAuthenticated) {
    return null;
  }

  return <Outlet />;
}
