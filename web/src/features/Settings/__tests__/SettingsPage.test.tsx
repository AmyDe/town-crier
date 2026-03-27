import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { SettingsPage } from '../SettingsPage';
import { SpySettingsRepository } from './spies/spy-settings-repository';
import { freeUserProfile, proUserProfile } from './fixtures/user-profile.fixtures';

// Stub useTheme — ThemeToggle receives theme/toggleTheme as props so we
// only need to verify the toggle is rendered and clickable.
vi.mock('../../../hooks/useTheme', () => ({
  useTheme: () => ({ theme: 'light' as const, toggleTheme: vi.fn() }),
}));

// Stub useAuth0 — SettingsPage gets logout from Auth0.
const mockLogout = vi.fn();
vi.mock('@auth0/auth0-react', () => ({
  useAuth0: () => ({ logout: mockLogout }),
}));

function renderSettingsPage(repository: SpySettingsRepository) {
  return render(
    <MemoryRouter>
      <SettingsPage repository={repository} />
    </MemoryRouter>,
  );
}

describe('SettingsPage', () => {
  let spy: SpySettingsRepository;

  beforeEach(() => {
    spy = new SpySettingsRepository();
    mockLogout.mockReset();
  });

  it('renders the page heading', async () => {
    renderSettingsPage(spy);

    expect(await screen.findByRole('heading', { name: /settings/i })).toBeInTheDocument();
  });

  it('renders profile info after loading', async () => {
    spy.fetchProfileResult = proUserProfile({
      postcode: 'SW1A 1AA',
    });

    renderSettingsPage(spy);

    expect(await screen.findByText('SW1A 1AA')).toBeInTheDocument();
    expect(screen.getByText('Pro')).toBeInTheDocument();
  });

  it('renders Free tier for free users', async () => {
    spy.fetchProfileResult = freeUserProfile();

    renderSettingsPage(spy);

    expect(await screen.findByText('Free')).toBeInTheDocument();
  });

  it('renders theme toggle', async () => {
    renderSettingsPage(spy);

    expect(await screen.findByLabelText(/switch to dark mode/i)).toBeInTheDocument();
  });

  it('renders export data button', async () => {
    renderSettingsPage(spy);

    expect(await screen.findByRole('button', { name: /export.*data/i })).toBeInTheDocument();
  });

  it('calls exportData when export button is clicked', async () => {
    const user = userEvent.setup();
    renderSettingsPage(spy);

    const button = await screen.findByRole('button', { name: /export.*data/i });
    await user.click(button);

    expect(spy.exportDataCalls).toBe(1);
  });

  it('renders delete account button', async () => {
    renderSettingsPage(spy);

    expect(await screen.findByRole('button', { name: /delete.*account/i })).toBeInTheDocument();
  });

  it('shows confirm dialog when delete account is clicked', async () => {
    const user = userEvent.setup();
    renderSettingsPage(spy);

    const deleteButton = await screen.findByRole('button', { name: /delete.*account/i });
    await user.click(deleteButton);

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText(/are you sure/i)).toBeInTheDocument();
  });

  it('deletes account when confirm dialog is confirmed', async () => {
    const user = userEvent.setup();
    renderSettingsPage(spy);

    const deleteButton = await screen.findByRole('button', { name: /delete.*account/i });
    await user.click(deleteButton);

    const dialog = screen.getByRole('dialog');
    const confirmButton = within(dialog).getByRole('button', { name: /delete/i });
    await user.click(confirmButton);

    expect(spy.deleteAccountCalls).toBe(1);
  });

  it('renders attribution section', async () => {
    renderSettingsPage(spy);

    expect(await screen.findByText(/planit/i)).toBeInTheDocument();
    expect(screen.getByText(/open government licence/i)).toBeInTheDocument();
    expect(screen.getByText(/ordnance survey/i)).toBeInTheDocument();
    expect(screen.getByText(/openstreetmap/i)).toBeInTheDocument();
  });

  it('renders legal links', async () => {
    renderSettingsPage(spy);

    const privacyLink = await screen.findByRole('link', { name: /privacy/i });
    const termsLink = screen.getByRole('link', { name: /terms/i });

    expect(privacyLink).toHaveAttribute('href', '/legal/privacy');
    expect(termsLink).toHaveAttribute('href', '/legal/terms');
  });

  it('shows error message when profile load fails', async () => {
    spy.fetchProfileError = new Error('Network error');

    renderSettingsPage(spy);

    expect(await screen.findByText(/network error/i)).toBeInTheDocument();
  });

  it('shows loading state initially', () => {
    renderSettingsPage(spy);

    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });
});
