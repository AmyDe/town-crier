using TownCrier.Application.PlanningApplications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.Tests.SavedApplications;

public sealed class GetSavedApplicationsQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnEmptyList_When_UserHasNoSavedApplications()
    {
        // Arrange
        var savedRepository = new FakeSavedApplicationRepository();
        var applicationRepository = new FakePlanningApplicationRepository();
        var handler = new GetSavedApplicationsQueryHandler(savedRepository, applicationRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ProjectFromEmbeddedSnapshot_WithoutTouchingPlanningRepository_When_AllSavesCarrySnapshot()
    {
        // Arrange — the saved-list endpoint must render with one partitioned
        // query and zero hydration calls on the happy path. A spy fake fails the
        // test if the handler reaches into the planning repository at all.
        // See bd tc-udby for the 429 storm this design eliminates.
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FailIfCalledPlanningApplicationRepository();
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);

        var app1 = new PlanningApplicationBuilder()
            .WithUid("planit-uid-abc").WithName("APP/2026/001").WithAreaName("Wiltshire").Build();
        var app2 = new PlanningApplicationBuilder()
            .WithUid("planit-uid-def").WithName("APP/2026/002").WithAreaName("Somerset").Build();

        await savedRepository.SaveAsync(SavedApplication.Create("auth0|user-1", app1, savedAt), CancellationToken.None);
        await savedRepository.SaveAsync(SavedApplication.Create("auth0|user-1", app2, savedAt.AddHours(1)), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, planningRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert — both items projected from the embedded snapshot
        await Assert.That(result).HasCount().EqualTo(2);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("planit-uid-abc");
        await Assert.That(result[0].Application.Name).IsEqualTo("APP/2026/001");
        await Assert.That(result[0].Application.AreaName).IsEqualTo("Wiltshire");
        await Assert.That(result[1].ApplicationUid).IsEqualTo("planit-uid-def");
        await Assert.That(result[1].Application.Name).IsEqualTo("APP/2026/002");
    }

    [Test]
    public async Task Should_OnlyReturnOwnApplications_When_MultipleUsersHaveSaved()
    {
        // Arrange
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FailIfCalledPlanningApplicationRepository();
        var savedAt = DateTimeOffset.UtcNow;

        var app1 = new PlanningApplicationBuilder().WithUid("planit-uid-abc").Build();
        var app2 = new PlanningApplicationBuilder().WithUid("planit-uid-def").Build();

        await savedRepository.SaveAsync(SavedApplication.Create("auth0|user-1", app1, savedAt), CancellationToken.None);
        await savedRepository.SaveAsync(SavedApplication.Create("auth0|user-2", app2, savedAt), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, planningRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("planit-uid-abc");
    }

    [Test]
    public async Task Should_LazilyBackfillSnapshot_When_LegacyRowHasUidOnly()
    {
        // Arrange — rows persisted before the snapshot column existed hold only
        // the uid. The handler hydrates them once via the planning repo and
        // upserts the snapshot back so subsequent reads are zero-hydration.
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FakePlanningApplicationRepository();
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);

        // Seed: planning record exists, saved row does NOT carry the snapshot.
        var app = new PlanningApplicationBuilder()
            .WithUid("planit-uid-legacy").WithName("APP/legacy").WithAreaName("Camden").Build();
        await planningRepository.UpsertAsync(app, CancellationToken.None);
        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-legacy", savedAt), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, planningRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act — first read triggers backfill.
        var firstResult = await handler.HandleAsync(query, CancellationToken.None);

        // Assert — first read returned the hydrated snapshot.
        await Assert.That(firstResult).HasCount().EqualTo(1);
        await Assert.That(firstResult[0].ApplicationUid).IsEqualTo("planit-uid-legacy");
        await Assert.That(firstResult[0].Application.Name).IsEqualTo("APP/legacy");

        // Assert — saved row was rewritten with the embedded snapshot persisted.
        var rows = await savedRepository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(rows).HasCount().EqualTo(1);
        await Assert.That(rows[0].Application).IsNotNull();
        await Assert.That(rows[0].Application!.Name).IsEqualTo("APP/legacy");

        // Act — second read with the planning repo unavailable: still works
        // because backfill self-healed and there are no more legacy rows.
        var sealedRepository = new FailIfCalledPlanningApplicationRepository();
        var sealedHandler = new GetSavedApplicationsQueryHandler(savedRepository, sealedRepository);
        var secondResult = await sealedHandler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(secondResult).HasCount().EqualTo(1);
        await Assert.That(secondResult[0].Application.Name).IsEqualTo("APP/legacy");
    }

    [Test]
    public async Task Should_ExcludeSavedApplication_When_LegacyRowAndPlanningApplicationNoLongerExists()
    {
        // Arrange — legacy uid-only row whose master planning application is
        // no longer in Cosmos. Excluded from the result rather than failing the
        // entire response.
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FakePlanningApplicationRepository();
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);

        var present = new PlanningApplicationBuilder()
            .WithUid("planit-uid-abc").WithName("APP/abc").Build();
        await planningRepository.UpsertAsync(present, CancellationToken.None);

        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", present, savedAt), CancellationToken.None);
        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-orphaned", savedAt.AddHours(1)), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, planningRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("planit-uid-abc");
    }
}
