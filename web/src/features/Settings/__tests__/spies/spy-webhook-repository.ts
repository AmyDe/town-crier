import type { WebhookRepository } from '../../../../domain/ports/webhook-repository';
import type {
  SaveWebhookRequest,
  SavedWebhook,
  Webhook,
  WebhookTestResult,
} from '../../../../domain/types';
import { activeWebhook } from '../fixtures/webhook.fixtures';

export class SpyWebhookRepository implements WebhookRepository {
  fetchWebhookCalls = 0;
  fetchWebhookResult: Webhook | null = null;
  fetchWebhookError: Error | null = null;

  async fetchWebhook(): Promise<Webhook | null> {
    this.fetchWebhookCalls++;
    if (this.fetchWebhookError) {
      throw this.fetchWebhookError;
    }
    return this.fetchWebhookResult;
  }

  saveWebhookCalls: SaveWebhookRequest[] = [];
  saveWebhookResult: SavedWebhook = { webhook: activeWebhook(), secret: 'tcwh_created' };
  saveWebhookError: Error | null = null;

  async saveWebhook(request: SaveWebhookRequest): Promise<SavedWebhook> {
    this.saveWebhookCalls.push(request);
    if (this.saveWebhookError) {
      throw this.saveWebhookError;
    }
    return this.saveWebhookResult;
  }

  removeWebhookCalls = 0;
  removeWebhookError: Error | null = null;

  async removeWebhook(): Promise<void> {
    this.removeWebhookCalls++;
    if (this.removeWebhookError) {
      throw this.removeWebhookError;
    }
  }

  rotateSecretCalls = 0;
  rotateSecretResult = 'tcwh_rotated';
  rotateSecretError: Error | null = null;

  async rotateSecret(): Promise<string> {
    this.rotateSecretCalls++;
    if (this.rotateSecretError) {
      throw this.rotateSecretError;
    }
    return this.rotateSecretResult;
  }

  sendTestEventCalls = 0;
  sendTestEventResult: WebhookTestResult = { delivered: true, status: 200, failureReason: null };
  sendTestEventError: Error | null = null;
  /** Set to hold the next test event open until the returned promise settles. */
  sendTestEventGate: Promise<void> | null = null;

  async sendTestEvent(): Promise<WebhookTestResult> {
    this.sendTestEventCalls++;
    await this.sendTestEventGate;
    if (this.sendTestEventError) {
      throw this.sendTestEventError;
    }
    return this.sendTestEventResult;
  }
}
