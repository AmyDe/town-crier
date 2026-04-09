import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { describe, it, expect } from 'vitest';
import { ConnectedLegalPage } from '../ConnectedLegalPage';

describe('ConnectedLegalPage', () => {
  it('renders without crashing', () => {
    render(
      <MemoryRouter initialEntries={['/legal/privacy']}>
        <Routes>
          <Route path="/legal/:type" element={<ConnectedLegalPage />} />
        </Routes>
      </MemoryRouter>,
    );

    // It should show loading state initially since the real API won't respond in test
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });
});
