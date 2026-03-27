import { Routes, Route } from 'react-router-dom';
import { LandingPage } from './features/LandingPage/LandingPage';
import { CallbackPage } from './auth/CallbackPage';
import { LegalPage } from './features/legal/LegalPage';
import { AuthGuard } from './auth/AuthGuard';
import { OnboardingGate } from './auth/OnboardingGate';
import { AppShell } from './components/AppShell/AppShell';
import { ConnectedMapPage } from './features/Map/ConnectedMapPage';
import { ConnectedOnboardingPage } from './features/onboarding/ConnectedOnboardingPage';
import { ConnectedApplicationsPage } from './features/Applications/ConnectedApplicationsPage';
import { ConnectedDashboardPage } from './features/Dashboard/ConnectedDashboardPage';
import { WiredNotificationsPage } from './features/Notifications/WiredNotificationsPage';
import { WiredSettingsPage } from './features/Settings/WiredSettingsPage';
import { ConnectedSearchPage } from './features/Search/ConnectedSearchPage';
import { ConnectedApplicationDetailPage } from './features/ApplicationDetail/ConnectedApplicationDetailPage';
import { WiredWatchZoneListPage } from './features/WatchZones/WiredWatchZoneListPage';
import { WiredWatchZoneCreatePage } from './features/WatchZones/WiredWatchZoneCreatePage';
import { WiredWatchZoneEditPage } from './features/WatchZones/WiredWatchZoneEditPage';
import { WiredGroupsListPage } from './features/Groups/WiredGroupsListPage';
import { WiredGroupCreatePage } from './features/Groups/WiredGroupCreatePage';
import { WiredGroupDetailPage } from './features/Groups/WiredGroupDetailPage';
import { WiredAcceptInvitationPage } from './features/Groups/WiredAcceptInvitationPage';
import { WiredSavedApplicationsPage } from './features/SavedApplications/WiredSavedApplicationsPage';

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
            <Route path="/watch-zones" element={<WiredWatchZoneListPage />} />
            <Route path="/watch-zones/new" element={<WiredWatchZoneCreatePage />} />
            <Route path="/watch-zones/:zoneId" element={<WiredWatchZoneEditPage />} />
            <Route path="/map" element={<ConnectedMapPage />} />
            <Route path="/search" element={<ConnectedSearchPage />} />
            <Route path="/saved" element={<WiredSavedApplicationsPage />} />
            <Route path="/groups" element={<WiredGroupsListPage />} />
            <Route path="/groups/new" element={<WiredGroupCreatePage />} />
            <Route path="/groups/:groupId" element={<WiredGroupDetailPage />} />
            <Route path="/invitations/:invitationId/accept" element={<WiredAcceptInvitationPage />} />
            <Route path="/notifications" element={<WiredNotificationsPage />} />
            <Route path="/settings" element={<WiredSettingsPage />} />
          </Route>
        </Route>
      </Route>
    </Routes>
  );
}
