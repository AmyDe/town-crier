# iOS Zone Filtering

Date: 2026-04-13

## Overview

Add a horizontal scrollable zone pill bar to both the **Applications** and **Map** tabs on iOS, letting users switch which watch zone's data they're viewing. When only one zone exists, the picker is hidden entirely. This brings feature parity with the web app's zone selection, adapted to native iOS patterns.

## Design Decisions

- **Pill bar pattern** (not two-stage flow or nav bar dropdown) — always visible, instantly switchable, familiar iOS pattern that works well with 2-5 zones
- **Stacked layout** — zone pills sit in their own row above the existing status filter chips, creating a clear filter hierarchy (zone → status)
- **Independent pickers per tab** — Applications and Map tabs each manage their own selected zone independently, no shared state via coordinator
- **Remember last selection** — each tab persists its last-selected zone ID to `UserDefaults`, so users return to where they left off
- **Hidden for single zone** — when `zones.count <= 1`, the pill bar is not rendered. Free tier users (max 1 zone) never see it.

## Zone Picker Component

A reusable `ZonePickerView` in the presentation package.

### Interface

```swift
struct ZonePickerView: View {
    let zones: [WatchZoneSummary]
    let selectedZoneId: WatchZoneId?
    let onSelect: (WatchZoneSummary) -> Void
}
```

### Behavior

- Horizontal `ScrollView` of capsule-shaped buttons showing zone names
- Selected zone uses `tcAmber` background with `tcTextOnAccent` foreground (matching existing status filter chip styling)
- Unselected zones use `tcSurface` background with `tcBorder` stroke
- Uses `TCTypography.captionEmphasis` font and `TCSpacing` values for consistency
- Component is not rendered when `zones.count <= 1`

## Applications Tab

### Layout

```
┌─────────────────────────────┐
│ Applications        (title) │
├─────────────────────────────┤
│ [Cambridge] [Oxford]        │  ← zone pills (hidden if 1 zone)
├─────────────────────────────┤
│ [All] [Pending] [Approved]  │  ← status filters (paid tiers only)
├─────────────────────────────┤
│ Application rows...         │
└─────────────────────────────┘
```

### View Changes (`ApplicationListView.swift`)

- Add a `zonePickerSection` above the existing `filterSection` in the `List`
- Zone picker section uses same `listRowInsets(EdgeInsets())` and `listRowBackground(Color.tcBackground)` pattern as filter section
- Only rendered when ViewModel exposes `zones.count > 1`

### ViewModel Changes (`ApplicationListViewModel.swift`)

New published properties:
- `zones: [WatchZoneSummary]` — loaded on init via `WatchZoneRepository.loadAll()`
- `selectedZone: WatchZoneSummary?` — currently selected zone

New behavior:
- `loadApplications()` waits for zone resolution before fetching applications
- Zone resolution order: check `UserDefaults` for `lastSelectedZone.applications` → if found and present in zones array, use it → otherwise default to first zone
- `selectZone(_ zone: WatchZoneSummary)` sets `selectedZone`, persists zone ID to `UserDefaults`, and triggers `loadApplications()` for the new zone
- Existing `loadApplications()` changes: instead of always loading first zone, uses `selectedZone`

Dependencies added:
- `WatchZoneRepository` (already exists as protocol in domain package)
- `UserDefaults` access for persistence (inject key prefix for testability)

## Map Tab

### Layout

Zone pill bar sits above the map view. Switching zone recenters the map on that zone's centre coordinates and radius, and reloads application pins.

### ViewModel Changes (`MapViewModel.swift`)

Same pattern as ApplicationListViewModel:
- `zones: [WatchZoneSummary]` and `selectedZone: WatchZoneSummary?` published properties
- Zone resolution from `UserDefaults` key `lastSelectedZone.map`
- `selectZone(_ zone:)` persists selection, reloads map data for new zone
- Map region updates to center on `selectedZone.centre` with appropriate span for `selectedZone.radiusMetres`

## State Persistence

Two `UserDefaults` keys:
- `lastSelectedZone.applications` — stores `WatchZoneId` (String)
- `lastSelectedZone.map` — stores `WatchZoneId` (String)

On load, if the persisted zone ID is not found in the current zones array (zone was deleted), fall back to the first zone.

## Loading States

- **Initial load**: existing `ListSkeletonView` while zones + applications load together
- **Zone switch**: show skeleton/loading state while new zone's applications load
- **Pull-to-refresh**: reloads applications for the currently selected zone
- **Error state**: existing `ErrorStateView` with retry, scoped to the selected zone

## Edge Cases

- **User deletes currently selected zone** (via Zones tab): persisted zone ID becomes invalid → next load falls back to first zone
- **User adds a new zone** (first time having multiple): zone picker appears on next load of Applications/Map tab
- **User downgrades to free tier**: can only have 1 zone → picker hidden automatically
- **Empty zones**: if user has no zones at all, existing empty state view handles this

## Files Affected

| Package | File | Change |
|---------|------|--------|
| presentation | New: `ZonePickerView.swift` | Reusable zone pill bar component |
| presentation | `ApplicationListView.swift` | Add zone picker section above status filters |
| presentation | `ApplicationListViewModel.swift` | Add zone loading, selection, UserDefaults persistence |
| presentation | `MapView.swift` | Add zone picker above map |
| presentation | `MapViewModel.swift` | Add zone loading, selection, UserDefaults persistence |
| presentation | `AppCoordinator.swift` | Pass `WatchZoneRepository` to ViewModel factory methods |
| app | `TownCrierApp.swift` | Wire `WatchZoneRepository` into coordinator |

## What Doesn't Change

- Status filter chips — layout and tier gating untouched
- Zone CRUD — the Zones tab remains the place to add/edit/delete zones
- API endpoints — `GET /v1/me/watch-zones` and `GET /v1/me/watch-zones/{id}/applications` already support everything
- Notification handling and deep linking — unaffected
- Offline caching decorator — continues to work as-is since it wraps the same repository calls
