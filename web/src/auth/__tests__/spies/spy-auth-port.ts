import type { AuthPort, LoginWithRedirectOptions } from '../../../domain/ports/auth-port.ts';

export class SpyAuthPort implements AuthPort {
  isAuthenticated = false;
  isLoading = false;
  error: Error | undefined = undefined;
  returnTo: string | undefined = undefined;
  loginWithRedirectCalls = 0;
  lastLoginWithRedirectOptions: LoginWithRedirectOptions | undefined = undefined;
  logoutCalls = 0;

  loginWithRedirect = async (options?: LoginWithRedirectOptions): Promise<void> => {
    this.loginWithRedirectCalls += 1;
    this.lastLoginWithRedirectOptions = options;
  };

  logout = async (): Promise<void> => {
    this.logoutCalls += 1;
  };
}
