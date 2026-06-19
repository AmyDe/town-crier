---
name: ios-whats-new
description: >-
  Produce the App Store "What's New" release-notes copy for the Town Crier iOS app,
  written in the user's voice and containing only changes that actually affect iOS
  users. MUST use this skill whenever the user wants the App Store "What's New" text,
  the release notes for an App Store submission, the App Store Connect changelog, or
  the "What's New in this version" copy for an iOS release — including phrasings like
  "write the What's New", "App Store release notes", "What's New for the new version",
  "copy for App Store Connect", or "release notes for the store". This is the public,
  human-facing store copy, distinct from the automated TestFlight changelog (handled
  by scripts/ios-release-notes.sh in CD) and distinct from the GitHub release notes
  (handled by the `release` skill). Do NOT use for: cutting a GitHub release/tag (use
  `release`), the TestFlight changelog (already automated), or any non-iOS release notes.
---

# iOS "What's New"

Produce the App Store **What's New** copy for a Town Crier iOS release: short, in the
user's voice, and mentioning *only* things an iOS user can actually see or do.

Two things make App Store copy different from the engineering changelog:

- **It's public and read by non-technical users.** Backend, infra, CI, API, and web
  work must never appear. The reader doesn't know or care that a Cosmos query got
  faster or a worker job was added.
- **It's marketing-adjacent, not a commit log.** Lead with the change that matters
  most to the user, phrase everything as a benefit, and bundle the small stuff.

The hard part — deciding what counts as an iOS change — is already solved by
`scripts/ios-release-notes.sh`, which filters to `Release-Note:` trailers and
`mobile/ios` feat/fix subjects and excludes everything else. Reuse it as the source
of truth so the relevance filter never drifts. This skill is the layer on top that
turns that filtered material into polished copy in the user's voice.

## Step 1 — Resolve the release range

The store copy covers everything shipped since the **last App Store submission**, which
may span several `v*` tags (not every tag goes to the store). Resolve the range:

```bash
git fetch --tags origin
git tag --sort=-v:refname | head -10
```

- Default `<current>` to the tag being submitted (or `HEAD` if the user is preparing
  before tagging).
- Default `<previous>` to the previous `v*` tag.
- If the user names the last shipped store version (e.g. "everything since 1.2.0"),
  use that tag as `<previous>` even if newer tags exist in between — the store reader
  hasn't seen any of it.

Confirm the resolved range with the user in one line before generating if it's at all
ambiguous; otherwise just state which range you used.

## Step 2 — Gather iOS-relevant material only

Get the canonical filtered change list:

```bash
bash scripts/ios-release-notes.sh "<previous-tag>" "<current-ref>"
```

Then read the underlying iOS commits for the *why* and the user benefit — commit
subjects alone are often too terse to write good copy from:

```bash
git log --no-merges "<previous-tag>..<current-ref>" -- mobile/ios/
```

Rules that keep the copy honest:

- **Only mobile/ios changes.** If a change isn't reachable under `mobile/ios/`, it
  doesn't go in the copy. No backend, infra, CI, API, or web.
- **If the script returns only the stock line** ("Bug fixes and performance
  improvements.") there were no user-facing iOS changes in the range. Say so — there
  is nothing to announce. Don't invent features or pad with the stock line unless the
  user explicitly wants a placeholder.
- **Don't leak internals.** No bead IDs, PR numbers, scopes, file names, or jargon.

## Step 3 — Write the copy in the user's voice

Apply the **`voice`** skill for all wording — this is product copy in the user's
register (it auto-triggers on app/marketing copy; invoke it explicitly if it hasn't
loaded). Voice owns the language rules (plain, dry British English, concrete over
abstract, no em dashes, no hype, no AI-tell). This skill only adds the App Store
structure on top:

- **Lead with the single most meaningful change.** One strong line up front beats a
  flat list.
- **Benefit first, mechanism second.** Say what the user gets, not what was built.
  "Saved searches now refresh the moment an application changes" beats "Added a
  refresh trigger to SavedSearchViewModel".
- **Bundle the trivia.** Collapse minor fixes into a single closing line rather than
  itemising every one.
- **Keep it short.** A few short lines or tight bullets. App Store Connect caps
  "What's New" at 4000 characters; you'll be far under that. Prefer fewer, better lines.
- **Reuse, don't restate.** `Release-Note:` trailers are already authored in plain
  English — lean on them; only rewrite where the store reader needs more or less.

## Step 4 — Deliver

1. Print the final copy in a fenced block so the user can copy it straight out.
2. Offer to write it to `mobile/ios/fastlane/metadata/en-GB/release_notes.txt` (the
   path the future `deliver` lane will use — bead tc-3eim). Don't write or commit
   without the user's say-so.
3. Point the user at App Store Connect to paste it. The deepest addressable link is the
   app's App Store page; selecting the version and editing the "What's New" field has no
   deep link:
   https://appstoreconnect.apple.com/apps/6764095657/appstore

## Example

Raw filtered material (from the script + commits):

```
- Saved searches now refresh the moment an application changes.
- Export your data from Settings.
- Fixed a crash when opening a zone with no applications.
- Fixed alert toggles showing for free accounts.
```

What's New (voice-led, benefit-first, trivia bundled):

```
Saved searches now update the moment a planning application changes, so you hear
about decisions as they happen.

You can also export all your data from Settings whenever you want it.

Plus a handful of fixes to make everything steadier.
```
