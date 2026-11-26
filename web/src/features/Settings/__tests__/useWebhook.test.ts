import { act, renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { WebhookError, type WebhookErrorCode } from '../../../domain/types';
import { useWebhook } from '../useWebhook';
import { activeWebhook } from './fixtures/webhook.fixtures';
import { SpyWebhookRepository } from './spies/spy-webhook-repository';

const REQUEST = { url: 'https://hooks.example.com/abc', format: 'json' } as const;

async function loadedHook(repository: SpyWebhookRepository) {
  const hook = renderHook(() => useWebhook(repository));
  await waitFor(() => expect(hook.result.current.isLoading).toBe(false));
  return hook;
}

describe('useWebhook', () => {
  let spy: SpyWebhookRepository;

  beforeEach(() => {
    spy = new SpyWebhookRepository();
  });

  it('loads the current webhook', async () => {
    spy.fetchWebhookResult = activeWebhook();

    const { result } = await loadedHook(spy);

    expect(result.current.webhook).toEqual(activeWebhook());
    expect(result.current.loadError).toBeNull();
    expect(result.current.revealedSecret).toBeNull();
    expect(spy.fetchWebhookCalls).toBe(1);
  });

  it('loads as null when no webhook is set', async () => {
    const { result } = await loadedHook(spy);

    expect(result.current.webhook).toBeNull();
    expect(result.current.loadError).toBeNull();
  });

  it('reports a load failure and can try again', async () => {
    spy.fetchWebhookError = new Error('boom');
    const { result } = await loadedHook(spy);

    expect(result.current.loadError).toBe(
      "We couldn't load your webhook. Try again in a moment.",
    );

    spy.fetchWebhookError = null;
    spy.fetchWebhookResult = activeWebhook();
    await act(async () => result.current.reload());

    await waitFor(() => expect(result.current.webhook).toEqual(activeWebhook()));
    expect(result.current.loadError).toBeNull();
  });

  it('reveals the secret once when a save creates the webhook', async () => {
    const hook = await loadedHook(spy);

    await act(() => hook.result.current.save(REQUEST));

    expect(spy.saveWebhookCalls).toEqual([REQUEST]);
    expect(hook.result.current.webhook).toEqual(activeWebhook());
    expect(hook.result.current.revealedSecret).toBe('tcwh_created');

    hook.unmount();
    spy.fetchWebhookResult = activeWebhook();
    const second = await loadedHook(spy);

    expect(second.result.current.revealedSecret).toBeNull();
  });

  it('keeps no copy of the secret in web storage', async () => {
    window.localStorage.clear();
    window.sessionStorage.clear();
    const { result } = await loadedHook(spy);

    await act(() => result.current.save(REQUEST));

    expect(window.localStorage.length).toBe(0);
    expect(window.sessionStorage.length).toBe(0);
  });

  it('replaces the webhook without revealing a secret', async () => {
    spy.fetchWebhookResult = activeWebhook({ status: 'disabled', disabledReason: 'gone' });
    const replaced = activeWebhook({ url: 'https://new.example.com', format: 'slack' });
    spy.saveWebhookResult = { webhook: replaced, secret: null };
    const { result } = await loadedHook(spy);

    await act(() => result.current.save({ url: replaced.url, format: 'slack' }));

    expect(result.current.webhook).toEqual(replaced);
    expect(result.current.revealedSecret).toBeNull();
  });

  it('removes the webhook and forgets the secret', async () => {
    const { result } = await loadedHook(spy);
    await act(() => result.current.save(REQUEST));

    await act(() => result.current.remove());

    expect(spy.removeWebhookCalls).toBe(1);
    expect(result.current.webhook).toBeNull();
    expect(result.current.revealedSecret).toBeNull();
  });

  it('keeps the webhook when remove fails', async () => {
    spy.fetchWebhookResult = activeWebhook();
    spy.removeWebhookError = new Error('boom');
    const { result } = await loadedHook(spy);

    await act(() => result.current.remove());

    expect(result.current.webhook).toEqual(activeWebhook());
    expect(result.current.actionError).toBe(
      "We couldn't remove your webhook. Try again in a moment.",
    );
  });

  it('reveals the new secret after a rotation', async () => {
    spy.fetchWebhookResult = activeWebhook();
    const { result } = await loadedHook(spy);

    await act(() => result.current.rotateSecret());

    expect(spy.rotateSecretCalls).toBe(1);
    expect(result.current.revealedSecret).toBe('tcwh_rotated');
  });

  it('returns the test result, and clears it after a save', async () => {
    spy.fetchWebhookResult = activeWebhook();
    spy.sendTestEventResult = { delivered: false, status: 500, failureReason: 'http_status' };
    spy.saveWebhookResult = { webhook: activeWebhook(), secret: null };
    const { result } = await loadedHook(spy);

    await act(() => result.current.sendTest());

    expect(result.current.testResult).toEqual(spy.sendTestEventResult);

    await act(() => result.current.save(REQUEST));

    expect(result.current.testResult).toBeNull();
  });

  it('starts no second action while one is running', async () => {
    spy.fetchWebhookResult = activeWebhook();
    let release: () => void = () => {};
    spy.sendTestEventGate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const { result } = await loadedHook(spy);

    let first: Promise<void> = Promise.resolve();
    act(() => {
      first = result.current.sendTest();
    });

    expect(result.current.pendingAction).toBe('test');
    await act(async () => {
      await result.current.sendTest();
      await result.current.remove();
    });
    await act(async () => {
      release();
      await first;
    });

    expect(spy.sendTestEventCalls).toBe(1);
    expect(spy.removeWebhookCalls).toBe(0);
    expect(result.current.pendingAction).toBeNull();
  });

  it.each<[WebhookErrorCode, string]>([
    ['webhook_url_invalid', 'Enter a public https address. Only port 443 works.'],
    ['webhook_format_invalid', 'Choose JSON or Slack.'],
    ['webhook_requires_pro', 'Webhooks need a Pro subscription.'],
    ['webhook_test_cooldown', 'Wait 10 seconds before you send another test.'],
  ])('explains %s', async (code, message) => {
    spy.saveWebhookError = new WebhookError(code);
    const { result } = await loadedHook(spy);

    await act(() => result.current.save(REQUEST));

    expect(result.current.actionError).toBe(message);
    expect(result.current.webhook).toBeNull();
  });

  it.each<['save' | 'sendTest' | 'rotateSecret', string]>([
    ['save', "We couldn't save your webhook. Try again in a moment."],
    ['sendTest', "We couldn't send the test event. Try again in a moment."],
    ['rotateSecret', "We couldn't rotate the secret. Try again in a moment."],
  ])('hides the raw error when %s fails', async (action, message) => {
    spy.saveWebhookError = new Error('Request failed with status 500');
    spy.sendTestEventError = new Error('Request failed with status 500');
    spy.rotateSecretError = new Error('Request failed with status 500');
    const { result } = await loadedHook(spy);

    await act(() =>
      action === 'save' ? result.current.save(REQUEST) : result.current[action](),
    );

    expect(result.current.actionError).toBe(message);
  });

  it('drops a webhook the API no longer has', async () => {
    spy.fetchWebhookResult = activeWebhook();
    spy.sendTestEventError = new WebhookError('webhook_not_found');
    const { result } = await loadedHook(spy);

    await act(() => result.current.sendTest());

    expect(result.current.webhook).toBeNull();
    expect(result.current.actionError).toBe('That webhook no longer exists.');
  });

  it('clears an earlier error when the next action starts', async () => {
    spy.fetchWebhookResult = activeWebhook();
    spy.sendTestEventError = new WebhookError('webhook_test_cooldown');
    const { result } = await loadedHook(spy);
    await act(() => result.current.sendTest());

    spy.sendTestEventError = null;
    await act(() => result.current.sendTest());

    expect(result.current.actionError).toBeNull();
  });
});
