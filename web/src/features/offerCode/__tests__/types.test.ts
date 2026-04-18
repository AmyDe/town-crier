import { describe, it, expect } from 'vitest';
import { RedeemError, type RedeemErrorCode } from '../api/types';

describe('RedeemError', () => {
  it('is an instance of Error', () => {
    const error = new RedeemError('invalid_code', 'boom');

    expect(error).toBeInstanceOf(Error);
  });

  it('exposes the error code and message', () => {
    const error = new RedeemError('code_already_redeemed', 'already used');

    expect(error.code).toBe('code_already_redeemed');
    expect(error.message).toBe('already used');
  });

  it('accepts each supported error code', () => {
    const codes: readonly RedeemErrorCode[] = [
      'invalid_code_format',
      'invalid_code',
      'code_already_redeemed',
      'already_subscribed',
      'network',
    ];

    for (const code of codes) {
      const error = new RedeemError(code, 'message');
      expect(error.code).toBe(code);
    }
  });
});
