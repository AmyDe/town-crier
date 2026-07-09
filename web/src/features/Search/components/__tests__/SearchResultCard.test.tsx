import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, beforeEach } from 'vitest';
import { SearchResultCard } from '../SearchResultCard';
import { aSearchResult, anotherSearchResult } from '../../__tests__/fixtures/search-result.fixtures';

class SpyClipboard {
  writeTextCalls: string[] = [];
  writeTextError: Error | null = null;

  writeText = async (text: string): Promise<void> => {
    this.writeTextCalls.push(text);
    if (this.writeTextError) {
      throw this.writeTextError;
    }
  };
}

describe('SearchResultCard', () => {
  let clipboard: SpyClipboard;

  beforeEach(() => {
    clipboard = new SpyClipboard();
  });

  // `userEvent.setup()` installs its own clipboard stub on `navigator.clipboard`
  // (testing-library/user-event's attachClipboardStubToView), unconditionally
  // overwriting anything set beforehand — so the spy must be installed AFTER
  // `userEvent.setup()`, not before, or clicks silently hit user-event's stub
  // instead of this test's spy.
  function setupUserWithSpyClipboard() {
    const user = userEvent.setup();
    Object.defineProperty(navigator, 'clipboard', {
      value: clipboard,
      configurable: true,
      writable: true,
    });
    return user;
  }

  it('renders the reference, address, and authority name', () => {
    render(<SearchResultCard result={aSearchResult({ reference: '22/1234/FUL' })} />);

    expect(screen.getByRole('heading', { name: '22/1234/FUL' })).toBeInTheDocument();
    expect(screen.getByText('12 Mill Road, Cambridge, CB1 2AD')).toBeInTheDocument();
    expect(screen.getByText('Cambridge City Council')).toBeInTheDocument();
  });

  it('shows a status badge with the user-facing label for a known appState', () => {
    render(<SearchResultCard result={aSearchResult({ appState: 'Permitted' })} />);

    expect(screen.getByText('Granted')).toBeInTheDocument();
  });

  it('renders no status badge when appState is null', () => {
    render(<SearchResultCard result={anotherSearchResult({ appState: null })} />);

    expect(screen.queryByText(/granted|refused|permitted|rejected/i)).not.toBeInTheDocument();
  });

  it('pairs the status badge with an icon glyph (colour is never the sole indicator)', () => {
    render(<SearchResultCard result={aSearchResult({ appState: 'Permitted' })} />);

    const badge = screen.getByText('Granted').closest('span');
    expect(badge).not.toBeNull();
    expect(within(badge!).getByTestId('status-icon')).toBeInTheDocument();
  });

  it('renders received and decided dates when present', () => {
    render(
      <SearchResultCard
        result={aSearchResult({ startDate: '2026-01-15', decidedDate: '2026-03-01' })}
      />,
    );

    expect(screen.getByText(/received/i)).toHaveTextContent('Received 15 Jan 2026');
    expect(screen.getByText(/decided/i)).toHaveTextContent('Decided 1 Mar 2026');
  });

  it('omits date rows when both dates are null', () => {
    render(<SearchResultCard result={anotherSearchResult({ startDate: null, decidedDate: null })} />);

    expect(screen.queryByText(/received/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/decided/i)).not.toBeInTheDocument();
  });

  it('renders a link to the public share page in a new tab', () => {
    render(
      <SearchResultCard
        result={aSearchResult({ authoritySlug: 'cambridge', reference: '22/1234/FUL' })}
      />,
    );

    const link = screen.getByRole('link', { name: /view share page/i });
    expect(link).toHaveAttribute('href', 'https://share.towncrierapp.uk/a/cambridge/22/1234/FUL');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('copies the exact share URL to the clipboard when the copy button is clicked', async () => {
    const user = setupUserWithSpyClipboard();
    render(
      <SearchResultCard
        result={aSearchResult({ authoritySlug: 'adur', reference: '9/P/2026/0044/HH' })}
      />,
    );

    await user.click(screen.getByRole('button', { name: /copy link/i }));

    await waitFor(() => {
      expect(clipboard.writeTextCalls).toEqual([
        'https://share.towncrierapp.uk/a/adur/9/P/2026/0044/HH',
      ]);
    });
  });

  it('shows confirmation feedback after a successful copy', async () => {
    const user = setupUserWithSpyClipboard();
    render(<SearchResultCard result={aSearchResult()} />);

    const button = screen.getByRole('button', { name: /copy link/i });
    await user.click(button);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /copied/i })).toBeInTheDocument();
    });
    const status = screen.getByText(/share link copied/i);
    expect(status).toHaveAttribute('aria-live', 'polite');
  });

  it('shows an error state instead of crashing when the clipboard write fails', async () => {
    clipboard.writeTextError = new Error('denied');
    const user = setupUserWithSpyClipboard();
    render(<SearchResultCard result={aSearchResult()} />);

    await user.click(screen.getByRole('button', { name: /copy link/i }));

    await waitFor(() => {
      expect(screen.getByText(/couldn.?t copy/i)).toBeInTheDocument();
    });
  });
});
