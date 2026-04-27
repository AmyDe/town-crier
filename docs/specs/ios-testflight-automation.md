# iOS TestFlight Release Automation

Date: 2026-04-27
Bead: tc-pi8t
Related ADR: [0025](../adr/0025-ios-testflight-release-automation.md)

## Goal

Automate iOS TestFlight uploads so that every `v*` tag pushed by the `/release` skill conditionally produces a new TestFlight build, with **zero manual Xcode interaction**, **zero recurring cost**, and a clear paper trail on the GitHub release.

## Trigger

Same trigger as `cd-prod.yml`: push of a tag matching `v*`. The `/release` skill creates these tags via `gh release create`.

## Conditional release

Most tags are backend-only. A guard job decides whether to run the iOS build:

- Find the previous `v*` tag with `git describe --tags --abbrev=0 ${tag}^`.
- Run `git diff --quiet <prev>..<tag> -- mobile/ios/`.
- If the diff is non-empty, or there is no previous tag, **release**. Otherwise **skip**.

Skipping costs ~10 seconds (one Ubuntu job). Releasing burns ~25–35 minutes of free macOS runner time.

## Build number

Source of truth: App Store Connect, queried by `fastlane latest_testflight_build_number`. The lane uses `latest + 1` and writes the value to `mobile/ios/fastlane/build-number.txt` so the workflow can include it in the release annotation.

This handles three failure modes for free:
1. Manual TestFlight uploads done in Xcode never collide with automated ones.
2. Re-running a workflow never produces the same build number twice.
3. We don't need to track build state anywhere in the repo.

## Marketing version

Stays in `mobile/ios/project.yml` (`MARKETING_VERSION`). Bumped manually when iOS has a real user-visible release. The git tag (`v0.9.x`) does **not** drive marketing version — backend release cadence is decoupled from iOS release cadence by design.

## Code signing

Fastlane `match` with `storage_mode: git` against a private GH repo (`AmyDe/town-crier-ios-certs`). CI runs `match` in `readonly: true` mode — it never creates or rotates certificates. Initial cert generation happens once locally (see Setup).

## TestFlight changelog

The workflow fetches the GH release body via `gh release view --json body` and writes it to `mobile/ios/fastlane/metadata/changelog.txt`, truncated to 3,900 chars (TestFlight allows 4,000). Fastlane reads that file and passes it as the `changelog:` argument to `upload_to_testflight`.

## Distribution

`distribute_external: false` — internal testers only for now. Adding external testers is a separate manual step in App Store Connect (requires Beta App Review for the first external build).

## Workflow shape

```
push: tags: v*
  ↓
guard (ubuntu, ~10s)
  ↓ should-release == true
test (macos, ~10–15 min)        ← swiftlint + xcodebuild test
  ↓
testflight (macos, ~15–20 min)  ← match + xcodegen + archive + upload
  ↓
annotate GH release with build number
```

`test` and `testflight` are sequential. We don't fan out because the GH release annotation needs the upload to have happened.

## One-time setup

Do these **before** the first tag push that should trigger TestFlight upload. They are idempotent — re-running them is safe.

### 1. App Store Connect API key

1. Go to **appstoreconnect.apple.com → Users and Access → Integrations → Team Keys**.
2. Click **+**, name it "Town Crier CI", role **App Manager**.
3. Click **Generate**, then **download the `.p8` file immediately** — Apple won't show it again.
4. Note the **Key ID** and **Issuer ID** displayed on the page.

### 2. Match git repo

```bash
gh repo create AmyDe/town-crier-ios-certs \
  --private \
  --description "Fastlane match storage for Town Crier iOS signing certificates"
```

### 3. SSH deploy key for the cert repo

```bash
ssh-keygen -t ed25519 -C "town-crier-ios-certs deploy key" \
  -f ~/.ssh/town-crier-match -N ""

# Add public key as a deploy key on the cert repo (write access required —
# you'll add new app identifiers later via fastlane locally)
gh repo deploy-key add ~/.ssh/town-crier-match.pub \
  --repo AmyDe/town-crier-ios-certs \
  --title "fastlane-match-ci" \
  --allow-write
```

### 4. Initial cert + profile generation (one-off, local)

Requires fastlane installed locally:

```bash
cd mobile/ios
brew install fastlane xcodegen
xcodegen generate    # produces TownCrier.xcodeproj

# Run match in CREATE mode — populates the certs repo with a freshly
# encrypted certificate and provisioning profile for uk.towncrierapp.mobile.
# It will prompt for MATCH_PASSWORD — pick a long random one and save it.
GIT_SSH_COMMAND="ssh -i ~/.ssh/town-crier-match -o IdentitiesOnly=yes" \
  bundle exec fastlane match appstore \
    --git_url "git@github.com:AmyDe/town-crier-ios-certs.git" \
    --app_identifier "uk.towncrierapp.mobile"
```

