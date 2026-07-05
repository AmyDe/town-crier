import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, afterEach } from 'vitest';
import { SearchPage } from '../SearchPage';
import { SpySearchPort } from './spies/spy-search-port';
import { aSearchResult, anotherSearchResult } from './fixtures/search-result.fixtures';

describe('SearchPage', () => {
  afterEach(() => {
    document.title = '';
  });

  it('renders a heading and a search input', () => {
    render(<SearchPage port={new SpySearchPort()} />);

    expect(screen.getByRole('heading', { name: /search planning applications/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/search/i)).toBeInTheDocument();
  });

  it('sets an indexable, search-specific document title and meta description', () => {
    const originalDescription = document.createElement('meta');
    originalDescription.setAttribute('name', 'description');
    originalDescription.setAttribute('content', 'Town Crier home');
    document.head.appendChild(originalDescription);

    const { unmount } = render(<SearchPage port={new SpySearchPort()} />);

    expect(document.title).toMatch(/search/i);
    expect(document.title).toMatch(/town crier/i);
    expect(originalDescription.getAttribute('content')).toMatch(/search/i);

    unmount();
    expect(originalDescription.getAttribute('content')).toBe('Town Crier home');
    document.head.removeChild(originalDescription);
  });

  it('shows results after the debounced search resolves', async () => {
    const spy = new SpySearchPort();
    spy.searchResult = { results: [aSearchResult(), anotherSearchResult()], refineQuery: false };
    render(<SearchPage port={spy} />);

    fireEvent.change(screen.getByLabelText(/search/i), { target: { value: 'mill road' } });

    await waitFor(
      () => {
        expect(screen.getByRole('heading', { name: '22/1234/FUL' })).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
    expect(screen.getByRole('heading', { name: '24/0001/FUL' })).toBeInTheDocument();
  });

  it('shows a refine-your-search notice when the match set was truncated', async () => {
    const spy = new SpySearchPort();
    spy.searchResult = { results: [aSearchResult()], refineQuery: true };
    render(<SearchPage port={spy} />);

    fireEvent.change(screen.getByLabelText(/search/i), { target: { value: 'road' } });

    await waitFor(
      () => {
        expect(screen.getByText(/more specific/i)).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it('shows an empty-results message when the search returns nothing', async () => {
    const spy = new SpySearchPort();
    spy.searchResult = { results: [], refineQuery: false };
    render(<SearchPage port={spy} />);

    fireEvent.change(screen.getByLabelText(/search/i), { target: { value: 'nonexistent' } });

    await waitFor(
      () => {
        expect(screen.getByText(/no applications matched/i)).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it('shows an error message when the search fails', async () => {
    const spy = new SpySearchPort();
    spy.searchError = new Error('Request failed with status 500');
    render(<SearchPage port={spy} />);

    fireEvent.change(screen.getByLabelText(/search/i), { target: { value: 'mill road' } });

    await waitFor(
      () => {
        expect(screen.getByText('Request failed with status 500')).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it('passes the authority filter through to the port', async () => {
    const spy = new SpySearchPort();
    render(<SearchPage port={spy} />);

    fireEvent.change(screen.getByLabelText(/council/i), { target: { value: 'cambridge' } });
    fireEvent.change(screen.getByLabelText(/search/i), { target: { value: 'mill road' } });

    await waitFor(
      () => {
        expect(spy.searchCalls).toHaveLength(1);
      },
      { timeout: 2000 },
    );
    expect(spy.searchCalls[0]).toEqual({ query: 'mill road', authority: 'cambridge' });
  });

  it('does not search on mount, before the user has typed anything', async () => {
    const spy = new SpySearchPort();
    render(<SearchPage port={spy} />);

    await new Promise((r) => setTimeout(r, 600));
    expect(spy.searchCalls).toHaveLength(0);
  });
});
