import { useState } from 'react';
import { Link } from 'react-router';
import { ConfirmDialog } from '../../components/ConfirmDialog/ConfirmDialog';
import type { WebhookRepository } from '../../domain/ports/webhook-repository';
import type { SubscriptionTier, Webhook, WebhookTestResult } from '../../domain/types';
import { useWebhook } from './useWebhook';
import { WebhookForm } from './WebhookForm';
import { WebhookSecret } from './WebhookSecret';
import { describeSwitchOff, describeWebhookFailure, formatWebhookTime } from './webhookCopy';
import settingsStyles from './SettingsPage.module.css';
import styles from './WebhookSection.module.css';

interface Props {
  repository: WebhookRepository;
  tier: SubscriptionTier;
}

// A full page load, so the browser scrolls to the anchor on the landing page.
function PricingLink() {
  return (
    <a href="/#pricing" className={styles.link}>
      See Pro pricing
    </a>
  );
}

function WebhookStatus({ webhook }: { webhook: Webhook }) {
  const isDisabled = webhook.status === 'disabled';
  const status = webhook.paused
    ? { label: 'Paused', className: styles.pausedStatus }
    : isDisabled
      ? { label: 'Switched off', className: styles.switchedOff }
      : { label: 'Active', className: styles.active };
  return (
    <>
      <dl className={styles.details}>
        <div className={styles.detail}>
          <dt className={styles.term}>Status</dt>
          <dd className={status.className}>{status.label}</dd>
        </div>
        <div className={styles.detail}>
          <dt className={styles.term}>Last success</dt>
          <dd className={styles.description}>
            {webhook.lastSuccessAt ? formatWebhookTime(webhook.lastSuccessAt) : 'None yet'}
          </dd>
        </div>
        <div className={styles.detail}>
          <dt className={styles.term}>Last failure</dt>
          <dd className={styles.description}>
            {webhook.lastFailureAt
              ? `${formatWebhookTime(webhook.lastFailureAt)}. ${describeWebhookFailure(webhook.lastFailureReason, webhook.lastFailureStatus)}`
              : 'None yet'}
          </dd>
        </div>
      </dl>
      {isDisabled && (
        <p className={styles.notice}>
          {describeSwitchOff(webhook.disabledReason)}
          {!webhook.paused && ' To switch it back on, check the address and save it again.'}
        </p>
      )}
    </>
  );
}

function TestOutcome({ result }: { result: WebhookTestResult }) {
  if (result.delivered) {
    return (
      <p className={styles.success} role="status">
        {result.status === null
          ? 'Test event delivered.'
          : `Test event delivered (status ${result.status}).`}
      </p>
    );
  }
  return (
    <p className={styles.error} role="status">
      {`Test event not delivered. ${describeWebhookFailure(result.failureReason, result.status)}`}
    </p>
  );
}

export function WebhookSection({ repository, tier }: Props) {
  const {
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
  } = useWebhook(repository);
  const [showRemoveDialog, setShowRemoveDialog] = useState(false);
  const isBusy = pendingAction !== null;

  const removeButton = (
    <button
      type="button"
      className={styles.dangerButton}
      onClick={() => setShowRemoveDialog(true)}
      disabled={isBusy}
    >
      {pendingAction === 'remove' ? 'Removing...' : 'Remove'}
    </button>
  );

  function renderContent() {
    if (isLoading) {
      return <p className={styles.hint}>Loading...</p>;
    }

    if (loadError) {
      return (
        <>
          <p className={styles.error} role="alert">{loadError}</p>
          <button type="button" className={styles.button} onClick={reload}>
            Try again
          </button>
        </>
      );
    }

    // A webhook can outlive Pro. It stays visible so that it can be removed.
    if (webhook?.paused) {
      return (
        <>
          <p className={styles.notice}>
            We&apos;ve paused alerts to this webhook. They&apos;ll resume when Pro is active again.{' '}
            <PricingLink />
          </p>
          <p className={styles.url}>{webhook.url}</p>
          <p className={styles.hint}>{webhook.format === 'slack' ? 'Slack' : 'JSON'}</p>
          <WebhookStatus webhook={webhook} />
          <div className={styles.actions}>{removeButton}</div>
        </>
      );
    }

    if (!webhook && tier !== 'Pro') {
      return (
        <p className={styles.hint}>
          Webhooks are part of Pro. <PricingLink />
        </p>
      );
    }

    return (
      <>
        <p className={styles.hint}>
          Send your alerts to your own URL, as signed JSON or a Slack message. They go out at the
          end of each hourly check.{' '}
          <Link to="/developers/webhooks" className={styles.link}>
            Read the webhook docs
          </Link>
        </p>
        <WebhookForm
          key={webhook ? 'edit' : 'create'}
          initialUrl={webhook?.url ?? ''}
          initialFormat={webhook?.format ?? 'json'}
          isBusy={isBusy}
          isSaving={pendingAction === 'save'}
          onSave={save}
        />
        {revealedSecret && <WebhookSecret key={revealedSecret} secret={revealedSecret} />}
        {webhook && (
          <>
            <WebhookStatus webhook={webhook} />
            <div className={styles.actions}>
              <button type="button" className={styles.button} onClick={sendTest} disabled={isBusy}>
                {pendingAction === 'test' ? 'Sending...' : 'Send test event'}
              </button>
              <button
                type="button"
                className={styles.button}
                onClick={rotateSecret}
                disabled={isBusy}
              >
                {pendingAction === 'rotate' ? 'Rotating...' : 'Rotate secret'}
              </button>
              {removeButton}
            </div>
            {testResult && <TestOutcome result={testResult} />}
          </>
        )}
      </>
    );
  }

  return (
    <section className={settingsStyles.section}>
      <h2 className={settingsStyles.sectionTitle}>Webhooks</h2>
      <div className={settingsStyles.card}>
        <div className={styles.body}>
          {renderContent()}
          {actionError && (
            <p className={styles.error} role="alert">{actionError}</p>
          )}
        </div>
      </div>
      <ConfirmDialog
        open={showRemoveDialog}
        title="Remove webhook"
        message="We'll stop sending alerts to this address and delete its signing secret. Your alerts still arrive in the app as normal."
        confirmLabel="Remove"
        onConfirm={async () => {
          setShowRemoveDialog(false);
          await remove();
        }}
        onCancel={() => setShowRemoveDialog(false)}
      />
    </section>
  );
}
