import { RedeemError, type RedeemErrorCode, type RedeemResult } from './types';

export type RedeemOfferCodeClient = (code: string) => Promise<RedeemResult>;

const KNOWN_SERVER_ERROR_CODES = new Set<string>([
  'invalid_code_format',
  'invalid_code',
  'code_already_redeemed',
  'already_subscribed',
]);

export function createRedeemOfferCodeClient(
  getAccessToken: () => Promise<string>,
  apiBaseUrl: string,
  fetchFn: typeof globalThis.fetch = globalThis.fetch,
): RedeemOfferCodeClient {
  return async (code) => {
    const token = await getAccessToken();
    const response = await fetchFn(`${apiBaseUrl}/v1/offer-codes/redeem`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ code }),
    });

    if (response.ok) {
      return (await response.json()) as RedeemResult;
    }

    const serverCode = await readServerErrorCode(response);
    const mapped: RedeemErrorCode =
      serverCode !== null && KNOWN_SERVER_ERROR_CODES.has(serverCode)
        ? (serverCode as RedeemErrorCode)
        : 'network';
    throw new RedeemError(mapped, `Redeem failed (${response.status})`);
  };
}

async function readServerErrorCode(response: Response): Promise<string | null> {
  try {
    const body = (await response.json()) as unknown;
    if (
      typeof body === 'object'
      && body !== null
      && 'error' in body
      && typeof (body as { error: unknown }).error === 'string'
    ) {
      return (body as { error: string }).error;
    }
    return null;
  } catch {
    return null;
  }
}
