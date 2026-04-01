import { useCallback } from 'react';
import { useAuth } from '../../auth/auth-context';

interface NavbarAuthState {
  readonly isAuthenticated: boolean;
  readonly handleSignIn: (() => Promise<void>) | undefined;
}

export function useNavbarAuth(): NavbarAuthState {
  const { isAuthenticated, loginWithRedirect } = useAuth();

  const handleSignIn = useCallback(async () => {
    await loginWithRedirect();
  }, [loginWithRedirect]);

  return {
    isAuthenticated,
    handleSignIn: isAuthenticated ? undefined : handleSignIn,
  };
}
