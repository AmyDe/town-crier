# Watch Zone Edit — Hide Postcode Field — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Hide the postcode field on `WatchZoneEditorView` when the editor is in edit mode, so users only see name + radius.

**Architecture:** Expose visibility as a view model property (`isPostcodeFieldVisible`). The view binds to it. No domain or schema changes. View model behaviour for `submitPostcode()` is unchanged — just unreachable from production UI in edit mode.

**Tech Stack:** Swift, SwiftUI, Swift Testing (`@Test`/`@Suite`), MVVM, SPM.

**Spec:** `docs/specs/watch-zone-edit-hide-postcode.md`

**Bead:** tc-gdl0

---

## File Map

- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/WatchZones/WatchZoneEditorViewModel.swift` — add `isPostcodeFieldVisible` computed property.
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/WatchZones/WatchZoneEditorView.swift:17` — gate `postcodeSection` on the new property.
- Modify: `mobile/ios/town-crier-tests/Sources/Features/WatchZoneEditorViewModelTests.swift` — add visibility tests in both create-mode and edit-mode suites.

No test file is created. The existing `WatchZoneEditorViewModelTests.swift` already has `WatchZoneEditorCreateTests` and `WatchZoneEditorEditTests` suites that we extend.

---

## Task 1: Add `isPostcodeFieldVisible` to view model

**Files:**
- Test: `mobile/ios/town-crier-tests/Sources/Features/WatchZoneEditorViewModelTests.swift`
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/WatchZones/WatchZoneEditorViewModel.swift`

- [ ] **Step 1: Write the failing tests**

In `WatchZoneEditorViewModelTests.swift`, add one `@Test` to the `WatchZoneEditorCreateTests` suite (insert after the existing `radiusOptions_*` tests, before `submitPostcode_autoFillsNameFromPostcode_whenNameEmpty`):

```swift
  @Test func isPostcodeFieldVisible_inCreateMode_isTrue() {
    #expect(sut.isPostcodeFieldVisible)
  }
```

And add one `@Test` to the `WatchZoneEditorEditTests` suite (insert after `initialState_freeformName_populatesNameInput`, before `save_preservesExistingId`):

```swift
  @Test func isPostcodeFieldVisible_inEditMode_isFalse() {
    #expect(!sut.isPostcodeFieldVisible)
  }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd mobile/ios && swift test --filter "isPostcodeFieldVisible"`

Expected: both tests fail to compile with "value of type 'WatchZoneEditorViewModel' has no member 'isPostcodeFieldVisible'".

- [ ] **Step 3: Add the property**

In `WatchZoneEditorViewModel.swift`, add a public computed property next to the other view-state computed properties (immediately after `maxRadiusMetres` on line 49):

```swift
  public var isPostcodeFieldVisible: Bool {
    !isEditing
  }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd mobile/ios && swift test --filter "isPostcodeFieldVisible"`

Expected: both tests pass.

- [ ] **Step 5: Run the full editor test suite to confirm nothing else broke**

Run: `cd mobile/ios && swift test --filter "WatchZoneEditor"`

Expected: all tests in `WatchZoneEditorCreateTests` and `WatchZoneEditorEditTests` pass.

- [ ] **Step 6: Commit**

```bash
git add mobile/ios/packages/town-crier-presentation/Sources/Features/WatchZones/WatchZoneEditorViewModel.swift \
        mobile/ios/town-crier-tests/Sources/Features/WatchZoneEditorViewModelTests.swift
git commit -m "$(cat <<'EOF'
feat: expose isPostcodeFieldVisible on WatchZoneEditorViewModel (tc-gdl0)

Edit mode hides the postcode field; create mode shows it. View
binds to this property in the next task.
EOF
)"
```

---

## Task 2: View consumes `isPostcodeFieldVisible` to gate the postcode section

**Files:**
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/WatchZones/WatchZoneEditorView.swift:17`

- [ ] **Step 1: Modify the view**

In `WatchZoneEditorView.swift`, change the `body` so `postcodeSection` only renders when `viewModel.isPostcodeFieldVisible`. Replace the current lines 15-25:

