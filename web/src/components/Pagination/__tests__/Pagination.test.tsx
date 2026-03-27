import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { Pagination } from '../Pagination';

describe('Pagination', () => {
  it('renders current page and total pages', () => {
    render(
      <Pagination page={2} totalPages={5} onNext={() => {}} onPrevious={() => {}} />,
    );

    expect(screen.getByText('Page 2 of 5')).toBeInTheDocument();
  });

  it('calls onNext when Next button is clicked', async () => {
    const user = userEvent.setup();
    const onNext = vi.fn();

    render(
      <Pagination page={1} totalPages={5} onNext={onNext} onPrevious={() => {}} />,
    );

    await user.click(screen.getByRole('button', { name: /next/i }));
    expect(onNext).toHaveBeenCalledOnce();
  });

  it('calls onPrevious when Previous button is clicked', async () => {
    const user = userEvent.setup();
    const onPrevious = vi.fn();

    render(
      <Pagination page={3} totalPages={5} onNext={() => {}} onPrevious={onPrevious} />,
    );

    await user.click(screen.getByRole('button', { name: /previous/i }));
    expect(onPrevious).toHaveBeenCalledOnce();
  });

  it('disables Previous button on first page', () => {
    render(
      <Pagination page={1} totalPages={5} onNext={() => {}} onPrevious={() => {}} />,
    );

    expect(screen.getByRole('button', { name: /previous/i })).toBeDisabled();
  });

  it('disables Next button on last page', () => {
    render(
      <Pagination page={5} totalPages={5} onNext={() => {}} onPrevious={() => {}} />,
    );

    expect(screen.getByRole('button', { name: /next/i })).toBeDisabled();
  });

  it('is hidden when there is only a single page', () => {
    const { container } = render(
      <Pagination page={1} totalPages={1} onNext={() => {}} onPrevious={() => {}} />,
    );

    expect(container.firstChild).toBeNull();
  });
});
