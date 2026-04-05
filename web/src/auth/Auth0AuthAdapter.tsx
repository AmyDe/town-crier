import type { ReactNode } from 'react';
import { useMemo, useCallback } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { AuthProvider } from './auth-context';
import type { AuthPort } from '../domain/ports/auth-port';

interface Props {
  children: ReactNode;
}

export function Auth0AuthAdapter({ children }: Props) {
  const { isAuthenticated, isLoading, loginWithRedirect, logout: auth0Logout, error } = useAuth0();

  const logout = useCallback(
    () =>
      auth0Logout({
        logoutParams: {
          returnTo: `${window.location.origin}?signed_out=true`,
        },
      }),
    [auth0Logout],
  );

  const auth: AuthPort = useMemo(
    () => ({
      isAuthenticated,
      isLoading,
      error,
      loginWithRedirect: () => loginWithRedirect(),
      logout,
    }),
    [isAuthenticated, isLoading, error, loginWithRedirect, logout],
  );

  return (
    <AuthProvider value={auth}>
      {children}
    </AuthProvider>
  );
}
