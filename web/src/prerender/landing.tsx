import { renderToString } from 'react-dom/server';
import { MemoryRouter } from 'react-router';
import { AuthProvider } from '../auth/auth-context';
import { LandingPage } from '../features/LandingPage/LandingPage';
import { buildLandingJsonLd } from '../features/LandingPage/structuredData';
import { signedOutAuthPort } from './signedOutAuthPort';

/** The landing page markup and its JSON-LD, as a signed-out visitor sees it. */
export function renderLanding(): { html: string; jsonLd: string } {
  const html = renderToString(
    <MemoryRouter initialEntries={['/']}>
      <AuthProvider value={signedOutAuthPort}>
        <LandingPage />
      </AuthProvider>
    </MemoryRouter>,
  );

  return { html, jsonLd: JSON.stringify(buildLandingJsonLd()) };
}
