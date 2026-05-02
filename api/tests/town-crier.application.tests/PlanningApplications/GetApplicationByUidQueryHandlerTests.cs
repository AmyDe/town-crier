using TownCrier.Application.PlanningApplications;
using TownCrier.Application.SavedApplications;
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

        var planItClient = new FakePlanItClient();
        var savedRepository = new FakeSavedApplicationRepository();
        var handler = new GetApplicationByUidQueryHandler(repository, planItClient, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("planit-uid-001", UserId: null), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Uid).IsEqualTo("planit-uid-001");
        await Assert.That(result.Name).IsEqualTo("APP/2024/001");
        await Assert.That(result.AreaId).IsEqualTo(42);
        await Assert.That(result.AreaName).IsEqualTo("Camden");

        // Cosmos hit means PlanIt must NOT be called.
        await Assert.That(planItClient.GetByUidCalls).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_FetchFromPlanItAndUpsert_When_CosmosMisses()
    {
        // Arrange. Cosmos has never seen this uid because search no longer
        // upserts. See bead tc-if12. Handler must call PlanIt, upsert the
        // result, and return it, so search-then-tap-then-details still works
        // for never-polled uids.
        var repository = new FakePlanningApplicationRepository();
        var planItApp = new PlanningApplicationBuilder()
            .WithUid("planit-uid-002")
            .WithName("Camden/CAM/24/0042/FUL")
            .WithAreaId(42)
            .WithAreaName("Camden")
            .Build();
        var planItClient = new FakePlanItClient();
        planItClient.AddByUid(planItApp);
        var savedRepository = new FakeSavedApplicationRepository();

        var handler = new GetApplicationByUidQueryHandler(repository, planItClient, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("planit-uid-002", UserId: null), CancellationToken.None);

        // Assert — result is the PlanIt-fetched application
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Uid).IsEqualTo("planit-uid-002");
        await Assert.That(result.Name).IsEqualTo("Camden/CAM/24/0042/FUL");

        // PlanIt was called exactly once with the uid
        await Assert.That(planItClient.GetByUidCalls).HasCount().EqualTo(1);
        await Assert.That(planItClient.GetByUidCalls[0]).IsEqualTo("planit-uid-002");

        // Application was upserted into Cosmos for future cache hits
        await Assert.That(repository.UpsertCallCount).IsEqualTo(1);
        var stored = await repository.GetByUidAsync("planit-uid-002", CancellationToken.None);
        await Assert.That(stored).IsNotNull();
        await Assert.That(stored!.Name).IsEqualTo("Camden/CAM/24/0042/FUL");
    }

    [Test]
    public async Task Should_ReturnNull_When_BothCosmosAndPlanItMiss()
    {
        // Arrange — uid is unknown to both Cosmos and PlanIt (404). Handler
        // returns null so the endpoint can respond 404 to the client.
        var repository = new FakePlanningApplicationRepository();
        var planItClient = new FakePlanItClient();
        var savedRepository = new FakeSavedApplicationRepository();
        var handler = new GetApplicationByUidQueryHandler(repository, planItClient, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("nonexistent-uid", UserId: null), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
        await Assert.That(planItClient.GetByUidCalls).HasCount().EqualTo(1);
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

        var planItClient = new FakePlanItClient();
        var handler = new GetApplicationByUidQueryHandler(repository, planItClient, savedRepository);

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
        var planItClient = new FakePlanItClient();
        var handler = new GetApplicationByUidQueryHandler(repository, planItClient, savedRepository);

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

        var planItClient = new FakePlanItClient();
        var handler = new GetApplicationByUidQueryHandler(repository, planItClient, savedRepository);

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
        var planItClient = new FakePlanItClient();
        var handler = new GetApplicationByUidQueryHandler(repository, planItClient, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("planit-uid-001", "auth0|user-1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.AppState).IsEqualTo("Permitted");
    }
}
