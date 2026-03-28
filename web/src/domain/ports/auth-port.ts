export interface AuthPort {
  readonly isAuthenticated: boolean;
  readonly isLoading: boolean;
  readonly error: Error | undefined;
  loginWithRedirect(): Promise<void>;
}
