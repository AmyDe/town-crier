import { Navbar } from '../../components/Navbar/Navbar';
import { Hero } from '../../components/Hero/Hero';
import { StatsBar } from '../../components/StatsBar/StatsBar';
import { HowItWorks } from '../../components/HowItWorks/HowItWorks';
import { Pricing } from '../../components/Pricing/Pricing';
import { Faq } from '../../components/Faq/Faq';
import { Footer } from '../../components/Footer/Footer';

export function LandingPage() {
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
