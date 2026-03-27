import type { ReactNode } from 'react';
import { useMemo } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { AuthContext } from './auth-context';
import type { AuthPort } from '../domain/ports/auth-port';

interface Props {
  children: ReactNode;
}

export function Auth0AuthAdapter({ children }: Props) {
  const { isAuthenticated, isLoading, loginWithRedirect } = useAuth0();

  const auth: AuthPort = useMemo(
    () => ({
      isAuthenticated,
      isLoading,
      loginWithRedirect: () => loginWithRedirect(),
    }),
    [isAuthenticated, isLoading, loginWithRedirect],
  );

  return (
    <AuthContext.Provider value={auth}>
      {children}
    </AuthContext.Provider>
  );
}
