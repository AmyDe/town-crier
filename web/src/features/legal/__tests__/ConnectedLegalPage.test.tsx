import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
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

    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });
});
