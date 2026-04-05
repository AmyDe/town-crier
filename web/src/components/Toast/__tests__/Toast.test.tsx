import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { Toast } from '../Toast';

describe('Toast', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders the message', () => {
    render(<Toast message="You've been signed out" onDismiss={() => {}} />);

    expect(screen.getByText("You've been signed out")).toBeInTheDocument();
  });

  it('calls onDismiss after duration', () => {
    const onDismiss = vi.fn();
    render(<Toast message="Gone" onDismiss={onDismiss} duration={4000} />);

    expect(onDismiss).not.toHaveBeenCalled();

    vi.advanceTimersByTime(4000);

    expect(onDismiss).toHaveBeenCalledTimes(1);
  });
});
