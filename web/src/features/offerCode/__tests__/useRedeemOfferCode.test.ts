import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useRedeemOfferCode } from '../useRedeemOfferCode';
import { SpyRedeemOfferCodeClient } from './spies/spy-redeem-offer-code-client';
import { RedeemError, type RedeemErrorCode, type RedeemResult } from '../api/types';

const PRO_RESULT: RedeemResult = {
  tier: 'Pro',
  expiresAt: '2026-05-18T00:00:00Z',
};

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

  describe('setCode', () => {
    it('stores the raw user input uppercased', () => {
      const { result } = renderHook(() => useRedeemOfferCode(spy.client));

      act(() => {
        result.current.setCode('a7km-zqr3-fnxp');
      });

      expect(result.current.code).toBe('A7KM-ZQR3-FNXP');
    });

    it('trims surrounding whitespace', () => {
      const { result } = renderHook(() => useRedeemOfferCode(spy.client));

      act(() => {
        result.current.setCode('  a7km-zqr3-fnxp  ');
      });

      expect(result.current.code).toBe('A7KM-ZQR3-FNXP');
    });

  });

  describe('submit — happy path', () => {
    it('calls the client with the current normalized code', async () => {
      spy.result = PRO_RESULT;
      const { result } = renderHook(() => useRedeemOfferCode(spy.client));

      act(() => {
        result.current.setCode('a7km-zqr3-fnxp');
      });

      await act(async () => {
        await result.current.submit();
      });

      expect(spy.calls).toEqual(['A7KM-ZQR3-FNXP']);
    });

    it('transitions to success with the parsed result', async () => {
      spy.result = PRO_RESULT;
      const { result } = renderHook(() => useRedeemOfferCode(spy.client));

      act(() => {
        result.current.setCode('A7KM-ZQR3-FNXP');
      });

      await act(async () => {
        await result.current.submit();
      });

      expect(result.current.status).toBe('success');
      expect(result.current.result).toEqual(PRO_RESULT);
      expect(result.current.errorMessage).toBeNull();
    });

    it('reports submitting status while the request is in flight', async () => {
      spy.deferNext();
      const { result } = renderHook(() => useRedeemOfferCode(spy.client));

      act(() => {
        result.current.setCode('A7KM-ZQR3-FNXP');
      });

      let submitPromise: Promise<void> = Promise.resolve();
      act(() => {
        submitPromise = result.current.submit();
      });

      expect(result.current.status).toBe('submitting');
      expect(result.current.errorMessage).toBeNull();

      await act(async () => {
        spy.resolvePending(PRO_RESULT);
        await submitPromise;
      });

      expect(result.current.status).toBe('success');
    });

    it('invokes onSuccess with the result after a successful redemption', async () => {
      spy.result = PRO_RESULT;
      const onSuccessCalls: RedeemResult[] = [];
      const { result } = renderHook(() =>
        useRedeemOfferCode(spy.client, { onSuccess: (r) => { onSuccessCalls.push(r); } }),
      );

      act(() => {
        result.current.setCode('A7KM-ZQR3-FNXP');
      });

      await act(async () => {
        await result.current.submit();
      });

      expect(onSuccessCalls).toEqual([PRO_RESULT]);
    });
  });
});
