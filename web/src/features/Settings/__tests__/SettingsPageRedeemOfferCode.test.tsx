import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { SettingsPage } from '../SettingsPage';
import { SpySettingsRepository } from './spies/spy-settings-repository';
import { freeUserProfile, proUserProfile } from './fixtures/user-profile.fixtures';
import { AuthProvider } from '../../../auth/auth-context';
import { SpyAuthPort } from '../../../auth/__tests__/spies/spy-auth-port';
import { SpyRedeemOfferCodeClient } from '../../offerCode/__tests__/spies/spy-redeem-offer-code-client';
import type { RedeemResult } from '../../offerCode/api/types';

vi.mock('../../../hooks/useTheme', () => ({
  useTheme: () => ({ theme: 'light' as const, toggleTheme: vi.fn() }),
}));

const PRO_RESULT: RedeemResult = {
  tier: 'Pro',
  expiresAt: '2026-05-18T00:00:00Z',
};

describe('SettingsPage — RedeemOfferCode integration', () => {
  let spyRepo: SpySettingsRepository;
  let spyAuth: SpyAuthPort;
  let spyClient: SpyRedeemOfferCodeClient;

  beforeEach(() => {
    spyRepo = new SpySettingsRepository();
    spyAuth = new SpyAuthPort();
    spyClient = new SpyRedeemOfferCodeClient();
  });

  function renderPage(options: { onRedeemSuccess?: (r: RedeemResult) => void } = {}) {
    return render(
      <MemoryRouter>
        <AuthProvider value={spyAuth}>
          <SettingsPage
            repository={spyRepo}
            redeemOfferCodeClient={spyClient.client}
            onRedeemSuccess={options.onRedeemSuccess}
          />
        </AuthProvider>
      </MemoryRouter>,
    );
  }

  it('renders the redeem offer code section', async () => {
    spyRepo.fetchProfileResult = freeUserProfile();

    renderPage();

    expect(
      await screen.findByRole('heading', { name: /redeem offer code/i }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/offer code/i)).toBeInTheDocument();
  });

  it('does not render the section when no redeem client is provided', async () => {
    spyRepo.fetchProfileResult = freeUserProfile();

    render(
      <MemoryRouter>
        <AuthProvider value={spyAuth}>
          <SettingsPage repository={spyRepo} />
        </AuthProvider>
      </MemoryRouter>,
    );

    // Wait for page to finish loading so the assertion isn't racing the skeleton.
    await screen.findByRole('heading', { name: /settings/i });
    expect(
      screen.queryByRole('heading', { name: /redeem offer code/i }),
    ).not.toBeInTheDocument();
  });

  it('submits the entered code through the injected client', async () => {
    const user = userEvent.setup();
    spyRepo.fetchProfileResult = freeUserProfile();
    spyClient.result = PRO_RESULT;

    renderPage();

    await screen.findByRole('heading', { name: /redeem offer code/i });

    await user.type(screen.getByLabelText(/offer code/i), 'a7km-zqr3-fnxp');
    await user.click(screen.getByRole('button', { name: /^redeem$/i }));

    expect(spyClient.calls).toEqual(['A7KM-ZQR3-FNXP']);
  });

  it('invokes onRedeemSuccess with the result after a successful redemption', async () => {
    const user = userEvent.setup();
    spyRepo.fetchProfileResult = freeUserProfile();
    spyClient.result = PRO_RESULT;

    const onRedeemSuccessCalls: RedeemResult[] = [];
    renderPage({ onRedeemSuccess: (r) => { onRedeemSuccessCalls.push(r); } });

    await screen.findByRole('heading', { name: /redeem offer code/i });

    await user.type(screen.getByLabelText(/offer code/i), 'A7KM-ZQR3-FNXP');
    await user.click(screen.getByRole('button', { name: /^redeem$/i }));

    // Wait for the redemption to complete (success status renders).
    await screen.findByText(/you're on pro/i);

    expect(onRedeemSuccessCalls).toEqual([PRO_RESULT]);
  });

  it('re-fetches the profile after a successful redemption so tier-gated UI updates', async () => {
    const user = userEvent.setup();
    // First load: free. After redemption: pro.
    spyRepo.fetchProfileResult = freeUserProfile();
    spyClient.result = PRO_RESULT;

    renderPage();

    // Initial render shows Free tier.
    expect(await screen.findByText('Free')).toBeInTheDocument();
    const initialFetchCount = spyRepo.fetchProfileCalls;

    // Script the repo to return Pro on the next fetch.
    spyRepo.fetchProfileResult = proUserProfile();

    await user.type(screen.getByLabelText(/offer code/i), 'A7KM-ZQR3-FNXP');
    await user.click(screen.getByRole('button', { name: /^redeem$/i }));

    // Wait for the success state to render.
    await screen.findByText(/you're on pro/i);

    // The settings page should have triggered a profile refresh.
    expect(spyRepo.fetchProfileCalls).toBeGreaterThan(initialFetchCount);
    expect(await screen.findByText('Pro')).toBeInTheDocument();
  });
});
