import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import { WatchZoneEditPage } from '../WatchZoneEditPage';
import { SpyWatchZoneRepository } from './spies/spy-watch-zone-repository';
import { aWatchZone, zonePreferences } from './fixtures/watch-zone.fixtures';

function renderWithRouter(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe('WatchZoneEditPage', () => {
  let spy: SpyWatchZoneRepository;

  beforeEach(() => {
    spy = new SpyWatchZoneRepository();
  });

  it('renders zone name and four notification preference checkboxes', async () => {
    spy.getPreferencesResult = zonePreferences({
      newApplicationPush: true,
      newApplicationEmail: true,
      decisionPush: false,
      decisionEmail: true,
    });

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    const nameInput = screen.getByRole('textbox', { name: /zone name/i });
    expect(nameInput).toHaveValue('Home');

    // Wait for preferences to load — each event x channel pair has its own toggle
    const newAppPushCheckbox = await screen.findByRole('checkbox', {
      name: /new applications.*push/i,
    });
    expect(newAppPushCheckbox).toBeChecked();

    const newAppEmailCheckbox = screen.getByRole('checkbox', {
      name: /new applications.*email/i,
    });
    expect(newAppEmailCheckbox).toBeChecked();

    const decisionPushCheckbox = screen.getByRole('checkbox', {
      name: /decision.*push/i,
    });
    expect(decisionPushCheckbox).not.toBeChecked();

    const decisionEmailCheckbox = screen.getByRole('checkbox', {
      name: /decision.*email/i,
    });
    expect(decisionEmailCheckbox).toBeChecked();
  });

  it('saves updated preferences when a per-channel toggle is clicked', async () => {
    const user = userEvent.setup();
    spy.getPreferencesResult = zonePreferences({
      newApplicationPush: true,
      newApplicationEmail: true,
      decisionPush: false,
      decisionEmail: true,
    });

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    const decisionPushCheckbox = await screen.findByRole('checkbox', {
      name: /decision.*push/i,
    });

    // Update preferences result for the refetch
    spy.getPreferencesResult = zonePreferences({
      newApplicationPush: true,
      newApplicationEmail: true,
      decisionPush: true,
      decisionEmail: true,
    });

    await user.click(decisionPushCheckbox);

    expect(spy.updatePreferencesCalls).toHaveLength(1);
    expect(spy.updatePreferencesCalls[0]?.data).toEqual({
      newApplicationPush: true,
      newApplicationEmail: true,
      decisionPush: true,
      decisionEmail: true,
    });
  });

  it('shows loading state while preferences load', () => {
    spy.getPreferences = () => new Promise(() => {});

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('shows error when preferences fail to load', async () => {
    spy.getPreferencesError = new Error('Not found');

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    expect(await screen.findByText('Not found')).toBeInTheDocument();
  });

  it('shows zone details (radius)', async () => {
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone({ radiusMetres: 2000 })} />,
    );

    await screen.findByRole('checkbox', { name: /new applications.*push/i });
    const selectedRadio = screen.getByRole('radio', { name: '2 km' });
    expect(selectedRadio).toBeChecked();
  });

  it('has a back link to the list', () => {
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    const backLink = screen.getByRole('link', { name: /back to watch zones/i });
    expect(backLink).toHaveAttribute('href', '/watch-zones');
  });

  it('renders zone name in an editable text input', async () => {
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone({ name: 'Home' })} />,
    );

    const nameInput = screen.getByRole('textbox', { name: /zone name/i });
    expect(nameInput).toHaveValue('Home');
  });

  it('renders radius as an editable radio group', async () => {
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone({ radiusMetres: 2000 })} />,
    );

    const radiusGroup = screen.getByRole('radiogroup', { name: /radius/i });
    expect(radiusGroup).toBeInTheDocument();

    const selectedRadio = screen.getByRole('radio', { name: '2 km' });
    expect(selectedRadio).toBeChecked();
  });

  it('shows save button when name changes', async () => {
    const user = userEvent.setup();
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone({ name: 'Home' })} />,
    );

    const nameInput = screen.getByRole('textbox', { name: /zone name/i });
    await user.clear(nameInput);
    await user.type(nameInput, 'Office');

    const saveButton = screen.getByRole('button', { name: /save/i });
    expect(saveButton).toBeInTheDocument();
    expect(saveButton).toBeEnabled();
  });

  it('hides save button when no changes are made', () => {
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    expect(screen.queryByRole('button', { name: /save/i })).not.toBeInTheDocument();
  });

  it('calls updateZone on save', async () => {
    const user = userEvent.setup();
    spy.getPreferencesResult = zonePreferences();
    spy.updateZoneResult = aWatchZone({ name: 'Office' });

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone({ name: 'Home' })} />,
    );

    const nameInput = screen.getByRole('textbox', { name: /zone name/i });
    await user.clear(nameInput);
    await user.type(nameInput, 'Office');

    const saveButton = screen.getByRole('button', { name: /save/i });
    await user.click(saveButton);

    expect(spy.updateZoneCalls).toHaveLength(1);
    expect(spy.updateZoneCalls[0]?.data).toEqual({ name: 'Office' });
  });

  it('shows validation error for empty zone name', async () => {
    const user = userEvent.setup();
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone({ name: 'Home' })} />,
    );

    const nameInput = screen.getByRole('textbox', { name: /zone name/i });
    await user.clear(nameInput);

    expect(screen.getByText('Zone name is required')).toBeInTheDocument();
  });

  it('shows save button when radius changes', async () => {
    const user = userEvent.setup();
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone({ radiusMetres: 2000 })} />,
    );

    const fiveKmRadio = screen.getByRole('radio', { name: '5 km' });
    await user.click(fiveKmRadio);

    const saveButton = screen.getByRole('button', { name: /save/i });
    expect(saveButton).toBeInTheDocument();
  });

  describe('large-radius warning', () => {
    it('shows the warning copy when the zone radius is at or above 2km', () => {
      spy.getPreferencesResult = zonePreferences();

      renderWithRouter(
        <WatchZoneEditPage repository={spy} zone={aWatchZone({ radiusMetres: 2000 })} />,
      );

      expect(
        screen.getByText(/hundreds of notifications a day/i),
      ).toBeInTheDocument();
      expect(screen.getByText(/100[–-]500m/i)).toBeInTheDocument();
      expect(screen.getByText(/under 2km/i)).toBeInTheDocument();
    });

    it('hides the warning when the zone radius is below 2km', () => {
      spy.getPreferencesResult = zonePreferences();

      renderWithRouter(
        <WatchZoneEditPage repository={spy} zone={aWatchZone({ radiusMetres: 1000 })} />,
      );

      expect(
        screen.queryByText(/hundreds of notifications a day/i),
      ).not.toBeInTheDocument();
    });

    it('appears when the user picks a large radius and disappears when they pick a small one', async () => {
      const user = userEvent.setup();
      spy.getPreferencesResult = zonePreferences();

      renderWithRouter(
        <WatchZoneEditPage repository={spy} zone={aWatchZone({ radiusMetres: 1000 })} />,
      );

      expect(
        screen.queryByText(/hundreds of notifications a day/i),
      ).not.toBeInTheDocument();

      await user.click(screen.getByRole('radio', { name: '5 km' }));

      expect(
        screen.getByText(/hundreds of notifications a day/i),
      ).toBeInTheDocument();

      await user.click(screen.getByRole('radio', { name: '1 km' }));

      expect(
        screen.queryByText(/hundreds of notifications a day/i),
      ).not.toBeInTheDocument();
    });
  });

  describe('per-zone notification toggles', () => {
    it('renders push and instant-email toggles for Pro tier reflecting zone state', async () => {
      spy.getPreferencesResult = zonePreferences();

      renderWithRouter(
        <WatchZoneEditPage
          repository={spy}
          zone={aWatchZone({ pushEnabled: true, emailInstantEnabled: false })}
          tier="Pro"
        />,
      );

      const pushSwitch = await screen.findByRole('switch', { name: /push notifications/i });
      const emailSwitch = screen.getByRole('switch', { name: /instant emails/i });

      expect(pushSwitch).toHaveAttribute('aria-checked', 'true');
      expect(emailSwitch).toHaveAttribute('aria-checked', 'false');
    });

    it('renders push and instant-email toggles for Personal tier', async () => {
      spy.getPreferencesResult = zonePreferences();

      renderWithRouter(
        <WatchZoneEditPage
          repository={spy}
          zone={aWatchZone({ pushEnabled: true, emailInstantEnabled: true })}
          tier="Personal"
        />,
      );

      expect(
        await screen.findByRole('switch', { name: /push notifications/i }),
      ).toBeInTheDocument();
      expect(screen.getByRole('switch', { name: /instant emails/i })).toBeInTheDocument();
    });

    it('hides push and instant-email toggles for Free tier', () => {
      spy.getPreferencesResult = zonePreferences();

      renderWithRouter(
        <WatchZoneEditPage repository={spy} zone={aWatchZone()} tier="Free" />,
      );

      expect(
        screen.queryByRole('switch', { name: /push notifications/i }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole('switch', { name: /instant emails/i }),
      ).not.toBeInTheDocument();
    });

    it('toggling push enables the save button and patches the flag on save', async () => {
      const user = userEvent.setup();
      spy.getPreferencesResult = zonePreferences();
      spy.updateZoneResult = aWatchZone({ pushEnabled: false, emailInstantEnabled: true });

      renderWithRouter(
        <WatchZoneEditPage
          repository={spy}
          zone={aWatchZone({ pushEnabled: true, emailInstantEnabled: true })}
          tier="Pro"
        />,
      );

      const pushSwitch = await screen.findByRole('switch', { name: /push notifications/i });
      await user.click(pushSwitch);

      const saveButton = screen.getByRole('button', { name: /save/i });
      expect(saveButton).toBeEnabled();

      await user.click(saveButton);

      expect(spy.updateZoneCalls).toHaveLength(1);
      expect(spy.updateZoneCalls[0]?.data).toEqual({ pushEnabled: false });
    });

    it('toggling instant-email enables the save button and patches the flag on save', async () => {
      const user = userEvent.setup();
      spy.getPreferencesResult = zonePreferences();
      spy.updateZoneResult = aWatchZone({ pushEnabled: true, emailInstantEnabled: false });

      renderWithRouter(
        <WatchZoneEditPage
          repository={spy}
          zone={aWatchZone({ pushEnabled: true, emailInstantEnabled: true })}
          tier="Pro"
        />,
      );

      const emailSwitch = await screen.findByRole('switch', { name: /instant emails/i });
      await user.click(emailSwitch);

      const saveButton = screen.getByRole('button', { name: /save/i });
      await user.click(saveButton);

      expect(spy.updateZoneCalls).toHaveLength(1);
      expect(spy.updateZoneCalls[0]?.data).toEqual({ emailInstantEnabled: false });
    });
  });
});
