export interface AuthConfig {
  readonly domain: string;
  readonly clientId: string;
  readonly audience: string;
}

export function loadAuthConfig(): AuthConfig {
  const domain = import.meta.env.VITE_AUTH0_DOMAIN as string | undefined;
  const clientId = import.meta.env.VITE_AUTH0_CLIENT_ID as string | undefined;
  const audience = import.meta.env.VITE_AUTH0_AUDIENCE as string | undefined;

  if (!domain || !clientId || !audience) {
    throw new Error(
      'Missing Auth0 configuration. Set VITE_AUTH0_DOMAIN, VITE_AUTH0_CLIENT_ID, and VITE_AUTH0_AUDIENCE environment variables.',
    );
  }

  return { domain, clientId, audience };
}
