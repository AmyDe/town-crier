import type { SaveWebhookRequest, SavedWebhook, Webhook, WebhookTestResult } from '../types';

/** The signed-in user's single webhook. Methods reject with `WebhookError` for known API refusals. */
export interface WebhookRepository {
  /** Resolves to null when no webhook is set. */
  fetchWebhook(): Promise<Webhook | null>;
  /** Creates the webhook, or replaces its URL and format and switches it back on. */
  saveWebhook(request: SaveWebhookRequest): Promise<SavedWebhook>;
  removeWebhook(): Promise<void>;
  /** Resolves to the new signing secret. */
  rotateSecret(): Promise<string>;
  sendTestEvent(): Promise<WebhookTestResult>;
}
