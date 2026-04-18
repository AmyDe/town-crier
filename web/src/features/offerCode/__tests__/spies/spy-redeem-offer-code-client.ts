import type { RedeemOfferCodeClient } from '../../api/redeemOfferCode';
import { RedeemError, type RedeemResult } from '../../api/types';

/**
 * Hand-written test double for the offer-code redemption client.
 *
 * The production client is a function `(code: string) => Promise<RedeemResult>`,
 * so the spy exposes `.client` bound to this instance as the drop-in replacement.
 * Tests can read `calls` to verify invocations and set `result` / `error` to
 * script outcomes. No `vi.fn()` / `vi.mock()` — consistent with the repo-wide
 * manual-fakes convention.
 */
export class SpyRedeemOfferCodeClient {
  calls: string[] = [];
  result: RedeemResult = { tier: 'Pro', expiresAt: '2026-05-18T00:00:00Z' };
  error: RedeemError | null = null;

  private _deferred: {
    promise: Promise<RedeemResult>;
    resolve: (value: RedeemResult) => void;
    reject: (reason: unknown) => void;
  } | null = null;

  readonly client: RedeemOfferCodeClient = async (code) => {
    this.calls.push(code);
    if (this._deferred) {
      return this._deferred.promise;
    }
    if (this.error) {
      throw this.error;
    }
    return this.result;
  };

  /**
   * Switch the spy into "pending" mode — the next call to `client` returns a
   * promise that only resolves/rejects when `resolvePending` / `rejectPending`
   * is invoked. Useful for asserting transient `submitting` state.
   */
  deferNext(): void {
    let resolve: (value: RedeemResult) => void = () => {};
    let reject: (reason: unknown) => void = () => {};
    const promise = new Promise<RedeemResult>((res, rej) => {
      resolve = res;
      reject = rej;
    });
    this._deferred = { promise, resolve, reject };
  }

  resolvePending(value: RedeemResult): void {
    this._deferred?.resolve(value);
    this._deferred = null;
  }

  rejectPending(reason: unknown): void {
    this._deferred?.reject(reason);
    this._deferred = null;
  }
}
