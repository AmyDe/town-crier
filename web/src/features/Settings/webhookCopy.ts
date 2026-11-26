import type { Webhook, WebhookFailureReason } from '../../domain/types';

export function describeWebhookFailure(
  reason: WebhookFailureReason | null,
  status: number | null,
): string {
  switch (reason) {
    case 'gone':
      return 'It answered 410, which means the address no longer exists.';
    case 'http_status':
      return status === null
        ? 'It answered with an error status.'
        : `It answered with status ${status}.`;
    case 'blocked_address':
      return "That address points somewhere we're not allowed to send to.";
    case 'timeout':
      return 'It took too long to answer.';
    case 'tls':
      return "We couldn't make a secure connection. Check its certificate.";
    case 'connection':
      return "We couldn't connect to it.";
    case null:
      return '';
  }
}

export function describeSwitchOff(reason: Webhook['disabledReason']): string {
  return reason === 'gone'
    ? "Your webhook told us it no longer exists, so we've switched it off."
    : "We couldn't deliver alerts to your webhook for a day, so we've switched it off.";
}

export function formatWebhookTime(instant: string): string {
  return new Date(instant).toLocaleString('en-GB', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}
