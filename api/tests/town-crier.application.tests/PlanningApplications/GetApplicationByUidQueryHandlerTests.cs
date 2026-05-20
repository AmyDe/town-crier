using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.SavedApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.Tests.PlanningApplications;

public sealed class GetApplicationByUidQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnApplication_When_FoundByUid()
    {
        // Arrange
        var application = new PlanningApplicationBuilder()
            .WithUid("planit-uid-001")
            .WithName("APP/2024/001")
            .WithAreaId(42)
            .WithAreaName("Camden")
            .Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(application, CancellationToken.None);

        var savedRepository = new FakeSavedApplicationRepository();
        var handler = new GetApplicationByUidQueryHandler(repository, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("planit-uid-001", UserId: null), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Uid).IsEqualTo("planit-uid-001");
        await Assert.That(result.Name).IsEqualTo("APP/2024/001");
        await Assert.That(result.AreaId).IsEqualTo(42);
        await Assert.That(result.AreaName).IsEqualTo("Camden");
    }

    [Test]
    public async Task Should_ReturnNull_When_CosmosReturnsNoMatch()
    {
        // Arrange — uid unknown to Cosmos; handler returns null (404 to caller).
        // No PlanIt fallback — user paths never call PlanIt (GH#395 Invariant 1).
        var repository = new FakePlanningApplicationRepository();
        var savedRepository = new FakeSavedApplicationRepository();
        var handler = new GetApplicationByUidQueryHandler(repository, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("nonexistent-uid", UserId: null), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
        await Assert.That(repository.UpsertCallCount).IsEqualTo(0);
    }

    [Test]
    public async Task Should_RefreshSavedSnapshot_When_UserHasApplicationSaved()
    {
        // Arrange — opening a saved application silently upserts the latest
        // snapshot back into the saved row so the saved-list self-heals over
        // time on the items the user actually engages with. See bd tc-udby.
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);
        var stale = new PlanningApplicationBuilder()
            .WithUid("planit-uid-001").WithName("APP/2024/001").WithAppState("Undecided").Build();
        var fresh = new PlanningApplicationBuilder()
            .WithUid("planit-uid-001").WithName("APP/2024/001").WithAppState("Permitted").Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(fresh, CancellationToken.None);

        var savedRepository = new FakeSavedApplicationRepository();
        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", stale, savedAt), CancellationToken.None);

        var handler = new GetApplicationByUidQueryHandler(repository, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("planit-uid-001", "auth0|user-1"), CancellationToken.None);

        // Assert — caller sees the fresh snapshot
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.AppState).IsEqualTo("Permitted");

        // Assert — the saved row was updated with the fresh snapshot
        var rows = await savedRepository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(rows).HasCount().EqualTo(1);
        await Assert.That(rows[0].Application).IsNotNull();
        await Assert.That(rows[0].Application!.AppState).IsEqualTo("Permitted");
        await Assert.That(rows[0].SavedAt).IsEqualTo(savedAt); // savedAt preserved
    }

    [Test]
    public async Task Should_RefreshSavedSnapshot_When_MasterUidFormatDiffersFromCanonicalKey()
    {
        // Arrange — the regression PR #398 introduced: the master record's raw Uid
        // field is in a stale format, but the saved row is keyed on the canonical
        // {areaId}/{name} uid. Refresh-on-tap must align on the canonical key, not
        // the raw Uid, or snapshot healing is a silent no-op (bd tc-o88i).
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);
        var stale = new PlanningApplicationBuilder()
            .WithUid("legacy-uid-format").WithName("APP/2024/001").WithAreaId(42)
            .WithAppState("Undecided").Build();
        var fresh = new PlanningApplicationBuilder()
            .WithUid("legacy-uid-format").WithName("APP/2024/001").WithAreaId(42)
            .WithAppState("Permitted").Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(fresh, CancellationToken.None);

        var savedRepository = new FakeSavedApplicationRepository();
        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", stale, savedAt), CancellationToken.None);

        var handler = new GetApplicationByUidQueryHandler(repository, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("legacy-uid-format", "auth0|user-1"), CancellationToken.None);

        // Assert — the saved row's snapshot was healed despite the raw-uid mismatch.
        await Assert.That(result).IsNotNull();
        var rows = await savedRepository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(rows).HasCount().EqualTo(1);
        await Assert.That(rows[0].Application!.AppState).IsEqualTo("Permitted");
        await Assert.That(rows[0].SavedAt).IsEqualTo(savedAt);
    }

    [Test]
    public async Task Should_NotRefreshSavedSnapshot_When_UserHasNotSavedApplication()
    {
        // Arrange — refresh-on-tap must only touch the saved row of the
        // requesting user. A user opening an unsaved item must not write
        // anything to their saved-applications container.
        var fresh = new PlanningApplicationBuilder()
            .WithUid("planit-uid-001").WithAppState("Permitted").Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(fresh, CancellationToken.None);

        var savedRepository = new FakeSavedApplicationRepository();
        var handler = new GetApplicationByUidQueryHandler(repository, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("planit-uid-001", "auth0|user-1"), CancellationToken.None);

        // Assert — caller sees the application
        await Assert.That(result).IsNotNull();

        // Assert — saved repo unaffected
        var rows = await savedRepository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(rows).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotRefreshSavedSnapshot_When_RequestIsAnonymous()
    {
        // Arrange — public/unauthenticated reads (UserId == null) must skip the
        // refresh step entirely. The detail endpoint requires auth today; this
        // guards against handler misuse if that ever changes.
        var fresh = new PlanningApplicationBuilder()
            .WithUid("planit-uid-001").WithAppState("Permitted").Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(fresh, CancellationToken.None);

        var savedRepository = new FakeSavedApplicationRepository();

        // Pre-seed a save under a different user — handler must not touch this.
        var prior = new PlanningApplicationBuilder()
            .WithUid("planit-uid-001").WithAppState("Undecided").Build();
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);
        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|other-user", prior, savedAt), CancellationToken.None);

        var handler = new GetApplicationByUidQueryHandler(repository, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("planit-uid-001", UserId: null), CancellationToken.None);

        // Assert — other user's saved row is unchanged
        await Assert.That(result).IsNotNull();
        var rows = await savedRepository.GetByUserIdAsync("auth0|other-user", CancellationToken.None);
        await Assert.That(rows).HasCount().EqualTo(1);
        await Assert.That(rows[0].Application).IsNotNull();
        await Assert.That(rows[0].Application!.AppState).IsEqualTo("Undecided");
    }

    [Test]
    public async Task Should_StillReturnApplication_When_RefreshUpsertFails()
    {
        // Arrange — refresh-on-tap is a side effect; failure to write the
        // refreshed snapshot must NOT fail the read. The user still gets the
        // application back. See bd tc-udby.
        var fresh = new PlanningApplicationBuilder()
            .WithUid("planit-uid-001").WithAppState("Permitted").Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(fresh, CancellationToken.None);

        var savedRepository = new ThrowingOnSaveSavedApplicationRepository(
            existsForUser: ("auth0|user-1", "planit-uid-001"));
        var handler = new GetApplicationByUidQueryHandler(repository, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("planit-uid-001", "auth0|user-1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.AppState).IsEqualTo("Permitted");
    }
}
