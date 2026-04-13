# Handoff: Fix Xcode Build Errors After Map Watch Zone Change

## Manual Cleanup First

The previous session's worktree was cleaned up mid-session, breaking the shell. Run these commands before anything else:

```bash
cd ~/Dev/town-crier
git worktree prune
git pull --rebase origin main
git branch -D worktree-fix-debug-real-api 2>/dev/null
git remote prune origin
bd close tc-2eb
bd dolt push
```

Verify you're on the squash-merged commit from PR #222:
```bash
git log --oneline -3
# Should show: fix(ios): center map on user's actual watch zone (#222)
```

## What Changed (PR #222)

PR #222 replaced a hardcoded watch zone (NW5 1SU, Camden) with a real API fetch. The map now centers on the user's actual watch zone from `GET /v1/me/watch-zones`.

### Files changed:

| File | Change |
|------|--------|
| `mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapViewModel.swift` | Replaced `WatchZone` init param with `WatchZoneRepository`. Made `centreLat`/`centreLon`/`radiusMetres` `@Published` (were `let`). Added watch zone fetch in `loadApplications()`. |
| `mobile/ios/packages/town-crier-presentation/Sources/Coordinators/AppCoordinator.swift` | Added `watchZoneRepository: WatchZoneRepository` stored property and init param. Changed `makeMapViewModel(watchZone:)` to `makeMapViewModel()` (no params). |
| `mobile/ios/town-crier-app/Sources/TownCrierApp.swift` | Created `APIWatchZoneRepository(apiClient:)`. Passed it to `AppCoordinator` init. Changed `makeMapViewModel(watchZone: try! WatchZone(...))` to `makeMapViewModel()`. Removed `force_try` and swiftlint disable comments. |
| `mobile/ios/town-crier-tests/Sources/Features/MapViewModelTests.swift` | Updated `makeSUT` to accept `watchZones: [WatchZone]` and return 3-element tuple with `SpyWatchZoneRepository`. Added 3 new tests for watch zone fetch behavior. Updated all destructuring from 2-tuple to 3-tuple. |
| `mobile/ios/town-crier-tests/Sources/Features/AppCoordinatorTests.swift` | Added `watchZoneRepository: SpyWatchZoneRepository()` to all `AppCoordinator(...)` calls. Changed `makeMapViewModel(watchZone: .cambridge)` to `makeMapViewModel()`. |
| `mobile/ios/town-crier-tests/Sources/Features/CompositionRootTests.swift` | Added `watchZoneRepository:` to all `AppCoordinator(...)` calls (4 locations). Changed `makeMapViewModel(watchZone: .cambridge)` to `makeMapViewModel()`. |
| `mobile/ios/town-crier-tests/Sources/Features/DeepLinkTests.swift` | Added `watchZoneRepository: SpyWatchZoneRepository()` to `AppCoordinator(...)` call. |

### API changes (breaking):

1. **`MapViewModel` init** — All 3 initializers now take `watchZoneRepository: WatchZoneRepository` instead of `watchZone: WatchZone`
2. **`AppCoordinator` init** — New required param `watchZoneRepository: WatchZoneRepository` (inserted between `authorityRepository:` and `onboardingRepository:`)
3. **`AppCoordinator.makeMapViewModel()`** — No longer takes a `watchZone:` parameter

## Xcode Build Errors — Likely Causes

The `swift build` and `swift test` (765 tests) passed in the CI and locally via command line. If Xcode is showing build errors, likely causes:

### 1. Stale Xcode build cache
The Xcode project may have cached the old API signatures. Try:
```
Product > Clean Build Folder (Cmd+Shift+K)
```
Then rebuild.

### 2. Xcode project file not updated
If there's a `.xcodeproj` or `.xcworkspace` that references files explicitly (rather than SPM auto-discovery), it may need updating. Check:
```bash
find mobile/ios -name "*.xcodeproj" -o -name "*.xcworkspace" | head -10
ls mobile/ios/*.xcodeproj/project.pbxproj 2>/dev/null
```

### 3. Package resolution stale
SPM packages may need re-resolving:
```
File > Packages > Reset Package Caches
File > Packages > Resolve Package Versions
```

### 4. Other callers not found by grep
The previous session searched for `AppCoordinator(` and `makeMapViewModel(watchZone` across the test and source directories. But if there are other targets (UI tests, app extensions, previews) that reference the old API, those would compile-fail in Xcode but not in `swift test`.

Check for any remaining old-API references:
```bash
cd mobile/ios
grep -r "makeMapViewModel(watchZone" --include="*.swift" .
grep -r "watchZone: WatchZone" --include="*.swift" . | grep -v "WatchZoneRepository"
```

### 5. Xcode scheme targets different from `swift build`
`swift build` builds the SPM packages. Xcode may build the app target (`town-crier-app`) which includes the `TownCrierApp.swift` composition root. If `TownCrierApp.swift` changes didn't land (stale checkout), the old `makeMapViewModel(watchZone:)` call would fail.

## Key Architectural Context

- `WatchZoneRepository` protocol is in `town-crier-domain` (`Sources/Protocols/WatchZoneRepository.swift`)
- `APIWatchZoneRepository` concrete class is in `town-crier-data` (`Sources/Repositories/APIWatchZoneRepository.swift`)
- `SpyWatchZoneRepository` test spy is in `town-crier-tests` (`Sources/Spies/SpyWatchZoneRepository.swift`)
- `MapView.swift` was NOT changed — `Map(initialPosition:)` reads the VM's centre values once on first render, which happens after `hasLoaded=true`, so the correct values are already set
- Watch zone fetch failure is handled gracefully with `try?` — falls back to London defaults (51.5074, -0.1278, radius 2000)
