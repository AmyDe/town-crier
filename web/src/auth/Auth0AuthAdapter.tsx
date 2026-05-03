import type { ReactNode } from 'react';
import { useMemo, useCallback, useState } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { AuthProvider } from './auth-context';
import type { AuthPort, LoginWithRedirectOptions } from '../domain/ports/auth-port';

/**
 * Module-level holder for the `returnTo` path captured from `appState.returnTo`
 * by Auth0's `onRedirectCallback`. The Auth0 React SDK only exposes `appState`
 * via that callback prop on `Auth0Provider` (not via `useAuth0()`), so we stash
 * it here and let `Auth0AuthAdapter` surface it through `AuthPort.returnTo`.
 *
 * Wire `onRedirectCallback={(appState) => captureAuth0RedirectReturnTo(appState?.returnTo)}`
 * on the `Auth0Provider` in the composition root.
 */
let pendingReturnTo: string | undefined = undefined;

export function captureAuth0RedirectReturnTo(value: string | undefined): void {
  pendingReturnTo = value;
}

interface Props {
  children: ReactNode;
}

export function Auth0AuthAdapter({ children }: Props) {
  const { isAuthenticated, isLoading, loginWithRedirect, logout: auth0Logout, error } = useAuth0();

  // Snapshot the captured returnTo at mount so consumers see a stable value.
  // The capture itself happens before React mounts the auth subtree, when
  // Auth0Provider invokes onRedirectCallback during the post-callback bootstrap.
  const [returnTo] = useState<string | undefined>(() => pendingReturnTo);

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
