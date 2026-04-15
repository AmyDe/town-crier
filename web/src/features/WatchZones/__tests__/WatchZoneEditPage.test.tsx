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

  it('renders zone name and notification preferences', async () => {
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    expect(screen.getByText('Home')).toBeInTheDocument();

    // Wait for preferences to load
    const newAppsCheckbox = await screen.findByRole('checkbox', { name: /new applications/i });
    expect(newAppsCheckbox).toBeChecked();

    const statusCheckbox = screen.getByRole('checkbox', { name: /status changes/i });
    expect(statusCheckbox).toBeChecked();

    const decisionCheckbox = screen.getByRole('checkbox', { name: /decision updates/i });
    expect(decisionCheckbox).not.toBeChecked();
  });

  it('saves updated preferences on toggle', async () => {
    const user = userEvent.setup();
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    const decisionCheckbox = await screen.findByRole('checkbox', { name: /decision updates/i });

    // Update preferences result for the refetch
    spy.getPreferencesResult = zonePreferences({ decisionUpdates: true });

    await user.click(decisionCheckbox);

    expect(spy.updatePreferencesCalls).toHaveLength(1);
    expect(spy.updatePreferencesCalls[0]?.data.decisionUpdates).toBe(true);
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
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    await screen.findByRole('checkbox', { name: /new applications/i });
    expect(screen.getByText('2 km radius')).toBeInTheDocument();
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
});
