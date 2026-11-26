import { useState, type FormEvent } from 'react';
import type { SaveWebhookRequest, WebhookFormat } from '../../domain/types';
import styles from './WebhookSection.module.css';

interface Props {
  initialUrl: string;
  initialFormat: WebhookFormat;
  isBusy: boolean;
  isSaving: boolean;
  onSave: (request: SaveWebhookRequest) => void;
}

export function WebhookForm({ initialUrl, initialFormat, isBusy, isSaving, onSave }: Props) {
  const [url, setUrl] = useState(initialUrl);
  const [format, setFormat] = useState<WebhookFormat>(initialFormat);

  function handleSubmit(event: FormEvent) {
    event.preventDefault();
    onSave({ url: url.trim(), format });
  }

  return (
    // The API owns URL validation, so its message is shown in place of the browser's.
    <form className={styles.form} onSubmit={handleSubmit} noValidate>
      <label htmlFor="webhook-url" className={styles.fieldLabel}>
        Webhook URL
      </label>
      <input
        id="webhook-url"
        type="url"
        className={styles.input}
        value={url}
        placeholder="https://"
        autoComplete="off"
        spellCheck={false}
        onChange={(event) => setUrl(event.target.value)}
      />
      <label htmlFor="webhook-format" className={styles.fieldLabel}>
        Format
      </label>
      <select
        id="webhook-format"
        className={styles.input}
        value={format}
        onChange={(event) => setFormat(event.target.value === 'slack' ? 'slack' : 'json')}
      >
        <option value="json">JSON</option>
        <option value="slack">Slack</option>
      </select>
      <button
        type="submit"
        className={styles.primaryButton}
        disabled={isBusy || url.trim() === ''}
      >
        {isSaving ? 'Saving...' : 'Save'}
      </button>
    </form>
  );
}
