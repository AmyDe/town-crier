# Workflow & Naming (reference)

Read when running build/test/lint, bootstrapping ktlint/detekt, editing the version catalog, or naming anything.

## Commands (run from `mobile/android/`)

```bash
./gradlew test                       # All JVM unit tests, every module
./gradlew ktlintFormat               # Auto-format (run before committing)
./gradlew ktlintCheck detekt         # Lint + static analysis — CI-blocking, zero findings
./gradlew :app:assembleDevDebug      # Debug build, dev flavor (emulator work)
./gradlew :app:assembleProdRelease   # Release build, prod flavor
```

Two Gradle wirings the lint/test loop silently depends on:

- **JUnit 5 in Android modules.** AGP doesn't run Jupiter tests natively — every Android module with tests needs `testOptions { unitTests.all { it.useJUnitPlatform() } }` (`:domain`, being `kotlin("jvm")`, just calls `useJUnitPlatform()`). Missing wiring = green build, zero tests run.
- **Type-resolving detekt.** The plain `detekt` task skips type-resolution rules (the coroutines-safety ones in `detekt.yml` marked as such). The full gate also runs `detektMain`/`detektTest` on JVM modules and the `detekt<Variant>` tasks on Android modules.

The pre-commit gate for any Android bead: `ktlintFormat`, then `test ktlintCheck detekt :app:assembleDevDebug` all green. CI runs the same four via the `android-lint` / `android-build-test` PR-gate jobs — a local pass should make CI boring. User-visible changes get a `Release-Note:` git trailer (the Play "what's new" is generated from trailers, same as TestFlight).

## Lint configs are skill assets

The canonical configs live in this skill and are copied into the project (same pattern as the Go skill's `.golangci.yml`):

- `assets/.editorconfig` → `mobile/android/.editorconfig` — ktlint (`ktlint_official` code style, 120 cols, Compose function-naming carve-out).
- `assets/detekt.yml` → `mobile/android/detekt.yml` — applied with `buildUponDefaultConfig = true`, so the file holds only deliberate deviations. Philosophy matches `.golangci.yml`: bug-catching rules on (coroutines misuse, swallowed exceptions, forbidden imports enforcing the skill's bans), style-opinion rules off (ktlint owns formatting).

If a rule fires on genuinely correct code, prefer restructuring the code; a `@Suppress` needs a justifying comment and should be rare enough to stand out in review. Ratcheting a rule off happens in the skill's asset (with the reasoning), never ad-hoc per module — the asset is the source of truth and the project copy must not drift.

## Gradle discipline

- **Everything through the version catalog** (`gradle/libs.versions.toml`); exact versions pinned — no dynamic versions (`+`), no ranges. The Compose BOM is pinned like any other entry; ktlint/detekt plugin + CLI versions pinned (ruleset drift broke SwiftLint once — tc-6kdu; don't repeat it on Android).
- Build logic stays boring: Kotlin DSL, no custom plugins until at least two modules repeat the same block, no `buildSrc` speculation.
- `kotlin { explicitApi() }` on `:domain` and `:data` — mechanical enforcement of `internal`-by-default (see `architecture-and-modules.md`).
- New dependencies are an architectural decision, not a convenience — the Forbidden list in `SKILL.md` bans the usual suspects; anything else new warrants a line in the PR body saying why.

## Naming

| Thing | Convention | Example |
|---|---|---|
| Gradle modules | lowercase | `:domain`, `:data`, `:presentation`, `:app` |
| Packages | `uk.towncrierapp.<module>.<feature>`, lowercase, no underscores | `uk.towncrierapp.domain.watchzones` |
| Classes/interfaces | PascalCase; interfaces get the plain domain name, no `I` prefix | `WatchZoneRepository` |
| Implementations | named by what distinguishes them — never `Impl` | `HttpWatchZoneRepository`, `InMemoryApplicationCache`, `DataStoreOnboardingStateStore` |
| Composables | PascalCase noun (what it *is*, not what it does) | `StatusBadge`, `WatchZonesScreen` |
| Functions/properties | camelCase; no `get` prefix on Kotlin properties | `val effectiveTier`, `fun redeem(code)` |
| Constants | UPPER_SNAKE_CASE `const val`, top-level or companion | `MAX_METRES` |
| Backing property | leading underscore, private | `_uiState` / `uiState` |
| Fakes | `Fake` + port name; call-recording lists `<method>Calls` | `FakeWatchZoneRepository`, `deleteCalls` |
| Fixture factories | indefinite article + type | `aWatchZone()`, `aPlanningApplication()` |
| Test classes / names | `<Subject><Concern>Test`; backtick behaviour sentences | `WatchZonesViewModelRefreshTest` |
| DTOs | wire name + `Dto`, `internal` to `:data` | `WatchZoneDto` |
| Files | one primary type per file, named after it; small sealed families may share the parent's file | `WatchZone.kt` |
