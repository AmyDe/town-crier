import type { RedeemOfferCodeClient } from '../../api/redeemOfferCode';
import { RedeemError, type RedeemResult } from '../../api/types';

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

  /** The next call to `client` stays pending until `resolvePending` or `rejectPending`. */
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
