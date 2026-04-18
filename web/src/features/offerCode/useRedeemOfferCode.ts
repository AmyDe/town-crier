import { useState } from 'react';
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

export function useRedeemOfferCode(_client: RedeemOfferCodeClient) {
  const [state] = useState<RedeemState>(INITIAL_STATE);

  return {
    status: state.status,
    code: state.code,
    result: state.result,
    errorMessage: state.errorMessage,
  };
}
