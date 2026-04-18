import { useCallback, useState } from 'react';
import type { RedeemOfferCodeClient } from './api/redeemOfferCode';
import { RedeemError, type RedeemErrorCode, type RedeemResult } from './api/types';

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

/**
 * Maps a `RedeemErrorCode` to a user-facing message. Copy is kept in sync with
 * the iOS implementation per docs/specs/offer-codes.md §"iOS — Redemption in
 * Settings" / §"Web — Redemption in Settings".
 */
const ERROR_MESSAGES: Record<RedeemErrorCode, string> = {
  invalid_code_format: 'Please check the code and try again.',
  invalid_code: "This code isn't valid.",
  code_already_redeemed: 'This code has already been used.',
  already_subscribed:
    'You already have an active subscription. Offer codes are only for new subscribers.',
  network: 'Something went wrong. Please check your connection and try again.',
};

function messageForError(err: unknown): string {
  if (err instanceof RedeemError) {
    return ERROR_MESSAGES[err.code];
  }
  return ERROR_MESSAGES.network;
}

export function useRedeemOfferCode(
  client: RedeemOfferCodeClient,
  options: UseRedeemOfferCodeOptions = {},
) {
  const [state, setState] = useState<RedeemState>(INITIAL_STATE);
  const { onSuccess } = options;

  const setCode = useCallback((raw: string) => {
    setState((prev) => {
      const next = normalizeCodeInput(raw);
      // If we're showing an error from a previous submit, clear it as soon as
      // the user edits the code — they're taking corrective action and shouldn't
      // keep seeing the stale message.
      if (prev.status === 'error') {
        return { ...INITIAL_STATE, code: next };
      }
      return { ...prev, code: next };
    });
  }, []);

  const submit = useCallback(async () => {
    // Guard against double-submits while a request is in flight.
    if (state.status === 'submitting') return;

    const submittedCode = state.code;
    setState((prev) => ({ ...prev, status: 'submitting', errorMessage: null }));

    try {
      const result = await client(submittedCode);
      setState((prev) => ({
        ...prev,
        status: 'success',
        result,
        errorMessage: null,
      }));
      onSuccess?.(result);
    } catch (err: unknown) {
      const errorMessage = messageForError(err);
      setState((prev) => ({
        ...prev,
        status: 'error',
        result: null,
        errorMessage,
      }));
    }
  }, [client, onSuccess, state.code, state.status]);

  const reset = useCallback(() => {
    setState(INITIAL_STATE);
  }, []);

  return {
    status: state.status,
    code: state.code,
    result: state.result,
    errorMessage: state.errorMessage,
    setCode,
    submit,
    reset,
  };
}
