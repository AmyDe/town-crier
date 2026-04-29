# Applications / Saved Tab Realignment

Date: 2026-04-29

## Background

Two recent changes shipped on 2026-04-28 and 2026-04-29:
- **tc-wnap** scoped the iOS Saved filter to the active watch zone.
- **tc-nb5u** added a synthetic 'All' zone chip to the iOS Applications list as an escape hatch for orphan saves (saves outside any zone).
- **tc-kmcp** retired the web `/saved` page and folded a Saved filter chip into `/applications`.

In testing, the 'All' chip combined with status filter pills produces an empty list with no recovery path: 'All' was implemented as a Saved-only mode, but its label leads users to expect "all applications across my zones". This was filed as **tc-34cg**.

Rather than patch the dead-end, this spec realigns the model: separate Applications and Saved into their own surfaces on both iOS and web, so each surface has one job.

## Goals

- Eliminate the 'All' zone chip dead-end on iOS.
- Make iOS and web feature-parity on Applications and Saved.
- Open the status filter to all tiers (it is no longer paywalled).
- Re-introduce a dedicated Saved surface on web that was retired in tc-kmcp.

## Non-goals

- Saved-application status-change notifications. Parked as **tc-ah9c** (deferred); revisit when this work ships.
- Map-view changes to zone selection. Map stays per-zone (no 'All' option).
- Backend endpoint changes. Both required endpoints already exist.

## Structure

Two surfaces on each platform, each with one job:

| Surface | Purpose | Source | Filters |
|---|---|---|---|
| **Applications** | "What's happening in my watch zones" | `GET /me/watch-zones/{id}/applications` per zone | Zone selector (real zones only) → status pills |
| **Saved** | "What I'm personally tracking" | `GET /me/saved-applications` (cross-zone, includes orphans) | Status pills |

**Map** stays as-is — zone-scoped only, no change.

## Tier matrix

| Surface | Free | Personal | Pro |
|---|---|---|---|
| Applications: zone selector | ✓ | ✓ | ✓ |
| Applications: status filter | ✓ | ✓ | ✓ |
| Saved tab/page | ✓ | ✓ | ✓ |
| Saved: status filter | ✓ | ✓ | ✓ |
| Map: status filter | ✓ | ✓ | ✓ |

Confirmed paid-only (untouched by this work): more than 1 watch zone, push notifications, hourly cadence emails.

## iOS

### Tab bar

Five tabs in this order: **Applications · Saved · Map · Zones · Settings**.

Saved sits next to Applications (both list views — one is "the world", one is "yours"). Icon: `bookmark.fill`.

### New: SavedApplicationList feature

Path: `mobile/ios/packages/town-crier-presentation/Sources/Features/SavedApplicationList/`

`SavedApplicationListViewModel`:
- `applications: [PlanningApplication]` — sorted by `SavedApplication.savedAt` desc (most-recently-bookmarked first; personal-list semantics).
- `selectedStatusFilter: ApplicationStatus?` — nil means "All". Free for all tiers.
- `filteredApplications: [PlanningApplication]` — filters `applications` by `selectedStatusFilter`.
- `loadAll()` — calls `SavedApplicationRepository.loadAll()`. Reuses existing repository; the denormalised `SavedApplication.application` payload (from tc-a1x8 + tc-kmcp) is what enables cross-zone listing without N+1 fetches.
- Tap-through: `onApplicationSelected: ((PlanningApplicationId) -> Void)?` → existing `ApplicationDetailView`.

`SavedApplicationListView`:
- Status filter pill row (always visible).
- List of `ApplicationListRow` rows (reuse the existing component, no new row type needed).
- Empty state when `applications.isEmpty`: "Bookmark applications you want to track. Tap the bookmark icon on any application detail."
- Empty state when filter has no matches: "No saved applications match this filter."
- Standard error/loading patterns matching `ApplicationListView`.

### ApplicationListViewModel cleanup

In `mobile/ios/packages/town-crier-presentation/Sources/Features/ApplicationList/ApplicationListViewModel.swift`:

Delete:
- `selectAllZones()`, `isAllZonesSelected`, `Self.allZonesSentinel`
- `EmptyStateKind.allZonesNoSavedFilter` (and the enum collapses to a single case — consider removing the enum entirely)
- `activateSavedFilter()`, `deactivateSavedFilter()`
- `isSavedFilterActive`, `isLoadingSaved`, `savedApplicationUids`
- The `didSet` mutual-exclusion logic on `selectedStatusFilter`
- `canFilter` (status filter is now free for all tiers)

Update:
- `filteredApplications`: `guard let filter = selectedStatusFilter else { return applications }` (drops `canFilter` check).
- `loadApplications()`: remove the `isAllZonesSelected` early-return branch.
- `selectZone()`: remove `isSavedFilterActive = false`.

`ApplicationListView`:
- Remove the prepended 'All' chip from the zone scroller.
- Remove the Saved filter pill.
- Remove the `if viewModel.canFilter` wrapping the status pill row (always render when zone has applications).

### MapViewModel cleanup

In `mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapViewModel.swift`:
- Delete `canFilter`.
- `filteredAnnotations`: drop the `canFilter` check.

`MapView`:
- Remove the `if viewModel.canFilter` wrapping (always render status pills when there are annotations).

### iOS tests

New file: `mobile/ios/town-crier-tests/Sources/Features/SavedApplicationListViewModelTests.swift`
- Empty state when no saves.
- Loads saves via repository, sorts by saved-at desc.
- Status filter passthrough (each `ApplicationStatus` case + nil/All).
- Empty state for "no matches under this filter".
- Repository error handling.

