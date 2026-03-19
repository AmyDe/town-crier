import { Navbar } from './components/Navbar/Navbar';
import { Hero } from './components/Hero/Hero';
import { StatsBar } from './components/StatsBar/StatsBar';
import { HowItWorks } from './components/HowItWorks/HowItWorks';
import { CommunityGroups } from './components/CommunityGroups/CommunityGroups';
import { Pricing } from './components/Pricing/Pricing';
import { Faq } from './components/Faq/Faq';
import { Footer } from './components/Footer/Footer';

export function App() {
  return (
    <>
      <Navbar />
      <Hero />
      <main>
        <StatsBar />
        <HowItWorks />
        <CommunityGroups />
        <Pricing />
        <Faq />
      </main>
      <Footer />
    </>
  );
}
