import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, beforeEach } from 'vitest';
import { RedeemOfferCode } from '../RedeemOfferCode';
import { SpyRedeemOfferCodeClient } from './spies/spy-redeem-offer-code-client';
import type { RedeemResult } from '../api/types';
import { RedeemError } from '../api/types';

const PRO_RESULT: RedeemResult = {
  tier: 'Pro',
  expiresAt: '2026-05-18T00:00:00Z',
};

describe('RedeemOfferCode', () => {
  let spy: SpyRedeemOfferCodeClient;

  beforeEach(() => {
    spy = new SpyRedeemOfferCodeClient();
  });

  it('renders a labelled input and a disabled submit button when the code is empty', () => {
    render(<RedeemOfferCode client={spy.client} />);

    const input = screen.getByLabelText(/offer code/i);
    expect(input).toBeInTheDocument();

    const button = screen.getByRole('button', { name: /redeem/i });
    expect(button).toBeDisabled();
  });

  it('uppercases typed input', async () => {
    const user = userEvent.setup();
    render(<RedeemOfferCode client={spy.client} />);

    const input = screen.getByLabelText(/offer code/i) as HTMLInputElement;
    await user.type(input, 'a7km-zqr3-fnxp');

    expect(input.value).toBe('A7KM-ZQR3-FNXP');
  });

  it('enables the submit button once a code is entered', async () => {
    const user = userEvent.setup();
    render(<RedeemOfferCode client={spy.client} />);

    await user.type(screen.getByLabelText(/offer code/i), 'A7KM-ZQR3-FNXP');

    expect(screen.getByRole('button', { name: /redeem/i })).toBeEnabled();
  });

  it('submits the normalized code when the button is clicked', async () => {
    const user = userEvent.setup();
    spy.result = PRO_RESULT;
    render(<RedeemOfferCode client={spy.client} />);

    await user.type(screen.getByLabelText(/offer code/i), 'a7km-zqr3-fnxp');
    await user.click(screen.getByRole('button', { name: /redeem/i }));

    expect(spy.calls).toEqual(['A7KM-ZQR3-FNXP']);
  });

  it('shows the error message when submission fails with a known code', async () => {
    const user = userEvent.setup();
    spy.error = new RedeemError('invalid_code', 'nope');
    render(<RedeemOfferCode client={spy.client} />);

    await user.type(screen.getByLabelText(/offer code/i), 'BAD1-CODE-XXXX');
    await user.click(screen.getByRole('button', { name: /redeem/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/isn't valid/i);
  });

  it('shows a success message with the tier on a successful redemption', async () => {
    const user = userEvent.setup();
    spy.result = PRO_RESULT;
    render(<RedeemOfferCode client={spy.client} />);

    await user.type(screen.getByLabelText(/offer code/i), 'A7KM-ZQR3-FNXP');
    await user.click(screen.getByRole('button', { name: /redeem/i }));

    expect(await screen.findByRole('status')).toHaveTextContent(/pro/i);
  });

  it('calls the onSuccess prop with the result after a successful redemption', async () => {
    const user = userEvent.setup();
    spy.result = PRO_RESULT;
    const onSuccessCalls: RedeemResult[] = [];

    render(
      <RedeemOfferCode
        client={spy.client}
        onSuccess={(r) => { onSuccessCalls.push(r); }}
      />,
    );

    await user.type(screen.getByLabelText(/offer code/i), 'A7KM-ZQR3-FNXP');
    await user.click(screen.getByRole('button', { name: /redeem/i }));

    // Wait for the success status to appear
    await screen.findByRole('status');

    expect(onSuccessCalls).toEqual([PRO_RESULT]);
  });

  it('shows a loading indicator on the submit button while submitting', async () => {
    const user = userEvent.setup();
    spy.deferNext();
    render(<RedeemOfferCode client={spy.client} />);

    await user.type(screen.getByLabelText(/offer code/i), 'A7KM-ZQR3-FNXP');
    await user.click(screen.getByRole('button', { name: /redeem/i }));

    // The button should now indicate loading and be disabled.
    const button = screen.getByRole('button', { name: /redeem/i });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('aria-busy', 'true');

    spy.resolvePending(PRO_RESULT);
    await screen.findByRole('status');
  });

  it('allows redeeming another code after a successful redemption via a reset button', async () => {
    const user = userEvent.setup();
    spy.result = PRO_RESULT;
    render(<RedeemOfferCode client={spy.client} />);

    await user.type(screen.getByLabelText(/offer code/i), 'A7KM-ZQR3-FNXP');
    await user.click(screen.getByRole('button', { name: /redeem/i }));

    await screen.findByRole('status');

    // In the success state, a "Redeem another" button should appear.
    const resetButton = screen.getByRole('button', { name: /redeem another/i });
    await user.click(resetButton);

    // After reset, the input is cleared and the submit button is disabled again.
    const input = screen.getByLabelText(/offer code/i) as HTMLInputElement;
    expect(input.value).toBe('');
    expect(screen.getByRole('button', { name: /redeem/i })).toBeDisabled();
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });
});
