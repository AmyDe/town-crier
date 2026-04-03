import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import { ApplicationsPage } from '../ApplicationsPage';
import { SpyUserAuthoritiesPort } from './spies/spy-user-authorities-port';
import { SpyApplicationsBrowsePort } from './spies/spy-applications-browse-port';
import { cornwallAuthority, bathAuthority } from './fixtures/authority.fixtures';
import {
  undecidedApplication,
  approvedApplication,
} from '../../../components/ApplicationCard/__tests__/fixtures/planning-application-summary.fixtures';

function renderPage(
  userAuthoritiesPort: SpyUserAuthoritiesPort,
  browsePort: SpyApplicationsBrowsePort,
) {
  return render(
    <MemoryRouter>
      <ApplicationsPage userAuthoritiesPort={userAuthoritiesPort} browsePort={browsePort} />
    </MemoryRouter>,
  );
}

describe('ApplicationsPage', () => {
  let userAuthoritiesPort: SpyUserAuthoritiesPort;
  let browsePort: SpyApplicationsBrowsePort;

  beforeEach(() => {
    userAuthoritiesPort = new SpyUserAuthoritiesPort();
    browsePort = new SpyApplicationsBrowsePort();
  });

  it('renders page heading', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [];
    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Applications' })).toBeInTheDocument();
    });
  });

  it('shows loading state while fetching authorities', () => {
    renderPage(userAuthoritiesPort, browsePort);

    expect(screen.getByText('Loading authorities...')).toBeInTheDocument();
  });

  it('shows empty state when user has no watch zones', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [];
    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(
        screen.getByText('Set up a watch zone to start browsing applications.'),
      ).toBeInTheDocument();
    });
  });

  it('shows authority cards when user has watch zones', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [cornwallAuthority(), bathAuthority()];
    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Cornwall Council')).toBeInTheDocument();
    });
    expect(screen.getByText('Bath and NE Somerset')).toBeInTheDocument();
  });

  it('shows applications when authority card is clicked', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [cornwallAuthority()];
    browsePort.fetchByAuthorityResult = [undecidedApplication(), approvedApplication()];
    const user = userEvent.setup();

    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Cornwall Council')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cornwall Council'));

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });

    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
    expect(browsePort.fetchByAuthorityCalls).toEqual([cornwallAuthority().id]);
  });

  it('shows breadcrumb when viewing applications', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [cornwallAuthority()];
    browsePort.fetchByAuthorityResult = [undecidedApplication()];
    const user = userEvent.setup();

    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Cornwall Council')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cornwall Council'));

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });

    expect(screen.getByRole('link', { name: 'Authorities' })).toBeInTheDocument();
  });

  it('returns to authority list when breadcrumb is clicked', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [cornwallAuthority(), bathAuthority()];
    browsePort.fetchByAuthorityResult = [undecidedApplication()];
    const user = userEvent.setup();

    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Cornwall Council')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cornwall Council'));

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('link', { name: 'Authorities' }));

    await waitFor(() => {
      expect(screen.getByText('Bath and NE Somerset')).toBeInTheDocument();
    });
    expect(screen.getByText('Cornwall Council')).toBeInTheDocument();
  });

  it('shows empty state when authority has no applications', async () => {
    userAuthoritiesPort.fetchMyAuthoritiesResult = [cornwallAuthority()];
    browsePort.fetchByAuthorityResult = [];
    const user = userEvent.setup();

    renderPage(userAuthoritiesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Cornwall Council')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cornwall Council'));

    await waitFor(() => {
      expect(
        screen.getByText('No applications found for this authority.'),
      ).toBeInTheDocument();
    });
  });
});
