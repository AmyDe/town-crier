import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Navbar } from '../../components/Navbar/Navbar';
import { Hero } from '../../components/Hero/Hero';
import { StatsBar } from '../../components/StatsBar/StatsBar';
import { HowItWorks } from '../../components/HowItWorks/HowItWorks';
import { Pricing } from '../../components/Pricing/Pricing';
import { Faq } from '../../components/Faq/Faq';
import { Footer } from '../../components/Footer/Footer';
import { Toast } from '../../components/Toast/Toast';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL as string || 'http://localhost:5000';

export function LandingPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [showSignedOut, setShowSignedOut] = useState(
    () => searchParams.get('signed_out') === 'true',
  );

  // Wake the API container while the user reads the landing page.
  // Azure Container Apps can take ~12s to cold-start from zero replicas;
  // by the time the user clicks sign-in and completes Auth0, it's warm.
  useEffect(() => {
    fetch(`${API_BASE_URL}/health`).catch(() => {});
  }, []);

  // Remove the query param so the toast doesn't reappear on refresh
  useEffect(() => {
    if (searchParams.has('signed_out')) {
      searchParams.delete('signed_out');
      setSearchParams(searchParams, { replace: true });
    }
  }, [searchParams, setSearchParams]);

  return (
    <>
      <Navbar />
      <Hero />
      <main>
        <StatsBar />
        <HowItWorks />
        <Pricing />
        <Faq />
      </main>
      <Footer />
      {showSignedOut && (
        <Toast
          message="You've been signed out"
          onDismiss={() => setShowSignedOut(false)}
        />
      )}
    </>
  );
}
