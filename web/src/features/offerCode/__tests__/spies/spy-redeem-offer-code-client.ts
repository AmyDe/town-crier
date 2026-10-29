import type { RedeemOfferCodeClient } from '../../api/redeemOfferCode';
import { RedeemError, type RedeemResult } from '../../api/types';

export class SpyRedeemOfferCodeClient {
  calls: string[] = [];
  result: RedeemResult = { tier: 'Pro', expiresAt: '2026-05-18T00:00:00Z' };
  error: RedeemError | null = null;

  private _deferred: {
    promise: Promise<RedeemResult>;
    resolve: (value: RedeemResult) => void;
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

  /** The next call to `client` stays pending until `resolvePending`. */
  deferNext(): void {
    let resolve: (value: RedeemResult) => void = () => {};
    const promise = new Promise<RedeemResult>((res) => {
      resolve = res;
    });
    this._deferred = { promise, resolve };
  }

  resolvePending(value: RedeemResult): void {
    this._deferred?.resolve(value);
    this._deferred = null;
  }
}
