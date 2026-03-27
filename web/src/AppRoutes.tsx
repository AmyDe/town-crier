import { Routes, Route } from 'react-router-dom';
import { LandingPage } from './features/LandingPage/LandingPage';
import { CallbackPage } from './auth/CallbackPage';
import { LegalPage } from './features/legal/LegalPage';
import { AuthGuard } from './auth/AuthGuard';
import { OnboardingGate } from './auth/OnboardingGate';
import { AppShell } from './components/AppShell/AppShell';
import { PlaceholderPage } from './features/placeholder/PlaceholderPage';
import { ConnectedApplicationsPage } from './features/Applications/ConnectedApplicationsPage';
import { ConnectedDashboardPage } from './features/Dashboard/ConnectedDashboardPage';
import { WiredNotificationsPage } from './features/Notifications/WiredNotificationsPage';
import { WiredSettingsPage } from './features/Settings/WiredSettingsPage';
import { ConnectedSearchPage } from './features/Search/ConnectedSearchPage';
import { ConnectedApplicationDetailPage } from './features/ApplicationDetail/ConnectedApplicationDetailPage';

export function AppRoutes() {
  return (
    <Routes>
      {/* Public routes */}
      <Route path="/" element={<LandingPage />} />
      <Route path="/callback" element={<CallbackPage />} />
      <Route path="/legal/:type" element={<LegalPage />} />

      {/* Authenticated routes */}
      <Route element={<AuthGuard />}>
        <Route path="/onboarding" element={<PlaceholderPage title="Onboarding" />} />
        <Route element={<OnboardingGate />}>
          <Route element={<AppShell />}>
            <Route path="/dashboard" element={<ConnectedDashboardPage />} />
            <Route path="/applications" element={<ConnectedApplicationsPage />} />
            <Route path="/applications/:uid" element={<ConnectedApplicationDetailPage />} />
            <Route path="/watch-zones" element={<PlaceholderPage title="Watch Zones" />} />
            <Route path="/map" element={<PlaceholderPage title="Map" />} />
            <Route path="/search" element={<ConnectedSearchPage />} />
            <Route path="/saved" element={<PlaceholderPage title="Saved" />} />
            <Route path="/groups" element={<PlaceholderPage title="Groups" />} />
            <Route path="/notifications" element={<WiredNotificationsPage />} />
            <Route path="/settings" element={<WiredSettingsPage />} />
          </Route>
        </Route>
      </Route>
    </Routes>
  );
}
