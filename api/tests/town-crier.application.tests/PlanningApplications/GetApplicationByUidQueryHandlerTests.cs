using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Tests.Polling;

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
        var handler = new GetApplicationByUidQueryHandler(repository, planItClient);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("planit-uid-001"), CancellationToken.None);

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

        var handler = new GetApplicationByUidQueryHandler(repository, planItClient);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("planit-uid-002"), CancellationToken.None);

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
        var handler = new GetApplicationByUidQueryHandler(repository, planItClient);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("nonexistent-uid"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
        await Assert.That(planItClient.GetByUidCalls).HasCount().EqualTo(1);
        await Assert.That(repository.UpsertCallCount).IsEqualTo(0);
    }
}