```swift
      Form {
        nameSection
        postcodeSection
        if viewModel.geocodedCoordinate != nil {
          radiusSection
          mapPreviewSection
        }
        if let error = viewModel.error {
          errorSection(error)
        }
      }
```

with:

```swift
      Form {
        nameSection
        if viewModel.isPostcodeFieldVisible {
          postcodeSection
        }
        if viewModel.geocodedCoordinate != nil {
          radiusSection
          mapPreviewSection
        }
        if let error = viewModel.error {
          errorSection(error)
        }
      }
```

- [ ] **Step 2: Build the iOS package to verify it compiles**

Run: `cd mobile/ios && swift build`

Expected: builds with no errors. (Warnings about unrelated code are fine.)

- [ ] **Step 3: Run the full editor test suite**

Run: `cd mobile/ios && swift test --filter "WatchZoneEditor"`

Expected: all tests pass — same as before. (The view change has no automated test path; the view model property test from Task 1 is the closest assertion.)

- [ ] **Step 4: Manual verification (no automated equivalent)**

Open the iOS app in the simulator. Create a new watch zone — confirm the postcode field is visible and the lookup flow works. Then tap an existing zone to edit — confirm the postcode field and "Look up" button are not present, but name and radius are editable and save works.

If manual verification cannot be performed in this environment, note that explicitly when reporting the task complete; do not claim it passed.

- [ ] **Step 5: Commit**

```bash
git add mobile/ios/packages/town-crier-presentation/Sources/Features/WatchZones/WatchZoneEditorView.swift
git commit -m "$(cat <<'EOF'
feat: hide postcode field on watch zone edit screen (tc-gdl0)

Locks the zone centre on edit. Users who want a different
location delete and recreate.
EOF
)"
```

---

## Task 3: Run lints and full test suite, then close the bead

- [ ] **Step 1: Run SwiftLint strict**

Run: `cd mobile/ios && swiftlint lint --strict`

Expected: no violations.

- [ ] **Step 2: Run swift-format check**

Run: `cd mobile/ios && swift-format format --in-place --recursive .`

Then: `git diff --exit-code`

Expected: no diff. If swift-format produced changes, stage and amend them into the most recent commit.

- [ ] **Step 3: Run the full iOS test suite**

Run: `cd mobile/ios && swift test`

Expected: all tests pass.

- [ ] **Step 4: Update bead notes and close**

```bash
bd update tc-gdl0 --notes "$(cat <<'EOF'
COMPLETED: Postcode field hidden in edit mode via isPostcodeFieldVisible view-model property.
IN PROGRESS: —
NEXT: Open PR, watch CI.
BLOCKER: —
KEY DECISIONS: View-model retains submitPostcode() — unreachable from edit UI but still part of create-mode contract; existing edit-mode test for re-locating intentionally kept as a contract test.
EOF
)"
bd close tc-gdl0
```

- [ ] **Step 5: Push and open PR**

```bash
git push -u origin fix/watch-zone-edit-hide-postcode
```

Then use the `/ship` skill (or `gh pr create`) to open a PR targeting `main`. Link `tc-gdl0` and the spec in the PR body.

---

## Self-Review

**Spec coverage:**
- Spec "In scope: hide postcode TextField + lookup button on edit" → Task 2.
- Spec "In scope: view model no behavioural change required" → confirmed; only an additive computed property is added.
- Spec "In scope: tests covering both modes" → Task 1 adds visibility tests for both modes; existing save() and re-locate tests kept.
- Spec "Out of scope: domain/schema/API changes" → no such changes in any task.
- Spec "Out of scope: removing submitPostcode/postcodeInput from view model" → kept as-is.

**Placeholder scan:** No "TBD", "TODO", or "implement later" tokens. Each step shows the exact code or command. Manual verification in Task 2 Step 4 is explicit about what to look for and what to do if it can't be performed.

**Type consistency:** Property name `isPostcodeFieldVisible` is identical across the view model declaration (Task 1 Step 3), the view consumption site (Task 2 Step 1), and both tests (Task 1 Step 1). Bead id `tc-gdl0` is consistent.
