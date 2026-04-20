# iOS test consolidation (Swift Testing + muter)

## Where tests live

- `mobile/ios/town-crier-tests/Tests/` — Swift Testing tests (ViewModel / UseCase / Coordinator)
- `mobile/ios/town-crier-tests/Sources/Spies/` — hand-written spies and stubs
- `mobile/ios/town-crier-tests/Sources/Fixtures/` — fixtures (static extensions on the type under test)

ViewModels and UseCases are the main consolidation targets. Views themselves are not unit-tested. Coordinators occasionally are.

**Note on framework:** The project uses Swift Testing (`@Test`, `#expect`), not XCTest. Don't suggest `XCTAssert`-style assertions in consolidation proposals.

## Test framework idioms

- `@Test` attribute, `#expect(...)` macro, `#require(...)` for guard-style preconditions
- `async` on the test function when the SUT is async
- `@Suite` for grouping
- Fixtures as static extensions on the type under test (e.g. `extension User { static let fixture = ... }`)
- Spies named `SpyX`, stubs named `StubX`

## What to scan for

1. **Suite-level shared setup.** A `@Suite` with many `@Test` methods that each construct the same ViewModel and the same spies in the same way. Target: fold into fewer verbose tests whose names describe the user journey the test exercises.

2. **Spy-observation tests that test the spy, not the behaviour.** A `@Test` whose only `#expect` is that a spy recorded a call. If two such tests differ only in the input and assert the same spy method, parameterise with `@Test(arguments: [...])`.

3. **Published-property tests one property at a time.** ViewModel tests that each assert one property after the same trigger (e.g. one tests `viewModel.isLoading`, the next tests `viewModel.items`, the next tests `viewModel.error`). Target: one verbose test that asserts the whole observable state after the trigger.

4. **Coordinator tests over-specifying the navigation sequence.** If multiple tests each assert one push/pop in the same flow, consolidate into one test asserting the full navigation sequence.

## Mutation testing — muter

muter (https://github.com/muter-mutation-testing/muter) is the Swift mutation testing tool. It is less mature than Stryker; expect occasional flakes, especially on SPM-heavy configurations.

If not configured:

```bash
cd mobile/ios
brew install muter
muter init
```

Baseline run:

```bash
cd mobile/ios
muter run --files-to-mutate <sut-file-glob>
```

### Fallback: assertion-preservation rule

When muter cannot produce a clean run on a target file (e.g. SPM-module-boundary issues, flakes that don't reproduce, toolchain skew), **fall back to the assertion-preservation rule** and record which gate applies in the bead:

> Every distinct `#expect(...)` condition present in the suite *before* consolidation must be present in the consolidated suite afterwards. Rewording is fine; dropping a condition is not.

This is a textual backstop, not equivalent to mutation testing. The bead must state explicitly which gate (muter or assertion-preservation) applies; don't silently downgrade.

## Naming target

Bad:

```swift
@Test func test_isLoading_true_while_fetching()
@Test func test_items_empty_while_fetching()
@Test func test_error_nil_while_fetching()
```

Good:

```swift
// Consolidates three tests that each asserted one field of the loading state.
// Rationale: "loading" is one observable shape; regressions should fail with the
// full picture rather than one field at a time.
@Test
func when_fetching_starts_the_view_model_sets_isLoading_true_clears_items_and_clears_error() async {
    let spy = SpyFetchItemsUseCase(result: .pending)
    let viewModel = ItemsViewModel(fetchItems: spy)

    await viewModel.start()

    #expect(viewModel.isLoading == true)
    #expect(viewModel.items == [])
    #expect(viewModel.error == nil)
}
```

## ViewModel floor

Every ViewModel must retain at least one `@Test` whose name begins with `when_<verb>ing_` and covers the happy path. Grep-discoverability matters here too.
