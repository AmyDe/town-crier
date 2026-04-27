# 0025. iOS TestFlight release automation

Date: 2026-04-27

## Status

Accepted

## Context

The first iOS TestFlight build (v1.0.0 / build 1) was uploaded manually from Xcode on 2026-04-27. Repeating that ritual for every release is slow, error-prone, and depends on a single workstation having the right certificates and Xcode version installed.

We already have a release process: `/release` skill creates a `v*` GitHub release, which fires `cd-prod.yml` and ships the backend. iOS needs to plug into the same trigger so a single command releases the whole product.

Constraints:
- **No paid CI.** The repo is public, so standard GH-hosted macOS runners are free with no minute cap. Larger runners (`*-xlarge`) are billed even on public repos and are off the table.
- **No iOS-only release ritual.** The user does not want a separate "release iOS" command. One tag should ship everything that needs shipping.
- **Don't push pointless TestFlight builds.** Most releases are backend-only; testers should not see identical builds with no changes.

## Decision

Add `.github/workflows/cd-ios-testflight.yml`, triggered on `push: tags: 'v*'` (same as `cd-prod.yml`), with three jobs:

1. **`guard`** (Ubuntu, ~10s): runs `git diff --quiet <prev-tag>..<this-tag> -- mobile/ios/`. Sets `should-release` output. Subsequent jobs are gated `if: needs.guard.outputs.should-release == 'true'`.
2. **`test`** (macOS, ~10–15 min): xcodegen, swiftlint --strict, xcodebuild test on simulator. Provides the only CI gate for iOS code.
3. **`testflight`** (macOS, ~15–20 min): fastlane `beta` lane — `match` (readonly) → bump build number to `latest_testflight_build_number + 1` → `build_app` (Release archive) → `upload_to_testflight` with the GH release body as changelog → annotate the GH release with the resulting build number.

Code signing uses **fastlane match** with a private GH repo (`AmyDe/town-crier-ios-certs`) for cert/profile storage. Build numbers come from App Store Connect via `latest_testflight_build_number + 1` so manual and automated uploads can coexist without collision.

Marketing version stays manually managed in `mobile/ios/project.yml`. Git tags do not drive marketing version — backend releases happen far more frequently than iOS user-visible releases, and forcing them to share a version line would either spam the App Store with noise versions or delay backend ships until iOS catches up.

## Consequences

### Easier
- Cutting a release is a single `/release` command — same flow as before, iOS now goes along for the ride.
- TestFlight build numbers can never collide; `latest + 1` is authoritative.
- Adding iOS coding standards as a release gate (lint + tests on macOS) closes the gap that pr-gate doesn't cover today.
- All credentials live as GH secrets; no workstation-specific Xcode keychain dependency.

### Harder
- **Match cert rotation needs human action.** CI runs `match` in `readonly: true`. When certs expire (Apple distribution certs last 1 year), a human must run `fastlane match appstore` locally to refresh the repo. There is no autorenewal.
- **First-time setup has six manual steps.** App Store Connect API key, cert repo, deploy key, initial match run, six GH secrets, project.yml team ID. Documented in `docs/specs/ios-testflight-automation.md`. One-time only.
- **macOS runner concurrency.** Standard runners are limited to 5 concurrent macOS jobs across the account. Not a problem today; flag if it ever is.
- **Backend-only releases trigger a `guard` job that always exits "skip".** Costs ~10s of free Ubuntu time per release. Acceptable.
- **iOS-only changes still require a `v*` tag** to ship to TestFlight. There is no "iOS-only" release path. If iOS needs to ship without backend, just cut a release — backend will redeploy harmlessly (idempotent Pulumi up).

### Considered and rejected

- **Xcode Cloud.** Apple's native CI. 25 free hours/month, then paid. Would mean running two CI systems (GH Actions + Xcode Cloud) and losing the ability to gate the iOS build on `git diff` against the previous tag. Free tier could also run out if test iterations spike.
- **Per-platform release tags (`ios-v*` and `api-v*`).** Cleaner conceptually but adds a release ritual, breaks the "single command ships everything" goal, and complicates the `/release` skill.
- **Auto-bump build number from `github.run_number`.** Doesn't survive workflow renames or repo migrations, and collides with the existing manual TestFlight upload (build 1). The App Store Connect query approach is robust to all of those.
- **Build on every push to main, not on tags.** Would push a TestFlight build for every merged PR — testers would drown in noise, and most merges don't represent a coherent release.
