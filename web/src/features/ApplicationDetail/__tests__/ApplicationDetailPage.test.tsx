import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ApplicationDetailPage } from '../ApplicationDetailPage';
import { SpyApplicationRepository } from './spies/spy-application-repository';
import { SpyDesignationRepository } from './spies/spy-designation-repository';
import { SpySavedApplicationRepository } from './spies/spy-saved-application-repository';
import {
  fullApplication,
  approvedWithDecision,
  applicationWithoutCoordinates,
  applicationWithoutUrl,
} from './fixtures/planning-application.fixtures';
import {
  conservationAreaDesignation,
  allDesignations,
  noDesignations,
} from './fixtures/designation-context.fixtures';
import { asApplicationUid } from '../../../domain/types';

function renderPage(
  appRepo: SpyApplicationRepository = new SpyApplicationRepository(),
  desigRepo: SpyDesignationRepository = new SpyDesignationRepository(),
  savedRepo: SpySavedApplicationRepository = new SpySavedApplicationRepository(),
  uid = 'APP-001',
) {
  return render(
    <MemoryRouter initialEntries={[`/applications/${uid}`]}>
      <Routes>
        <Route
          path="/applications/*"
          element={
            <ApplicationDetailPage
              applicationRepository={appRepo}
              designationRepository={desigRepo}
              savedApplicationRepository={savedRepo}
            />
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe('ApplicationDetailPage', () => {
  it('renders the application reference as a heading', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication();

    renderPage(appRepo);

    expect(
      await screen.findByRole('heading', { name: '2026/0042/FUL' }),
    ).toBeInTheDocument();
  });

  it('renders the address', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication();

    renderPage(appRepo);

    expect(
      await screen.findByText('12 Mill Road, Cambridge, CB1 2AD'),
    ).toBeInTheDocument();
  });

  it('renders the description', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication();

    renderPage(appRepo);

    expect(
      await screen.findByText(
        'Erection of two-storey rear extension with associated landscaping',
      ),
    ).toBeInTheDocument();
  });

  it('renders the status badge', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication();

    renderPage(appRepo);

    expect(await screen.findByText('Undecided')).toBeInTheDocument();
  });

  it('renders the application type', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication();

    renderPage(appRepo);

    expect(await screen.findByText('Full Planning')).toBeInTheDocument();
  });

  it('renders the authority name', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication();

    renderPage(appRepo);

    expect(
      await screen.findByText('Cambridge City Council'),
    ).toBeInTheDocument();
  });

  it('renders the received date', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication({ startDate: '2026-01-15' });

    renderPage(appRepo);

    expect(await screen.findByText('15 Jan 2026')).toBeInTheDocument();
  });

  it('renders the decided date when present', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = approvedWithDecision();

    renderPage(appRepo);

    expect(await screen.findByText('10 Mar 2026')).toBeInTheDocument();
  });

  it('renders the consultation date when present', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication({ consultedDate: '2026-02-01' });

    renderPage(appRepo);

    expect(await screen.findByText('1 Feb 2026')).toBeInTheDocument();
  });

  it('renders a council portal link when URL is available', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication();

    renderPage(appRepo);

    const link = await screen.findByRole('link', {
      name: /view on council portal/i,
    });
    expect(link).toHaveAttribute(
      'href',
      'https://council.example.com/planning/APP-001',
    );
  });

  it('does not render a council portal link when URL is null', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = applicationWithoutUrl();

    renderPage(appRepo);

    await screen.findByRole('heading', { name: '2026/0042/FUL' });

    expect(
      screen.queryByRole('link', { name: /view on council portal/i }),
    ).not.toBeInTheDocument();
  });

  it('renders designation context when coordinates are present', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication();

    const desigRepo = new SpyDesignationRepository();
    desigRepo.fetchDesignationsResult = conservationAreaDesignation();

    renderPage(appRepo, desigRepo);

    expect(
      await screen.findByText(/Mill Road Conservation Area/),
    ).toBeInTheDocument();
  });

  it('renders all designation types', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication();

    const desigRepo = new SpyDesignationRepository();
    desigRepo.fetchDesignationsResult = allDesignations();

    renderPage(appRepo, desigRepo);

    expect(
      await screen.findByText(/Historic Town Centre/),
    ).toBeInTheDocument();
    expect(screen.getByText(/Grade II/)).toBeInTheDocument();
    expect(screen.getByText(/Article 4/i)).toBeInTheDocument();
  });

  it('does not render designation section when no designations apply', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication();

    const desigRepo = new SpyDesignationRepository();
    desigRepo.fetchDesignationsResult = noDesignations();

    renderPage(appRepo, desigRepo);

    await screen.findByRole('heading', { name: '2026/0042/FUL' });

    expect(screen.queryByText(/Conservation Area/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Listed Building/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Article 4/i)).not.toBeInTheDocument();
  });

  it('does not fetch designations when coordinates are null', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = applicationWithoutCoordinates();

    const desigRepo = new SpyDesignationRepository();

    renderPage(appRepo, desigRepo);

    await screen.findByRole('heading', { name: '2026/0042/FUL' });

    expect(desigRepo.fetchDesignationsCalls).toHaveLength(0);
  });

  it('renders a save button that toggles on click', async () => {
    const user = userEvent.setup();
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication();

    const savedRepo = new SpySavedApplicationRepository();
    savedRepo.listSavedResult = [];

    renderPage(appRepo, undefined, savedRepo);

    const saveButton = await screen.findByRole('button', { name: /save/i });
    expect(saveButton).toBeInTheDocument();

    await user.click(saveButton);

    expect(savedRepo.saveCalls).toEqual([asApplicationUid('APP-001')]);
  });

  it('shows unsave when application is already saved', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationResult = fullApplication();

    const savedRepo = new SpySavedApplicationRepository();
    savedRepo.listSavedResult = [
      { applicationUid: asApplicationUid('APP-001'), savedAt: '2026-03-01' },
    ];

    renderPage(appRepo, undefined, savedRepo);

    const unsaveButton = await screen.findByRole('button', { name: /saved/i });
    expect(unsaveButton).toBeInTheDocument();
  });

  it('shows a loading state', () => {
    const appRepo = new SpyApplicationRepository();

    renderPage(appRepo);

    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('shows an error state when fetch fails', async () => {
    const appRepo = new SpyApplicationRepository();
    appRepo.fetchApplicationError = new Error('Application not found');

    renderPage(appRepo);

    expect(
      await screen.findByText(/application not found/i),
    ).toBeInTheDocument();
  });
});
