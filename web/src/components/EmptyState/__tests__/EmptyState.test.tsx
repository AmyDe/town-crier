import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { EmptyState } from '../EmptyState';

describe('EmptyState', () => {
  it('renders the message', () => {
    render(<EmptyState message="No applications found" />);

    expect(screen.getByText('No applications found')).toBeInTheDocument();
  });

  it('renders an optional title', () => {
    render(
      <EmptyState title="Nothing here" message="No applications found" />,
    );

    expect(screen.getByText('Nothing here')).toBeInTheDocument();
  });

  it('renders an optional icon', () => {
    render(
      <EmptyState message="No applications found" icon="📭" />,
    );

    expect(screen.getByText('📭')).toBeInTheDocument();
  });

  it('renders an optional action button', async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();

    render(
      <EmptyState
        message="No applications found"
        actionLabel="Create a watch zone"
        onAction={onAction}
      />,
    );

    const button = screen.getByRole('button', { name: 'Create a watch zone' });
    expect(button).toBeInTheDocument();

    await user.click(button);
    expect(onAction).toHaveBeenCalledOnce();
  });

  it('does not render an action button when no actionLabel is provided', () => {
    render(<EmptyState message="No applications found" />);

    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});
