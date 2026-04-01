import { Routes, Route } from 'react-router-dom';
import { LandingPage } from './features/LandingPage/LandingPage';
import { CallbackPage } from './auth/CallbackPage';
import { LegalPage } from './features/legal/LegalPage';
import { AuthGuard } from './auth/AuthGuard';
import { OnboardingGate } from './auth/OnboardingGate';
import { AppShell } from './components/AppShell/AppShell';
import { ConnectedOnboardingPage } from './features/onboarding/ConnectedOnboardingPage';
import { ConnectedApplicationsPage } from './features/Applications/ConnectedApplicationsPage';
import { ConnectedDashboardPage } from './features/Dashboard/ConnectedDashboardPage';
import { ConnectedNotificationsPage } from './features/Notifications/ConnectedNotificationsPage';
import { ConnectedSettingsPage } from './features/Settings/ConnectedSettingsPage';
import { ConnectedSearchPage } from './features/Search/ConnectedSearchPage';
import { ConnectedApplicationDetailPage } from './features/ApplicationDetail/ConnectedApplicationDetailPage';
import { ConnectedWatchZoneListPage } from './features/WatchZones/ConnectedWatchZoneListPage';
import { ConnectedWatchZoneCreatePage } from './features/WatchZones/ConnectedWatchZoneCreatePage';
import { ConnectedWatchZoneEditPage } from './features/WatchZones/ConnectedWatchZoneEditPage';
import { ConnectedSavedApplicationsPage } from './features/SavedApplications/ConnectedSavedApplicationsPage';
import { ConnectedMapPage } from './features/Map/ConnectedMapPage';

export function AppRoutes() {
  return (
    <Routes>
      {/* Public routes */}
      <Route path="/" element={<LandingPage />} />
      <Route path="/callback" element={<CallbackPage />} />
      <Route path="/legal/:type" element={<LegalPage />} />

      {/* Authenticated routes */}
      <Route element={<AuthGuard />}>
        <Route path="/onboarding" element={<ConnectedOnboardingPage />} />
        <Route element={<OnboardingGate />}>
          <Route element={<AppShell />}>
            <Route path="/dashboard" element={<ConnectedDashboardPage />} />
            <Route path="/applications" element={<ConnectedApplicationsPage />} />
            <Route path="/applications/:uid" element={<ConnectedApplicationDetailPage />} />
            <Route path="/watch-zones" element={<ConnectedWatchZoneListPage />} />
            <Route path="/watch-zones/new" element={<ConnectedWatchZoneCreatePage />} />
            <Route path="/watch-zones/:zoneId" element={<ConnectedWatchZoneEditPage />} />
            <Route path="/map" element={<ConnectedMapPage />} />
            <Route path="/search" element={<ConnectedSearchPage />} />
            <Route path="/saved" element={<ConnectedSavedApplicationsPage />} />
            <Route path="/notifications" element={<ConnectedNotificationsPage />} />
            <Route path="/settings" element={<ConnectedSettingsPage />} />
          </Route>
        </Route>
      </Route>
    </Routes>
  );
}
