# Town Crier — Android

Native Android client (Kotlin + Jetpack Compose), part of the Android parity epic
([#770](https://github.com/AmyDe/town-crier/issues/770)). This module mirrors the
iOS package split — `:domain` / `:data` / `:presentation` / `:app` — with the same
dependency rule: `:domain` has zero Android dependencies, `:data` and `:presentation`
both depend on `:domain` only (`:presentation` may **never** depend on `:data`), and
`:app` is the sole composition root that sees everything. See the
`android-coding-standards` Claude skill (`.claude/skills/android-coding-standards/`)
for the full architecture, testing, and style rules this module follows.

## Toolchain setup (one-time, per Mac)

```bash
brew install --cask temurin@21 android-commandlinetools
yes | sdkmanager --licenses
sdkmanager --install "platform-tools" "platforms;android-35" "build-tools;35.0.0" \
  "emulator" "system-images;android-35;google_apis;arm64-v8a"
avdmanager create avd -n towncrier -k "system-images;android-35;google_apis;arm64-v8a" -d pixel_7
```

Environment (add to `~/.zshrc` or equivalent):

```bash
export ANDROID_HOME="$HOME/Library/Android/sdk"   # or the brew cask location, e.g.
                                                    # /opt/homebrew/share/android-commandlinetools
export ANDROID_SDK_ROOT="$ANDROID_HOME"
export PATH="$ANDROID_HOME/platform-tools:$ANDROID_HOME/emulator:$ANDROID_HOME/cmdline-tools/latest/bin:$PATH"
```

Use `google_apis` system images, **not** `google_apis_playstore` — Play images block
`adb root` and are only needed for on-device Play Billing, which license testers on
real devices already cover.

The build itself needs **Java 21** on `JAVA_HOME` when invoking Gradle — this project
pins Kotlin's `jvmToolchain` to 21, and (see "Toolchain pins" below) the Gradle/AGP
combination in use here is incompatible with newer JDKs as the Gradle daemon's own
launcher JVM:

```bash
export JAVA_HOME=/Library/Java/JavaVirtualMachines/temurin-21.jdk/Contents/Home
```

## Build & test

Run from `mobile/android/`:

```bash
./gradlew test                       # All JVM unit tests, every module
./gradlew ktlintFormat                # Auto-format (run before committing)
./gradlew ktlintCheck detekt          # Lint + static analysis — CI-blocking
./gradlew :app:assembleDevDebug       # Debug build, dev flavor (emulator work)
./gradlew :app:assembleProdRelease    # Release build, prod flavor
```

The pre-flight gate for any change here: `ktlintFormat`, then
`test ktlintCheck detekt :app:assembleDevDebug :app:assembleProdRelease` all green.

### Toolchain pins (read before bumping any version)

`gradle/libs.versions.toml` pins Gradle **9.5.1** and AGP **8.13.2** together
deliberately — not the newest available versions of either:

- **AGP 9.x removed the separate Kotlin Android plugin** in favour of AGP's own
  "built-in Kotlin" support, with a materially different DSL for Compose/Kotlin
  options. AGP 8.13.2 is the newest release still on the classic
  `org.jetbrains.kotlin.android` plugin model this project (and the
  android-coding-standards skill) documents.
- **AGP 8.x cannot run on Gradle 9.6+**: `com.android.internal.library` depends on a
  Gradle-internal API (`InternalProblems`) that Gradle removed in 9.6.0. Gradle 9.5.1
  is the newest release that still works with AGP 8.x.
- **`compileSdk`/`targetSdk` are pinned to 35**, matching the SDK platforms actually
  installed by the setup commands above. Several AndroidX libraries (`core-ktx`,
  `activity-compose`, `lifecycle-*-compose`, `navigation-compose`) have started
  shipping AAR metadata that demands `compileSdk` 36/37 and AGP 9.1+ in their newest
  releases — `libs.versions.toml` pins those specific artifacts to the latest stable
  version that still targets `compileSdk` 35 cleanly (see the comment there).

Bumping `compileSdk` past 35 means installing the matching SDK platform locally
first (`sdkmanager --install "platforms;android-36" ...`) — do that deliberately,
not as a side effect of an unrelated dependency bump.

## Emulator verification loop (agent-driven, no human in the loop)

Per this repo's UI verification policy (`CLAUDE.md`), Android UI changes are
verified live on the emulator by the agent itself — never by asking a human to
click through the app.

```bash
# 1. Boot the emulator (background; first boot takes ~20-60s, subsequent boots ~15-20s)
emulator -avd towncrier -no-snapshot -no-audio -no-boot-anim &
adb wait-for-device

# 2. Wait for the boot to actually finish (wait-for-device only means the ADB
#    transport is up, not that the system UI is ready)
until [ "$(adb shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ]; do
  sleep 2
done

# 3. Install and launch the dev debug build
./gradlew :app:installDevDebug
adb shell monkey -p uk.towncrierapp.mobile.dev -c android.intent.category.LAUNCHER 1

# 4. Screenshot — use an ABSOLUTE path
adb exec-out screencap -p > /absolute/path/to/screenshot.png
```

With **mobile-mcp** (the MCP server that drives both the iOS simulator and this
Android emulator in the dev environment), the equivalent loop is: list devices →
launch `uk.towncrierapp.mobile.dev` → screenshot. mobile-mcp screenshot paths must
be absolute, same as the raw `adb exec-out` form above.

What a passing screenshot looks like for this scaffold: the app name ("Town Crier")
centered on the themed background — `tcBackground`/`tcSurface` per the active system
appearance (light/dark), Inter typography, no crash, no blank white screen.

## Module map

| Module | Kind | Depends on | Contains |
|---|---|---|---|
| `:domain` | `kotlin("jvm")` | stdlib, `kotlinx-coroutines-core`, `java.time` only | Entities, value classes, repository ports (later phases) |
| `:data` | android-library | `:domain` | `ApiClient`, DTOs, DataStore repositories (later phases) |
| `:presentation` | android-library, Compose | `:domain` only — never `:data` | Design system (`designsystem/`), ViewModels, screens |
| `:app` | android-application, Compose | everything | `MainActivity`, `AppGraph` composition root, manifest, flavors |

`:presentation` → `:data` is blocked both structurally (the dependency is simply
never declared) and by a Gradle-level guard: `./gradlew verifyModuleGraph` (wired
into the plain `test` task name, so `./gradlew test` runs it too) walks
`:presentation`'s declared project dependencies and fails the build if `:data` is
reachable, directly or transitively.

### Flavors

`:app` has two product flavors on the `environment` dimension, the *only*
environment mechanism (no `.env` files, no per-flavor secrets):

| Flavor | applicationId | `API_BASE_URL` |
|---|---|---|
| `dev` | `uk.towncrierapp.mobile.dev` | `https://api-dev.towncrierapp.uk` |
| `prod` | `uk.towncrierapp.mobile` | `https://api.towncrierapp.uk` |

## Design system

`:presentation`'s `designsystem/` package implements the Town Crier design language
(`.claude/skills/design-language/`) for Compose:

- `Color.kt` — raw token values per theme (`TcPalette` + `LightPalette`/
  `DarkPalette`/`OledPalette`).
- `Theme.kt` — `Appearance` (System/Light/Dark/OledDark), the pure resolution
  functions (`resolveIsDark`, `resolveIsOled`, `resolvePalette`), the Material 3
  role mapping (`colorScheme`), and the `TownCrierTheme` composable + companion
  object (`TownCrierTheme.colors`, `TownCrierTheme.bodyEmphasis`) for tokens with no
  Material 3 slot.
- `Type.kt` — Inter (the same upstream release `web/public/fonts/inter-latin*.woff2`
  self-hosts, v4.001) bundled as four static-weight TTFs, mapped onto the Material 3
  type scale.
- `Spacing.kt` — the 4dp-base spacing scale, corner radius scale, and the M3
  `Shapes` override.
- `components/` — `PrimaryButton`, `StatusBadge`, `CapsuleChip`.

Dynamic color is off by design — brand colors are pinned tokens, never derived from
device wallpaper. `ThemeMappingTest` asserts the exact light/dark/OLED hex values
from the epic's token table against the resolved `ColorScheme`/`TownCrierColors`.