After this, the cert repo contains an encrypted `certs/distribution/*.p12` and `profiles/appstore/*.mobileprovision`. CI can read them with the same `MATCH_PASSWORD`.

### 5. GitHub repository secrets

Add to `https://github.com/AmyDe/town-crier/settings/secrets/actions` (or via CLI):

| Secret name | Value |
|---|---|
| `APP_STORE_CONNECT_API_KEY_ID` | Key ID from step 1 |
| `APP_STORE_CONNECT_ISSUER_ID` | Issuer ID from step 1 |
| `APP_STORE_CONNECT_API_KEY_CONTENT` | `base64 -i AuthKey_XXXXX.p8` (the .p8 file from step 1, base64-encoded) |
| `MATCH_PASSWORD` | The password chosen in step 4 |
| `MATCH_GIT_DEPLOY_KEY` | Contents of `~/.ssh/town-crier-match` (the **private** key from step 3) |
| `APPLE_TEAM_ID` | Your 10-character Apple Developer Team ID (find in Xcode → Signing & Capabilities, or appstoreconnect.apple.com → Membership) |

```bash
# Faster via CLI — pipe values directly
gh secret set APP_STORE_CONNECT_API_KEY_ID --body "ABC123XYZ"
gh secret set APP_STORE_CONNECT_ISSUER_ID --body "00000000-0000-0000-0000-000000000000"
base64 -i AuthKey_ABC123XYZ.p8 | gh secret set APP_STORE_CONNECT_API_KEY_CONTENT
gh secret set MATCH_PASSWORD --body "your-long-random-password"
cat ~/.ssh/town-crier-match | gh secret set MATCH_GIT_DEPLOY_KEY
gh secret set APPLE_TEAM_ID --body "ABCD123456"
```

### 6. Add `DEVELOPMENT_TEAM` to project.yml

Currently signing requires manual selection in Xcode every time `xcodegen generate` runs. Add the team ID to `mobile/ios/project.yml` so it survives regeneration:

```yaml
targets:
  TownCrierApp:
    settings:
      base:
        DEVELOPMENT_TEAM: YOUR_TEAM_ID   # same value as APPLE_TEAM_ID secret
```

(This is also blocked by bead `tc-v71z` — it should be done as part of this work.)

## Operational notes

### Verifying a TestFlight upload

After the workflow turns green:
- Check **App Store Connect → TestFlight → Builds**. Build appears in "Processing" within ~30 seconds, completes in ~5–10 min.
- Internal testers receive the build automatically once processing finishes.
- The GH release page now shows a "TestFlight build N uploaded" annotation at the top of the release notes.

### Rotating the App Store Connect API key

Apple keys never expire by default but can be revoked. To rotate:
1. Generate a new key in App Store Connect (step 1).
2. Update the three `APP_STORE_CONNECT_*` GH secrets.
3. Revoke the old key.

### Re-running a failed upload

Re-run from the GitHub Actions UI — `latest_testflight_build_number + 1` ensures no collision. If the upload succeeded but the release annotation failed, edit the release body manually.

### Common failure modes

| Symptom | Likely cause | Fix |
|---|---|---|
| `match` fails with "remote: Permission denied" | `MATCH_GIT_DEPLOY_KEY` is wrong key or the deploy key wasn't added with write access | Regenerate, re-add as deploy key with write access |
| `match` fails decrypting | `MATCH_PASSWORD` mismatch | Confirm secret matches the password used in step 4 |
| Upload fails with "Invalid Profile" | Cert/profile expired or revoked | Re-run `fastlane match nuke distribution && fastlane match appstore` locally |
| Build fails with "No development team" | `DEVELOPMENT_TEAM` missing from project.yml | Add it (step 6) |
| Build number rejected by App Store | Marketing version was decreased, or this build number was already used | Bump `MARKETING_VERSION` in project.yml |

## Limitations

- **Internal testers only** — external distribution requires a separate Beta App Review trip the first time. Add external testers manually in App Store Connect when ready.
- **No App Store submission** — this only ships to TestFlight. App Store releases (production) remain manual.
- **No iOS PR-gate checks** — pr-gate.yml does not run iOS tests/lint today. The TestFlight workflow's `test` job is the *only* CI check that runs iOS code on macOS. Filed as a separate concern.
- **Apple ID 2FA** — not used here. The App Store Connect API key bypasses 2FA entirely, which is why we use it instead of Apple ID + app-specific password.

## Cost

£0. Standard `macos-latest` runners are free for public repos with no minute cap. The cert repo is a private GH repo (free, unlimited).
