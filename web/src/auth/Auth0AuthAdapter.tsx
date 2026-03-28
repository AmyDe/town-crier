import type { ReactNode } from 'react';
import { useMemo } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { AuthProvider } from './auth-context';
import type { AuthPort } from '../domain/ports/auth-port';

interface Props {
  children: ReactNode;
}

export function Auth0AuthAdapter({ children }: Props) {
  const { isAuthenticated, isLoading, loginWithRedirect, error } = useAuth0();

  const auth: AuthPort = useMemo(
    () => ({
      isAuthenticated,
      isLoading,
      error,
      loginWithRedirect: () => loginWithRedirect(),
    }),
    [isAuthenticated, isLoading, error, loginWithRedirect],
  );

  return (
    <AuthProvider value={auth}>
      {children}
    </AuthProvider>
  );
}
