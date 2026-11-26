import type { Webhook } from '../../../../domain/types';

export function activeWebhook(overrides?: Partial<Webhook>): Webhook {
  return {
    url: 'https://hooks.example.com/abc',
    format: 'json',
    status: 'active',
    disabledReason: null,
    paused: false,
    createdAt: '2026-11-26T14:00:00Z',
    lastSuccessAt: null,
    lastFailureAt: null,
    lastFailureReason: null,
    lastFailureStatus: null,
    ...overrides,
  };
}
