export interface AuthPort {
  readonly isAuthenticated: boolean;
  readonly isLoading: boolean;
  loginWithRedirect(): Promise<void>;
}
