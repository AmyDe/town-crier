import { useEffect } from 'react';
import { Navbar } from '../../components/Navbar/Navbar';
import { Hero } from '../../components/Hero/Hero';
import { StatsBar } from '../../components/StatsBar/StatsBar';
import { HowItWorks } from '../../components/HowItWorks/HowItWorks';
import { Pricing } from '../../components/Pricing/Pricing';
import { Faq } from '../../components/Faq/Faq';
import { Footer } from '../../components/Footer/Footer';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL as string || 'http://localhost:5000';

export function LandingPage() {
  // Wake the API container while the user reads the landing page.
  // Azure Container Apps can take ~12s to cold-start from zero replicas;
  // by the time the user clicks sign-in and completes Auth0, it's warm.
  useEffect(() => {
    fetch(`${API_BASE_URL}/health`).catch(() => {});
  }, []);

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
    </>
  );
}
