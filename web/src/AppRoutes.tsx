import { lazy, Suspense } from 'react';
import { Routes, Route } from 'react-router-dom';
import { LandingPage } from './features/LandingPage/LandingPage';
import { CallbackPage } from './auth/CallbackPage';
import { ConnectedLegalPage } from './features/legal/ConnectedLegalPage';
import { AuthGuard } from './auth/AuthGuard';
import { OnboardingGate } from './auth/OnboardingGate';
import { AppShell } from './components/AppShell/AppShell';

const ConnectedOnboardingPage = lazy(() =>
  import('./features/onboarding/ConnectedOnboardingPage').then((m) => ({ default: m.ConnectedOnboardingPage })),
);
const ConnectedDashboardPage = lazy(() =>
  import('./features/Dashboard/ConnectedDashboardPage').then((m) => ({ default: m.ConnectedDashboardPage })),
);
const ConnectedApplicationsPage = lazy(() =>
  import('./features/Applications/ConnectedApplicationsPage').then((m) => ({ default: m.ConnectedApplicationsPage })),
);
const ConnectedApplicationDetailPage = lazy(() =>
  import('./features/ApplicationDetail/ConnectedApplicationDetailPage').then((m) => ({
    default: m.ConnectedApplicationDetailPage,
  })),
);
const ConnectedSavedApplicationsPage = lazy(() =>
  import('./features/SavedApplications/ConnectedSavedApplicationsPage').then((m) => ({
    default: m.ConnectedSavedApplicationsPage,
  })),
);
const ConnectedWatchZoneListPage = lazy(() =>
  import('./features/WatchZones/ConnectedWatchZoneListPage').then((m) => ({ default: m.ConnectedWatchZoneListPage })),
);
const ConnectedWatchZoneCreatePage = lazy(() =>
  import('./features/WatchZones/ConnectedWatchZoneCreatePage').then((m) => ({
    default: m.ConnectedWatchZoneCreatePage,
  })),
);
const ConnectedWatchZoneEditPage = lazy(() =>
  import('./features/WatchZones/ConnectedWatchZoneEditPage').then((m) => ({ default: m.ConnectedWatchZoneEditPage })),
);
const ConnectedMapPage = lazy(() =>
  import('./features/Map/ConnectedMapPage').then((m) => ({ default: m.ConnectedMapPage })),
);
const ConnectedNotificationsPage = lazy(() =>
  import('./features/Notifications/ConnectedNotificationsPage').then((m) => ({
    default: m.ConnectedNotificationsPage,
  })),
);
const ConnectedSettingsPage = lazy(() =>
  import('./features/Settings/ConnectedSettingsPage').then((m) => ({ default: m.ConnectedSettingsPage })),
);

export function AppRoutes() {
  return (
    <Routes>
      {/* Public routes */}
      <Route path="/" element={<LandingPage />} />
      <Route path="/callback" element={<CallbackPage />} />
      <Route path="/legal/:type" element={<ConnectedLegalPage />} />

      {/* Authenticated routes */}
      <Route element={<AuthGuard />}>
        <Route path="/onboarding" element={<Suspense fallback={null}><ConnectedOnboardingPage /></Suspense>} />
        <Route element={<OnboardingGate />}>
          <Route element={<AppShell />}>
            <Route path="/dashboard" element={<ConnectedDashboardPage />} />
            <Route path="/applications" element={<ConnectedApplicationsPage />} />
            <Route path="/applications/*" element={<ConnectedApplicationDetailPage />} />
            <Route path="/saved" element={<Suspense fallback={null}><ConnectedSavedApplicationsPage /></Suspense>} />
            <Route path="/watch-zones" element={<ConnectedWatchZoneListPage />} />
            <Route path="/watch-zones/new" element={<ConnectedWatchZoneCreatePage />} />
            <Route path="/watch-zones/:zoneId" element={<ConnectedWatchZoneEditPage />} />
            <Route path="/map" element={<ConnectedMapPage />} />
            <Route path="/notifications" element={<ConnectedNotificationsPage />} />
            <Route path="/settings" element={<ConnectedSettingsPage />} />
          </Route>
        </Route>
      </Route>
    </Routes>
  );
}
