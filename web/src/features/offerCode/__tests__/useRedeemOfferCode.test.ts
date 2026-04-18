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

  describe('submit — error mapping', () => {
    it.each<[RedeemErrorCode, string]>([
      ['invalid_code_format', 'Please check the code and try again.'],
      ['invalid_code', "This code isn't valid."],
      ['code_already_redeemed', 'This code has already been used.'],
      [
        'already_subscribed',
        'You already have an active subscription. Offer codes are only for new subscribers.',
      ],
      [
        'network',
        "Something went wrong. Please check your connection and try again.",
      ],
    ])('maps RedeemError code %s to the expected user-facing message', async (code, expectedMessage) => {
      spy.error = new RedeemError(code, 'server said no');
      const { result } = renderHook(() => useRedeemOfferCode(spy.client));

      act(() => {
        result.current.setCode('A7KM-ZQR3-FNXP');
      });

      await act(async () => {
        await result.current.submit();
      });

      expect(result.current.status).toBe('error');
      expect(result.current.errorMessage).toBe(expectedMessage);
      expect(result.current.result).toBeNull();
    });

    it('falls back to a generic message for non-RedeemError exceptions', async () => {
      spy.error = Object.assign(new Error('boom'), {});
      // Any non-RedeemError thrown by the client should surface as a generic error.
      spy.client; // eslint-disable-line @typescript-eslint/no-unused-expressions
      const customSpy = new SpyRedeemOfferCodeClient();
      // Override the spy so it rejects with a plain Error, not a RedeemError.
      const clientThatThrowsPlainError: typeof customSpy.client = async (code) => {
        customSpy.calls.push(code);
        throw new Error('unexpected');
      };
      const { result } = renderHook(() => useRedeemOfferCode(clientThatThrowsPlainError));

      act(() => {
        result.current.setCode('A7KM-ZQR3-FNXP');
      });

      await act(async () => {
        await result.current.submit();
      });

      expect(result.current.status).toBe('error');
      expect(result.current.errorMessage).toBe(
        'Something went wrong. Please check your connection and try again.',
      );
    });

    it('does not invoke onSuccess when submit fails', async () => {
      spy.error = new RedeemError('invalid_code', 'nope');
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

      expect(onSuccessCalls).toEqual([]);
    });
  });

  describe('editing clears error state', () => {
    it('clears errorMessage and returns to idle when the user edits the code after an error', async () => {
      spy.error = new RedeemError('invalid_code', 'nope');
      const { result } = renderHook(() => useRedeemOfferCode(spy.client));

      act(() => {
        result.current.setCode('A7KM-ZQR3-FNXP');
      });

      await act(async () => {
        await result.current.submit();
      });

      expect(result.current.status).toBe('error');
      expect(result.current.errorMessage).not.toBeNull();

      act(() => {
        result.current.setCode('NEW-INPUT');
      });

      expect(result.current.status).toBe('idle');
      expect(result.current.errorMessage).toBeNull();
      expect(result.current.code).toBe('NEW-INPUT');
    });
  });

  describe('reset', () => {
    it('restores initial state after a successful redemption', async () => {
      spy.result = PRO_RESULT;
      const { result } = renderHook(() => useRedeemOfferCode(spy.client));

      act(() => {
        result.current.setCode('A7KM-ZQR3-FNXP');
      });

      await act(async () => {
        await result.current.submit();
      });

      expect(result.current.status).toBe('success');

      act(() => {
        result.current.reset();
      });

      expect(result.current.status).toBe('idle');
      expect(result.current.code).toBe('');
      expect(result.current.result).toBeNull();
      expect(result.current.errorMessage).toBeNull();
    });

    it('restores initial state after an error', async () => {
      spy.error = new RedeemError('invalid_code', 'nope');
      const { result } = renderHook(() => useRedeemOfferCode(spy.client));

      act(() => {
        result.current.setCode('BAD');
      });

      await act(async () => {
        await result.current.submit();
      });

      expect(result.current.status).toBe('error');

      act(() => {
        result.current.reset();
      });

      expect(result.current.status).toBe('idle');
      expect(result.current.code).toBe('');
      expect(result.current.errorMessage).toBeNull();
    });
  });

  describe('submit guards', () => {
    it('does not call the client when already submitting', async () => {
      spy.deferNext();
      const { result } = renderHook(() => useRedeemOfferCode(spy.client));

      act(() => {
        result.current.setCode('A7KM-ZQR3-FNXP');
      });

      let firstSubmit: Promise<void> = Promise.resolve();
      act(() => {
        firstSubmit = result.current.submit();
      });

      // While the first call is in flight, a second submit should be a no-op.
      await act(async () => {
        await result.current.submit();
      });

      expect(spy.calls).toEqual(['A7KM-ZQR3-FNXP']);

      await act(async () => {
        spy.resolvePending(PRO_RESULT);
        await firstSubmit;
      });
    });
  });
});
