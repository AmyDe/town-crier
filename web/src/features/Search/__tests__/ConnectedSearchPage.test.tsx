import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { ConnectedSearchPage } from '../ConnectedSearchPage';

describe('ConnectedSearchPage', () => {
  it('renders the public search page without any authentication context', () => {
    // Deliberately rendered with NO Auth0Provider/AuthGuard/router context beyond
    // what the page itself needs — proves the anonymous /search route never
    // depends on being signed in (#821 Phase 4 acceptance criterion).
    render(<ConnectedSearchPage />);

    expect(
      screen.getByRole('heading', { name: /search planning applications/i }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/search/i)).toBeInTheDocument();
  });
});
