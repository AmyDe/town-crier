import { useState } from 'react';
import styles from './WebhookSection.module.css';

type CopyState = 'idle' | 'copied' | 'failed';

export function WebhookSecret({ secret }: { secret: string }) {
  const [copyState, setCopyState] = useState<CopyState>('idle');

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(secret);
      setCopyState('copied');
    } catch {
      setCopyState('failed');
    }
  }

  return (
    <div className={styles.secretPanel}>
      <label htmlFor="webhook-secret" className={styles.fieldLabel}>
        Signing secret
      </label>
      <div className={styles.secretRow}>
        <textarea
          id="webhook-secret"
          className={styles.secretInput}
          value={secret}
          rows={2}
          spellCheck={false}
          readOnly
          onFocus={(event) => event.currentTarget.select()}
        />
        <button type="button" className={styles.button} onClick={handleCopy}>
          {copyState === 'copied' ? 'Copied' : 'Copy'}
        </button>
      </div>
      <p className={styles.hint}>
        Copy this now. We only show it once. If you lose it, rotate the secret to get a new one.
      </p>
      {copyState === 'failed' && (
        <p className={styles.error} role="alert">
          We couldn&apos;t copy it. Select the secret and copy it yourself.
        </p>
      )}
    </div>
  );
}
