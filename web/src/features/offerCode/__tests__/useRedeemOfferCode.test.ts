import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useRedeemOfferCode } from '../useRedeemOfferCode';
import { SpyRedeemOfferCodeClient } from './spies/spy-redeem-offer-code-client';
import { RedeemError, type RedeemErrorCode, type RedeemResult } from '../api/types';

describe('useRedeemOfferCode', () => {
  let spy: SpyRedeemOfferCodeClient;

  beforeEach(() => {
    spy = new SpyRedeemOfferCodeClient();
  });

  it('starts in idle state with no code, result, or error', () => {
    const { result } = renderHook(() => useRedeemOfferCode(spy.client));

    expect(result.current.status).toBe('idle');
    expect(result.current.code).toBe('');
    expect(result.current.result).toBeNull();
    expect(result.current.errorMessage).toBeNull();
  });
});
