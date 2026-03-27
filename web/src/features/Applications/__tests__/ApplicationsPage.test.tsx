import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import { ApplicationsPage } from '../ApplicationsPage';
import { SpyApplicationsBrowsePort } from './spies/spy-applications-browse-port';
import { SpyAuthoritySearchPort } from '../../../components/AuthoritySelector/__tests__/spies/spy-authority-search-port';
import {
  undecidedApplication,
  approvedApplication,
} from '../../../components/ApplicationCard/__tests__/fixtures/planning-application-summary.fixtures';
import {
  cambridgeAuthority,
  twoAuthorityResults,
} from '../../../components/AuthoritySelector/__tests__/fixtures/authority.fixtures';

function renderPage(
  browsePort: SpyApplicationsBrowsePort,
  searchPort: SpyAuthoritySearchPort,
) {
  return render(
    <MemoryRouter>
      <ApplicationsPage browsePort={browsePort} searchPort={searchPort} />
    </MemoryRouter>,
  );
}

describe('ApplicationsPage', () => {
  let browsePort: SpyApplicationsBrowsePort;
  let searchPort: SpyAuthoritySearchPort;

  beforeEach(() => {
    browsePort = new SpyApplicationsBrowsePort();
    searchPort = new SpyAuthoritySearchPort();
  });

  it('renders page heading and authority selector', () => {
    renderPage(browsePort, searchPort);

    expect(screen.getByRole('heading', { name: 'Applications' })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Search authorities' })).toBeInTheDocument();
  });

  it('shows empty state before authority is selected', () => {
    renderPage(browsePort, searchPort);

    expect(screen.getByText('Select an authority to browse planning applications.')).toBeInTheDocument();
  });

  it('shows application cards after authority is selected', async () => {
    searchPort.searchResult = twoAuthorityResults();
    browsePort.fetchByAuthorityResult = [undecidedApplication(), approvedApplication()];
    const user = userEvent.setup();

    renderPage(browsePort, searchPort);

    // Type to search for authority
    const input = screen.getByRole('combobox', { name: 'Search authorities' });
    await user.type(input, 'Cambridge');

    // Wait for search results to appear
    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    // Select the authority
    const option = screen.getByText('Cambridge City Council');
    await user.click(option);

    // Wait for applications to load
    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });

    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
    expect(browsePort.fetchByAuthorityCalls).toEqual([cambridgeAuthority().id]);
  });

  it('shows empty state when authority has no applications', async () => {
    searchPort.searchResult = twoAuthorityResults();
    browsePort.fetchByAuthorityResult = [];
    const user = userEvent.setup();

    renderPage(browsePort, searchPort);

    const input = screen.getByRole('combobox', { name: 'Search authorities' });
    await user.type(input, 'Cambridge');

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cambridge City Council'));

    await waitFor(() => {
      expect(screen.getByText('No applications found for this authority.')).toBeInTheDocument();
    });
  });
});
