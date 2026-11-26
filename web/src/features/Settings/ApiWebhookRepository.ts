import { ApiRequestError, type ApiClient } from '../../api/client';
import type { WebhookRepository } from '../../domain/ports/webhook-repository';
import {
  WebhookError,
  type SaveWebhookRequest,
  type SavedWebhook,
  type Webhook,
  type WebhookErrorCode,
  type WebhookTestResult,
} from '../../domain/types';

const PATH = '/v1/me/webhook';

const ERROR_CODES: readonly WebhookErrorCode[] = [
  'webhook_not_found',
  'webhook_requires_pro',
  'webhook_url_invalid',
  'webhook_format_invalid',
  'webhook_test_cooldown',
];

function toDomainError(err: unknown): unknown {
  if (err instanceof ApiRequestError) {
    const code = ERROR_CODES.find((known) => known === err.code);
    if (code) {
      return new WebhookError(code);
    }
  }
  return err;
}

export class ApiWebhookRepository implements WebhookRepository {
  private readonly client: ApiClient;

  constructor(client: ApiClient) {
    this.client = client;
  }

  async fetchWebhook(): Promise<Webhook | null> {
    try {
      return await this.client.get<Webhook>(PATH);
    } catch (err) {
      const error = toDomainError(err);
      if (error instanceof WebhookError && error.code === 'webhook_not_found') {
        return null;
      }
      throw error;
    }
  }

  async saveWebhook(request: SaveWebhookRequest): Promise<SavedWebhook> {
    const { secret, ...webhook } = await this.call(() =>
      this.client.put<Webhook & { secret?: string }>(PATH, request),
    );
    return { webhook, secret: secret ?? null };
  }

  async removeWebhook(): Promise<void> {
    await this.call(() => this.client.delete(PATH));
  }

  async rotateSecret(): Promise<string> {
    const response = await this.call(() =>
      this.client.post<{ secret: string }>(`${PATH}/secret`),
    );
    return response.secret;
  }

  async sendTestEvent(): Promise<WebhookTestResult> {
    return this.call(() => this.client.post<WebhookTestResult>(`${PATH}/test`));
  }

  private async call<T>(operation: () => Promise<T>): Promise<T> {
    try {
      return await operation();
    } catch (err) {
      throw toDomainError(err);
    }
  }
}
