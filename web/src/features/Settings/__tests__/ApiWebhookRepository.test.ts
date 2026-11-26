import { describe, it, expect } from 'vitest';
import { createApiClient } from '../../../api/client';
import { WebhookError } from '../../../domain/types';
import { ApiWebhookRepository } from '../ApiWebhookRepository';
import { activeWebhook } from './fixtures/webhook.fixtures';

interface RecordedRequest {
  url: string;
  method: string;
  body: string | undefined;
}

function repositoryReturning(status: number, body?: unknown) {
  const requests: RecordedRequest[] = [];
  const fetchFn: typeof fetch = async (input, init) => {
    requests.push({
      url: String(input),
      method: init?.method ?? 'GET',
      body: init?.body as string | undefined,
    });
    return new Response(body === undefined ? null : JSON.stringify(body), { status });
  };
  const client = createApiClient('https://api.test', async () => 'token', fetchFn);
  return { repository: new ApiWebhookRepository(client), requests };
}

async function rejectionOf(promise: Promise<unknown>): Promise<unknown> {
  try {
    await promise;
  } catch (err) {
    return err;
  }
  throw new Error('Expected a rejection');
}

describe('ApiWebhookRepository', () => {
  it('reads the webhook', async () => {
    const { repository, requests } = repositoryReturning(200, activeWebhook());

    await expect(repository.fetchWebhook()).resolves.toEqual(activeWebhook());
    expect(requests).toEqual([
      { url: 'https://api.test/v1/me/webhook', method: 'GET', body: undefined },
    ]);
  });

  it('returns null when no webhook is set', async () => {
    const { repository } = repositoryReturning(404, {
      error: 'webhook_not_found',
      message: "You haven't set up a webhook.",
    });

    await expect(repository.fetchWebhook()).resolves.toBeNull();
  });

  it('does not read any other 404 as "no webhook"', async () => {
    const { repository } = repositoryReturning(404);

    await expect(repository.fetchWebhook()).rejects.toThrow();
  });

  it('returns the secret apart from the webhook on create', async () => {
    const { repository, requests } = repositoryReturning(201, {
      ...activeWebhook(),
      secret: 'tcwh_new',
    });

    const result = await repository.saveWebhook({
      url: 'https://hooks.example.com/abc',
      format: 'json',
    });

    expect(result).toStrictEqual({ webhook: activeWebhook(), secret: 'tcwh_new' });
    expect(requests).toEqual([
      {
        url: 'https://api.test/v1/me/webhook',
        method: 'PUT',
        body: '{"url":"https://hooks.example.com/abc","format":"json"}',
      },
    ]);
  });

  it('returns a null secret on replace', async () => {
    const { repository } = repositoryReturning(200, activeWebhook());

    const result = await repository.saveWebhook({ url: 'https://a.example', format: 'slack' });

    expect(result.secret).toBeNull();
  });

  it('removes the webhook', async () => {
    const { repository, requests } = repositoryReturning(204);

    await repository.removeWebhook();

    expect(requests[0]).toMatchObject({ method: 'DELETE', url: 'https://api.test/v1/me/webhook' });
  });

  it('rotates the secret', async () => {
    const { repository, requests } = repositoryReturning(200, { secret: 'tcwh_rotated' });

    await expect(repository.rotateSecret()).resolves.toBe('tcwh_rotated');
    expect(requests[0]).toMatchObject({
      method: 'POST',
      url: 'https://api.test/v1/me/webhook/secret',
    });
  });

  it('sends a test event', async () => {
    const outcome = { delivered: false, status: 500, failureReason: 'http_status' };
    const { repository, requests } = repositoryReturning(200, outcome);

    await expect(repository.sendTestEvent()).resolves.toEqual(outcome);
    expect(requests[0]).toMatchObject({
      method: 'POST',
      url: 'https://api.test/v1/me/webhook/test',
    });
  });

  it.each([
    [400, 'webhook_url_invalid'],
    [400, 'webhook_format_invalid'],
    [403, 'webhook_requires_pro'],
    [404, 'webhook_not_found'],
    [429, 'webhook_test_cooldown'],
  ])('maps a %i %s response to a WebhookError', async (status, code) => {
    const { repository } = repositoryReturning(status, { error: code, message: 'Nope.' });

    const error = await rejectionOf(repository.sendTestEvent());

    expect(error).toBeInstanceOf(WebhookError);
    expect((error as WebhookError).code).toBe(code);
  });

  it('passes an unknown error through', async () => {
    const { repository } = repositoryReturning(500);

    const error = await rejectionOf(repository.removeWebhook());

    expect(error).not.toBeInstanceOf(WebhookError);
  });
});
