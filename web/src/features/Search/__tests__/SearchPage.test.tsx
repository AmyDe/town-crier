import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { SearchPage } from '../SearchPage';
import { SpySearchPort } from './spies/spy-search-port';
import { SpyPostcodeLookupPort } from './spies/spy-postcode-lookup-port';
import { aSearchResult, anotherSearchResult } from './fixtures/search-result.fixtures';

const h = vi.hoisted(() => {
  let capturedClickHandler: ((event: { latlng: { lat: number; lng: number } }) => void) | null = null;
  return {
    setClickHandler: (fn: typeof capturedClickHandler) => {
      capturedClickHandler = fn;
    },
    fireMapClick: (lat: number, lng: number) => {
      capturedClickHandler?.({ latlng: { lat, lng } });
    },
  };
});

vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: ({ position }: { position: [number, number] }) => (
    <div data-testid="marker" data-lat={position[0]} data-lon={position[1]} />
  ),
  useMapEvents: (handlers: { click: (event: { latlng: { lat: number; lng: number } }) => void }) => {
    h.setClickHandler(handlers.click);
    return null;
  },
}));

/** Confirms a location the fastest way available in a test: a direct map tap. */
function confirmALocationViaMap() {
  fireEvent.click(screen.getByRole('button', { name: /set a location to search/i }));
  act(() => {
    h.fireMapClick(51.5, -0.1);
  });
}

function renderPage(searchPort: SpySearchPort, postcodePort = new SpyPostcodeLookupPort()) {
  return render(<SearchPage port={searchPort} postcodePort={postcodePort} />);
}

describe('SearchPage', () => {
  afterEach(() => {
    document.title = '';
  });

  it('renders a heading and a search input', () => {
    renderPage(new SpySearchPort());

    expect(screen.getByRole('heading', { name: /search planning applications/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/search/i)).toBeInTheDocument();
  });

  it('sets an indexable, search-specific document title and meta description', () => {
    const originalDescription = document.createElement('meta');
    originalDescription.setAttribute('name', 'description');
    originalDescription.setAttribute('content', 'Town Crier home');
    document.head.appendChild(originalDescription);

    const { unmount } = renderPage(new SpySearchPort());

    expect(document.title).toMatch(/search/i);
    expect(document.title).toMatch(/town crier/i);
    expect(originalDescription.getAttribute('content')).toMatch(/search/i);

    unmount();
    expect(originalDescription.getAttribute('content')).toBe('Town Crier home');
    document.head.removeChild(originalDescription);
  });

  it('disables the search input while no location is set, regardless of query text', () => {
    renderPage(new SpySearchPort());

    const input = screen.getByLabelText(/search/i);
    expect(input).toBeDisabled();
  });

  it('shows a "Set a location" prompt before any location is confirmed', () => {
    renderPage(new SpySearchPort());

    expect(screen.getByRole('button', { name: /set a location to search/i })).toBeInTheDocument();
    expect(screen.queryByTestId('map-container')).not.toBeInTheDocument();
  });

  it('expands the inline picker when the prompt is clicked, never a full-page picker or modal', () => {
    renderPage(new SpySearchPort());

    fireEvent.click(screen.getByRole('button', { name: /set a location to search/i }));

    expect(screen.getByTestId('map-container')).toBeInTheDocument();
    // Still on the same page — the heading and search input are still present,
    // proving this is an inline expansion, not a route change or a dialog.
    expect(screen.getByRole('heading', { name: /search planning applications/i })).toBeInTheDocument();
  });

  it('enables the search input and shows a location chip once a location is confirmed', () => {
    renderPage(new SpySearchPort());

    confirmALocationViaMap();

    expect(screen.getByLabelText(/search/i)).not.toBeDisabled();
    expect(screen.getByText(/near custom pin/i)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /set a location to search/i })).not.toBeInTheDocument();
  });

  it('reopens the picker, prefilled with the current selection, via Change', () => {
    renderPage(new SpySearchPort());
    confirmALocationViaMap();

    fireEvent.click(screen.getByRole('button', { name: /change/i }));

    expect(screen.getByTestId('map-container')).toBeInTheDocument();
    const marker = screen.getByTestId('marker');
    expect(marker.getAttribute('data-lat')).toBe('51.5');
    expect(marker.getAttribute('data-lon')).toBe('-0.1');
  });

  it('re-confirming after Change updates the chip label', () => {
    renderPage(new SpySearchPort());
    confirmALocationViaMap();

    fireEvent.click(screen.getByRole('button', { name: /change/i }));
    act(() => {
      h.fireMapClick(52.2, 0.1);
    });

    expect(screen.getByText(/near custom pin/i)).toBeInTheDocument();
    expect(screen.queryByTestId('map-container')).not.toBeInTheDocument();
  });

  it('never renders any radius slider or radius UI', () => {
    renderPage(new SpySearchPort());
    confirmALocationViaMap();

    expect(screen.queryByRole('slider')).not.toBeInTheDocument();
    expect(screen.queryByText(/radius/i)).not.toBeInTheDocument();
  });

  it('shows results after the debounced search resolves', async () => {
    const spy = new SpySearchPort();
    spy.searchResult = { results: [aSearchResult(), anotherSearchResult()], refineQuery: false };
    renderPage(spy);
    confirmALocationViaMap();

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
    renderPage(spy);
    confirmALocationViaMap();

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
    renderPage(spy);
    confirmALocationViaMap();

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
    renderPage(spy);
    confirmALocationViaMap();

    fireEvent.change(screen.getByLabelText(/search/i), { target: { value: 'mill road' } });

    await waitFor(
      () => {
        expect(screen.getByText('Request failed with status 500')).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it('passes the authority filter and the confirmed location through to the port', async () => {
    const spy = new SpySearchPort();
    renderPage(spy);
    confirmALocationViaMap();

    fireEvent.change(screen.getByLabelText(/council/i), { target: { value: 'cambridge' } });
    fireEvent.change(screen.getByLabelText(/search/i), { target: { value: 'mill road' } });

    await waitFor(
      () => {
        expect(spy.searchCalls).toHaveLength(1);
      },
      { timeout: 2000 },
    );
    expect(spy.searchCalls[0]).toEqual({
      query: 'mill road',
      authority: 'cambridge',
      location: { lat: 51.5, lon: -0.1 },
    });
  });

  it('does not search on mount, before the user has typed anything', async () => {
    const spy = new SpySearchPort();
    renderPage(spy);
    confirmALocationViaMap();

    await new Promise((r) => setTimeout(r, 600));
    expect(spy.searchCalls).toHaveLength(0);
  });

  it('never calls the port while location is unset, even if a query were somehow present', async () => {
    const spy = new SpySearchPort();
    renderPage(spy);

    await new Promise((r) => setTimeout(r, 600));
    expect(spy.searchCalls).toHaveLength(0);
  });
});
