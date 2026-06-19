# .NET test consolidation (TUnit + Stryker.NET)

> **Dormant (2026-06-19 onward):** the repository no longer contains any .NET code. The API and worker were migrated to Go ([ADR 0028](../../../../docs/adr/0028-migrate-backend-from-dotnet-to-go.md)), the Pulumi infra was ported to Go ([ADR 0029](../../../../docs/adr/0029-migrate-infrastructure-from-dotnet-to-go.md)), and the `tc` CLI was rebuilt in Go ([ADR 0030](../../../../docs/adr/0030-migrate-admin-cli-from-dotnet-to-go.md)). There is **no .NET test suite left to consolidate.** This reference is kept only as a record of the TUnit idioms used while .NET was in the tree; resurrect it if a .NET component is ever reintroduced.

## Where tests lived

- `cli/tests/tc.tests/` — the self-contained .NET CLI's tests (TUnit); removed when the CLI moved to Go (`cli/internal/tc/*_test.go`, `go test`).

The TUnit idioms, mutation-testing gate, and naming conventions below applied to that work.

## Test framework idioms

- `[Test]` attribute, async `Task` methods
- `await Assert.That(actual).IsEqualTo(expected);` / `.HasCount().EqualTo(n);`
- `[Arguments(...)]` on a parameterised method for data-driven tests
- Fakes are hand-written (e.g. `FakeSavedApplicationRepository`), data uses the builder pattern

## What to scan for

1. **Shared-setup merge candidates.** Multiple `[Test]` methods in one class that construct the same handler + fakes and differ only in the Arrange data or the Act input. Signal: within a single test class, `new <Handler>(` or `new <Repo>(` appears >3 times near-identically. Target: fold into one verbose `[Test]` whose name describes the behaviour being exercised.

2. **Parameterisation candidates.** Tests whose bodies differ only in scalar inputs or one enum. Signal: tests like `Should_X_When_A`, `Should_X_When_B`, `Should_X_When_C` in the same class with the same Arrange/Act skeleton. Target: one `[Test]` with `[Arguments(...)]` per case.

3. **Tiny single-assertion tests.** Tests whose Act is the same as the neighbouring test but assert only one field of the result. Signal: classes with >3 tests averaging <2 asserts each. Target: one verbose test with all the asserts for that Act.

4. **Over-asserted command tests.** A command test asserting many incidental side effects that a single observable outcome already covers. Signal: tests pinning internal state no caller of the CLI command would observe. Target: trim the redundant assertion, confirm the behaviour is still covered by a focused test, add one if missing.

## Mutation testing — Stryker.NET

If not installed:

```bash
dotnet tool install -g dotnet-stryker
```

Baseline run for a consolidation candidate (run from `cli/`):

```bash
dotnet stryker \
  --project tests/<test-project>/<test-project>.csproj \
  --mutate "<sut-glob>"
```

Workflow in the bead:

1. Record baseline mutation score for the named SUT files.
2. Perform the consolidation.
3. Re-run. The new score must be **≥** baseline. **A drop is a failure, not a negotiation** — revert and report.

## Naming target

> The example below is illustrative of the consolidation *pattern* — the `SaveApplication*` types it names belonged to the now-deleted .NET API and no longer exist. Apply the same shape to the CLI's TUnit tests.

Bad (over-granular):

```csharp
[Test] public async Task Should_SaveApplication_When_NotAlreadySaved()
[Test] public async Task Should_SetSavedAt_When_NotAlreadySaved()
[Test] public async Task Should_UseProvidedUserId_When_NotAlreadySaved()
```

Good (consolidated):

```csharp
// Consolidates three behaviours that are really one observable outcome of the
// first-save path: aggregate creation, SavedAt stamping, UserId scoping.
// Regressions in any one of them should fail with the full picture.
[Test]
public async Task When_saving_a_new_application_it_persists_exactly_one_aggregate_stamped_with_clock_time_and_scoped_to_user()
{
    // Arrange
    var repo = new FakeSavedApplicationRepository();
    var clock = new FakeTimeProvider(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
    var handler = new SaveApplicationCommandHandler(repo, clock);
    var command = new SaveApplicationCommand("auth0|user-1", "planit-uid-abc");

    // Act
    await handler.HandleAsync(command, CancellationToken.None);

    // Assert — every observable facet of the first-save outcome
    var saved = await repo.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
    await Assert.That(saved).HasCount().EqualTo(1);
    await Assert.That(saved[0].ApplicationUid).IsEqualTo("planit-uid-abc");
    await Assert.That(saved[0].SavedAt).IsEqualTo(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
}
```

## Handler floor

Every handler must retain at least one `[Test]` whose name begins with `When_<verb>ing_<noun>_` and covers the happy path, even if a parameterised test would subsume it. This keeps the handler's existence discoverable by grep on `When_<verb>ing`.
