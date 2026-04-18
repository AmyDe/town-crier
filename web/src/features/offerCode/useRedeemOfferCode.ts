import { useCallback, useState } from 'react';
import type { RedeemOfferCodeClient } from './api/redeemOfferCode';
import type { RedeemResult } from './api/types';

export type RedeemStatus = 'idle' | 'submitting' | 'success' | 'error';

interface RedeemState {
  status: RedeemStatus;
  code: string;
  result: RedeemResult | null;
  errorMessage: string | null;
}

const INITIAL_STATE: RedeemState = {
  status: 'idle',
  code: '',
  result: null,
  errorMessage: null,
};

export interface UseRedeemOfferCodeOptions {
  /**
   * Called once after a successful redemption with the parsed API result.
   * Callers use this to trigger a tier refresh — typically
   * `getAccessTokenSilently({ cacheMode: 'off' })` plus a profile re-fetch
   * so tier-gated UI updates. See docs/specs/offer-codes.md §"Web — Redemption
   * in Settings".
   */
  readonly onSuccess?: (result: RedeemResult) => void;
}

/**
 * Normalizes user input for the code field — trims surrounding whitespace and
 * uppercases the remainder. Separators (`-`, spaces) are preserved so the user
 * can see the display format as they type; the server normalizes them away at
 * redeem time.
 */
function normalizeCodeInput(raw: string): string {
  return raw.trim().toUpperCase();
}

export function useRedeemOfferCode(
  client: RedeemOfferCodeClient,
  options: UseRedeemOfferCodeOptions = {},
) {
  const [state, setState] = useState<RedeemState>(INITIAL_STATE);
  const { onSuccess } = options;

  const setCode = useCallback((raw: string) => {
    setState((prev) => ({
      ...prev,
      code: normalizeCodeInput(raw),
    }));
  }, []);

  const submit = useCallback(async () => {
    const submittedCode = state.code;
    setState((prev) => ({ ...prev, status: 'submitting', errorMessage: null }));

    const result = await client(submittedCode);
    setState((prev) => ({
      ...prev,
      status: 'success',
      result,
      errorMessage: null,
    }));
    onSuccess?.(result);
  }, [client, onSuccess, state.code]);

  return {
    status: state.status,
    code: state.code,
    result: state.result,
    errorMessage: state.errorMessage,
    setCode,
    submit,
  };
}
