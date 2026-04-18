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

/**
 * Normalizes user input for the code field — trims surrounding whitespace and
 * uppercases the remainder. Separators (`-`, spaces) are preserved so the user
 * can see the display format as they type; they're stripped at submit time by
 * the API client on the server.
 */
function normalizeCodeInput(raw: string): string {
  return raw.trim().toUpperCase();
}

export function useRedeemOfferCode(_client: RedeemOfferCodeClient) {
  const [state, setState] = useState<RedeemState>(INITIAL_STATE);

  const setCode = useCallback((raw: string) => {
    setState((prev) => ({
      ...prev,
      code: normalizeCodeInput(raw),
    }));
  }, []);

  return {
    status: state.status,
    code: state.code,
    result: state.result,
    errorMessage: state.errorMessage,
    setCode,
  };
}
