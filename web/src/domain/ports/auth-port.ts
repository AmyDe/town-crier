export interface LoginWithRedirectOptions {
  readonly appState?: {
    readonly returnTo?: string;
  };
}

export interface AuthPort {
  readonly isAuthenticated: boolean;
  readonly isLoading: boolean;
  readonly error: Error | undefined;
  /** Post-callback returnTo path captured from `appState.returnTo`, if present. */
  readonly returnTo: string | undefined;
  loginWithRedirect(options?: LoginWithRedirectOptions): Promise<void>;
  logout(): Promise<void>;
}
