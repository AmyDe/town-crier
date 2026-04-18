import { useId } from 'react';
import type { RedeemOfferCodeClient } from './api/redeemOfferCode';
import type { RedeemResult } from './api/types';
import { useRedeemOfferCode } from './useRedeemOfferCode';
import styles from './RedeemOfferCode.module.css';

interface Props {
  client: RedeemOfferCodeClient;
  onSuccess?: (result: RedeemResult) => void;
}

/**
 * Form that lets an authenticated user redeem an offer code.
 *
 * Passive view bound to `useRedeemOfferCode` — all state and orchestration live
 * in the hook. On success the hook invokes `onSuccess` so consumers (the
 * Settings page) can refresh the Auth0 access token and the user profile.
 */
export function RedeemOfferCode({ client, onSuccess }: Props) {
  const inputId = useId();
  const {
    status,
    code,
    result,
    errorMessage,
    setCode,
    submit,
    reset,
  } = useRedeemOfferCode(client, { onSuccess });

  const isSubmitting = status === 'submitting';

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await submit();
  }

  if (status === 'success' && result !== null) {
    return (
      <div className={styles.container}>
        <div className={styles.success} role="status">
          <p className={styles.successTitle}>
            You&apos;re on {result.tier}.
          </p>
          <p className={styles.successDetail}>
            {formatExpiryMessage(result.expiresAt)}
          </p>
        </div>
        <button
          type="button"
          className={styles.resetButton}
          onClick={reset}
        >
          Redeem another code
        </button>
      </div>
    );
  }

  return (
    <form className={styles.container} onSubmit={handleSubmit} noValidate>
      <label htmlFor={inputId} className={styles.label}>
        Offer code
      </label>
      <div className={styles.row}>
        <input
          id={inputId}
          type="text"
          className={styles.input}
          value={code}
          onChange={(e) => setCode(e.target.value)}
          placeholder="XXXX-XXXX-XXXX"
          autoComplete="off"
          autoCapitalize="characters"
          spellCheck={false}
          disabled={isSubmitting}
        />
        <button
          type="submit"
          className={styles.button}
          disabled={isSubmitting || code.trim() === ''}
          aria-busy={isSubmitting}
        >
          {isSubmitting ? 'Redeeming…' : 'Redeem'}
        </button>
      </div>
      {status === 'error' && errorMessage !== null ? (
        <p className={styles.error} role="alert">
          {errorMessage}
        </p>
      ) : (
        <p className={styles.hint}>
          Enter the 12-character code you received. Dashes are optional.
        </p>
      )}
    </form>
  );
}

function formatExpiryMessage(expiresAtIso: string): string {
  const date = new Date(expiresAtIso);
  if (Number.isNaN(date.getTime())) {
    return 'Enjoy your upgraded plan.';
  }
  const formatted = date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  });
  return `Active until ${formatted}.`;
}
