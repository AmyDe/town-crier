import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, beforeEach } from 'vitest';
import { MemoryRouter } from 'react-router';
import { WebhookError, type SubscriptionTier, type Webhook } from '../../../domain/types';
import { WebhookSection } from '../WebhookSection';
import { activeWebhook } from './fixtures/webhook.fixtures';
import { SpyWebhookRepository } from './spies/spy-webhook-repository';

class SpyClipboard {
  writeTextCalls: string[] = [];
  writeTextError: Error | null = null;

  async writeText(text: string): Promise<void> {
    this.writeTextCalls.push(text);
    if (this.writeTextError) {
      throw this.writeTextError;
    }
  }
}

function installClipboard(clipboard: SpyClipboard) {
  Object.defineProperty(navigator, 'clipboard', { value: clipboard, configurable: true });
}

describe('WebhookSection', () => {
  let spy: SpyWebhookRepository;

  beforeEach(() => {
    spy = new SpyWebhookRepository();
  });

  async function renderSection(tier: SubscriptionTier = 'Pro') {
    render(
      <MemoryRouter>
        <WebhookSection repository={spy} tier={tier} />
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.queryByText('Loading...')).not.toBeInTheDocument());
  }

  describe('not Pro', () => {
    it.each<SubscriptionTier>(['Free', 'Personal'])(
      'shows %s users one line that links to pricing',
      async (tier) => {
        await renderSection(tier);

        expect(screen.getByText(/webhooks are part of pro\./i)).toBeInTheDocument();
        expect(screen.getByRole('link', { name: 'See Pro pricing' })).toHaveAttribute(
          'href',
          '/#pricing',
        );
        expect(screen.queryByLabelText('Webhook URL')).not.toBeInTheDocument();
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
      },
    );
  });

  describe('Pro with no webhook', () => {
    it('offers a URL, a format and Save, and links to the docs', async () => {
      await renderSection();

      expect(screen.getByRole('heading', { name: 'Webhooks' })).toBeInTheDocument();
      expect(screen.getByLabelText('Webhook URL')).toHaveValue('');
      const format = screen.getByLabelText('Format');
      expect(within(format).getAllByRole('option').map((o) => o.textContent)).toEqual([
        'JSON',
        'Slack',
      ]);
      expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
      expect(screen.getByRole('link', { name: 'Read the webhook docs' })).toHaveAttribute(
        'href',
        '/developers/webhooks',
      );
    });

    it('saves the trimmed URL and format, then shows the secret once with Copy', async () => {
      const user = userEvent.setup();
      const clipboard = new SpyClipboard();
      installClipboard(clipboard);
      spy.saveWebhookResult = {
        webhook: activeWebhook({ format: 'slack' }),
        secret: 'tcwh_created',
      };
      await renderSection();

      await user.type(screen.getByLabelText('Webhook URL'), ' https://hooks.example.com/abc ');
      await user.selectOptions(screen.getByLabelText('Format'), 'slack');
      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(spy.saveWebhookCalls).toEqual([
        { url: 'https://hooks.example.com/abc', format: 'slack' },
      ]);
      const secret = await screen.findByLabelText('Signing secret');
      expect(secret).toHaveValue('tcwh_created');
      expect(secret).toHaveAttribute('readonly');
      expect(secret.tagName).toBe('TEXTAREA');
      expect(secret).toHaveAttribute('rows', '3');
      expect(
        screen.getByText(
          "Copy this now. We only show it once. If you lose it, rotate the secret to get a new one.",
        ),
      ).toBeInTheDocument();

      installClipboard(clipboard);
      await user.click(screen.getByRole('button', { name: 'Copy' }));

      expect(clipboard.writeTextCalls).toEqual(['tcwh_created']);
      expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument();
    });

    it('says how to copy by hand when the clipboard refuses', async () => {
      const user = userEvent.setup();
      const clipboard = new SpyClipboard();
      clipboard.writeTextError = new Error('denied');
      await renderSection();
      await user.type(screen.getByLabelText('Webhook URL'), 'https://hooks.example.com/abc');
      await user.click(screen.getByRole('button', { name: 'Save' }));
      await screen.findByLabelText('Signing secret');

      installClipboard(clipboard);
      await user.click(screen.getByRole('button', { name: 'Copy' }));

      expect(
        await screen.findByText("We couldn't copy it. Select the secret and copy it yourself."),
      ).toBeInTheDocument();
    });

    it('shows why a save was refused and keeps what was typed', async () => {
      const user = userEvent.setup();
      spy.saveWebhookError = new WebhookError('webhook_url_invalid');
      await renderSection();

      await user.type(screen.getByLabelText('Webhook URL'), 'http://nope');
      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(await screen.findByRole('alert')).toHaveTextContent(
        'Enter a public https address. Only port 443 works.',
      );
      expect(screen.getByLabelText('Webhook URL')).toHaveValue('http://nope');
    });
  });

  describe('Pro with a webhook', () => {
    it('shows the URL, format, active status, times and actions', async () => {
      spy.fetchWebhookResult = activeWebhook({
        lastSuccessAt: '2026-11-26T12:00:00Z',
      });

      await renderSection();

      expect(screen.getByLabelText('Webhook URL')).toHaveValue('https://hooks.example.com/abc');
      expect(screen.getByLabelText('Format')).toHaveValue('json');
      expect(screen.getByText('Active')).toBeInTheDocument();
      expect(screen.getByText(/26 Nov 2026/)).toBeInTheDocument();
      expect(screen.getByText('None yet')).toBeInTheDocument();
      for (const name of ['Save', 'Send test event', 'Rotate secret', 'Remove']) {
        expect(screen.getByRole('button', { name })).toBeEnabled();
      }
      expect(screen.queryByLabelText('Signing secret')).not.toBeInTheDocument();
    });

    it.each([
      ['gone', 410, 'It answered 410, which means the address no longer exists.'],
      ['http_status', 500, 'It answered with status 500.'],
      ['http_status', null, 'It answered with an error status.'],
      ['blocked_address', null, "That address points somewhere we're not allowed to send to."],
      ['timeout', null, 'It took too long to answer.'],
      ['tls', null, "We couldn't make a secure connection. Check its certificate."],
      ['connection', null, "We couldn't connect to it."],
    ] as const)('explains a last failure of %s %s', async (reason, status, text) => {
      spy.fetchWebhookResult = activeWebhook({
        lastFailureAt: '2026-11-25T09:00:00Z',
        lastFailureReason: reason,
        lastFailureStatus: status,
      });

      await renderSection();

      expect(screen.getByText(text, { exact: false })).toHaveTextContent(/25 Nov 2026/);
    });

    it.each([
      [
        'failures',
        "We couldn't deliver alerts to your webhook for a day, so we've switched it off. To switch it back on, check the address and save it again.",
      ],
      [
        'gone',
        "Your webhook told us it no longer exists, so we've switched it off. To switch it back on, check the address and save it again.",
      ],
    ] as const)('says it is switched off because of %s', async (reason, text) => {
      spy.fetchWebhookResult = activeWebhook({ status: 'disabled', disabledReason: reason });

      await renderSection();

      expect(screen.getByText('Switched off')).toBeInTheDocument();
      expect(screen.getByText(text)).toBeInTheDocument();
    });

    it('saves an edited URL and format', async () => {
      const user = userEvent.setup();
      spy.fetchWebhookResult = activeWebhook();
      spy.saveWebhookResult = {
        webhook: activeWebhook({ url: 'https://new.example.com/x', format: 'slack' }),
        secret: null,
      };
      await renderSection();

      const url = screen.getByLabelText('Webhook URL');
      await user.clear(url);
      await user.type(url, 'https://new.example.com/x');
      await user.selectOptions(screen.getByLabelText('Format'), 'slack');
      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(spy.saveWebhookCalls).toEqual([
        { url: 'https://new.example.com/x', format: 'slack' },
      ]);
      expect(await screen.findByLabelText('Format')).toHaveValue('slack');
      expect(screen.queryByLabelText('Signing secret')).not.toBeInTheDocument();
    });

    it.each([
      [{ delivered: true, status: 204, failureReason: null }, 'Test event delivered (status 204).'],
      [
        { delivered: false, status: 500, failureReason: 'http_status' },
        'Test event not delivered. It answered with status 500.',
      ],
      [
        { delivered: false, status: null, failureReason: 'timeout' },
        'Test event not delivered. It took too long to answer.',
      ],
    ] as const)('shows the test result inline: %o', async (outcome, text) => {
      const user = userEvent.setup();
      spy.fetchWebhookResult = activeWebhook();
      spy.sendTestEventResult = outcome;
      await renderSection();

      await user.click(screen.getByRole('button', { name: 'Send test event' }));

      expect(await screen.findByRole('status')).toHaveTextContent(text);
    });

    it('rotates the secret and shows the new one', async () => {
      const user = userEvent.setup();
      spy.fetchWebhookResult = activeWebhook();
      await renderSection();

      await user.click(screen.getByRole('button', { name: 'Rotate secret' }));

      expect(await screen.findByLabelText('Signing secret')).toHaveValue('tcwh_rotated');
    });

    it('removes only after confirmation', async () => {
      const user = userEvent.setup();
      spy.fetchWebhookResult = activeWebhook();
      await renderSection();

      await user.click(screen.getByRole('button', { name: 'Remove' }));
      const dialog = screen.getByRole('dialog', { name: 'Remove webhook' });
      expect(dialog).toHaveTextContent(
        "We'll stop sending alerts to this address and delete its signing secret. Your alerts still arrive in the app as normal.",
      );
      await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
      expect(spy.removeWebhookCalls).toBe(0);

      await user.click(screen.getByRole('button', { name: 'Remove' }));
      await user.click(
        within(screen.getByRole('dialog', { name: 'Remove webhook' })).getByRole('button', {
          name: 'Remove',
        }),
      );

      expect(spy.removeWebhookCalls).toBe(1);
      await waitFor(() => expect(screen.getByLabelText('Webhook URL')).toHaveValue(''));
    });
  });

  describe('paused', () => {
    it('says alerts resume with Pro and offers only removal', async () => {
      spy.fetchWebhookResult = activeWebhook({ paused: true });

      await renderSection('Free');

      expect(
        screen.getByText(/we've paused alerts to this webhook\. they'll resume when pro is active again\./i),
      ).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'See Pro pricing' })).toBeInTheDocument();
      expect(screen.getByText('https://hooks.example.com/abc')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Remove' })).toBeInTheDocument();
      for (const name of ['Save', 'Send test event', 'Rotate secret']) {
        expect(screen.queryByRole('button', { name })).not.toBeInTheDocument();
      }
    });

    it.each<Partial<Webhook>>([
      { status: 'active' },
      { status: 'disabled', disabledReason: 'failures' },
    ])('shows a Paused status instead of %o', async (state) => {
      spy.fetchWebhookResult = activeWebhook({ ...state, paused: true });

      await renderSection('Free');

      expect(screen.getByText('Paused')).toBeInTheDocument();
      expect(screen.queryByText('Active')).not.toBeInTheDocument();
      expect(screen.queryByText('Switched off')).not.toBeInTheDocument();
    });
  });

  describe('load failure', () => {
    it('explains and loads again on request', async () => {
      const user = userEvent.setup();
      spy.fetchWebhookError = new Error('boom');
      await renderSection();

      expect(screen.getByRole('alert')).toHaveTextContent(
        "We couldn't load your webhook. Try again in a moment.",
      );

      spy.fetchWebhookError = null;
      await user.click(screen.getByRole('button', { name: 'Try again' }));

      expect(await screen.findByLabelText('Webhook URL')).toBeInTheDocument();
    });
  });
});
