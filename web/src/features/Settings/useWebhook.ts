import { useCallback, useEffect, useRef, useState } from 'react';
import type { WebhookRepository } from '../../domain/ports/webhook-repository';
import {
  WebhookError,
  type SaveWebhookRequest,
  type Webhook,
  type WebhookErrorCode,
  type WebhookTestResult,
} from '../../domain/types';

export type WebhookAction = 'save' | 'remove' | 'test' | 'rotate';

const LOAD_ERROR = "We couldn't load your webhook. Try again in a moment.";

const ACTION_ERRORS: Record<WebhookAction, string> = {
  save: "We couldn't save your webhook. Try again in a moment.",
  remove: "We couldn't remove your webhook. Try again in a moment.",
  test: "We couldn't send the test event. Try again in a moment.",
  rotate: "We couldn't rotate the secret. Try again in a moment.",
};

const CODE_ERRORS: Record<WebhookErrorCode, string> = {
  webhook_not_found: 'That webhook no longer exists.',
  webhook_requires_pro: 'Webhooks need a Pro subscription.',
  webhook_url_invalid: 'Enter a public https address. Only port 443 works.',
  webhook_format_invalid: 'Choose JSON or Slack.',
  webhook_test_cooldown: 'Wait 10 seconds before you send another test.',
};

export function useWebhook(repository: WebhookRepository) {
  const [webhook, setWebhook] = useState<Webhook | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [pendingAction, setPendingAction] = useState<WebhookAction | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [revealedSecret, setRevealedSecret] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<WebhookTestResult | null>(null);
  const isBusy = useRef(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const loaded = await repository.fetchWebhook();
        if (!cancelled) {
          setWebhook(loaded);
          setIsLoading(false);
        }
      } catch {
        if (!cancelled) {
          setLoadError(LOAD_ERROR);
          setIsLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [repository, loadAttempt]);

  const reload = useCallback(() => {
    setIsLoading(true);
    setLoadError(null);
    setLoadAttempt((attempt) => attempt + 1);
  }, []);

  const run = useCallback(async (action: WebhookAction, operation: () => Promise<void>) => {
    if (isBusy.current) return;
    isBusy.current = true;
    setPendingAction(action);
    setActionError(null);
    setTestResult(null);
    try {
      await operation();
    } catch (err) {
      if (err instanceof WebhookError) {
        if (err.code === 'webhook_not_found') {
          setWebhook(null);
          setRevealedSecret(null);
        }
        setActionError(CODE_ERRORS[err.code]);
      } else {
        setActionError(ACTION_ERRORS[action]);
      }
    } finally {
      isBusy.current = false;
      setPendingAction(null);
    }
  }, []);

  const save = useCallback(
    (request: SaveWebhookRequest) =>
      run('save', async () => {
        const saved = await repository.saveWebhook(request);
        setWebhook(saved.webhook);
        setRevealedSecret(saved.secret);
      }),
    [repository, run],
  );

  const remove = useCallback(
    () =>
      run('remove', async () => {
        await repository.removeWebhook();
        setWebhook(null);
        setRevealedSecret(null);
      }),
    [repository, run],
  );

  const rotateSecret = useCallback(
    () =>
      run('rotate', async () => {
        setRevealedSecret(await repository.rotateSecret());
      }),
    [repository, run],
  );

  const sendTest = useCallback(
    () =>
      run('test', async () => {
        setTestResult(await repository.sendTestEvent());
      }),
    [repository, run],
  );

  return {
    webhook,
    isLoading,
    loadError,
    pendingAction,
    actionError,
    revealedSecret,
    testResult,
    reload,
    save,
    remove,
    rotateSecret,
    sendTest,
  };
}
