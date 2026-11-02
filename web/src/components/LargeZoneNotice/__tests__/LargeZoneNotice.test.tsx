import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { LargeZoneLoadingMessage } from '../LargeZoneNotice';

describe('LargeZoneLoadingMessage', () => {
  it('renders nothing when hidden', () => {
    const { container } = render(<LargeZoneLoadingMessage isVisible={false} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('renders the exact loading message as a status', () => {
    render(<LargeZoneLoadingMessage isVisible />);

    expect(screen.getByRole('status')).toHaveTextContent(
      'Loading a large zone. This can take a few seconds.',
    );
  });
});
