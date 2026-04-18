export type Tier = 'Personal' | 'Pro';

export interface RedeemResult {
  readonly tier: Tier;
  readonly expiresAt: string; // ISO-8601
}

export type RedeemErrorCode =
  | 'invalid_code_format'
  | 'invalid_code'
  | 'code_already_redeemed'
  | 'already_subscribed'
  | 'network';

export class RedeemError extends Error {
  readonly code: RedeemErrorCode;

  constructor(code: RedeemErrorCode, message: string) {
    super(message);
    this.name = 'RedeemError';
    this.code = code;
  }
}
