import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { AuthoritySelector } from '../AuthoritySelector';
import { SpyAuthoritySearchPort } from './spies/spy-authority-search-port';
import { twoAuthorityResults, cambridgeAuthority } from './fixtures/authority.fixtures';

describe('AuthoritySelector', () => {
  it('renders a search input', () => {
    const spy = new SpyAuthoritySearchPort();

    render(<AuthoritySelector searchPort={spy} onSelect={() => {}} />);

    expect(screen.getByRole('combobox')).toBeInTheDocument();
  });

  it('shows matching authorities after typing', async () => {
    const spy = new SpyAuthoritySearchPort();
    spy.searchResult = twoAuthorityResults();
    const user = userEvent.setup();

    render(<AuthoritySelector searchPort={spy} onSelect={() => {}} />);

    await user.type(screen.getByRole('combobox'), 'cam');

    await waitFor(() => {
      expect(screen.getByText('Cambridge City Council')).toBeInTheDocument();
      expect(screen.getByText('Oxford City Council')).toBeInTheDocument();
    });
  });

  it('calls onSelect when an authority is clicked', async () => {
    const spy = new SpyAuthoritySearchPort();
    spy.searchResult = twoAuthorityResults();
    const handleSelect = vi.fn();
    const user = userEvent.setup();

    render(<AuthoritySelector searchPort={spy} onSelect={handleSelect} />);

    await user.type(screen.getByRole('combobox'), 'cam');

    await waitFor(() => {
      expect(screen.getByText('Cambridge City Council')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cambridge City Council'));

    expect(handleSelect).toHaveBeenCalledWith(cambridgeAuthority());
  });

  it('clears results after selecting an authority', async () => {
    const spy = new SpyAuthoritySearchPort();
    spy.searchResult = twoAuthorityResults();
    const user = userEvent.setup();

    render(<AuthoritySelector searchPort={spy} onSelect={() => {}} />);

    await user.type(screen.getByRole('combobox'), 'cam');

    await waitFor(() => {
      expect(screen.getByText('Cambridge City Council')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cambridge City Council'));

    expect(screen.queryByText('Oxford City Council')).not.toBeInTheDocument();
  });

  it('does not reopen dropdown after selecting an authority', async () => {
    const spy = new SpyAuthoritySearchPort();
    spy.searchResult = twoAuthorityResults();
    const user = userEvent.setup();

    render(<AuthoritySelector searchPort={spy} onSelect={() => {}} />);

    await user.type(screen.getByRole('combobox'), 'cam');

    await waitFor(() => {
      expect(screen.getByText('Cambridge City Council')).toBeInTheDocument();
    });

    const searchCallsBefore = spy.searchCalls.length;
    await user.click(screen.getByText('Cambridge City Council'));

    // Wait long enough for the debounce to fire if it was going to
    await new Promise((r) => setTimeout(r, 350));

    expect(spy.searchCalls.length).toBe(searchCallsBefore);
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('sets the input value to the selected authority name', async () => {
    const spy = new SpyAuthoritySearchPort();
    spy.searchResult = twoAuthorityResults();
    const user = userEvent.setup();

    render(<AuthoritySelector searchPort={spy} onSelect={() => {}} />);

    await user.type(screen.getByRole('combobox'), 'cam');

    await waitFor(() => {
      expect(screen.getByText('Cambridge City Council')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Cambridge City Council'));

    expect(screen.getByRole('combobox')).toHaveValue('Cambridge City Council');
  });
});
