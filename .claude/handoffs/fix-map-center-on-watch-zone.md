# Handoff: Center Map on User's Watch Zone

## Problem

The Map tab centers on a hardcoded coordinate (NW5 1SU, Camden) regardless of where the user's actual watch zones are. Applications load correctly (appType/appState fix merged in PR #221), but pins are invisible because they're outside the visible area — e.g., Cornwall pins are 250 miles away, Lewisham pins are 10km south.

The hardcoded watch zone is in `TownCrierApp.swift:72-78`:
```swift
let mapVM = appCoordinator.makeMapViewModel(
    watchZone: try! WatchZone(
        postcode: try! Postcode("NW5 1SU"),
        centre: try! Coordinate(latitude: 51.5550, longitude: -0.1450),
        radiusMetres: 2000
    )
)
```

## What Needs to Happen

Replace the hardcoded watch zone with the user's actual watch zone fetched from the API. The infrastructure already exists:

### API Endpoint
`GET /v1/me/watch-zones` → returns `ListWatchZonesResult`:
```json
{
  "zones": [
    {
      "id": "guid",
      "name": "TR18 4QG",
      "latitude": 50.12,
      "longitude": -5.53,
      "radiusMetres": 2000,
      "authorityId": 52
    }
  ]
}
```
- **API handler:** `api/src/town-crier.application/WatchZones/ListWatchZonesQueryHandler.cs`
- **Result type:** `api/src/town-crier.application/WatchZones/ListWatchZonesResult.cs` / `WatchZoneSummary.cs`

### iOS Repository (already implemented)
- **Repository:** `mobile/ios/packages/town-crier-data/Sources/Repositories/APIWatchZoneRepository.swift`
  - `loadAll()` → calls `GET /v1/me/watch-zones`, returns `[WatchZone]`
  - DTO: `WatchZoneSummaryDTO` (lines 71-93) — decodes and maps to domain `WatchZone`
- **Protocol:** `mobile/ios/packages/town-crier-domain/Sources/Protocols/WatchZoneRepository.swift`
  - `func loadAll() async throws -> [WatchZone]`

### iOS Domain Model
- `WatchZone` at `mobile/ios/packages/town-crier-domain/Sources/ValueObjects/WatchZone.swift`
  - Properties: `id`, `postcode`, `centre` (Coordinate), `radiusMetres`, `authorityId`
  - `centre` has `.latitude` and `.longitude`

## Approach Options

### Option A: Fetch watch zone at app init (simplest)
Fetch the user's first watch zone in `TownCrierApp.init()` before creating the MapViewModel. Problem: `loadAll()` is async but `init()` is sync. Would need to restructure initialization or use a placeholder and update later.

### Option B: Lazy map initialization (recommended)
Have MapViewModel fetch the watch zone itself before loading applications. The MapViewModel already has `centreLat`, `centreLon`, `radiusMetres` as stored properties — these would need to become `@Published` so the map camera can update when the real watch zone arrives.

Changes needed:
1. **MapViewModel** (`mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapViewModel.swift`):
   - Add a `WatchZoneRepository` dependency
   - In `loadApplications()`, fetch watch zones first, use the first zone's centre/radius
   - Make `centreLat`, `centreLon`, `radiusMetres` `@Published` so MapView reacts

2. **MapView** (`mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapView.swift`):
   - The `Map(initialPosition:)` only reads once. May need to switch to `@State var position` + `.onChange` to re-center when the watch zone arrives.

3. **AppCoordinator** (`mobile/ios/packages/town-crier-presentation/Sources/Coordinators/AppCoordinator.swift:55-67`):
   - `makeMapViewModel(watchZone:)` → could change to `makeMapViewModel()` if the VM fetches its own zone

4. **TownCrierApp** (`mobile/ios/town-crier-app/Sources/TownCrierApp.swift:72-78`):
   - Remove the hardcoded WatchZone, pass the WatchZoneRepository to the coordinator/VM instead

5. **Tests** (`mobile/ios/town-crier-tests/Sources/Features/MapViewModelTests.swift`):
   - Update to provide a spy WatchZoneRepository

### Option C: Fit all pins (alternative)
Instead of centering on a watch zone, compute a bounding box from all loaded application coordinates and set the map region to fit them all. Doesn't need the watch zone API at all. Downside: loses the watch zone radius circle.

## Composition Root Wiring

The `WatchZoneRepository` is NOT currently wired into `AppCoordinator`. It would need to be:
1. Created in `TownCrierApp.init()`: `let watchZoneRepository = APIWatchZoneRepository(apiClient: apiClient)`
2. Passed to `AppCoordinator` (add parameter) or directly to `MapViewModel`

## Key Files

| File | Role |
|------|------|
| `mobile/ios/town-crier-app/Sources/TownCrierApp.swift` | Composition root — hardcoded watch zone lives here (lines 72-78) |
| `mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapViewModel.swift` | VM — uses `centreLat`/`centreLon`/`radiusMetres` from WatchZone |
| `mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapView.swift` | View — `Map(initialPosition:)` uses VM's centre/radius |
| `mobile/ios/packages/town-crier-presentation/Sources/Coordinators/AppCoordinator.swift` | Factory — `makeMapViewModel(watchZone:)` at line 55 |
| `mobile/ios/packages/town-crier-data/Sources/Repositories/APIWatchZoneRepository.swift` | API client for watch zones — already implemented |
| `mobile/ios/packages/town-crier-domain/Sources/Protocols/WatchZoneRepository.swift` | Protocol — `loadAll()` |
| `mobile/ios/packages/town-crier-domain/Sources/ValueObjects/WatchZone.swift` | Domain model |
| `mobile/ios/town-crier-tests/Sources/Features/MapViewModelTests.swift` | Tests to update |
| `mobile/ios/town-crier-tests/Sources/Features/AppCoordinatorTests.swift` | Tests at line 125 |

## Multi-Zone Consideration

Users can have multiple watch zones. For the map, the simplest approach is to use the **first** zone. A more complete solution would compute a bounding region that encompasses all zones — but that's a separate feature.

## Simulator State

- Simulator: iPhone 17 Pro (iOS 26.4), device ID `5B769350-F902-4115-A9DA-CCFEEAAFF3D5` (Booted)
- App is installed and authenticated — just build and run to test
- `xcode-select` points to `/Applications/Xcode.app/Contents/Developer`
