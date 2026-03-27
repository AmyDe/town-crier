import type { AuthPort } from '../../../domain/ports/auth-port.ts';

export class SpyAuthPort implements AuthPort {
  isAuthenticated = false;
  isLoading = false;
  loginWithRedirectCalls = 0;

  loginWithRedirect = async (): Promise<void> => {
    this.loginWithRedirectCalls += 1;
  };
}
