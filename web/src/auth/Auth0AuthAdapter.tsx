import type { ReactNode } from 'react';
import { useMemo, useCallback, useState } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { AuthProvider } from './auth-context';
import { readPendingAuth0RedirectReturnTo } from './auth0-redirect-return-to';
import type { AuthPort, LoginWithRedirectOptions } from '../domain/ports/auth-port';

interface Props {
  children: ReactNode;
}

export function Auth0AuthAdapter({ children }: Props) {
  const { isAuthenticated, isLoading, loginWithRedirect, logout: auth0Logout, error } = useAuth0();

  const [returnTo] = useState<string | undefined>(() => readPendingAuth0RedirectReturnTo());

  const logout = useCallback(
    () =>
      auth0Logout({
        logoutParams: {
          returnTo: `${window.location.origin}?signed_out=true`,
        },
      }),
    [auth0Logout],
  );

  const wrappedLoginWithRedirect = useCallback(
    (options?: LoginWithRedirectOptions) =>
      loginWithRedirect(options ? { appState: options.appState } : undefined),
    [loginWithRedirect],
  );

  const auth: AuthPort = useMemo(
    () => ({
      isAuthenticated,
      isLoading,
      error,
      returnTo,
      loginWithRedirect: wrappedLoginWithRedirect,
      logout,
    }),
    [isAuthenticated, isLoading, error, returnTo, wrappedLoginWithRedirect, logout],
  );

  return (
    <AuthProvider value={auth}>
      {children}
    </AuthProvider>
  );
}
