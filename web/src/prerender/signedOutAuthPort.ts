import type { AuthPort } from '../domain/ports/auth-port';

/** The auth state of an anonymous visitor, for rendering outside a browser. */
export const signedOutAuthPort: AuthPort = {
  isAuthenticated: false,
  isLoading: false,
  error: undefined,
  returnTo: undefined,
  loginWithRedirect: () => Promise.resolve(),
  logout: () => Promise.resolve(),
};
