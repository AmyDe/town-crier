# Handoff: Fix iOS Simulator Real API Failures

## Branch & PR
- **Branch:** `worktree-fix-debug-real-api` (pushed, 4 commits ahead of main)
- **Worktree:** `/Users/christy/Dev/town-crier/.claude/worktrees/fix-debug-real-api`
- **PR:** #221 — needs revision before merge
- **Bead:** `tc-t7n` (in_progress) — investigate iOS simulator timeout

## Root Cause (solved)
PlanIt returns `null` for `appType` (150/3051 Cornwall apps) and `appState` (25/1000 Lewisham apps). The Swift `PlanningApplicationDTO` declared these as non-optional `String`, causing `JSONDecoder` to fail on the entire array with "The data couldn't be read because it is missing."

**Fix:** Make `appType` and `appState` `String?` (optional) in the Swift DTO (`APIPlanningApplicationRepository.swift:54-55`). The `toDomain()` method already handles the fallback — `mapAppState(appState ?? "")` maps to `.unknown`.

## What needs doing

### 1. Revert unnecessary .NET changes
The .NET `string` vs `string?` distinction is **compile-time only** — it does not affect JSON serialization. The web app works fine against the same API, proving the .NET side was never broken. Revert all .NET changes from commit `b25e1d2` to keep the PR iOS-only. Files to revert:
- `api/src/town-crier.application/DecisionAlerts/DispatchDecisionAlertCommandHandler.cs`
- `api/src/town-crier.application/DemoAccount/DemoApplicationResult.cs`
- `api/src/town-crier.application/Notifications/NotificationItem.cs`
- `api/src/town-crier.application/PlanningApplications/PlanningApplicationResult.cs`
- `api/src/town-crier.application/Search/PlanningApplicationSummary.cs`
- `api/src/town-crier.domain/Notifications/Notification.cs`
- `api/src/town-crier.domain/PlanningApplications/PlanningApplication.cs`
- `api/src/town-crier.infrastructure/Notifications/AcsEmailSender.cs`
- `api/src/town-crier.infrastructure/Notifications/NotificationDocument.cs`
- `api/src/town-crier.infrastructure/PlanningApplications/PlanningApplicationDocument.cs`

### 2. Fix Xcode build failure
User gets build failure when doing Cmd+Shift+K (clean) then Cmd+R (run) in Xcode. Likely causes:
- `xcodegen generate` was run in the **worktree** but the regenerated `.xcodeproj` was NOT committed (it's in `.gitignore`). The user may be opening Xcode from the main repo, which still has the stale `.xcodeproj` referencing deleted `SampleData.swift`.
- **Fix:** Run `xcodegen generate` in `mobile/ios/` from the main repo (or whichever directory the user opens in Xcode), or commit the generated project if it's not gitignored.
- Check: `mobile/ios/.gitignore` — does it ignore `*.xcodeproj`? If so, each developer needs to run `xcodegen generate` locally.

### 3. Fix Map view
The Map tab in the iOS app still fails to load. Not yet investigated. Likely similar decode issue or a different API endpoint failure. Use the debug logging (subsystem `uk.towncrierapp`, category `APIClient`) to capture what endpoint the Map view calls and what error it hits.

### 4. Consider removing debug logging before merge
The debug `os.Logger` diagnostics in `URLSessionAPIClient.swift` (lines 14-16, 32-33, 37-38, 43-45, 51-52, 57-58, 63-64, 78-79, 85-86, 91-95, 107-109) are useful for development but add noise. Decide whether to keep them (they're `#if DEBUG` only so harmless in release) or strip them now that the bug is found.

### 5. Close bead and update PR
- Close `tc-t7n` once all issues are resolved
- Update PR #221 description to reflect iOS-only changes
- Squash or reorganize commits if desired

## Key Files
- `mobile/ios/packages/town-crier-data/Sources/API/URLSessionAPIClient.swift` — debug logging
- `mobile/ios/packages/town-crier-data/Sources/Repositories/APIPlanningApplicationRepository.swift` — the **actual fix** (lines 54-55: `String?` for appType/appState)
- `mobile/ios/project.yml` — xcodegen spec (source of truth for `.xcodeproj`)

## Auth0 M2M Client
A Machine-to-Machine client grant was added during investigation: client `ZD56SkSPbPCFP3zTerdrY3ud48mpOq84` ("Town Crier API (M2M)") → audience `https://api-dev.towncrierapp.uk`. Clean up if not needed long-term.

## Simulator Setup
- `xcode-select` was switched to `/Applications/Xcode.app/Contents/Developer` (was pointing to CommandLineTools)
- Simulator: iPhone 17 Pro (iOS 26.4), device ID `5B769350-F902-4115-A9DA-CCFEEAAFF3D5`
- The app is installed and authenticated in this simulator — just launch to test
