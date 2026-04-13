import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { DashboardPage } from '../DashboardPage';
import { SpyDashboardPort } from './spies/spy-dashboard-port';
import {
  cambridgeZone,
  oxfordZone,
  recentApplication,
  anotherRecentApplication,
} from './fixtures/dashboard.fixtures';

function renderDashboard(spy: SpyDashboardPort) {
  return render(
    <MemoryRouter>
      <DashboardPage port={spy} />
    </MemoryRouter>,
  );
}

describe('DashboardPage', () => {
  it('renders watch zone summary cards', async () => {
    const spy = new SpyDashboardPort();
    spy.fetchWatchZonesResult = [cambridgeZone(), oxfordZone()];

    renderDashboard(spy);

    expect(await screen.findByText('Home - Cambridge')).toBeInTheDocument();
    expect(screen.getByText('Office - Oxford')).toBeInTheDocument();
  });

  it('renders recent applications using ApplicationCard', async () => {
    const spy = new SpyDashboardPort();
    spy.fetchWatchZonesResult = [cambridgeZone()];
    spy.fetchRecentApplicationsResults.set('zone-001', [
      recentApplication(),
      anotherRecentApplication(),
    ]);

    renderDashboard(spy);

    expect(await screen.findByText('2026/0042/FUL')).toBeInTheDocument();
    expect(screen.getByText('2026/0088/LBC')).toBeInTheDocument();
  });

  it('shows empty state when no watch zones exist', async () => {
    const spy = new SpyDashboardPort();
    spy.fetchWatchZonesResult = [];

    renderDashboard(spy);

    expect(await screen.findByText('No watch zones yet')).toBeInTheDocument();
  });

  it('renders quick links to saved applications and notifications', async () => {
    const spy = new SpyDashboardPort();
    spy.fetchWatchZonesResult = [cambridgeZone()];

    renderDashboard(spy);

    expect(await screen.findByRole('link', { name: /saved/i })).toHaveAttribute('href', '/saved');
    expect(screen.getByRole('link', { name: /notifications/i })).toHaveAttribute('href', '/notifications');
  });

  it('shows loading state initially', () => {
    const spy = new SpyDashboardPort();
    spy.fetchWatchZones = () => new Promise(() => {}); // never resolves

    renderDashboard(spy);

    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('shows error state when fetch fails', async () => {
    const spy = new SpyDashboardPort();
    spy.fetchWatchZones = async () => {
      throw new Error('Network unavailable');
    };

    renderDashboard(spy);

    expect(await screen.findByText('Network unavailable')).toBeInTheDocument();
  });
});