Delete: `ApplicationListAllZonesTests.swift` (~15 tests).

Update: `ApplicationListViewModelTests.swift`
- Drop tests covering `canFilter == false` paywall behaviour.
- Drop Saved-filter-on-Applications tests.
- Restore `showZonePicker` to `zones.count > 1` (was relaxed to `>= 1` for the 'All' chip).

Update: `MapViewModelTests` — drop `canFilter == false` paywall tests.

Update or delete: `ApplicationListViewModelStaleZoneTests.swift` — keep zone-staleness tests, drop any Saved-cross-zone coverage.

### iOS wiring

`TownCrierApp.swift` — insert the new tab between Applications and Map:

```swift
NavigationStack {
  SavedApplicationListView(viewModel: coordinator.makeSavedApplicationListViewModel())
}
.sheet(item: $coordinator.detailApplication) { ... }   // shared sheet wiring
.tabItem {
  Label("Saved", systemImage: "bookmark.fill")
}
```

`AppCoordinator` — add `makeSavedApplicationListViewModel()` factory using the existing `SavedApplicationRepository` injection.

## Web

### Sidebar nav

Insert "Saved" between Applications and Watch Zones:

`Dashboard · Applications · Saved · Watch Zones · Map · Search · Notifications · Settings`

### New: SavedApplications feature

Path: `web/src/features/SavedApplications/`

Files:
- `useSavedApplications.ts` — hook-as-ViewModel.
- `SavedApplicationsPage.tsx` — view layer.
- `ConnectedSavedApplicationsPage.tsx` — DI wiring.
- `__tests__/useSavedApplications.test.ts`
- `__tests__/SavedApplicationsPage.test.tsx`

`useSavedApplications`:
- State: `{ applications, isLoading, error, selectedStatusFilter }`.
- Loads via injected `SavedApplicationRepository.listSaved()`.
- Sort: `savedAt` desc (the existing `SavedApplication.savedAt` field).
- `setStatusFilter(status: ApplicationStatus | null)` — nil = All.
- Returns derived `filteredApplications` via `useMemo`.

`SavedApplicationsPage`:
- Status pill row (always visible).
- List of saved applications with click-through to `/applications/:uid`.
- Empty state copy mirrors iOS.

### Route registration

In `web/src/AppRoutes.tsx`:

```tsx
<Route path="/saved" element={<ConnectedSavedApplicationsPage />} />
```

Lazy-loaded in line with the rest of the routes.

### useApplications cleanup

In `web/src/features/Applications/useApplications.ts`:

Delete:
- `selectAllZones`, `isAllZonesSelected`
- `activateSavedFilter`, `deactivateSavedFilter`
- `isSavedFilterActive`, `savedUids`
- The cross-zone payload-from-saved branch in `activateSavedFilter`

Update:
- `setStatusFilter`: drop the saved-mutual-exclusion branch.
- `selectZone`: drop the `isSavedFilterActive = false` reset.
- `filteredApplications`: collapse to status-filter-only.

`ApplicationsPage.tsx`:
- Remove the Saved chip and any 'All' zone UI.

### Web tests

New: `useSavedApplications.test.ts`, `SavedApplicationsPage.test.tsx`. Same coverage shape as the iOS ViewModel tests.

Update: `useApplications.test.ts` to drop saved-related cases.
Update: `ApplicationsPage.test.tsx` to drop saved-chip cases.
Update (if it exists): `Sidebar.test` to assert the new nav item.

### Side-quest fix (free)

`DashboardPage.tsx` line 38 has a `<Link to="/saved">` that currently 404s — tc-kmcp retired the route but left the link. Re-introducing `/saved` un-breaks it automatically. No additional code change required.

## Acceptance

iOS:
- Five tabs visible. Tapping Saved shows saved applications, sorted by saved-at desc.
- Status filter pills work on Saved (all tiers, including free).
- Applications tab no longer has 'All' chip or Saved filter pill.
- Free-tier user sees status filter pills on Applications and Map.
- Tap-through from Saved row → `ApplicationDetailView`.
- `swift test` and `swiftlint --strict` clean.

Web:
- `/saved` route renders the Saved page; sidebar entry navigates to it.
- Status filter pills work on Saved.
- `/applications` no longer has Saved chip or 'All' zone UI.
- Dashboard's "Saved" link no longer 404s.
- `npx tsc --noEmit` and `npx vitest run` green.

Both:
- Orphan saves (saves outside any current zone) appear in Saved.
- Existing zone notifications continue to fire as today (unchanged).

## Bead breakdown

| # | Bead | Scope | Depends on |
|---|---|---|---|
| 1 | iOS realignment | Add Saved tab + revert 'All' chip + revert Saved filter on Applications + remove `canFilter` (Application + Map) + tests. Single PR. | — |
| 2 | Web realignment | Add `/saved` page + revert Saved filter on `/applications` + tests. Single PR. | — |
| 3 | Bookkeeping | Close `tc-34cg` as superseded once #1 lands. | #1 |

iOS and web are independent and can ship in parallel via autopilot. Each platform's add+remove ships together to avoid a transient duplicate state in production.

## Out of scope

- **tc-ah9c** (Saved-app status-change notifications) — deferred. Trigger granularity (any transition vs decision-only vs per-save) and tier gating left open.
- Backend endpoints — none changed.
- Map view zone selection — unchanged.
- Search and Dashboard surfaces — unchanged.
